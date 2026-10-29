package ctrl

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog"
)

func RunAgent(ctx context.Context, addr string, interval time.Duration, log zerolog.Logger) {
	attempt := 0

	for {
		conn, err := Dial(ctx, addr)
		if err == nil {
			log.Info().Str("addr", addr).Msg("연결됨")

			err = runHeartbeat(ctx, conn, interval, func() { attempt = 0 })
			conn.Close()
		}
		if ctx.Err() != nil {
			return
		}

		wait := backoff(attempt)
		attempt++
		log.Warn().Err(err).Str("retry_in", wait.String()).Msg("끊김")

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func runHeartbeat(ctx context.Context, conn net.Conn, interval time.Duration, onPong func()) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() { errCh <- SendHeartbeats(ctx, conn, interval) }()
	go func() { errCh <- ExpectPongs(conn, DeadlineFactor*interval, onPong) }()

	err := <-errCh
	cancel()
	conn.Close()
	<-errCh

	return err
}
