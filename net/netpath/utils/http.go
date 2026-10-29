// Package utils — 코어가 공유하는 상태 없는 헬퍼.
package utils

import (
	"context"
	"net/http"
	"time"
)

// DefaultTransport 는 전역이라 직접 고치면 같은 프로세스 다른 요청까지 바뀐다 — Clone 은 커넥션 풀은 안 물려받는다.
func NewTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()

	// 재사용된 연결은 dial 을 건너뛰어 ConnectStart 훅이 안 불린다 — 측정이 사라지므로 필수.
	t.DisableKeepAlives = true
	return t
}

// 0 이하는 데드라인이 이미 지났다는 뜻 — net.Dialer.Timeout 에서 0 은 "제한 없음"이라 그대로 넘기면 안 된다.
func RemainingTimeout(ctx context.Context, want time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return want
	}
	if left := time.Until(deadline); left < want {
		return left
	}
	return want
}
