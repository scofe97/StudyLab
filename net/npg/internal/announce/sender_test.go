package announce

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const shortWait = 50 * time.Millisecond

// newPair는 loopback 에서 서로를 가리키는 UDP 소켓 두 개를 만듭니다.
// server 는 연결되지 않은 소켓이고 client 는 server 로 Dial 한 소켓입니다.
func newPair(t *testing.T) (server net.PacketConn, client net.Conn) {
	t.Helper()

	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { server.Close() })

	client, err = net.Dial("udp", server.LocalAddr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return server, client
}

func sendTo(t *testing.T, server net.PacketConn, client net.Conn, m Message) {
	t.Helper()

	data, err := m.Encode()
	require.NoError(t, err)
	_, err = server.WriteTo(data, client.LocalAddr())
	require.NoError(t, err)
}

func TestWaitAckReturnsTrueOnMatchingAck(t *testing.T) {
	// Given — nonce 7 인 ACK 가 client 쪽에 도착해 있다
	server, client := newPair(t)
	sendTo(t, server, client, Message{Type: TypeAck, Nonce: 7})

	// When — nonce 7 의 ACK 를 기다린다
	acked, err := waitAck(context.Background(), client, 7, time.Second)

	// Then — ACK 를 받았다
	require.NoError(t, err)
	require.True(t, acked)
}

func TestWaitAckIgnoresMismatchedMessages(t *testing.T) {
	// Given — nonce 가 다른 ACK 와 타입이 HELLO 인 메시지가 도착해 있다
	server, client := newPair(t)
	sendTo(t, server, client, Message{Type: TypeAck, Nonce: 99})
	sendTo(t, server, client, Message{Type: TypeHello, Nonce: 7})

	// When — nonce 7 의 ACK 를 기다린다
	acked, err := waitAck(context.Background(), client, 7, shortWait)

	// Then — 둘 다 무시하고 시간이 다 되어 false
	require.NoError(t, err)
	require.False(t, acked)
}

func TestWaitAckTimesOutWithoutData(t *testing.T) {
	// Given — 아무것도 도착하지 않은 소켓
	_, client := newPair(t)

	// When — 짧게 기다린다
	start := time.Now()
	acked, err := waitAck(context.Background(), client, 7, shortWait)

	// Then — 오류 없이 false 이고, 대기 시간만큼은 기다렸다
	require.NoError(t, err)
	require.False(t, acked)
	require.GreaterOrEqual(t, time.Since(start), shortWait)
}

func TestWaitAckKeepsWaitingAfterConnectionRefused(t *testing.T) {
	// Given — 듣는 소켓이 없는 포트로 Dial 한 client, HELLO 한 번으로 ICMP 거절을 유발한다
	closed, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := closed.LocalAddr().String()
	require.NoError(t, closed.Close())

	client, err := net.Dial("udp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	_, err = client.Write([]byte("x"))
	require.NoError(t, err)

	// When — ACK 를 기다린다
	start := time.Now()
	acked, err := waitAck(context.Background(), client, 7, 2*shortWait)

	// Then — 거절 오류로 중간에 끝내지 않고 대기 시간을 다 채운 뒤 false
	require.NoError(t, err)
	require.False(t, acked)
	require.GreaterOrEqual(t, time.Since(start), 2*shortWait)
}

func TestWaitAckReturnsErrorWhenConnClosedWhileWaiting(t *testing.T) {
	// Given — 대기를 시작한 뒤 다른 고루틴이 소켓을 닫는 상황
	// (미리 닫으면 SetReadDeadline 에서 끝나 읽기 루프를 지나지 않는다)
	_, client := newPair(t)
	go func() {
		time.Sleep(shortWait)
		client.Close()
	}()

	// When — 길게 기다린다 (오류를 삼키면 닫힌 뒤에도 영원히 돌므로 제한 시간을 둔다)
	type result struct {
		acked bool
		err   error
	}
	done := make(chan result, 1)
	go func() {
		acked, err := waitAck(context.Background(), client, 7, 5*time.Second)
		done <- result{acked, err}
	}()

	// Then — 대기 시간(5초)을 채우지 않고 곧바로 오류로 끝난다
	select {
	case r := <-done:
		require.Error(t, r.err)
		require.False(t, r.acked)
	case <-time.After(2 * time.Second):
		t.Fatal("소켓이 닫혔는데 waitAck 가 끝나지 않았습니다")
	}
}

func TestWaitAckReturnsContextError(t *testing.T) {
	// Given — 이미 취소된 context
	_, client := newPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When — 기다린다
	acked, err := waitAck(ctx, client, 7, time.Second)

	// Then — context 오류
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, acked)
}
