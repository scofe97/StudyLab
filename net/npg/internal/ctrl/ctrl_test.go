package ctrl

import (
	"testing"
	"time"
)

func TestBackoffStaysWithinJitterRange(t *testing.T) {

	tests := []struct {
		attempt int
		base    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},
		{20, 30 * time.Second},
	}

	for _, tt := range tests {
		// Given — 지터 없는 기준값과 허용 범위(±20%)
		low := time.Duration(float64(tt.base) * (1 - BackoffJitter))
		high := time.Duration(float64(tt.base) * (1 + BackoffJitter))

		// When — 같은 입력을 여러 번 호출해 지터가 흔들리게 한다
		for range 100 {
			got := backoff(tt.attempt)

			// Then — 항상 허용 범위 안에 있다
			if got < low || got > high {
				t.Fatalf("attempt=%d: got %v, want [%v, %v]", tt.attempt, got, low, high)
			}
		}
	}

}
