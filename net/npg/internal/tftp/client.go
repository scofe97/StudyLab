package tftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// Client는 TFTP 서버에서 파일 하나를 받아 오는 클라이언트입니다.
type Client struct {
	// Timeout은 첫 재전송까지의 대기입니다. 재전송마다 2배가 됩니다.
	Timeout time.Duration
	// Retries는 같은 패킷을 보내는 최대 횟수입니다.
	Retries int
	// Blksize가 0보다 크면 blksize 옵션을 요청합니다.
	Blksize int
	// Tsize가 참이면 tsize 옵션으로 파일 크기를 묻습니다.
	Tsize bool
	// OnData는 DATA 를 받은 뒤 ACK 를 보내기 전에 불립니다. 진행 표시나 시험용 지연 주입에 씁니다.
	OnData func(block uint16)
}

// Result는 전송 하나의 숫자입니다.
type Result struct {
	// Blksize는 실제로 쓴 블록 크기입니다.
	Blksize int
	// Size는 서버가 tsize 로 알려 준 파일 크기이고, 알리지 않았으면 -1 입니다.
	Size int64
	// Elapsed는 요청부터 마지막 DATA 를 받을 때까지의 시간입니다. 마지막 ACK 뒤의 기다림은 뺍니다.
	Elapsed   time.Duration
	Blocks    int
	Bytes     int
	DupData   int
	Timeouts  int
	Strangers int
}

// requestOptions는 RRQ 에 실을 옵션입니다. 옵션이 없으면 nil 입니다.
func (c *Client) requestOptions() map[string]string {
	var opts map[string]string

	if c.Blksize > 0 {
		opts = map[string]string{"blksize": strconv.Itoa(c.Blksize)}
	}
	if c.Tsize {
		if opts == nil {
			opts = map[string]string{}
		}
		opts["tsize"] = "0"
	}
	return opts
}

// exchange는 pkt 를 보내고 서버의 응답 패킷 하나를 기다립니다. 타임아웃마다 2배로 늘려 다시 보냅니다.
// tid 가 nil 이면 첫 응답이므로 서버 IP 에서 온 패킷이면 되고, 아니면 tid 와 같은 출처여야 합니다.
func (c *Client) exchange(conn *net.UDPConn, to, tid *net.UDPAddr, pkt, buf []byte, res *Result) (Packet, *net.UDPAddr, error) {
	timeout := c.Timeout

	// 재시도 반복
	for attempt := 0; attempt < c.Retries; attempt++ {

		// UDP 전송
		if _, err := conn.WriteToUDP(pkt, to); err != nil {
			return nil, nil, fmt.Errorf("보내기: %w", err)
		}

		// ACK 확인
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					res.Timeouts++
					break
				}
				return nil, nil, fmt.Errorf("읽기: %w", err)
			}
			if !samePeer(from, to, tid) {
				res.Strangers++
				_, _ = conn.WriteToUDP(ErrorPacket{Code: ErrUnknownTID, Message: "unknown transfer ID"}.Encode(), from)
				continue
			}

			p, err := Decode(buf[:n])
			if err != nil {
				continue
			}
			return p, from, nil
		}
		timeout *= 2
	}
	return nil, nil, ErrRetriesExhausted
}

// samePeer는 from 이 기다리던 서버 쪽 출처인지 알려 줍니다.
func samePeer(from, to, tid *net.UDPAddr) bool {
	if tid == nil {
		return from.IP.Equal(to.IP)
	}
	return from.IP.Equal(tid.IP) && from.Port == tid.Port
}

// Get은 서버에서 name 파일을 받아 w 에 씁니다.
func (c *Client) Get(ctx context.Context, server *net.UDPAddr, name string, w io.Writer) (Result, error) {
	res := Result{Blksize: DefaultBlksize, Size: -1}
	start := time.Now()

	// Listen
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		return res, fmt.Errorf("소켓 열기: %w", err)
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	// 처음 통신으로 사양 확인
	buf := make([]byte, 65535)
	rrq := ReadReq{Filename: name, Mode: "octet", Options: c.requestOptions()}.Encode()
	p, tid, err := c.exchange(conn, server, nil, rrq, buf, &res)
	if err != nil {
		return res, err
	}

	// 전송 결과에 따라 함수 실행
	var first *Data
	switch p := p.(type) {
	case ErrorPacket:
		return res, fmt.Errorf("서버 오류 %d: %s", p.Code, p.Message)
	case OAck:
		blksize, size, err := c.acceptOAck(p)
		if err != nil {
			_, _ = conn.WriteToUDP(ErrorPacket{Code: ErrBadOption, Message: "bad option"}.Encode(), tid)
			return res, err
		}
		res.Blksize, res.Size = blksize, size
		// 이 ACK 0 은 F4 의 수신 루프가 첫 DATA 를 기다리며 보냅니다
	case Data:
		first = &p
	default:
		return res, fmt.Errorf("뜻밖의 응답: %T", p)
	}
	want := uint16(1)
	reply := Ack{Block: 0}.Encode()
	for {
		var d Data
		if first != nil {
			d, first = *first, nil
		} else {
			p, _, err := c.exchange(conn, tid, tid, reply, buf, &res)
			if err != nil {
				return res, err
			}
			switch p := p.(type) {
			case Data:
				d = p
			case ErrorPacket:
				return res, fmt.Errorf("서버 오류 %d: %s", p.Code, p.Message)
			default:
				return res, fmt.Errorf("뜻밖의 응답: %T", p)
			}
		}

		// 기다리던 블록이 아니면 (중복) 지난 ACK 를 다시 보내려고 루프를 돈다
		if d.Block != want {
			res.DupData++
			continue
		}

		if _, err := w.Write(d.Payload); err != nil {
			return res, fmt.Errorf("쓰기: %w", err)
		}
		res.Blocks++
		res.Bytes += len(d.Payload)
		if c.OnData != nil {
			c.OnData(d.Block)
		}

		reply = Ack{Block: d.Block}.Encode()
		if len(d.Payload) < res.Blksize {
			res.Elapsed = time.Since(start)
			return res, c.dally(conn, tid, reply, buf, &res)
		}
		want++
	}
}

// acceptOAck는 서버가 받아들인 옵션을 확인하고 쓸 블록 크기와 파일 크기 돌려줌
// 요청하지 않은 옵션이나 요청보다 큰 blksize는 거절한다.
func (c *Client) acceptOAck(o OAck) (int, int64, error) {
	blksize, size := DefaultBlksize, int64(-1)

	// 옵션 확인
	for name, value := range o.Options {
		switch name {
		case "blksize":
			v, err := strconv.Atoi(value)
			if err != nil || c.Blksize == 0 || v < minBlksize || v > c.Blksize {
				return 0, -1, fmt.Errorf("%w: blksize=%s", ErrBadOptionReply, value)
			}
			blksize = v
		case "tsize":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil || !c.Tsize || v < 0 {
				return 0, -1, fmt.Errorf("%w: tsize=%s", ErrBadOptionReply, value)
			}
			size = v
		default:
			return 0, -1, fmt.Errorf("%w: %s", ErrBadOptionReply, name)
		}
	}
	return blksize, size, nil
}

// dally는 마지막 ACK 를 보낸 뒤 한 타임아웃 동안 기다립니다.
// 마지막 ACK 가 사라져 서버가 마지막 DATA 를 다시 보내면 ACK 를 다시 보냅니다.
func (c *Client) dally(conn *net.UDPConn, tid *net.UDPAddr, ack, buf []byte, res *Result) error {
	for attempt := 0; attempt < c.Retries; attempt++ {

		// 마지막 ACK
		if _, err := conn.WriteToUDP(ack, tid); err != nil {
			return fmt.Errorf("마지막 ACK 보내기: %w", err)
		}

		_ = conn.SetReadDeadline(time.Now().Add(c.Timeout))
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				// 조용하면 서버가 마지막 ACK 를 받은 것이다
				return nil
			}
			if !samePeer(from, tid, tid) {
				res.Strangers++
				continue
			}
			if p, err := Decode(buf[:n]); err == nil {
				if _, ok := p.(Data); ok {
					res.DupData++
					break
				}
			}
		}
	}
	return nil
}
