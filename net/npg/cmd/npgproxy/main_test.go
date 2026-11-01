package main

import (
	"context"
	"net"
	"net/http"
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
