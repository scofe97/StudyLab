package proxy

import (
	"net/http"
	"net/http/httptrace"
)

// [2→3] traceConn 은 나가는 요청에 연결 추적 훅을 달아 돌려준다.
// 훅은 [3] Transport 가 연결을 손에 쥔 순간(GotConn) 불려, 새 연결인지 풀에서 꺼낸 것인지 센다.
func (m *metrics) traceConn(out *http.Request) *http.Request {
	info := infoFrom(out.Context()) // 훅 안에서 쓰려고 미리 꺼내 둔다
	trace := &httptrace.ClientTrace{
		GotConn: func(c httptrace.GotConnInfo) {
			if c.Reused {
				m.connsReused.Add(1)
			} else {
				m.connsNew.Add(1)
			}
			// 기록장이 있으면 [5] 접근 로그에 나갈 reused·idle 을 적는다
			if info != nil {
				info.reused = c.Reused
				info.idle = c.IdleTime
			}
		},
	}
	return out.WithContext(httptrace.WithClientTrace(out.Context(), trace))
}
