package ctrl

import (
	"bufio"
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
			if _, err := io.WriteString(w, "ping\n"); err != nil {
				return err
			}
		}
	}
}

func ServeHeartbeats(conn net.Conn, deadline time.Duration) error {
	r := bufio.NewReader(conn)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(deadline)); err != nil {
			return err
		}

		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if line != "ping\n" {
			return fmt.Errorf("알 수 없는 메시지: %q", line)
		}
		if _, err := io.WriteString(conn, "pong\n"); err != nil {
			return err
		}
	}
}

func ExpectPongs(conn net.Conn, deadline time.Duration, onPong func()) error {
	r := bufio.NewReader(conn)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(deadline)); err != nil {
			return err
		}

		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if line != "pong\n" {
			return fmt.Errorf("알 수 없는 메시지")
		}
		onPong()
	}
}
