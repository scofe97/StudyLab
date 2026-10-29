# 0012. 업스트림은 전용 Transport 로 보내고, 요청 전체 15초·맺기 5초·빈 연결 4초 기한과 검색 8칸/나머지 24칸 격벽을 둔다

- 상태: 채택 (2026-10-28 I1 재확인 뒤 결정 6 보강)
- 날짜: 2026-10-28
- 장: ch08

## 맥락

v0 프록시는 Go `http.DefaultTransport` 를 그대로 썼다(ADR 0011). 장애 리허설 세 장면이 그 기본값의 서로 다른 얼굴을 보였다.

| 장면 | 신호 | 막힌 기본값 |
|---|---|---|
| I1 검색 하나가 단건 조회까지 멈춤 (`ch08-slow-search-starves-lookup.md`) | get p99 30s(손님 기한), 프록시 DC 3,300·UT 0, 백엔드 연결 2,307·대기 2,273 | 응답 기한 없음(`ResponseHeaderTimeout` 0), 동시 연결 상한 없음(`MaxConnsPerHost` 0) |
| I3 부하 시험만 하면 포트가 바닥남 (`ch08-port-exhaustion-time-wait.md`) | 새 연결 1,364/초, TIME_WAIT 28,213, UF 502 | 빈 연결 호스트당 2개(`MaxIdleConnsPerHost` 0 → 2) |
| I2 한산할 때 POST 만 502 (`ch08-server-idle-close-post-502.md`) | POST UC 56/1,207, GET 0, 502 의 idle 807~973ms | 빈 연결 보관 90초(`IdleConnTimeout`)가 백엔드 5초보다 김 |

그 밖에 DefaultTransport 의 `Proxy: ProxyFromEnvironment` 는 `HTTP_PROXY` 환경 변수가 있으면 업스트림 요청을 그 프록시로 돌린다(루프백 제외). DefaultTransport 는 프로세스 공용 객체라 필드를 고치면 같은 프로세스의 다른 HTTP 클라이언트까지 바뀐다.

기한은 두 규칙을 따른다(I1·I2 에서 학습자가 정리).

- 응답 기한은 바깥이 길고 안이 짧다: 손님(npgload 30s) > 프록시 > 백엔드.
- 유휴 기한은 구간마다 연결을 다시 쓰는 쪽(보내는 쪽)이 짧고 받는 쪽이 길다: 프록시 Transport < 백엔드 `http.Server.IdleTimeout`.

같은 문제를 푼 구현(원문 확인, 수집 `.omc/research/adr0012-sources.md`).

| 자리 | envoy v1.40.2 | nginx 1.31.8 (문서, 소스 release-1.31.8) | haproxy 3.4.0 | Go 1.25.1 DefaultTransport |
|---|---|---|---|---|
| 응답 기한 | route `timeout` 15s, 요청 전체 ("spans between … the entire downstream request … has been processed and when the upstream response has been completely processed") | `proxy_read_timeout` 60s, 읽기 사이 간격 ("only between two successive read operations") | `timeout server` 없음 ("An unspecified timeout results in an infinite timeout, which is not recommended.") | `ResponseHeaderTimeout` 0, 헤더까지만 |
| 맺기 기한 | `connect_timeout` 5s | `proxy_connect_timeout` 60s ("cannot usually exceed 75 seconds") | `timeout connect` 없음 | Dialer `Timeout` 30s |
| 빈 연결 보관 | `idle_timeout` 1시간 | upstream `keepalive_timeout` 60s | `pool-purge-delay` 5s (5초마다 절반을 닫음) | `IdleConnTimeout` 90s |
| 동시 연결 상한 | `max_connections` 1024 (클러스터 전체), 대기 `max_pending_requests` 1024, 넘치면 503 UO | `server max_conns=0` (무제한), 대기열 `queue` 는 상용판 | `server maxconn` 0, 대기 `maxqueue`·`timeout queue` | `MaxConnsPerHost` 0, 넘치면 `connsPerHostWait` 에서 기다림 |
| 남길 빈 연결 | 필드 없음 | `keepalive 32` (워커마다, 넘치면 오래된 것부터 닫음, 1.29.7 부터 기본 켬) | `pool-max-conn` -1 (무제한) | `MaxIdleConnsPerHost` 0 → 2 |

서버 쪽 빈 연결 기본값은 Node.js `server.keepAliveTimeout` 5s(v26.12.1 문서), Apache httpd `KeepAliveTimeout` 5s(2.4), Tomcat `keepAliveTimeout` = `connectionTimeout` 60s(배포판 `server.xml` 20s, 10.1), nginx `keepalive_timeout` 75s 다. AWS ALB 문서는 "configure the idle timeout of your application to be larger than the idle timeout configured for the load balancer" 라고 권한다. envoy 가 1시간을 고른 이유는 원문에서 찾지 못했다. Node.js 5초 백엔드 앞의 envoy 에서 503 UC 가 나 `idle_timeout` 을 4초로 낮춘 사례가 Istio 포럼에 있다.

## 선택지

판단을 다섯 자리로 나눠 각각 골랐다. 6번은 I1 재확인 뒤 보강했다.

### 1. 응답 기한

- A. `ResponseHeaderTimeout` 하나: 필드 하나, 스트리밍 본문을 자르지 않음. 헤더 뒤 본문을 끄는 백엔드는 못 막음.
- B. 경로별 기한: 경로마다 알맞은 값. 경로 표와 감싸개 코드가 늘어남.
- C. envoy 처럼 요청 전체 기한: 본문까지 포함해 최대 N초가 보장됨. 큰 응답이나 스트리밍도 잘림.

### 2. 연결 수

- A. 풀 하나에 상한 N, 빈 연결 N: 필드 둘. I3 닫힘을 없애지만 느린 검색이 N 칸을 다 차지하면 단건 조회도 막힌다(줄이 백엔드에서 프록시로 옮겨 올 뿐).
- B. A + 격벽(검색만 작은 풀): 느린 경로가 자기 몫만 쓴다. Transport 둘과 경로 고르기 코드가 듦. 백엔드 창구 수가 바뀌면 같이 고쳐야 함.
- C. 상한 그대로, 빈 연결 수만 키움: I3 만 고침. I1 의 연결 폭주는 그대로.

### 3. 빈 연결 보관 시간

- A. 흔한 서버 기본값(5초)보다 짧은 표준 값 하나: I2 경합의 출발점을 없앰. 백엔드마다 맞추지 않아도 됨. 한산할 때 연결을 조금 더 자주 맺음.
- B. 90초 그대로, 재시도로 덮기(ADR 0013): Go 는 POST 를 다시 보내지 않으니 POST 의 I2 가 남는다. 잘못 덮으면 I5(두 번 등록).
- C. 재사용 끄기(`DisableKeepAlives`): I2 는 사라지나 I3 이 돌아옴.

### 4. 맺기 기한

- A. 따로 짧게(5초): 닿지 않는 백엔드를 5초에 UF 로 알림.
- B. 30초 그대로: 요청 기한 15초가 덮지만 실패 사유가 UF 가 아니라 UT 로 섞임.
- C. 아주 짧게(1초): SYN 재전송 한 번에 걸릴 수 있음.

### 5. 환경 변수 프록시

- A. `Proxy: nil`: `-upstream` 주소로만 간다. 띄운 환경에 따라 경로가 바뀌지 않음.
- B. `ProxyFromEnvironment` 유지: 사내망 포워드 프록시를 꼭 거쳐야 할 때만 의미. 다른 목적으로 깔린 변수가 경로를 몰래 바꿈.

### 6. 칸이 다 찼을 때 프록시 안 줄 (보강)

결정 1~5 로 I1 을 다시 돌리자 검색은 15초에 UT 로 끝났지만 단건 조회도 성공 0 이었다. 백엔드에는 처리 중이 최대 2,433(일하는 32 + 줄 2,401)까지 쌓였다. 프록시가 묶는 것은 연결이고, 백엔드가 아끼는 것은 처리 창구 32다. 줄에서 오래 기다린 검색은 남은 기한이 백엔드 8초보다 짧다. 그래서 백엔드가 일하는 도중에 프록시가 UT 로 떠나고 같은 칸에 새 요청을 보낸다. 백엔드는 떠난 요청도 끝까지 처리해(떠난 뒤 작업 초당 약 41) 줄이 초당 약 13씩 길어졌고, 같은 창구를 쓰는 단건 조회까지 밀렸다(백엔드는 `honor_cancel=false`).

- A. 줄 없음(nginx 오픈소스): 칸이 다 차면 바로 실패. 떠난 작업이 생기지 않지만 순간 몰림도 거절한다.
- B. 줄 길이 상한(envoy `max_pending_requests` 기본 1024, 넘치면 줄에 넣지 않고 503 `UO`): 몰림을 줄 길이만큼 흡수한다. 기다림 상한은 요청 전체 기한뿐이다.
- C. 기다림 시간 상한(haproxy `timeout queue`, 기본 = `timeout connect`, 넘기면 "almost never be served" 로 보고 503): 기다린 시간을 직접 잰다. 줄 길이(`maxqueue`)는 기본 무제한.

## 결정

| 자리 | 고른 것 | 값 |
|---|---|---|
| 1. 응답 기한 | C. 요청 전체 기한 | 15초. 프록시 앞 감싸개가 요청 context 에 `context.WithTimeout` 을 건다. 손님 30초보다 짧아 손님이 떠나기 전에 UT(504)가 난다 |
| 2. 연결 수 | B. 격벽 | 검색(`GET /v1/devices?q=…`) Transport 상한 8, 그 밖의 경로 Transport 상한 24. 백엔드 창구 32 기준. 각 풀의 `MaxIdleConnsPerHost` 는 상한과 같게 둬 끝난 연결을 닫지 않는다 |
| 3. 빈 연결 보관 | A. 표준 값 하나 | 4초 (Node.js·Apache·우리 백엔드의 5초보다 짧게, Istio 사례 값) |
| 4. 맺기 기한 | A. 따로 짧게 | 5초 (envoy 기본값). TCP keepalive 탐침은 30초 그대로 |
| 5. 환경 변수 프록시 | A. `Proxy: nil` | 중간 프록시가 필요해지면 플래그로 명시적으로 켠다 |
| 6. 프록시 안 줄 | B. 줄 길이 상한 | 검색 7, 나머지 1024. 칸 + 줄이 다 차면 Transport 에 넘기지 않고 503, 원인 `UO`. 검색 7 은 셈: 처리량 8칸 ÷ 8초 = 초당 1건이라 k번째 줄은 약 k초를 기다리고, k + 8 ≤ 15 의 경계다. 나머지 1024 는 envoy 기본값 |

나머지 필드(`ExpectContinueTimeout` 1s, `TLSHandshakeTimeout` 10s)는 Go 기본값을 따른다. 업스트림이 http 라 TLS 기한은 지금 쓰이지 않는다. 두 Transport 는 `internal/upstream` 에 두고, DefaultTransport 공용 객체는 건드리지 않는다.

기한이 끝나면 에러는 `context.DeadlineExceeded` 이고 `classify` 가 UT 로 고른다. 손님이 먼저 떠나면 부모 context 가 `Canceled` 라 DC 로 남는다. 맺기 5초 초과는 `dial` 단계 `net.OpError` 라 UF 다.

## 결과

- 쉬워지는 것
  - I1: 프록시가 손님보다 먼저 포기해 원인이 UT 로 보인다. 검색은 8칸 안에서만 백엔드를 쓰고, 단건 조회는 나머지 24칸으로 지나간다.
  - I3: 동시 연결이 상한에 묶이고 끝난 연결이 모두 풀로 돌아가 TIME_WAIT 가 쌓이지 않는다.
  - I2: 프록시가 백엔드보다 먼저 빈 연결을 닫아 경합이 생기지 않는다.
- 어려워지는 것
  - 상한을 넘는 요청은 프록시 안 `connsPerHostWait` 에서 기다린다. 줄이 다 차면 503 UO 로 거절한다(결정 6). 느린 검색은 처리량(초당 1건)을 넘는 만큼 거절되고, 성공이 늘지는 않는다.
  - 검색 줄 7 은 백엔드 검색 8초와 칸 8 에 묶인 셈이다. 재확인(2026-10-29)에서 경계값의 값을 쟀다: 검색이 처리량을 계속 넘으면 줄은 늘 꽉 차고, 칸을 얻는 검색은 늘 약 7초를 기다린 것이라 7 + 8 ≈ 15초. 검색 200 은 처리량 초당 1건의 절반(0.53)이고 나머지 0.48 은 UT 로 끝나 백엔드 일이 헛일이 됐다. 단건 조회는 초당 50건 모두 200(p99 최대 119ms), 백엔드 처리 중 최대 13·대기 0 으로 결정 6 의 목적은 이뤘다. 8초 검색은 고장 때뿐이고 평소 30ms 에서는 줄이 순간 몰림만 받아 주므로 7 을 유지한다. 넘침이 계속될 때 줄 길이는 곧 기다림 시간이다(리틀 법칙).
  - 줄 세기는 응답 본문을 닫을 때 칸을 돌려준다. 본문을 감싸므로 101 Switching Protocols(업그레이드) 응답은 이 층을 지나지 못한다. 지금은 업그레이드를 쓰지 않는다.
  - 칸 수(8/24)는 백엔드 창구 32 에 묶인다. 백엔드가 바뀌면 같이 고쳐야 한다.
  - 요청 전체 기한은 본문 복사 중에도 끊는다. 헤더를 이미 보낸 뒤 끊기면 `ErrorHandler` 를 거치지 않고 `panic(http.ErrAbortHandler)` 로 연결을 끊으므로(Go 1.25.1 `httputil/reverseproxy.go:533-543`) 접근 로그의 flags 가 UT 로 남지 않는다. 지금 응답은 작은 JSON 이라 드물다.
  - 백엔드가 4초보다 짧은 유휴 기한을 쓰면 I2 가 돌아온다. 그 업스트림만 따로 값을 줘야 한다.
- 학습용으로 단순화한 것(프로덕션이면 더 필요한 것)
  - 경로별 기한과 칸 수를 설정 파일로 빼기.
  - 백엔드가 떠난 손님의 요청을 버리기(`r.Context()` 취소 존중)와 백엔드 쪽 줄 상한. 프록시 줄 상한(결정 6)은 프록시가 떠나는 일을 줄일 뿐, 백엔드 줄을 직접 묶지는 않는다.
  - 백엔드 여러 대와 로드 밸런싱. 지금은 업스트림이 하나다.
  - 프록시 손님 쪽 서버의 `IdleTimeout`. 지금은 없어 쉬는 손님 연결을 무한히 붙잡는다(`cmd/npgproxy/main.go:54`). 8장 범위(업스트림 층) 밖이라 알려진 한계로 둔다.
- 다시 볼 조건
  - I1·I3·I2 재확인에서 숫자가 예측과 다르면.
  - 재시도를 넣는 ADR 0013 에서 15초 기한을 시도마다 나눌지 정할 때.

## 관련 노트

- 08-02.Go HTTP 클라이언트는 응답 본문을 닫아야 연결을 재사용하고 context 로 요청을 끊는다.md
