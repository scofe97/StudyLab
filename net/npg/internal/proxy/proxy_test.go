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
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// syncBuffer 는 여러 고루틴이 동시에 로그를 써도 안전한 버퍼입니다.
// 손님이 끊는 경우 접근 로그는 테스트 고루틴이 아니라 서버 고루틴에서 찍히기 때문입니다.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// accessLine 은 지금까지 찍힌 로그에서 접근 로그 한 줄(method 키가 있는 줄)을 찾아 돌려줍니다. 없으면 nil.
func (s *syncBuffer) accessLine(t *testing.T) map[string]any {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := bufio.NewScanner(bytes.NewReader(s.buf.Bytes()))
	for sc.Scan() {
		var line map[string]any
		require.NoError(t, json.Unmarshal(sc.Bytes(), &line))
		if _, ok := line["method"]; ok {
			return line
		}
	}
	return nil
}

// startProxy 는 upstream 을 가리키는 프록시를 진짜 TCP 서버로 띄웁니다.
// httptest.NewRecorder 대신 서버를 쓰는 까닭은 RemoteAddr·hop-by-hop·연결 끊김이 실제 연결에서만 생기기 때문입니다.
func startProxy(t *testing.T, upstream string) (*httptest.Server, *syncBuffer, expvar.Var) {
	t.Helper()
	u, err := url.Parse(upstream)
	require.NoError(t, err)
	logs := &syncBuffer{}
	h, vars := New(u, zerolog.New(logs))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, logs, vars
}

func TestProxy(t *testing.T) {
	t.Run("손님이 보낸 X-Forwarded-For 는 버리고 진짜 IP 를 적는다, Host 는 업스트림 것", func(t *testing.T) {
		// Given — 받은 헤더를 그대로 돌려보내는 업스트림
		var got http.Header
		var gotHost string
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, gotHost = r.Header.Clone(), r.Host
		}))
		defer backend.Close()
		front, _, _ := startProxy(t, backend.URL)

		// When — 손님이 XFF 를 위조해 보낸다
		req, err := http.NewRequest(http.MethodGet, front.URL+"/v1/devices", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-For", "6.6.6.6")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Then — 위조 값은 사라지고 프록시가 본 손님 IP 만 남는다
		require.Equal(t, []string{"127.0.0.1"}, got.Values("X-Forwarded-For"))
		require.Equal(t, hostOf(backend.URL), gotHost)
	})

	t.Run("Connection 에 적힌 hop-by-hop 헤더는 업스트림에 가지 않는다", func(t *testing.T) {
		// Given
		var got http.Header
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
		}))
		defer backend.Close()
		front, _, _ := startProxy(t, backend.URL)

		// When — 손님이 X-Hop 을 이번 구간 전용이라고 선언한다
		req, err := http.NewRequest(http.MethodGet, front.URL+"/", nil)
		require.NoError(t, err)
		req.Header.Set("Connection", "X-Hop")
		req.Header.Set("X-Hop", "1")
		req.Header.Set("X-Keep", "1")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		// Then — X-Hop 은 지워지고 평범한 헤더는 남는다
		require.Empty(t, got.Get("X-Hop"))
		require.Equal(t, "1", got.Get("X-Keep"))
	})

	t.Run("성공하면 업스트림 상태 그대로, flags - 와 카운터가 남는다", func(t *testing.T) {
		// Given
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			io.WriteString(w, "tea")
		}))
		defer backend.Close()
		front, logs, vars := startProxy(t, backend.URL)

		// When
		resp, err := http.Get(front.URL + "/")
		require.NoError(t, err)
		resp.Body.Close()

		// Then
		require.Equal(t, http.StatusTeapot, resp.StatusCode)
		line := logs.accessLine(t)
		require.Equal(t, float64(http.StatusTeapot), line["status"])
		require.Equal(t, "-", line["flags"])
		require.Equal(t, float64(3), line["bytes"])
		require.JSONEq(t, `{"requests":1,"inflight":0,"conns_new":1,"conns_reused":0,"retries":0,"reasons":{}}`, vars.String())
	})

	t.Run("업스트림 포트가 닫혀 있으면 502 와 UF", func(t *testing.T) {
		// Given — 아무도 듣지 않는 주소
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().String()
		ln.Close()
		front, logs, vars := startProxy(t, "http://"+addr)

		// When
		resp, err := http.Get(front.URL + "/")
		require.NoError(t, err)
		resp.Body.Close()

		// Then — 손님은 502, 로그와 카운터는 원인 UF
		require.Equal(t, http.StatusBadGateway, resp.StatusCode)
		line := logs.accessLine(t)
		require.Equal(t, "UF", line["flags"])
		require.Equal(t, float64(http.StatusBadGateway), line["status"])
		require.Contains(t, vars.String(), `"reasons": {"UF": 1}`)
	})

	t.Run("손님이 기다리다 끊으면 DC 와 499 로 남는다", func(t *testing.T) {
		// Given — 요청을 받았다고 알린 뒤 손님이 떠날 때까지 붙잡는 업스트림
		arrived := make(chan struct{})
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(arrived)
			<-r.Context().Done()
		}))
		defer backend.Close()
		front, logs, vars := startProxy(t, backend.URL)

		// When — 업스트림에 닿은 걸 확인하고 손님이 요청을 취소한다
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, front.URL+"/", nil)
		require.NoError(t, err)
		go func() { <-arrived; cancel() }()
		_, err = http.DefaultClient.Do(req)
		require.ErrorIs(t, err, context.Canceled)

		// Then — 접근 로그는 서버 쪽에서 조금 뒤에 찍힌다
		var line map[string]any
		require.Eventually(t, func() bool { line = logs.accessLine(t); return line != nil }, 2*time.Second, 10*time.Millisecond)
		require.Equal(t, "DC", line["flags"])
		require.Equal(t, float64(499), line["status"])
		require.Contains(t, vars.String(), `"reasons": {"DC": 1}`)
	})

	t.Run("본문을 보내다 업스트림이 끊으면 로그는 남고 UC", func(t *testing.T) {
		// Given — 100바이트를 약속하고 10바이트만 보낸 뒤 TCP 를 끊는 업스트림
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "0123456789")
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
		}))
		defer backend.Close()
		front, logs, vars := startProxy(t, backend.URL)

		// When — 손님은 끝까지 읽으려 한다
		// 프록시가 쓴 헤더·10바이트가 아직 서버 버퍼에 있을 때 끊기면 손님은 헤더조차 못 받고 EOF 를 본다.
		// 어느 쪽이든 손님에게는 실패라서 손님 쪽 결과는 따지지 않는다.
		if resp, err := http.Get(front.URL + "/"); err == nil {
			io.ReadAll(resp.Body)
			resp.Body.Close()
		}

		// Then — 프록시는 WriteHeader(200) 를 이미 불렀으므로 로그엔 200 과 함께 UC 가 남는다
		var line map[string]any
		require.Eventually(t, func() bool { line = logs.accessLine(t); return line != nil }, 2*time.Second, 10*time.Millisecond)
		require.Equal(t, "UC", line["flags"])
		require.Equal(t, float64(http.StatusOK), line["status"])
		require.Contains(t, vars.String(), `"reasons": {"UC": 1}`)
	})

	t.Run("길이를 밝히지 않은 본문이 도중에 끊기면 손님도 실패를 본다", func(t *testing.T) {
		// Given — Content-Length 없이(chunked) 한 조각 보낸 뒤 TCP 를 끊는 업스트림
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "0123456789")
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
		}))
		defer backend.Close()
		front, _, _ := startProxy(t, backend.URL)

		// When
		var err error
		resp, err := http.Get(front.URL + "/")
		if err == nil {
			_, err = io.ReadAll(resp.Body)
			resp.Body.Close()
		}

		// Then — 프록시가 panic 을 삼키면 서버가 마지막 조각을 붙여 응답을 "정상 종료"하고,
		// 손님은 잘린 10바이트를 온전한 본문으로 믿게 된다. 다시 던져야 연결이 끊겨 에러가 된다.
		require.Error(t, err)
	})

	t.Run("본문을 받던 손님이 떠나면 로그는 남고 DC", func(t *testing.T) {
		// Given — 첫 조각을 보낸 뒤 손님이 떠날 때까지 붙잡는 업스트림
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "first")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer backend.Close()
		front, logs, vars := startProxy(t, backend.URL)

		// When — 손님이 첫 조각을 받고 취소한다
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, front.URL+"/", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		buf := make([]byte, 5)
		_, err = io.ReadFull(resp.Body, buf)
		require.NoError(t, err)
		cancel()
		resp.Body.Close()

		// Then
		var line map[string]any
		require.Eventually(t, func() bool { line = logs.accessLine(t); return line != nil }, 2*time.Second, 10*time.Millisecond)
		require.Equal(t, "DC", line["flags"])
		require.Contains(t, vars.String(), `"reasons": {"DC": 1}`)
	})
}

// hostOf 는 "http://127.0.0.1:1234" 에서 Host 부분만 떼어 냅니다.
func hostOf(raw string) string {
	u, _ := url.Parse(raw)
	return u.Host
}
