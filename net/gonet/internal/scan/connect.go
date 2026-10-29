package scan

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"
)

// ProbeConnect 는 net.Dial 로 3-way handshake 를 끝까지 맺어 보고 결과를 errno 로만 판정한다 (04-01 §1).
// SYN·SYN-ACK·ACK 는 커널이 주고받으므로 앱이 보는 것은 성공이나 에러 하나뿐이다.
func ProbeConnect(ctx context.Context, host string, port int, timeout time.Duration) Result {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	r := Result{Port: port}
	switch {
	case err == nil:
		// handshake 가 이미 끝났다. 대상 서버의 accept 대기열에도 올랐으니 바로 닫는다.
		conn.Close()
		r.Status = StatusOpen
	case errors.Is(err, syscall.ECONNREFUSED):
		// 대상 커널의 RST, 또는 방화벽 REJECT 의 ICMP port unreachable. 커널이 둘 다 이 errno 로 바꾼다 (04-01 §3).
		r.Status, r.Errno = StatusClosed, "ECONNREFUSED"
	case errors.Is(err, syscall.EHOSTUNREACH):
		// ICMP host unreachable·admin prohibited. 막혔다는 증거가 돌아왔으므로 무응답과 구별해 남긴다.
		r.Status, r.Errno = StatusFiltered, "EHOSTUNREACH"
	case isTimeout(err):
		// 아무것도 돌아오지 않았다. DROP·유실·ICMP 제한을 가를 수 없다 (04-01 §3).
		r.Status, r.Errno = StatusFiltered, errTimeout
	default:
		// 스캐너 쪽 자원 문제(ephemeral port 고갈·FD 한도 등)는 대상의 판정이 아니다.
		r.Status, r.Errno = StatusUnknown, errnoName(err)
	}
	return r
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// errnoName 은 실험 4 에서 만날 만한 errno 를 이름으로 돌려준다. 모르는 것은 에러 문장 그대로다.
func errnoName(err error) string {
	names := map[syscall.Errno]string{
		syscall.EADDRNOTAVAIL: "EADDRNOTAVAIL",
		syscall.EMFILE:        "EMFILE",
		syscall.ENETUNREACH:   "ENETUNREACH",
		syscall.ECONNRESET:    "ECONNRESET",
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if n, ok := names[errno]; ok {
			return n
		}
	}
	return err.Error()
}
