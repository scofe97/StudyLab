package announce

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"syscall"
	"time"
)

// waitAck는 wait 동안 conn에서 읽으며 nonce가 같은 ACK를 기다립니다.
// ACK가 오면 true, 시간이 다 되면 false를 돌려줍니다.
func waitAck(ctx context.Context, conn net.Conn, nonce uint64, wait time.Duration) (bool, error) {

	// 읽기 타임아웃 설정
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return false, err
	}

	// 최대 크기 +1만큼 읽기 가능
	buf := make([]byte, MaxAnnounceSize+1)

	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		// 연결 진행 (거부당하는건 계속)
		n, err := conn.Read(buf)
		if errors.Is(err, syscall.ECONNREFUSED) {
			continue
		}

		// 타입 단언하고, 타임아웃이면 실패
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		// 타입, nonce, body 3개 데이터 받아옴
		msg, err := Decode(buf[:n])
		if err != nil || msg.Type != TypeAck || msg.Nonce != nonce {
			continue
		}

		return true, nil
	}
}

// waitFor는 attempt번째 전송 뒤 기다릴 시간을 정합니다. 시험이 짧은 값으로 바꿔 끼웁니다.
var waitFor = retryWait

// ErrNoAck는 MaxAttempts번 보내도 ACK가 오지 않았을 때의 오류입니다.
var ErrNoAck = errors.New("공지 ACK를 받지 못했습니다")

// Send는 addr의 provd에 HELLO를 보내고, ACK가 올 때까지 최대 MaxAttempts번 다시 보냅니다.
func Send(ctx context.Context, addr, deviceID string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("provd %s 공지 소켓 열기 실패: %w", addr, err)
	}
	defer conn.Close()

	hello := Message{Type: TypeHello, Nonce: rand.Uint64(), Body: []byte(deviceID)}
	data, err := hello.Encode()
	if err != nil {
		return err
	}

	for attempt := 0; attempt < MaxAttempts; attempt++ {
		if _, err := conn.Write(data); err != nil && !errors.Is(err, syscall.ECONNREFUSED) {
			return fmt.Errorf("HELLO 전송 실패: %w", err)
		}

		acked, err := waitAck(ctx, conn, hello.Nonce, waitFor(attempt))
		if err != nil {
			return err
		}
		if acked {
			return nil
		}
	}

	return ErrNoAck
}
