package proxy

import "expvar"

// metrics 는 프록시 하나의 카운터 묶음이다. vars 하나로 묶어 /debug/vars 에 "npgproxy" 한 칸으로 내보낸다.
// [1·5] accessLog 가 requests·inflight·reasons 를, [3] traceConn 이 conns_* 를 올린다.
type metrics struct {
	vars        expvar.Map
	requests    expvar.Int // 받은 요청 수
	inflight    expvar.Int // 지금 업스트림에 나가 있는 요청 수
	connsNew    expvar.Int // 업스트림에 새로 맺은 연결 수
	connsReused expvar.Int // 풀에서 꺼내 다시 쓴 연결 수
	reasons     expvar.Map // 원인 코드별 실패 수 (UO·UT·UF·UC·DC)
}

// newMetrics 는 카운터를 만들고 vars 에 이름을 붙여 건다.
// 필드는 struct 안에 두고 vars 는 그 주소만 들고 있으므로, m.requests.Add 가 곧 vars 값을 바꾼다.
func newMetrics() *metrics {
	m := &metrics{}
	m.vars.Set("requests", &m.requests)
	m.vars.Set("inflight", &m.inflight)
	m.vars.Set("conns_new", &m.connsNew)
	m.vars.Set("conns_reused", &m.connsReused)
	m.vars.Set("reasons", &m.reasons)
	return m
}
