package ctrl

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// startAgent 는 RunAgent 를 고루틴으로 띄우고, 끝나면 닫히는 채널을 돌려준다.
func startAgent(ctx context.Context, addr string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunAgent(ctx, addr, 10*time.Millisecond, zerolog.Nop())
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

	// When — 서버가 연결을 받아 프레임 하나를 읽는다
	conn, err := ln.Accept()
	require.NoError(t, err)
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	body, err := ReadFrame(conn)

	// Then — ping 을 받았고, 취소하면 끝난다
	require.NoError(t, err)
	require.Equal(t, "ping", string(body))

	cancel()
	select {
	case <-done:
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

func TestRunAgentReconnectsWhenServerStopsReplying(t *testing.T) {
	// Given — 연결은 받지만 pong 을 한 번도 돌려주지 않는 서버와 agent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAgent(ctx, ln.Addr().String())

	first, err := ln.Accept()
	require.NoError(t, err)
	defer first.Close()

	// When — 서버는 닫지도 답하지도 않고 가만히 있는다 (데드라인 30ms 가 지난다)
	// Then — agent 가 침묵을 알아채고 백오프(약 1초) 뒤에 다시 붙는다
	ln.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second))
	second, err := ln.Accept()
	require.NoError(t, err, "pong 이 없는데 agent 가 재접속하지 않았다")
	second.Close()
}

func TestRunAgentStaysConnectedWhenServerReplies(t *testing.T) {
	// Given — ping 마다 pong 으로 답하는 서버 (ServeHeartbeats) 와 agent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	var accepted atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			go func() {
				defer conn.Close()
				ServeHeartbeats(conn, time.Second)
			}()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// When — agent 데드라인(30ms)의 여러 배인 300ms 동안 돌린다
	startAgent(ctx, ln.Addr().String())
	time.Sleep(300 * time.Millisecond)

	// Then — 연결은 한 번만 맺어졌다 (재접속이 없었다)
	require.Equal(t, int32(1), accepted.Load())
}

func TestRunAgentBacksOffWhenServerNeverReplies(t *testing.T) {
	// Given — 연결은 받아 주지만 pong 을 한 번도 돌려주지 않는 서버와 agent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAgent(ctx, ln.Addr().String())

	// When — 접속을 세 번 받는다
	ln.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second))
	var at [3]time.Time
	for i := range at {
		conn, err := ln.Accept()
		require.NoError(t, err)
		defer conn.Close()
		at[i] = time.Now()
	}

	// Then — 응답이 없으니 백오프가 늘어, 둘째 간격이 지터를 감안해도 1.5초를 넘는다
	// (늘지 않으면 둘 다 약 1초이고 지터 최대치도 1.25초다)
	first, second := at[1].Sub(at[0]), at[2].Sub(at[1])
	require.Greater(t, second, 1500*time.Millisecond, "첫 간격 %v, 둘째 간격 %v", first, second)
}
