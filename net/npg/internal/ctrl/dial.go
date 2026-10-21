package ctrl

import (
	"context"
	"fmt"
	"net"
	"time"
)

const dialTimeout = 5 * time.Second

func Dial(ctx context.Context, addr string) (net.Conn, error) {
	d := net.Dialer{Timeout: dialTimeout}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("provd %s 연결 실패: %w", addr, err)
	}

	return conn, nil
}
