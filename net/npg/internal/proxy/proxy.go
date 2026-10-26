package proxy

import (
	"expvar"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog"
)

// New 는 모든 요청을 upstream 하나로 넘기는 프록시와, /debug/vars 에 내보낼 카운터를 돌려준다.
// 조립 순서가 곧 흐름이다: 바깥 accessLog[1·5] → 안쪽 ReverseProxy(Rewrite[2] → Transport[3] → 응답[4a]/ErrorHandler[4b]).
func New(upstream *url.URL, log zerolog.Logger) (http.Handler, expvar.Var) {
	m := newMetrics()
	rp := &httputil.ReverseProxy{
		// [3] 연결 층. v0 는 Go 기본값(풀·타임아웃 기본) 그대로 쓴다.
		// 기한·풀 크기는 ADR 0012 에서 internal/upstream 으로 옮긴다.
		Transport: http.DefaultTransport,

		// [2] 나가는 요청(pr.Out)을 고친다. 들어온 요청(pr.In)은 건드리지 않는다.
		// Rewrite 가 불리기 전에 hop-by-hop 헤더와 손님의 X-Forwarded-* 는 이미 지워져 있다.
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)          // 주소와 Host 를 업스트림 것으로 (nginx proxy_set_header Host $proxy_host 와 같음)
			pr.SetXForwarded()           // 진짜 손님 IP·Host·Proto 를 새로 적는다
			pr.Out = m.traceConn(pr.Out) // [3] 에서 연결 재사용 여부를 기록장에 적을 훅
		},

		// [4b] 업스트림 요청이 실패하면 불린다. 기본 핸들러는 무조건 502 라서 직접 고른다.
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			reason := classify(r.Context(), err) // 에러 모양 → UT·UF·UC·DC
			if info := infoFrom(r.Context()); info != nil {
				info.reason = reason // [5] 접근 로그의 flags 로 간다
			}

			log.Warn().Err(err).Str("flags", string(reason)).Msg("upstream error")
			w.WriteHeader(reason.Status())
		},
	}
	// accessLog 로 감싸 손님 요청이 가장 먼저[1], 가장 나중에[5] 지나가게 한다.
	return m.accessLog(log, rp), &m.vars
}
