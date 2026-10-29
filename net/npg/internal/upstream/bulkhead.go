package upstream

import "net/http"

// split 은 요청마다 검색 풀과 나머지 풀 중 하나를 골라 보내는 격벽이다 (ADR 0012 결정 2).
// 느린 검색이 자기 풀 칸(searchConns)을 다 써도 나머지 경로는 다른 풀로 지나간다 (I1).
type split struct {
	search http.RoundTripper
	rest   http.RoundTripper
}

// RoundTrip 이 있으므로 split 은 http.RoundTripper 다. 고르기만 하고 보내는 일은 고른 풀이 한다.
func (s split) RoundTrip(r *http.Request) (*http.Response, error) {
	if isSearch(r) {
		return s.search.RoundTrip(r)
	}
	return s.rest.RoundTrip(r)
}

// isSearch 는 장비 검색(GET /v1/devices?q=…)인지 본다. 목록 조회(q 없음)와 단건 조회는 나머지 풀로 간다.
// r 은 나가는 요청이다. 업스트림 주소에 경로가 없어 SetURL 뒤에도 손님이 보낸 경로가 그대로 남는다.
func isSearch(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		r.URL.Path == "/v1/devices" &&
		r.URL.Query().Has("q")
}

// New 는 npgproxy 가 ReverseProxy.Transport 에 넣을 업스트림 연결 층을 만든다.
// 바깥에는 RoundTripper 하나로 보이고, 풀이 둘이라는 사실은 이 패키지 안에 둔다.
// 풀마다 limit 이 칸 + 줄을 넘는 요청을 보내지 않고 ErrOverflow 로 돌려보낸다 (ADR 0012 결정 6).
func New() http.RoundTripper {
	return split{
		search: newLimit(newTransport(searchConns), searchConns, searchPending),
		rest:   newLimit(newTransport(restConns), restConns, restPending),
	}
}
