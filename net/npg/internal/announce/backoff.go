package announce

import (
	"math/rand/v2"
	"time"
)

const (
	// MaxAttempts는 HELLO를 보내는 최대 횟수입니다(ADR 0006).
	MaxAttempts = 5

	RetryInitial = 1 * time.Second
	RetryFactor  = 2

	// 지터는 간격의 ±25% 입니다.
	RetryJitter = 0.25
)

// retryWait는 attempt번째(0부터) 전송 뒤 ACK를 기다리는 시간입니다.
func retryWait(attempt int) time.Duration {
	d := RetryInitial

	for i := 0; i < attempt; i++ {
		d *= RetryFactor
	}

	factor := 1 + RetryJitter*(2*rand.Float64()-1)
	return time.Duration(float64(d) * factor)
}
