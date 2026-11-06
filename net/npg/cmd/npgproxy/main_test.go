package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	upstreampkg "github.com/scofe97/runners-lab/net/npg/internal/upstream"
)

func TestNewFrontServer(t *testing.T) {
	// Given / When
	s := newFrontServer("127.0.0.1:0", http.NotFoundHandler())

	// Then — 전체 시계 둘은 끄고 proxy 의 틈 시계에 맡긴다 (ADR 0014 결정 1·2).
	// WriteTimeout 이 업스트림 기한과 겹치면 504 를 손님에게 못 쓴다(J2).
	require.Zero(t, s.ReadTimeout)
	require.Zero(t, s.WriteTimeout)
	require.Equal(t, 2*time.Second, s.ReadHeaderTimeout)
	require.Equal(t, 60*time.Second, s.IdleTimeout)
}

func TestShutdownTimeoutOrder(t *testing.T) {
	// Then — 드레인은 요청 하나가 가장 오래 걸릴 수 있는 시간(업스트림 기한)보다 길고,
	// 쿠버네티스가 강제로 죽이는 30초보다 짧아야 한다 (ADR 0014 결정 3, J4).
	require.Greater(t, shutdownTimeout, upstreampkg.RequestTimeout)
	require.Less(t, shutdownTimeout, 30*time.Second)
}

func TestShutdown(t *testing.T) {
	// startBusy 는 요청 하나를 hold 동안 붙잡는 서버를 띄우고, 그 요청이 핸들러에 들어온 뒤 돌려준다.
	startBusy := func(t *testing.T, hold time.Duration) *http.Server {
		t.Helper()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		entered := make(chan struct{})
		s := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			time.Sleep(hold)
		})}
		go func() { _ = s.Serve(ln) }()
		go func() {
			resp, err := http.Get("http://" + ln.Addr().String())
			if err == nil {
				resp.Body.Close()
			}
		}()
		<-entered
		t.Cleanup(func() { _ = s.Close() })
		return s
	}

	t.Run("처리 중인 요청이 기한 안에 끝나면 nil", func(t *testing.T) {
		// Given
		s := startBusy(t, 100*time.Millisecond)

		// When
		err := shutdown([]*http.Server{s}, 2*time.Second)

		// Then
		require.NoError(t, err)
	})

	t.Run("기한 안에 못 끝나면 오류를 돌려준다, 버리면 exit 0 으로 끝난다 (J4)", func(t *testing.T) {
		// Given
		s := startBusy(t, time.Second)

		// When
		err := shutdown([]*http.Server{s}, 100*time.Millisecond)

		// Then
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

// serveFront 는 withHealthz 로 감싼 서버를 임시 포트에 띄운다. 일반 경로는 "hello" 로 답한다.
func serveFront(t *testing.T, draining *atomic.Bool) (srv *http.Server, base string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hello") })
	srv = &http.Server{Handler: withHealthz(next, draining)}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv, "http://" + ln.Addr().String()
}

// get 은 연결을 재사용하지 않고 한 번 요청해 상태 코드와 본문을 돌려준다.
func get(t *testing.T, url string) (int, string) {
	t.Helper()
	c := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := c.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestHealthz(t *testing.T) {
	// Given — 드레인이 시작되지 않은 서버
	var draining atomic.Bool
	_, base := serveFront(t, &draining)

	// When
	code, body := get(t, base+"/healthz")
	other, otherBody := get(t, base+"/v1/devices/7")

	// Then — /healthz 는 200 ok, 그 밖의 경로는 안쪽 핸들러가 답한다
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "ok\n", body)
	require.Equal(t, http.StatusOK, other)
	require.Equal(t, "hello", otherBody)
}

func TestDrainKeepsServingWhileHealthzTurns503(t *testing.T) {
	// Given — 드레인 지연 400ms 로 종료 절차를 시작한 서버
	var draining atomic.Bool
	srv, base := serveFront(t, &draining)
	done := make(chan error, 1)
	go func() { done <- drainThenShutdown([]*http.Server{srv}, &draining, 400*time.Millisecond, time.Second) }()

	// When — 드레인이 시작된 직후 두 가지를 부른다
	require.Eventually(t, draining.Load, time.Second, time.Millisecond)
	hCode, hBody := get(t, base+"/healthz")
	oCode, oBody := get(t, base+"/v1/devices/7")

	// Then — /healthz 만 503 draining, 일반 요청은 여전히 처리된다. 지연이 지나면 닫히고 종료 오류는 없다
	require.Equal(t, http.StatusServiceUnavailable, hCode)
	require.Equal(t, "draining\n", hBody)
	require.Equal(t, http.StatusOK, oCode)
	require.Equal(t, "hello", oBody)
	require.NoError(t, <-done)
	c := &http.Client{Timeout: 200 * time.Millisecond}
	_, err := c.Get(base + "/healthz")
	require.Error(t, err)
}

func TestDrainDelayZeroClosesAtOnce(t *testing.T) {
	// Given — 드레인 지연 0 (옛 동작)
	var draining atomic.Bool
	srv, base := serveFront(t, &draining)

	// When — 종료 절차를 부른다
	start := time.Now()
	err := drainThenShutdown([]*http.Server{srv}, &draining, 0, time.Second)

	// Then — 기다림 없이 바로 닫힌다
	require.NoError(t, err)
	require.Less(t, time.Since(start), 200*time.Millisecond)
	c := &http.Client{Timeout: 200 * time.Millisecond}
	_, gerr := c.Get(base + "/v1/devices/7")
	require.Error(t, gerr)
}
