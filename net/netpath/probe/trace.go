package probe

import (
	"net/http/httptrace"
	"slices"
	"sync"
	"time"
)

// dial 타임아웃 시 ConnectDone 이 안 불린다(실측) — 그래서 duration 대신 시각 둘을 담는다.
type stageSpan struct {
	start time.Time
	end   time.Time
	err   error
}

func (s stageSpan) started() bool  { return !s.start.IsZero() }
func (s stageSpan) finished() bool { return !s.end.IsZero() }

// 시작만 하고 안 끝난 단계 — 에러의 진짜 주인이다.
func (s stageSpan) hung() bool { return s.started() && !s.finished() }

func (s stageSpan) duration() time.Duration {
	if !s.started() || !s.finished() {
		return 0
	}
	return s.end.Sub(s.start)
}

type traceData struct {
	dns  stageSpan
	conn stageSpan
	// GotConn 부터 첫 바이트까지 — 에러가 없어 조립 시 채운다.
	http stageSpan

	addresses  []string
	remoteAddr string
	localAddr  string
	reused     bool
	// ConnectStart 호출 횟수 — DNS 가 주소를 여럿 주면 차례로 시도한다.
	connAttempts int
}

// 훅은 임의 goroutine 에서, 일부는 요청 종료 후에도 불린다(표준 라이브러리 문서) — mutex 로 보호, go test -race 로 검사.
type tracer struct {
	mu sync.Mutex
	d  traceData
}

func (t *tracer) snapshot() traceData {
	t.mu.Lock()
	defer t.mu.Unlock()
	d := t.d
	d.addresses = slices.Clone(t.d.addresses) // 그냥 대입하면 배열을 공유한다
	return d
}

func (t *tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.d.dns.start = time.Now()
		},

		DNSDone: func(info httptrace.DNSDoneInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.d.dns.end = time.Now()
			t.d.dns.err = info.Err
			for _, a := range info.Addrs {
				t.d.addresses = append(t.d.addresses, a.IP.String())
			}
		},

		ConnectStart: func(string, string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.d.connAttempts++
			// 여러 주소를 시도해도 첫 시도 시각만 남긴다 — 전부가 한 연결 시간이다.
			if !t.d.conn.started() {
				t.d.conn.start = time.Now()
			}
		},

		ConnectDone: func(_, addr string, err error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			// 마지막 결과가 결론. 앞이 실패하고 뒤가 성공하면 nil 로 덮인다.
			t.d.conn.end = time.Now()
			t.d.conn.err = err
			if err == nil {
				t.d.remoteAddr = addr
			}
		},

		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.d.http.start = time.Now()
			t.d.reused = info.Reused
			if info.Conn == nil {
				return
			}
			if a := info.Conn.LocalAddr(); a != nil {
				t.d.localAddr = a.String()
			}
			// 재사용된 연결은 ConnectDone 이 안 불려 remoteAddr 이 비어 있다.
			if a := info.Conn.RemoteAddr(); a != nil && t.d.remoteAddr == "" {
				t.d.remoteAddr = a.String()
			}
		},

		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.d.http.end = time.Now()
		},
	}
}
