package announce

import (
	"context"
	"errors"
	"net"

	"github.com/rs/zerolog"
)

// Serve는 pc로 오는 HELLO마다 보낸 곳에 ACK를 돌려주고 onHello를 부릅니다.
// ctx가 끝나면 pc를 닫고 nil을 돌려줍니다.
func Serve(ctx context.Context, pc net.PacketConn, log zerolog.Logger, onHello func(deviceID string, from net.Addr)) error {
	go func() {
		<-ctx.Done()
		pc.Close()
	}()

	buf := make([]byte, MaxAnnounceSize+1)
	for {
		n, from, err := pc.ReadFrom(buf)

		// 소켓 종료
		if errors.Is(err, net.ErrClosed) {
			return nil
		}

		// 에러 있는경우
		if err != nil {
			return err
		}

		// 바디크기 초과
		if n > MaxAnnounceSize {
			log.Warn().Str("from", from.String()).Int("size", n).Msg("상한을 넘은 공지를 버림")
			continue
		}

		// 요청 타입 확인
		msg, err := Decode(buf[:n])
		if err != nil || msg.Type != TypeHello {
			log.Warn().Err(err).Str("from", from.String()).Msg("잘못된 공지를 버림")
			continue
		}

		// 상대방에게 전송
		ack, _ := Message{Type: TypeAck, Nonce: msg.Nonce}.Encode()
		if _, err := pc.WriteTo(ack, from); err != nil {
			log.Warn().Err(err).Str("from", from.String()).Msg("ACK 전송 실패")
		}

		onHello(string(msg.Body), from)
	}
}
