package dns

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Options 는 질의 한 번을 어떻게 물을지다.
type Options struct {
	Timeout  time.Duration // 시도 한 번에 기다리는 시간 (0 이면 2초)
	Attempts int           // UDP 로 같은 질의를 보내는 최대 횟수 (0 이면 1)
	TCPOnly  bool          // UDP 를 건너뛰고 TCP 로만 묻는다
}

// Exchange 는 질의 한 번의 기록이다. 실험에서 보낸 바이트와 받은 바이트를 그대로 찍으려고 남긴다.
type Exchange struct {
	Sent      []byte
	Truncated []byte // UDP 로 받은 TC=1 응답. TCP 로 다시 물었을 때만 채운다
	Raw       []byte
	Via       string // "udp" 또는 "tcp"
	Attempts  int    // UDP 로 보낸 횟수
	Skipped   int    // 짝이 맞지 않아 버린 데이터그램 수
	Resp      *Response
}

// Resolve 는 UDP 로 묻고, 응답에 TC 가 켜져 있으면 같은 질의를 TCP 로 다시 묻는다.
// 에러가 나도 그때까지의 기록을 담은 Exchange 를 함께 돌려준다.
func Resolve(ctx context.Context, server string, sent []byte, opt Options) (*Exchange, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = 2 * time.Second
	}
	ex := &Exchange{Sent: sent}
	if !opt.TCPOnly {
		raw, resp, err := ex.queryUDP(ctx, server, opt)
		if err != nil {
			return ex, err
		}

		ex.Raw, ex.Resp, ex.Via = raw, resp, "udp"
		if !resp.Header.TC() {
			return ex, nil
		}
		ex.Truncated = raw
	}
	raw, err := QueryTCP(ctx, server, sent, opt.Timeout)
	if err != nil {
		return ex, err
	}

	resp, err := Parse(raw, sent)
	if err != nil {
		return ex, err
	}

	ex.Raw, ex.Resp, ex.Via = raw, resp, "tcp"
	return ex, nil
}

// queryUDP 는 질의를 보내고 짝이 맞는 첫 응답을 돌려준다.
// UDP 는 손실을 알려 주지 않으므로 시도마다 Timeout 만큼만 기다리고, 그 안에 답이 없으면 같은 질의(같은 ID)를 다시 보낸다.
// 짝이 맞지 않는 데이터그램은 버리고 계속 기다린다. 앞선 시도의 늦은 응답은 짝이 맞으므로 그대로 받는다.
func (ex *Exchange) queryUDP(ctx context.Context, server string, opt Options) ([]byte, *Response, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close() // 이 함수가 소켓의 owner 다
	// ctx 가 취소되면 deadline 을 당겨 막혀 있는 Read 를 깨운다.
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	defer stop()

	buf := make([]byte, 64*1024)
	for ex.Attempts < max(opt.Attempts, 1) {
		ex.Attempts++
		if _, err := conn.Write(ex.Sent); err != nil {
			return nil, nil, err
		}
		conn.SetReadDeadline(time.Now().Add(opt.Timeout))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if ctx.Err() != nil {
					return nil, nil, ctx.Err()
				}
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					break // 이번 시도는 시간 초과, 같은 질의를 다시 보낸다
				}
				// connect 한 UDP 소켓은 ICMP port unreachable 을 ECONNREFUSED 로 받는다.
				return nil, nil, err
			}
			resp, perr := Parse(buf[:n], ex.Sent)
			if perr != nil {
				ex.Skipped++
				continue
			}
			return bytes.Clone(buf[:n]), resp, nil
		}
	}
	return nil, nil, fmt.Errorf("dns: udp %d 번 시도 모두 시간 초과", ex.Attempts)
}

// QueryTCP 는 TCP 로 묻는다. TCP 는 바이트 흐름이라 메시지 경계가 없으므로
// 앞에 길이 2바이트를 붙여 보내고, 응답도 길이 2바이트를 먼저 읽은 뒤 그 길이만큼 채운다 (RFC 1035 §4.2.2).
func QueryTCP(ctx context.Context, server string, sent []byte, timeout time.Duration) ([]byte, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, err
	}
	defer conn.Close() // 이 함수가 연결의 owner 다
	conn.SetDeadline(time.Now().Add(timeout))
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	defer stop()

	// 길이와 본문을 Write 한 번에 넣는다. TCP 가 경계를 지켜 주지는 않지만, 세그먼트 하나로 나가 캡처를 읽기 쉽다.
	msg := binary.BigEndian.AppendUint16(make([]byte, 0, 2+len(sent)), uint16(len(sent)))
	msg = append(msg, sent...)
	if _, err := conn.Write(msg); err != nil {
		return nil, err
	}

	var lenBuf [2]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	resp := make([]byte, binary.BigEndian.Uint16(lenBuf[:]))
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
