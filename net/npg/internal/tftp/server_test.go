package tftp

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// startServer 는 임시 루트에 files 를 만들고 루프백 UDP 로 서버를 띄워 요청 주소를 돌려줍니다.
func startServer(t *testing.T, files map[string][]byte, timeout time.Duration, retries int) *net.UDPAddr {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "root")
	require.NoError(t, os.Mkdir(dir, 0o755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(base, "secret.txt"), []byte("secret"), 0o644))
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &Server{Root: root, Timeout: timeout, Retries: retries, MaxBlksize: 1428, Log: zerolog.Nop()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, pc) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
		_ = root.Close()
	})
	return pc.LocalAddr().(*net.UDPAddr)
}

// testClient 는 시험용 클라이언트입니다. 서버가 새 포트(TID)에서 답하므로 첫 응답의 출처를 기억합니다.
type testClient struct {
	t      *testing.T
	conn   *net.UDPConn
	server *net.UDPAddr
	peer   *net.UDPAddr
}

func newClient(t *testing.T, server *net.UDPAddr) *testClient {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &testClient{t: t, conn: conn, server: server}
}

func (c *testClient) request(name string) {
	c.t.Helper()
	c.requestWith(name, nil)
}

func (c *testClient) requestWith(name string, opts map[string]string) {
	c.t.Helper()
	_, err := c.conn.WriteToUDP(ReadReq{Filename: name, Mode: "octet", Options: opts}.Encode(), c.server)
	require.NoError(c.t, err)
}

// recv 는 패킷 하나를 기다립니다. 시간 안에 오지 않으면 오류(타임아웃)를 돌려줍니다.
func (c *testClient) recv(wait time.Duration) (Packet, error) {
	c.t.Helper()
	buf := make([]byte, 65535)
	require.NoError(c.t, c.conn.SetReadDeadline(time.Now().Add(wait)))
	n, from, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	if c.peer == nil {
		c.peer = from
	}
	require.Equal(c.t, c.peer.Port, from.Port, "서버 응답은 하나의 TID 에서만 와야 합니다")
	return Decode(buf[:n])
}

func (c *testClient) mustRecvData(block uint16) []byte {
	c.t.Helper()
	p, err := c.recv(2 * time.Second)
	require.NoError(c.t, err)
	d, ok := p.(Data)
	require.True(c.t, ok, "DATA 가 와야 합니다: %#v", p)
	require.Equal(c.t, block, d.Block)
	return append([]byte(nil), d.Payload...)
}

func (c *testClient) ack(block uint16) {
	c.t.Helper()
	_, err := c.conn.WriteToUDP(Ack{Block: block}.Encode(), c.peer)
	require.NoError(c.t, err)
}

func TestServeSendsWholeFile(t *testing.T) {
	tests := []struct {
		name   string
		size   int
		blocks int
	}{
		{"블록 크기로 나누어떨어지지 않음", 1300, 3},
		{"블록 크기의 정확한 배수는 빈 블록이 끝을 알림", 1024, 3},
		{"블록 하나보다 작음", 10, 1},
		{"빈 파일", 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 내용이 있는 파일과 서버
			content := make([]byte, tt.size)
			for i := range content {
				content[i] = byte(i % 251)
			}
			addr := startServer(t, map[string][]byte{"fw.bin": content}, time.Second, 3)
			c := newClient(t, addr)

			// When — RRQ 를 보내고 블록마다 ACK 로 답하며 끝까지 받는다
			c.request("fw.bin")
			var got []byte
			for block := uint16(1); ; block++ {
				payload := c.mustRecvData(block)
				got = append(got, payload...)
				c.ack(block)
				if len(payload) < DefaultBlksize {
					require.Equal(t, int(block), tt.blocks)
					break
				}
			}

			// Then — 받은 내용이 원본과 같다
			require.Equal(t, string(content), string(got))
		})
	}
}

func TestServeRefusesBadRequests(t *testing.T) {
	tests := []string{"none.bin", "../secret.txt", "/etc/passwd", "sub"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			// Given — 서버와 루트 밖·없는 파일 이름
			addr := startServer(t, map[string][]byte{"fw.bin": []byte("x")}, time.Second, 3)
			c := newClient(t, addr)

			// When
			c.request(name)
			p, err := c.recv(2 * time.Second)

			// Then — DATA 가 아니라 ERROR 가 온다
			require.NoError(t, err)
			e, ok := p.(ErrorPacket)
			require.True(t, ok, "ERROR 가 와야 합니다: %#v", p)
			require.Equal(t, ErrFileNotFound, e.Code)
		})
	}
}

func TestRetransmitsOnTimeout(t *testing.T) {
	// Given — ACK 를 보내지 않는 클라이언트
	timeout := 30 * time.Millisecond
	addr := startServer(t, map[string][]byte{"fw.bin": make([]byte, 10)}, timeout, 4)
	c := newClient(t, addr)
	c.request("fw.bin")

	// When — 같은 블록이 다시 올 때까지 기다리며 도착 시각을 적는다
	var times []time.Time
	for i := 0; i < 3; i++ {
		c.mustRecvData(1)
		times = append(times, time.Now())
	}

	// Then — 간격이 타임아웃 이상이고 재전송마다 2배로 늘어난다 (하한만 본다)
	require.GreaterOrEqual(t, times[1].Sub(times[0]), timeout)
	require.GreaterOrEqual(t, times[2].Sub(times[1]), 2*timeout)
}

func TestGivesUpAfterRetries(t *testing.T) {
	// Given — 재시도 3번, ACK 를 보내지 않는 클라이언트
	addr := startServer(t, map[string][]byte{"fw.bin": make([]byte, 10)}, 20*time.Millisecond, 3)
	c := newClient(t, addr)
	c.request("fw.bin")

	// When — 서버가 포기할 때까지 받는다
	datas := 0
	var last Packet
	for {
		p, err := c.recv(2 * time.Second)
		require.NoError(t, err)
		if _, ok := p.(Data); !ok {
			last = p
			break
		}
		datas++
	}

	// Then — 같은 DATA 는 정확히 3번이고 마지막은 ERROR
	require.Equal(t, 3, datas)
	require.IsType(t, ErrorPacket{}, last)
}

func TestStaleAckDoesNotResend(t *testing.T) {
	// Given — 블록 둘짜리 파일, 타임아웃이 긴 서버
	content := make([]byte, DefaultBlksize+100)
	addr := startServer(t, map[string][]byte{"fw.bin": content}, 500*time.Millisecond, 3)
	c := newClient(t, addr)
	c.request("fw.bin")
	c.mustRecvData(1)
	c.ack(1)
	c.mustRecvData(2)

	// When — 지난 블록(1)의 ACK 가 늦게 한 번 더 도착한다
	c.ack(1)
	_, err := c.recv(150 * time.Millisecond)

	// Then — 서버는 블록 2 를 곧바로 다시 보내지 않는다 (RFC 1123 §4.2.3.1)
	var ne net.Error
	require.ErrorAs(t, err, &ne)
	require.True(t, ne.Timeout(), "지난 ACK 에 DATA 가 다시 오면 안 됩니다")

	// 그리고 제 ACK 를 받으면 전송이 끝난다
	c.ack(2)
}

func TestServeNegotiatesOptions(t *testing.T) {
	// Given — 3000바이트 파일과 옵션을 모두 요청하는 클라이언트
	content := make([]byte, 3000)
	for i := range content {
		content[i] = byte(i % 251)
	}
	addr := startServer(t, map[string][]byte{"fw.bin": content}, time.Second, 3)
	c := newClient(t, addr)

	// When — blksize 1024, tsize 0, 모르는 옵션까지 담아 RRQ 를 보낸다
	c.requestWith("fw.bin", map[string]string{"blksize": "1024", "tsize": "0", "windowsize": "8"})
	p, err := c.recv(2 * time.Second)

	// Then — OACK 에는 받아들인 옵션만 들어 있고 모르는 옵션은 빠진다
	require.NoError(t, err)
	oack, ok := p.(OAck)
	require.True(t, ok, "OACK 가 먼저 와야 합니다: %#v", p)
	require.Equal(t, map[string]string{"blksize": "1024", "tsize": "3000"}, oack.Options)

	// ACK 0 으로 답하면 협상한 크기(1024)로 DATA 가 온다
	c.ack(0)
	var got []byte
	for block := uint16(1); ; block++ {
		payload := c.mustRecvData(block)
		got = append(got, payload...)
		c.ack(block)
		if len(payload) < 1024 {
			break
		}
	}
	require.Equal(t, string(content), string(got))
}

func TestServeRetransmitsOAckUntilAcked(t *testing.T) {
	// Given — ACK 0 을 보내지 않는 클라이언트
	addr := startServer(t, map[string][]byte{"fw.bin": make([]byte, 10)}, 30*time.Millisecond, 4)
	c := newClient(t, addr)

	// When — 옵션을 담아 요청하고 OACK 를 두 번 받는다
	c.requestWith("fw.bin", map[string]string{"blksize": "1024"})
	first, err1 := c.recv(2 * time.Second)
	second, err2 := c.recv(2 * time.Second)

	// Then — 같은 OACK 가 타임아웃 뒤에 다시 온다
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.IsType(t, OAck{}, first)
	require.Equal(t, first, second)
}

func TestServeIgnoresUnknownOptionsWithoutOAck(t *testing.T) {
	// Given — 서버가 모르는 옵션만 요청하는 클라이언트
	addr := startServer(t, map[string][]byte{"fw.bin": []byte("hello")}, time.Second, 3)
	c := newClient(t, addr)

	// When
	c.requestWith("fw.bin", map[string]string{"windowsize": "8"})

	// Then — OACK 없이 곧바로 DATA 1 이 온다
	require.Equal(t, "hello", string(c.mustRecvData(1)))
}

func TestServeRejectsBadConfig(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Server)
	}{
		{"재시도 0", func(s *Server) { s.Retries = 0 }},
		{"타임아웃 0", func(s *Server) { s.Timeout = 0 }},
		{"블록 크기 상한 0", func(s *Server) { s.MaxBlksize = 0 }},
		{"블록 크기 상한 65465", func(s *Server) { s.MaxBlksize = 65465 }},
		{"루트 없음", func(s *Server) { s.Root = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 기본 설정에서 한 값만 범위 밖으로 바꾼 서버
			root, err := os.OpenRoot(t.TempDir())
			require.NoError(t, err)
			defer root.Close()
			srv := &Server{Root: root, Timeout: DefaultTimeout, Retries: DefaultRetries, MaxBlksize: DefaultMaxBlksize, Log: zerolog.Nop()}
			tt.edit(srv)
			pc, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)
			defer pc.Close()

			// When
			err = srv.Serve(context.Background(), pc)

			// Then — 소켓을 읽기 전에 설정 오류로 끝난다
			require.ErrorIs(t, err, ErrBadConfig)
		})
	}
}
