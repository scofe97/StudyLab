package announce

import (
	"context"
	"net"
	"strings"
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

// fakeProvd는 loopback 에서 HELLO 를 받는 가짜 provd 를 띄웁니다.
// ackFrom 번째(1부터) HELLO 부터 ACK 를 돌려주고, 0 이하이면 한 번도 답하지 않습니다.
// 받은 HELLO 는 got 채널에 쌓입니다.
func fakeProvd(t *testing.T, ackFrom int) (addr string, got <-chan Message) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() })

	ch := make(chan Message, 16)
	go func() {
		buf := make([]byte, MaxAnnounceSize+1)
		count := 0
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}

			msg, err := Decode(buf[:n])
			if err != nil || msg.Type != TypeHello {
				continue
			}

			count++
			ch <- Message{Type: msg.Type, Nonce: msg.Nonce, Body: append([]byte(nil), msg.Body...)}

			if ackFrom > 0 && count >= ackFrom {
				ack, _ := Message{Type: TypeAck, Nonce: msg.Nonce}.Encode()
				pc.WriteTo(ack, from)
			}
		}
	}()

	return pc.LocalAddr().String(), ch
}

// useShortWait는 재전송 대기를 짧게 바꾸고 시험이 끝나면 되돌립니다.
func useShortWait(t *testing.T) {
	t.Helper()

	orig := waitFor
	waitFor = func(int) time.Duration { return 30 * time.Millisecond }
	t.Cleanup(func() { waitFor = orig })
}

func TestSendStopsAfterFirstAck(t *testing.T) {
	// Given — 첫 HELLO 에 ACK 를 주는 provd
	useShortWait(t)
	addr, got := fakeProvd(t, 1)

	// When — HELLO 를 보낸다
	err := Send(context.Background(), addr, "dev1")

	// Then — 성공하고 HELLO 는 한 번만 갔다
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, []byte("dev1"), (<-got).Body)
}

func TestSendRetransmitsWithSameNonceUntilAck(t *testing.T) {
	// Given — 첫 두 HELLO 는 무시하고 세 번째부터 ACK 를 주는 provd (앞의 둘은 유실에 해당)
	useShortWait(t)
	addr, got := fakeProvd(t, 3)

	// When — HELLO 를 보낸다
	err := Send(context.Background(), addr, "dev1")

	// Then — 성공하고, HELLO 가 세 번 같은 nonce 로 갔다
	require.NoError(t, err)
	require.Len(t, got, 3)
	first := <-got
	require.Equal(t, first.Nonce, (<-got).Nonce)
	require.Equal(t, first.Nonce, (<-got).Nonce)
}

func TestSendGivesUpAfterMaxAttempts(t *testing.T) {
	// Given — 한 번도 답하지 않는 provd
	useShortWait(t)
	addr, got := fakeProvd(t, 0)

	// When — HELLO 를 보낸다
	err := Send(context.Background(), addr, "dev1")

	// Then — ErrNoAck 이고 HELLO 는 정확히 MaxAttempts 번 갔다
	require.ErrorIs(t, err, ErrNoAck)
	require.Eventually(t, func() bool { return len(got) == MaxAttempts }, time.Second, 10*time.Millisecond)
}

func TestSendRetriesWhenNobodyListens(t *testing.T) {
	// Given — 듣는 소켓이 없는 포트 (ICMP 거절이 돌아온다), 대기 30ms
	useShortWait(t)
	closed, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := closed.LocalAddr().String()
	require.NoError(t, closed.Close())

	// When — HELLO 를 보낸다
	start := time.Now()
	err = Send(context.Background(), addr, "dev1")

	// Then — 첫 거절에서 끝내지 않고 MaxAttempts 번의 대기를 모두 채운 뒤 ErrNoAck
	require.ErrorIs(t, err, ErrNoAck)
	require.GreaterOrEqual(t, time.Since(start), time.Duration(MaxAttempts)*30*time.Millisecond)
}

func TestSendRejectsOversizedDeviceID(t *testing.T) {
	// Given — 상한을 넘는 장비 id
	useShortWait(t)
	addr, got := fakeProvd(t, 1)
	id := strings.Repeat("x", MaxAnnounceSize)

	// When — HELLO 를 보낸다
	err := Send(context.Background(), addr, id)

	// Then — ErrTooLarge 이고 아무것도 전송되지 않았다
	require.ErrorIs(t, err, ErrTooLarge)
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, got)
}
