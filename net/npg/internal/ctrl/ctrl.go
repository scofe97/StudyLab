// Package ctrl 은 agent 와 provd 가 TCP 로 주고받는 제어 채널(연결 맺기·재접속·하트비트)을 담습니다.
package ctrl

import (
	"math/rand/v2"
	"time"
)

const (
	DefaultAddr = "127.0.0.1:7100"

	HeartbeatInterval = 5 * time.Second

	// 읽기 데드라인은 하트비트 간격의 3배입니다.
	DeadlineFactor = 3
	ReadDeadline   = DeadlineFactor * HeartbeatInterval

	BackoffInitial = 1 * time.Second
	BackoffFactor  = 2
	BackoffMax     = 30 * time.Second

	// 지터는 간격의 ±20% 입니다.
	BackoffJitter = 0.2
)

func backoff(attempt int) time.Duration {
	d := BackoffInitial

	for i := 0; i < attempt && d < BackoffMax; i++ {
		d *= BackoffFactor
	}

	if d > BackoffMax {
		d = BackoffMax
	}

	factor := 1 + BackoffJitter*(2*rand.Float64()-1)
	return time.Duration(float64(d) * factor)
}
