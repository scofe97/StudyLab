package upstream

import (
	"net"
	"net/http"
	"time"
)

// ADR 0012 에서 고른 값. 기한은 "바깥이 길고 안이 짧다", 유휴는 "보내는 쪽이 받는 쪽보다 짧다"를 따른다.
const (
	RequestTimeout = 15 * time.Second // 요청 전체 기한. 손님(npgload 30s)보다 짧아 손님보다 먼저 UT 가 난다 (I1)
	dialTimeout    = 5 * time.Second  // 연결 맺기 기한. 닿지 않는 백엔드를 UF 로 빨리 알린다 (envoy connect_timeout 기본값)
	idleTimeout    = 4 * time.Second  // 빈 연결 보관. 흔한 서버 기본값(Node.js·Apache·fakeinv 5s)보다 짧아 프록시가 먼저 닫는다 (I2)
	searchConns    = 8                // 검색 풀 칸 수. 느린 검색이 백엔드 창구를 이만큼만 쓴다 (I1 격벽)
	restConns      = 24               // 나머지 풀 칸 수. 백엔드 창구 32 = 8 + 24
	searchPending  = 7                // 검색 줄 길이. 처리량 8칸÷8초=초당 1건이라 k번째는 약 k초 기다림, k+8 ≤ 15 의 경계 (ADR 0012 결정 6)
	restPending    = 1024             // 나머지 줄 길이. envoy max_pending_requests 기본값
)

// newTransport 는 호스트당 동시 연결을 conns 개로 묶고, 끝난 연결도 conns 개까지 남기는 Transport 를 만든다.
// DefaultTransport 리터럴에서 바꾼 곳은 Proxy·Dialer Timeout·MaxConnsPerHost·MaxIdleConnsPerHost·IdleConnTimeout 다섯이다.
// MaxIdleConns(전 호스트 합계)는 0 = 무제한으로 둬 호스트당 값만 작동하게 한다.
func newTransport(conns int) *http.Transport {
	return &http.Transport{
		Proxy: nil, // HTTP_PROXY 같은 환경 변수를 보지 않고 -upstream 주소로만 간다
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second, // TCP keepalive 탐침 주기 (HTTP 연결 재사용과는 다른 것)
		}).DialContext,
		MaxConnsPerHost:       conns, // 동시에 여는 연결 상한. 넘는 요청은 연결이 빌 때까지 Transport 안에서 기다린다 (I1)
		MaxIdleConnsPerHost:   conns, // 끝난 뒤 풀에 남길 수. 상한과 같아 돌아온 연결을 닫지 않는다 (I3)
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   10 * time.Second, // Go 기본값. 업스트림이 http 라 지금은 쓰이지 않는다
		ExpectContinueTimeout: 1 * time.Second,  // Go 기본값. Expect: 100-continue 요청이 본문을 보내기 전 기다리는 시간
	}
}
