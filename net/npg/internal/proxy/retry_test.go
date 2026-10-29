package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"expvar"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// accessLines 는 지금까지 찍힌 접근 로그(method 키가 있는 줄)를 모두 순서대로 돌려준다.
func (s *syncBuffer) accessLines(t *testing.T) []map[string]any {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(s.buf.Bytes()))
	for sc.Scan() {
		var line map[string]any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
		if _, ok := line["method"]; ok {
			lines = append(lines, line)
		}
	}
	return lines
}

type connKey struct{}

// newSecondKiller 는 연결마다 첫 요청에는 답하고, 같은 연결의 둘째 요청은 답 없이 끊는 업스트림이다.
// I2 의 "쉬던 연결을 백엔드가 먼저 닫았는데 프록시가 그 연결에 요청을 실은" 경합을 매번 일으킨다.
func newSecondKiller(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	seen := map[net.Conn]int{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.Context().Value(connKey{}).(net.Conn)
		mu.Lock()
		seen[c]++
		n := seen[c]
		mu.Unlock()
		io.Copy(io.Discard, r.Body)
		if n == 2 {
			hc, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				hc.Close()
			}
			return
		}
		io.WriteString(w, "ok")
	}))
	srv.Config.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, connKey{}, c)
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// retries 는 /debug/vars 의 npgproxy.retries 값을 읽는다.
func retries(t *testing.T, vars expvar.Var) float64 {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(vars.String()), &m))
	v, ok := m["retries"].(float64)
	require.True(t, ok, "retries 카운터가 없다: %s", vars.String())
	return v
}

func TestTransportRetry(t *testing.T) {
	t.Run("쉬던 연결이 끊긴 GET 은 Transport 가 새 연결로 다시 보내 200, attempts 2·retries 1 (ADR 0013 결정 4)", func(t *testing.T) {
		// Given: 연결 하나를 풀에 남겨 둔다
		backend := newSecondKiller(t)
		front, logs, vars := startProxy(t, backend.URL)
		resp, err := http.Get(front.URL + "/v1/devices/1")
		require.NoError(t, err)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// When: 같은 연결로 나간 둘째 GET 을 백엔드가 끊는다
		resp, err = http.Get(front.URL + "/v1/devices/1")
		require.NoError(t, err)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Then: 손님은 실패를 못 보고, 접근 로그와 카운터에는 다시 보낸 흔적이 남는다
		require.Equal(t, http.StatusOK, resp.StatusCode)
		lines := logs.accessLines(t)
		require.Len(t, lines, 2)
		require.Equal(t, float64(1), lines[0]["attempts"])
		require.Equal(t, float64(2), lines[1]["attempts"])
		require.Equal(t, false, lines[1]["reused"]) // 마지막 시도는 새 연결
		require.Equal(t, float64(1), retries(t, vars))
	})

	t.Run("본문 있는 POST 는 다시 보내지 않아 502 UC, attempts 1·retries 0 (ADR 0013 결정 2)", func(t *testing.T) {
		// Given
		backend := newSecondKiller(t)
		front, logs, vars := startProxy(t, backend.URL)
		resp, err := http.Get(front.URL + "/v1/devices/1")
		require.NoError(t, err)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// When: 키가 붙어 있어도 프록시를 지나는 본문은 되감을 수 없다
		req, err := http.NewRequest(http.MethodPost, front.URL+"/v1/devices", strings.NewReader(`{"serial":"A1"}`))
		require.NoError(t, err)
		req.Header.Set("Idempotency-Key", "k-1")
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Then
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		lines := logs.accessLines(t)
		require.Len(t, lines, 2)
		require.Equal(t, float64(1), lines[0]["attempts"])
		require.Equal(t, "UC", lines[1]["flags"])
		require.Equal(t, float64(1), lines[1]["attempts"])
		require.Equal(t, float64(0), retries(t, vars))
	})
}
