package announce

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryWaitStaysWithinJitterRange(t *testing.T) {
	tests := []struct {
		attempt int
		base    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
	}

	for _, tt := range tests {
		// Given — attempt 번째의 기준 간격 (1·2·4·8·16초)과 ADR 0006 의 ±25% (상수를 따라가지 않도록 리터럴)
		low := time.Duration(float64(tt.base) * 0.75)
		high := time.Duration(float64(tt.base) * 1.25)

		for i := 0; i < 1000; i++ {
			// When — 대기 시간을 구한다
			got := retryWait(tt.attempt)

			// Then — 기준 간격의 ±25% 안이다
			require.GreaterOrEqual(t, got, low)
			require.LessOrEqual(t, got, high)
		}
	}
}

func TestRetryWaitAppliesJitter(t *testing.T) {
	// Given — 같은 attempt 로 여러 번 구한 값을 모을 집합
	seen := map[time.Duration]bool{}

	// When — 100번 구한다
	for i := 0; i < 100; i++ {
		seen[retryWait(0)] = true
	}

	// Then — 지터가 섞여 값이 하나로 고정되지 않는다
	require.Greater(t, len(seen), 1)
}

func TestTotalWaitMatchesADR(t *testing.T) {
	// Given — MaxAttempts 번 보내고 매번 기준 간격만큼 기다린다
	var total time.Duration
	d := RetryInitial

	// When — 기준 간격을 합한다
	for i := 0; i < MaxAttempts; i++ {
		total += d
		d *= RetryFactor
	}

	// Then — 1+2+4+8+16 = 31초 (ADR 0006)
	require.Equal(t, 31*time.Second, total)
}
