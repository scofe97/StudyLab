package tftp

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testContent(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func TestRequestOptions(t *testing.T) {
	tests := []struct {
		name string
		c    Client
		want map[string]string
	}{
		{"옵션 없음", Client{}, nil},
		{"blksize 만", Client{Blksize: 1428}, map[string]string{"blksize": "1428"}},
		{"tsize 만", Client{Tsize: true}, map[string]string{"tsize": "0"}},
		{"둘 다", Client{Blksize: 1428, Tsize: true}, map[string]string{"blksize": "1428", "tsize": "0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 옵션 설정이 다른 클라이언트
			// When
			got := tt.c.requestOptions()

			// Then — 켠 옵션만 요청에 실린다
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClientGetsFileFromServer(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		client     Client
		wantBlk    int
		wantBlocks int
		wantSize   int64
	}{
		{"기본 512", 1300, Client{}, 512, 3, -1},
		{"정확한 배수", 1024, Client{}, 512, 3, -1},
		{"blksize 협상", 3000, Client{Blksize: 1024}, 1024, 3, -1},
		{"tsize 로 크기 확인", 3000, Client{Blksize: 1024, Tsize: true}, 1024, 3, 3000},
		{"빈 파일", 0, Client{}, 512, 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 서버와 그 안의 파일
			content := testContent(tt.size)
			addr := startServer(t, map[string][]byte{"fw.bin": content}, time.Second, 3)
			c := tt.client
			c.Timeout, c.Retries = 200*time.Millisecond, 3
			var got bytes.Buffer

			// When
			res, err := c.Get(context.Background(), addr, "fw.bin", &got)

			// Then — 내용이 같고 협상 결과가 Result 에 담긴다
			require.NoError(t, err)
			require.Equal(t, string(content), got.String())
			require.Equal(t, tt.wantBlk, res.Blksize)
			require.Equal(t, tt.wantBlocks, res.Blocks)
			require.Equal(t, tt.wantSize, res.Size)
			require.Equal(t, tt.size, res.Bytes)
		})
	}
}

func TestClientReportsServerError(t *testing.T) {
	// Given — 없는 파일을 요청한다
	addr := startServer(t, map[string][]byte{"fw.bin": []byte("x")}, time.Second, 3)
	c := Client{Timeout: 200 * time.Millisecond, Retries: 3}

	// When
	_, err := c.Get(context.Background(), addr, "none.bin", &bytes.Buffer{})

	// Then — 서버가 보낸 ERROR 가 오류로 올라온다
	require.ErrorContains(t, err, "서버 오류 1")
}

func TestClientCallsOnDataBeforeAck(t *testing.T) {
	// Given — 블록 둘짜리 파일과, 첫 블록을 받은 뒤 ACK 를 늦추는 콜백
	content := testContent(DefaultBlksize + 100)
	addr := startServer(t, map[string][]byte{"fw.bin": content}, time.Second, 3)
	var blocks []uint16
	c := Client{Timeout: 200 * time.Millisecond, Retries: 3, OnData: func(b uint16) {
		blocks = append(blocks, b)
	}}

	// When
	_, err := c.Get(context.Background(), addr, "fw.bin", &bytes.Buffer{})

	// Then — 받은 블록마다 콜백이 순서대로 불린다
	require.NoError(t, err)
	require.Equal(t, []uint16{1, 2}, blocks)
}

// fakeServer 는 요청 하나를 받고 script 가 정한 대로 행동하는 가짜 서버입니다.
// script 는 요청을 받은 소켓(listener)과 클라이언트 주소, 새로 연 전용 소켓(data)을 받습니다.
func fakeServer(t *testing.T, script func(listener, data *net.UDPConn, client *net.UDPAddr)) *net.UDPAddr {
	t.Helper()
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	data, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close(); _ = data.Close() })

	go func() {
		buf := make([]byte, 65535)
		_, client, err := listener.ReadFromUDP(buf)
		if err != nil {
			return
		}
		script(listener, data, client)
	}()
	return listener.LocalAddr().(*net.UDPAddr)
}

func readOne(t *testing.T, conn *net.UDPConn, wait time.Duration) (Packet, error) {
	t.Helper()
	buf := make([]byte, 65535)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(wait)))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	return Decode(buf[:n])
}

func TestClientReAcksDuplicateData(t *testing.T) {
	// Given — DATA 1 을 두 번 보내는 서버 (ACK 가 늦은 것처럼 보이는 상황)
	acks := make(chan Ack, 8)
	addr := fakeServer(t, func(_, data *net.UDPConn, client *net.UDPAddr) {
		d1 := Data{Block: 1, Payload: bytes.Repeat([]byte{'a'}, DefaultBlksize)}.Encode()
		d2 := Data{Block: 2, Payload: []byte("end")}.Encode()
		_, _ = data.WriteToUDP(d1, client)
		_, _ = data.WriteToUDP(d1, client)
		for i := 0; i < 2; i++ {
			if p, err := readOne(t, data, time.Second); err == nil {
				if a, ok := p.(Ack); ok {
					acks <- a
				}
			}
		}
		_, _ = data.WriteToUDP(d2, client)
		for i := 0; i < 2; i++ {
			if _, err := readOne(t, data, time.Second); err != nil {
				return
			}
		}
	})
	c := Client{Timeout: 300 * time.Millisecond, Retries: 3}
	var got bytes.Buffer

	// When
	res, err := c.Get(context.Background(), addr, "fw.bin", &got)

	// Then — 중복 블록은 한 번만 쓰고, 중복 DATA 에는 ACK 1 을 다시 보낸다
	require.NoError(t, err)
	require.Equal(t, DefaultBlksize+3, got.Len())
	require.GreaterOrEqual(t, res.DupData, 1)
	require.Equal(t, Ack{Block: 1}, <-acks)
}

func TestClientRejectsStrangerTID(t *testing.T) {
	// Given — 서버 TID 가 정해진 뒤, 다른 포트에서 다음 블록을 가장한 DATA 가 끼어드는 상황
	stranger, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = stranger.Close() })
	first := bytes.Repeat([]byte{'a'}, DefaultBlksize)
	addr := fakeServer(t, func(_, data *net.UDPConn, client *net.UDPAddr) {
		_, _ = data.WriteToUDP(Data{Block: 1, Payload: first}.Encode(), client)
		_, _ = readOne(t, data, time.Second) // ACK 1
		_, _ = stranger.WriteToUDP(Data{Block: 2, Payload: []byte("evil")}.Encode(), client)
		time.Sleep(50 * time.Millisecond)
		_, _ = data.WriteToUDP(Data{Block: 2, Payload: []byte("hi")}.Encode(), client)
		_, _ = readOne(t, data, time.Second) // ACK 2
	})
	c := Client{Timeout: 300 * time.Millisecond, Retries: 3}
	var got bytes.Buffer

	// When
	res, err := c.Get(context.Background(), addr, "fw.bin", &got)

	// Then — 낯선 출처의 내용은 쓰지 않고 세기만 한다
	require.NoError(t, err)
	require.Equal(t, string(first)+"hi", got.String())
	require.Equal(t, 1, res.Strangers)

	// 낯선 출처는 ERROR 5(알 수 없는 전송 ID)를 돌려받는다
	p, err := readOne(t, stranger, time.Second)
	require.NoError(t, err)
	e, ok := p.(ErrorPacket)
	require.True(t, ok, "ERROR 가 와야 합니다: %#v", p)
	require.Equal(t, ErrUnknownTID, e.Code)
}

func TestClientRejectsOversizedBlksizeReply(t *testing.T) {
	// Given — 요청(1024)보다 큰 blksize(2048)로 답하는 서버
	got := make(chan Packet, 1)
	addr := fakeServer(t, func(_, data *net.UDPConn, client *net.UDPAddr) {
		_, _ = data.WriteToUDP(OAck{Options: map[string]string{"blksize": "2048"}}.Encode(), client)
		p, err := readOne(t, data, time.Second)
		if err == nil {
			got <- p
		}
	})
	c := Client{Timeout: 300 * time.Millisecond, Retries: 3, Blksize: 1024}

	// When
	_, err := c.Get(context.Background(), addr, "fw.bin", &bytes.Buffer{})

	// Then — 클라이언트는 ERROR 8(옵션 거절)을 보내고 끝낸다
	require.ErrorIs(t, err, ErrBadOptionReply)
	select {
	case p := <-got:
		e, ok := p.(ErrorPacket)
		require.True(t, ok, "ERROR 가 와야 합니다: %#v", p)
		require.Equal(t, ErrBadOption, e.Code)
	case <-time.After(2 * time.Second):
		t.Fatal("서버가 ERROR 를 받지 못했습니다")
	}
}

func TestClientGivesUpWhenServerIsSilent(t *testing.T) {
	// Given — 요청을 받고도 아무 응답이 없는 서버
	addr := fakeServer(t, func(_, _ *net.UDPConn, _ *net.UDPAddr) {})
	c := Client{Timeout: 20 * time.Millisecond, Retries: 3}

	// When
	res, err := c.Get(context.Background(), addr, "fw.bin", &bytes.Buffer{})

	// Then — 재시도를 모두 쓰고 포기하며 타임아웃을 센다
	require.ErrorIs(t, err, ErrRetriesExhausted)
	require.Equal(t, 3, res.Timeouts)
}

func TestClientRetransmitsRequestWithBackoff(t *testing.T) {
	// Given — 요청을 받고도 응답하지 않는 서버, 도착 시각을 적는다
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	arrivals := make(chan time.Time, 8)
	go func() {
		buf := make([]byte, 65535)
		for {
			if _, _, err := listener.ReadFromUDP(buf); err != nil {
				return
			}
			arrivals <- time.Now()
		}
	}()
	timeout := 40 * time.Millisecond
	c := Client{Timeout: timeout, Retries: 3}

	// When
	_, err = c.Get(context.Background(), listener.LocalAddr().(*net.UDPAddr), "fw.bin", &bytes.Buffer{})

	// Then — 요청이 3번 도착하고, 간격이 타임아웃 이상이며 매번 2배로 늘어난다 (하한만 본다)
	require.ErrorIs(t, err, ErrRetriesExhausted)
	t1, t2, t3 := <-arrivals, <-arrivals, <-arrivals
	require.GreaterOrEqual(t, t2.Sub(t1), timeout)
	require.GreaterOrEqual(t, t3.Sub(t2), 2*timeout)
}

func TestClientRejectsUnrequestedTsize(t *testing.T) {
	// Given — tsize 를 묻지 않았는데 tsize 로 답하는 서버
	addr := fakeServer(t, func(_, data *net.UDPConn, client *net.UDPAddr) {
		_, _ = data.WriteToUDP(OAck{Options: map[string]string{"tsize": "100"}}.Encode(), client)
		_, _ = readOne(t, data, time.Second)
	})
	c := Client{Timeout: 300 * time.Millisecond, Retries: 3}

	// When
	_, err := c.Get(context.Background(), addr, "fw.bin", &bytes.Buffer{})

	// Then — 요청하지 않은 옵션은 거절한다
	require.ErrorIs(t, err, ErrBadOptionReply)
}

func TestClientReAcksLastBlockWhileDallying(t *testing.T) {
	// Given — 마지막 블록을 보낸 서버가 ACK 를 못 받은 것처럼 같은 DATA 를 다시 보낸다
	acks := make(chan Ack, 4)
	addr := fakeServer(t, func(_, data *net.UDPConn, client *net.UDPAddr) {
		last := Data{Block: 1, Payload: []byte("hi")}.Encode()
		_, _ = data.WriteToUDP(last, client)
		for i := 0; i < 2; i++ {
			p, err := readOne(t, data, time.Second)
			if err != nil {
				return
			}
			if a, ok := p.(Ack); ok {
				acks <- a
			}
			if i == 0 {
				_, _ = data.WriteToUDP(last, client)
			}
		}
	})
	c := Client{Timeout: 300 * time.Millisecond, Retries: 3}

	// When
	res, err := c.Get(context.Background(), addr, "fw.bin", &bytes.Buffer{})

	// Then — 마지막 ACK 를 두 번 받는다 (처음 한 번, 중복 DATA 에 다시 한 번)
	require.NoError(t, err)
	require.Equal(t, Ack{Block: 1}, <-acks)
	require.Equal(t, Ack{Block: 1}, <-acks)
	require.Equal(t, 1, res.DupData)
}
