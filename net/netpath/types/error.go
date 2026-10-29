// Package types — 터미널·JSON·OpenTelemetry 가 공유하는 안정 계약.
package types

// 근거: docs/concepts/error-as-value.md
type ErrorKind string

// 값에 단계 접두사를 박는다 — 이 값 하나로 단계까지 알 수 있어야 한다. tls_ 는 1.5국면에서 추가.
const (
	ErrDNSNotFound ErrorKind = "dns_not_found" // NXDOMAIN
	ErrDNSTimeout  ErrorKind = "dns_timeout"
	ErrDNSOther    ErrorKind = "dns_other" // SERVFAIL 등. *net.DNSError 가 RCODE 를 안 넘긴다

	// refused 와 timeout 을 절대 합치지 않는다. 이 도구의 존재 이유다.
	// Kubernetes 에서 전자는 "앱이 포트를 안 열었다", 후자는 "NetworkPolicy DROP" 이다.
	ErrTCPRefused     ErrorKind = "tcp_refused"     // RST 를 받았다
	ErrTCPTimeout     ErrorKind = "tcp_timeout"     // 아무 답이 없다
	ErrTCPUnreachable ErrorKind = "tcp_unreachable" // ICMP unreachable 또는 로컬에 경로 없음
	ErrTCPOther       ErrorKind = "tcp_other"       // fd·임시포트 고갈 등 호출자 쪽 문제까지

	ErrHTTPStatus   ErrorKind = "http_status" // 4xx/5xx
	ErrHTTPTimeout  ErrorKind = "http_timeout"
	ErrHTTPProtocol ErrorKind = "http_protocol"

	ErrCanceled      ErrorKind = "canceled"       // 호출자가 멈췄다. 측정 실패가 아니다
	ErrInvalidTarget ErrorKind = "invalid_target" // *net.AddrError
	ErrOther         ErrorKind = "other"
)
