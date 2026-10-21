package announce

import (
	"context"
	"errors"
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
