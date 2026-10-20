package ctrl

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// startAgent 는 RunAgent 를 고루틴으로 띄우고, 끝났을 때 결과를 받을 채널을 돌려준다.
func startAgent(ctx context.Context, addr string) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- RunAgent(ctx, addr, 10*time.Millisecond, zerolog.Nop())
	}()
	return done
}

func TestRunAgentSendsHeartbeats(t *testing.T) {
	// Given — 듣고 있는 서버와 agent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := startAgent(ctx, ln.Addr().String())

	// When — 서버가 연결을 받아 한 줄을 읽는다
	conn, err := ln.Accept()
	require.NoError(t, err)
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')

	// Then — ping 을 받았고, 취소하면 nil 로 끝난다
	require.NoError(t, err)
	require.Equal(t, "ping\n", line)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("취소했는데 RunAgent 가 끝나지 않았다")
	}
}

func TestRunAgentReconnectsAfterServerDrops(t *testing.T) {
	// Given — 듣고 있는 서버와 agent, 그리고 첫 연결
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAgent(ctx, ln.Addr().String())

	first, err := ln.Accept()
	require.NoError(t, err)

	// When — 서버가 첫 연결을 끊는다
	first.Close()

	// Then — 백오프(약 1초) 뒤에 agent 가 다시 붙는다
	ln.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
	second, err := ln.Accept()
	require.NoError(t, err, "재접속이 5초 안에 오지 않았다")
	second.Close()
}
