package proxy

import (
	"context"
	"expvar"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/rs/zerolog"
	"github.com/scofe97/npg-lab/internal/upstream"
)

// New 는 모든 요청을 upstream 하나로 넘기는 프록시와, /debug/vars 에 내보낼 카운터를 돌려준다.
// 조립 순서가 곧 흐름이다: 바깥 accessLog[1·5] → withDeadline → 안쪽 ReverseProxy(Rewrite[2] → Transport[3] → 응답[4a]/ErrorHandler[4b]).
func New(target *url.URL, log zerolog.Logger) (http.Handler, expvar.Var) {
	m := newMetrics()

	rp := &httputil.ReverseProxy{
		// [3] 연결 층. 검색 8칸+줄 7·나머지 24칸+줄 1024 두 풀, 맺기 5초, 빈 연결 4초 (ADR 0012, internal/upstream).
		// v0 의 http.DefaultTransport 는 상한·응답 기한이 없고 빈 연결을 2개만 남겼다 (I1·I3·I2).
		Transport: upstream.New(),

		// [2] 나가는 요청(pr.Out)을 고친다. 들어온 요청(pr.In)은 건드리지 않는다.
		// Rewrite 가 불리기 전에 hop-by-hop 헤더와 손님의 X-Forwarded-* 는 이미 지워져 있다.
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)            // 주소와 Host 를 업스트림 것으로 (nginx proxy_set_header Host $proxy_host 와 같음)
			pr.SetXForwarded()           // 진짜 손님 IP·Host·Proto 를 새로 적는다
			pr.Out = m.traceConn(pr.Out) // [3] 에서 연결 재사용 여부를 기록장에 적을 훅
		},

		// [4b] 업스트림 요청이 실패하면 불린다. 기본 핸들러는 무조건 502 라서 직접 고른다.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			reason := classify(r.Context(), err) // 에러 모양 → UO·UT·UF·UC·DC
			if info := infoFrom(r.Context()); info != nil {
				info.reason = reason // [5] 접근 로그의 flags 로 간다
			}

			log.Warn().Err(err).Str("flags", string(reason)).Msg("upstream error")
			w.WriteHeader(reason.Status())
		},
	}
	// accessLog 로 감싸 손님 요청이 가장 먼저[1], 가장 나중에[5] 지나가게 한다.
	// 기한은 그 안쪽에 건다. 그래야 기한에 걸린 요청도 accessLog 가 UT 로 한 줄 남긴다.
	return m.accessLog(log, withDeadline(upstream.RequestTimeout, rp)), &m.vars
}

// withDeadline 은 요청 전체에 기한 d 를 건다 (ADR 0012 결정 1, envoy route timeout 과 같은 뜻).
// 풀 대기·연결 맺기·응답 헤더·본문 복사가 모두 d 안에 들어야 한다. 넘기면 Transport 가 DeadlineExceeded 로
// 멈추고 classify 가 UT(504)를 고른다. 손님이 먼저 떠나면 부모 context 의 Canceled 가 그대로 내려와 DC 로 남는다.
func withDeadline(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d) // 부모는 손님 요청 context. accessLog 의 기록장도 물려받는다
		defer cancel()                                     // 일찍 끝나면 타이머를 바로 치운다
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
