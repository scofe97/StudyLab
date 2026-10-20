package ctrl

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

func RunAgent(ctx context.Context, addr string, interval time.Duration, log zerolog.Logger) error {
	attempt := 0

	for {
		conn, err := Dial(ctx, addr)
		if err == nil {
			attempt = 0
			log.Info().Str("addr", addr).Msg("연결됨")

			err = SendHeartbeats(ctx, conn, interval)
			conn.Close()
		}
		if ctx.Err() != nil {
			return nil
		}

		wait := backoff(attempt)
		attempt++
		log.Warn().Err(err).Dur("retry_in", wait).Msg("끊김")

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}
