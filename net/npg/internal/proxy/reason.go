package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/scofe97/npg-lab/internal/upstream"
)

// Reason 은 요청이 왜 실패했는지 적는 원인 코드다. 이름은 envoy 응답 플래그(%RESPONSE_FLAGS%)를 따른다.
// [4b] classify 가 고르고, [5] 접근 로그의 flags 와 reasons 카운터로 나간다.
type Reason string

const (
	ReasonNone       Reason = "-"  // 실패 없음
	ReasonTimeout    Reason = "UT" // 업스트림이 기한 안에 답하지 않음
	ReasonConnFail   Reason = "UF" // 업스트림에 연결하지 못함
	ReasonConnReset  Reason = "UC" // 업스트림이 연결을 끊음
	ReasonClientGone Reason = "DC" // 손님이 먼저 연결을 끊음
	ReasonOverflow   Reason = "UO" // 풀의 칸과 줄이 다 차 보내지 않음 (envoy UpstreamOverflow)
)

// Status 는 원인 코드를 손님에게 돌려줄 상태 코드로 바꾼다 (nginx 방식).
// 줄이 넘쳐 보내지 않은 것은 503(envoy UO), 기다리다 끝난 것만 504, 손님이 떠난 것은 499(로그용, 손님은 못 받음), 나머지 업스트림 실패는 502.
func (r Reason) Status() int {
	switch r {
	case ReasonOverflow:
		return http.StatusServiceUnavailable
	case ReasonTimeout:
		return http.StatusGatewayTimeout
	case ReasonClientGone:
		return 499
	default:
		return http.StatusBadGateway
	}
}

// 업스트림 요청 에러를 원인 코드로 바꿉니다.
// ctx는 들어온 요청의 context입니다. 손님이 떠나면 Canceled가 됩니다.
func classify(ctx context.Context, err error) Reason {
	var op *net.OpError
	var ne net.Error
	switch {
	case err == nil:
		return ReasonNone
	// 보내지도 않고 거절한 것. 에러 모양이 분명하니 손님 이탈보다 먼저 본다.
	case errors.Is(err, upstream.ErrOverflow):
		return ReasonOverflow
	// 손님이 먼저 떠나면 Transport 에러는 아무 모양이나 될 수 있다. 그래서 업스트림 쪽 모양보다 먼저 본다.
	case errors.Is(ctx.Err(), context.Canceled):
		return ReasonClientGone
	// 연결 단계 실패. 연결 기한 초과도 여기로 온다 (envoy 도 연결 시간 초과를 UF 로 셈).
	case errors.As(err, &op) && op.Op == "dial":
		return ReasonConnFail
	// 연결은 됐는데 답을 기한 안에 못 받음.
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return ReasonTimeout

	// 그 밖은 연결된 뒤 업스트림이 끊은 것으로 본다.
	default:
		return ReasonConnReset
	}
}
