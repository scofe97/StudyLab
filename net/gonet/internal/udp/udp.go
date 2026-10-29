// Package udp 는 Phase 2 의 UDP primitive 를 담는다.
// TCP 패키지와 나란히 두고, 연결·메시지 경계·손실이 코드 모양을 어떻게 바꾸는지 비교하는 것이 목적이다.
package udp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

// Serve 는 pc 에서 데이터그램을 하나씩 읽어 보낸 쪽으로 그대로 돌려준다(echo).
// 소켓은 pc 하나뿐이고, 상대가 누구인지는 ReadFrom 이 돌려주는 주소로만 안다.
// bufSize 보다 큰 데이터그램은 잘리고 남는 부분은 버려진다. ReadFrom 은 이를 에러로 알리지 않는다.
// ctx 가 끝나면 pc 를 닫아 blocking ReadFrom 을 깨우고 nil 을 돌려준다.
func Serve(ctx context.Context, pc net.PacketConn, bufSize int, logger *log.Logger) error {
	stop := context.AfterFunc(ctx, func() { pc.Close() })
	defer stop()

	buf := make([]byte, bufSize)
	for {
		// ReadFrom 한 번은 데이터그램 하나다. 두 데이터그램이 쌓여 있어도 이어 붙이지 않는다.
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		logger.Printf("recv r=%d from=%s data=%q", n, from, buf[:n])

		// 보낸 쪽이 이미 떠났어도 UDP 는 알려 주지 않는다. 쓰기 실패만 남기고 다음 데이터그램으로 간다.
		if _, err := pc.WriteTo(buf[:n], from); err != nil {
			logger.Printf("echo error: to=%s %v", from, err)
		}
	}
}

// Send 는 addr 로 msgs 를 데이터그램 하나씩 보내고, wait 동안 돌아온 echo 를 out 에 한 줄씩 쓴다.
// 사라진 데이터그램은 재전송하지 않는다. wait 가 지나면 받은 만큼만 쓰고 끝난다.
func Send(ctx context.Context, addr string, msgs []string, wait time.Duration, out io.Writer) error {
	var d net.Dialer

	// UDP Dial 은 handshake 없이 connect() 로 상대 주소만 정한다. 이때 커널이 임시 포트를 붙인다.
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	fmt.Fprintf(out, "local=%s remote=%s\n", conn.LocalAddr(), conn.RemoteAddr())
	for _, m := range msgs {
		if _, err := conn.Write([]byte(m)); err != nil {
			return err
		}
	}

	buf := make([]byte, 64*1024)
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return err
	}
	for {
		n, err := conn.Read(buf)
		switch {
		case err == nil:
			fmt.Fprintf(out, "echo r=%d data=%q\n", n, buf[:n])
		case errors.Is(err, os.ErrDeadlineExceeded), ctx.Err() != nil:
			return nil
		default:
			return err
		}
	}
}
