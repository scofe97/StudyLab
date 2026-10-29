package ctrl

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

func SendHeartbeats(ctx context.Context, w io.Writer, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := WriteFrame(w, []byte("ping")); err != nil {
				return err
			}
		}
	}
}

func ServeHeartbeats(conn net.Conn, deadline time.Duration) error {
	for {
		if err := conn.SetReadDeadline(time.Now().Add(deadline)); err != nil {
			return err
		}

		line, err := ReadFrame(conn)
		if err != nil {
			return err
		}
		if string(line) != "ping" {
			return fmt.Errorf("알 수 없는 메시지: %q", line)
		}
		if err := WriteFrame(conn, []byte("pong")); err != nil {
			return err
		}
	}
}

func ExpectPongs(conn net.Conn, deadline time.Duration, onPong func()) error {
	for {
		if err := conn.SetReadDeadline(time.Now().Add(deadline)); err != nil {
			return err
		}

		line, err := ReadFrame(conn)
		if err != nil {
			return err
		}
		if string(line) != "pong" {
			return fmt.Errorf("알 수 없는 메시지")
		}
		onPong()
	}
}
