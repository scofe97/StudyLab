package announce

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type hello struct {
	deviceID string
	from     net.Addr
}

// startServe는 loopback 에서 Serve 를 띄우고 받은 HELLO 를 채널로 모읍니다.
func startServe(t *testing.T) (addr string, got <-chan hello, done <-chan error, cancel context.CancelFunc) {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan hello, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, pc, zerolog.Nop(), func(id string, from net.Addr) {
			ch <- hello{id, from}
		})
	}()
	t.Cleanup(cancel)

	return pc.LocalAddr().String(), ch, errCh, cancel
}

func dialTo(t *testing.T, addr string) net.Conn {
	t.Helper()

	conn, err := net.Dial("udp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func writeMsg(t *testing.T, conn net.Conn, m Message) {
	t.Helper()

	data, err := m.Encode()
	require.NoError(t, err)
	_, err = conn.Write(data)
	require.NoError(t, err)
}

func TestServeAcksHelloWithSameNonceAndCallsOnHello(t *testing.T) {
	// Given — 떠 있는 Serve 와 그곳으로 Dial 한 agent 소켓
	addr, got, _, _ := startServe(t)
	conn := dialTo(t, addr)

	// When — nonce 7 의 HELLO 를 보낸다
	writeMsg(t, conn, Message{Type: TypeHello, Nonce: 7, Body: []byte("dev1")})
	acked, err := waitAck(context.Background(), conn, 7, time.Second)

	// Then — 같은 nonce 의 ACK 가 오고, 콜백이 장비 id 와 agent 의 주소로 불렸다
	require.NoError(t, err)
	require.True(t, acked)
	h := <-got
	require.Equal(t, "dev1", h.deviceID)
	require.Equal(t, conn.LocalAddr().String(), h.from.String())
}

func TestServeAcksEveryRetransmittedHello(t *testing.T) {
	// Given — 같은 HELLO 를 두 번 보내는 agent (재전송)
	addr, got, _, _ := startServe(t)
	conn := dialTo(t, addr)

	for i := 0; i < 2; i++ {
		// When — 같은 nonce 의 HELLO 를 보낸다
		writeMsg(t, conn, Message{Type: TypeHello, Nonce: 7, Body: []byte("dev1")})

		// Then — 매번 ACK 가 오고 콜백도 매번 불린다 (ACK 가 유실됐을 수 있기 때문)
		acked, err := waitAck(context.Background(), conn, 7, time.Second)
		require.NoError(t, err)
		require.True(t, acked)
		<-got
	}
}

func TestServeDropsOversizedDatagramWithoutAck(t *testing.T) {
	// Given — 상한(1232)보다 1바이트 큰 데이터그램. 헤더는 정상 HELLO 형식이다
	addr, got, _, _ := startServe(t)
	conn := dialTo(t, addr)
	big := make([]byte, MaxAnnounceSize+1)
	big[0], big[1] = Version, byte(TypeHello)

	// When — 보낸다
	_, err := conn.Write(big)
	require.NoError(t, err)
	acked, err := waitAck(context.Background(), conn, 0, 100*time.Millisecond)

	// Then — ACK 도 콜백도 없다
	require.NoError(t, err)
	require.False(t, acked)
	require.Empty(t, got)
}

func TestServeIgnoresMalformedAndStaysAlive(t *testing.T) {
	// Given — Serve 와 agent 소켓
	addr, got, _, _ := startServe(t)
	conn := dialTo(t, addr)

	// When — 짧은 데이터, ACK 타입 메시지를 보낸 뒤 정상 HELLO 를 보낸다
	_, err := conn.Write([]byte{1, 1})
	require.NoError(t, err)
	writeMsg(t, conn, Message{Type: TypeAck, Nonce: 5})
	writeMsg(t, conn, Message{Type: TypeHello, Nonce: 9, Body: []byte("dev2")})

	// Then — 앞의 둘은 무시되고 정상 HELLO 에만 ACK 가 온다
	acked, err := waitAck(context.Background(), conn, 9, time.Second)
	require.NoError(t, err)
	require.True(t, acked)
	require.Equal(t, "dev2", (<-got).deviceID)
	require.Empty(t, got)
}

func TestServeReturnsNilWhenContextCancelled(t *testing.T) {
	// Given — 떠 있는 Serve
	_, _, done, cancel := startServe(t)

	// When — ctx 를 취소한다
	cancel()

	// Then — 오류 없이 끝난다
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("ctx 취소 뒤에도 Serve 가 끝나지 않았습니다")
	}
}
