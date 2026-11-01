package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// 시험용 손님 쪽 기한. 실제 값(10초)을 기다리지 않도록 짧게 둔다.
var testLimits = clientLimits{readGap: 300 * time.Millisecond, writeGap: 300 * time.Millisecond, minRate: 1000}

// startLimitedProxy 는 짧은 손님 쪽 기한을 단 프록시를 진짜 TCP 서버로 띄운다.
func startLimitedProxy(t *testing.T, upstream string, lim clientLimits) (*httptest.Server, *syncBuffer) {
	t.Helper()
	u, err := url.Parse(upstream)
	require.NoError(t, err)
	logs := &syncBuffer{}
	h, _ := newProxy(u, zerolog.New(logs), lim)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, logs
}

// readingBackend 는 몸통을 끝까지 읽고 201 을 돌려주는 업스트림이다.
func readingBackend(t *testing.T) *httptest.Server {
	t.Helper()
	be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(be.Close)
	return be
}

// slowPost 는 Content-Length 만큼 보내겠다고 하고 몸통을 every 마다 1바이트씩 sent 개 보낸다.
// 보낸 뒤에는 연결을 열어 둔 채 기다린다(멈춘 손님). 연결을 돌려줘서 시험이 닫는 시점을 정한다.
func slowPost(t *testing.T, addr string, length, sent int, every time.Duration) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	_, err = fmt.Fprintf(c, "POST /v1/devices HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", length)
	require.NoError(t, err)
	for range sent {
		if _, err := c.Write([]byte("x")); err != nil {
			break // 프록시가 먼저 끊었다
		}
		time.Sleep(every)
	}
	return c
}

// waitAccessLine 은 접근 로그 한 줄이 찍힐 때까지 최대 3초 기다린다.
func waitAccessLine(t *testing.T, logs *syncBuffer) map[string]any {
	t.Helper()
	var line map[string]any
	require.Eventually(t, func() bool { line = logs.accessLine(t); return line != nil }, 3*time.Second, 20*time.Millisecond)
	return line
}

func TestClientBodyDeadlines(t *testing.T) {
	t.Run("몸통을 최소 속도보다 느리게 흘리는 손님은 408 과 CT 로 남는다 (J1)", func(t *testing.T) {
		// Given — 50ms 마다 1바이트(초당 20바이트, 하한 1000 보다 느림). 틈은 300ms 보다 짧아 틈 시계로는 못 잡는다
		srv, logs := startLimitedProxy(t, readingBackend(t).URL, testLimits)

		// When
		start := time.Now()
		slowPost(t, srv.Listener.Addr().String(), 40, 40, 50*time.Millisecond)
		line := waitAccessLine(t, logs)

		// Then — 40바이트를 다 보내는 2초보다 일찍, 틈 하나(300ms) 안팎에서 끊긴다
		require.Equal(t, "CT", line["flags"])
		require.EqualValues(t, http.StatusRequestTimeout, line["status"])
		require.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("몸통을 보내다 멈춘 손님은 틈 시계에 걸려 408 과 CT 로 남는다", func(t *testing.T) {
		// Given — 최소 속도는 끄고 틈 시계만 본다. 5바이트를 보내고 멈춘다
		lim := testLimits
		lim.minRate = 0
		srv, logs := startLimitedProxy(t, readingBackend(t).URL, lim)

		// When
		slowPost(t, srv.Listener.Addr().String(), 40, 5, 10*time.Millisecond)
		line := waitAccessLine(t, logs)

		// Then
		require.Equal(t, "CT", line["flags"])
		require.EqualValues(t, http.StatusRequestTimeout, line["status"])
	})

	t.Run("몸통을 보내다 연결을 끊은 손님은 그대로 DC 로 남는다", func(t *testing.T) {
		// Given
		srv, logs := startLimitedProxy(t, readingBackend(t).URL, testLimits)

		// When — 5바이트를 보내고 기한이 오기 전에 손님이 연결을 닫는다
		c := slowPost(t, srv.Listener.Addr().String(), 40, 5, time.Millisecond)
		c.Close()
		line := waitAccessLine(t, logs)

		// Then — 기한 초과와 이탈이 구분된다
		require.Equal(t, "DC", line["flags"])
	})

	t.Run("몸통을 다 보낸 뒤 업스트림이 틈보다 오래 걸려도 이탈로 잘리지 않는다", func(t *testing.T) {
		// Given — 몸통을 다 읽은 뒤 틈(300ms)의 두 배를 쉬고 답하는 업스트림
		be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			time.Sleep(600 * time.Millisecond)
			w.WriteHeader(http.StatusCreated)
		}))
		t.Cleanup(be.Close)
		srv, logs := startLimitedProxy(t, be.URL, testLimits)

		// When
		resp, err := http.Post(srv.URL+"/v1/devices", "application/json", strings.NewReader(`{"serial":"SN-1"}`))
		require.NoError(t, err)
		resp.Body.Close()

		// Then — 읽기 기한을 지우지 않으면 Go 서버의 뒤쪽 읽기가 기한에 걸려 DC 로 끝난다
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Equal(t, "-", waitAccessLine(t, logs)["flags"])
	})
}

func TestClientWriteGap(t *testing.T) {
	t.Run("업스트림을 쓰기 틈보다 오래 기다려도 응답이 손님에게 닿는다, keep-alive 두 번째 요청까지 (J2)", func(t *testing.T) {
		// Given — 틈(300ms)의 두 배를 쉬고 답하는 업스트림. 전체 쓰기 시계였다면 이 시간에 걸린다
		be := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(600 * time.Millisecond)
			_, _ = io.WriteString(w, "ok")
		}))
		t.Cleanup(be.Close)
		srv, _ := startLimitedProxy(t, be.URL, testLimits)
		c, err := net.Dial("tcp", srv.Listener.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { c.Close() })
		br := bufio.NewReader(c)

		for i := range 2 {
			// When — 같은 연결로 두 번 보낸다. 두 번째는 첫 응답이 건 쓰기 기한이 이미 지난 뒤다
			_, err := io.WriteString(c, "GET /v1/devices/7 HTTP/1.1\r\nHost: x\r\n\r\n")
			require.NoError(t, err)
			resp, err := http.ReadResponse(br, nil)

			// Then
			require.NoError(t, err, "요청 %d", i+1)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "ok", string(body))
		}
	})
}
