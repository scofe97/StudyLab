package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// serveLogged 는 h 를 accessLog 로 감싸 요청 하나를 보내고, 찍힌 접근 로그 한 줄을 풀어 돌려줍니다.
func serveLogged(t *testing.T, m *metrics, h http.HandlerFunc, target string) (map[string]any, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	w := httptest.NewRecorder()
	m.accessLog(zerolog.New(&buf), h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	return line, w
}

func TestAccessLog(t *testing.T) {
	t.Run("성공한 요청은 상태 코드·바이트 수·flags - 를 남긴다", func(t *testing.T) {
		// Given — 201 과 본문 5바이트를 쓰는 핸들러
		m := newMetrics()
		h := func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, "hello")
		}

		// When
		line, _ := serveLogged(t, m, h, "/v1/devices?q=sw")

		// Then — 로그 한 줄과 카운터
		require.Equal(t, "GET", line["method"])
		require.Equal(t, "/v1/devices?q=sw", line["path"])
		require.Equal(t, float64(201), line["status"])
		require.Equal(t, float64(5), line["bytes"])
		require.Equal(t, "-", line["flags"])
		require.Equal(t, false, line["reused"])
		require.Contains(t, line, "dur")
		require.Equal(t, int64(1), m.requests.Value())
		require.Equal(t, int64(0), m.inflight.Value())
	})

	t.Run("WriteHeader 없이 Write 하면 200 으로 적는다", func(t *testing.T) {
		// Given
		m := newMetrics()
		h := func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }

		// When
		line, _ := serveLogged(t, m, h, "/")

		// Then
		require.Equal(t, float64(200), line["status"])
	})

	t.Run("기록장에 적힌 원인 코드가 flags 와 reasons 카운터로 간다", func(t *testing.T) {
		// Given — 안쪽 핸들러가 기록장에 UT 를 적고 504 를 쓴다
		m := newMetrics()
		h := func(w http.ResponseWriter, r *http.Request) {
			infoFrom(r.Context()).reason = ReasonTimeout
			w.WriteHeader(http.StatusGatewayTimeout)
		}

		// When
		line, _ := serveLogged(t, m, h, "/")

		// Then
		require.Equal(t, "UT", line["flags"])
		require.Equal(t, "1", m.reasons.Get("UT").String())
	})

	t.Run("처리 중에는 inflight 가 1, 끝나면 0", func(t *testing.T) {
		// Given — 처리 도중 inflight 를 읽어 두는 핸들러
		m := newMetrics()
		var during int64
		h := func(w http.ResponseWriter, r *http.Request) { during = m.inflight.Value() }

		// When
		serveLogged(t, m, h, "/")

		// Then
		require.Equal(t, int64(1), during)
		require.Equal(t, int64(0), m.inflight.Value())
	})

	t.Run("래퍼를 거쳐도 Flush 가 안쪽 writer 에 닿는다", func(t *testing.T) {
		// Given — ReverseProxy 처럼 ResponseController 로 Flush 하는 핸들러
		m := newMetrics()
		var flushErr error
		h := func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "chunk")
			flushErr = http.NewResponseController(w).Flush()
		}

		// When
		_, w := serveLogged(t, m, h, "/")

		// Then — Unwrap 이 없으면 http.ErrNotSupported 가 난다
		require.NoError(t, flushErr)
		require.True(t, w.Flushed)
	})
}
