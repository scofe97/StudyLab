package probe

import (
	"context"
	"errors"
	"net"
	"syscall"

	"github.com/scofe97/runners-lab/net/netpath/types"
)

// 메시지 매칭 금지 — errors.Is/As 만.
// stage 를 받는 건 타임아웃이 단계를 안 말해서다.
// 근거: docs/concepts/error-as-value.md
func classify(stage types.Stage, err error) types.ErrorKind {
	if err == nil {
		return ""
	}

	// url.Parse 는 통과하지만 dial 에서 드러나는 주소 오류(포트 범위 초과 등).
	// 체인 두 겹 안에 있어 타입 단언으로는 안 잡힌다.
	var addrErr *net.AddrError
	if errors.As(err, &addrErr) {
		return types.ErrInvalidTarget
	}

	// DNS 는 전용 에러 타입이 플래그로 이유를 알려준다.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		switch {
		case dnsErr.IsNotFound:
			return types.ErrDNSNotFound
		case dnsErr.IsTimeout:
			return types.ErrDNSTimeout
		default:
			return types.ErrDNSOther
		}
	}

	// 소켓 수준 실패. errno 는 값이라 errors.Is 로 비교한다.
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return types.ErrTCPRefused
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return types.ErrTCPUnreachable
	case errors.Is(err, syscall.ECONNRESET):
		// 연결이 세워진 뒤 끊긴 것이다. 전용 분류가 아직 없어 단계별 other 로 보낸다.
		return otherFor(stage)
	}

	// 취소와 타임아웃. 호출자가 멈춘 것과 시간이 다한 것을 가른다.
	if errors.Is(err, context.Canceled) {
		return types.ErrCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return timeoutFor(stage)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return timeoutFor(stage)
	}

	return otherFor(stage)
}

func timeoutFor(stage types.Stage) types.ErrorKind {
	switch stage {
	case types.StageDNS:
		return types.ErrDNSTimeout
	case types.StageTCP:
		return types.ErrTCPTimeout
	case types.StageHTTP:
		return types.ErrHTTPTimeout
	default:
		return types.ErrOther
	}
}

// _other 는 대기실 — 잦은 원인이 보이면 승격한다.
func otherFor(stage types.Stage) types.ErrorKind {
	switch stage {
	case types.StageDNS:
		return types.ErrDNSOther
	case types.StageTCP:
		return types.ErrTCPOther
	case types.StageHTTP:
		return types.ErrHTTPProtocol
	default:
		return types.ErrOther
	}
}
