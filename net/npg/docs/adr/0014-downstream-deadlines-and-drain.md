# 0014. 손님 쪽 기한은 읽기·쓰기 틈 시계와 최소 속도로 두고, 끌 때는 20초 기다린 뒤 다 못 끝내면 exit 1 로 알린다

- 상태: 채택
- 날짜: 2026-11-01
- 장: ch09

## 맥락

8장(ADR 0012)은 프록시↔서버 구간에 기한을 두었다. 손님↔프록시 구간과 프록시를 끌 때는 기한이 없거나 맞지 않았다. 장애 리허설 세 장면이 각각 다른 빈칸을 보였다.

| 장면 | 신호 | 원인 |
|---|---|---|
| J1 등록 몰림 시간대에 조회까지 느려짐 | 느린 POST(2초에 1바이트)가 자리를 오래 붙잡음. 핫픽스로 `ReadTimeout` 10초를 넣자 끊기기는 했으나 85건 모두 `499 DC`(손님 이탈)로 찍힘 | 몸통 읽기 기한이 없었다. 넣은 뒤에는 net/http 가 읽기 오류마다 연결 context 를 취소해서(`connReader` → `cancelCtx`) 기한 초과도 이탈처럼 보였다 |
| J2 느린 검색이 504 대신 연결 끊김으로 보임 | 프록시 로그는 `504 UT dur=15000 bytes=0`, 손님은 15.001초에 EOF | 핫픽스의 `WriteTimeout` 15초가 업스트림 기한 15초와 같았다. 쓰기 시계는 손님 헤더를 읽은 직후 시작하므로(server.go 1029-1031) 504 를 쓰려는 순간 이미 지나 있었다 |
| J4 배포 때마다 등록이 끊기고 로그에 없음 | 배포 두 번에 백엔드 등록 758, 프록시 로그·손님 성공 751, 손님 EOF 7. 프록시는 exit 0 | 드레인 2초가 업스트림 기한 15초보다 짧았다. `Shutdown` 이 돌려준 기한 초과 오류를 버리고(`_ =`) 정상 종료했다 |

기한은 ADR 0012 의 규칙을 그대로 따른다. 바깥 시계가 안쪽 시계보다 길어야 안쪽에서 난 결과를 바깥까지 전할 수 있다.

```
손님 앱 30초 > 드레인 > (손님 쪽 쓰기) > 업스트림 요청 15초 > 실제 작업
```

같은 문제를 푼 구현(원문 위치는 아래 「원문 근거」).

| 자리 | nginx 1.31.8 | envoy v1.40.2 | haproxy 3.4.0 | caddy v2.11.7 |
|---|---|---|---|---|
| 몸통 읽기 | `client_body_timeout` 60s, 읽기 사이 간격, 408 | `request_timeout` 꺼짐, `stream_idle_timeout` 5m | `timeout http-request` (몸통은 `option http-buffer-request` 일 때만), 408 | `read_timeout` 없음, `read_idle_timeout` 1m, `read_min_rate` 0 |
| 느림 vs 떠남 표기 | 408 vs 499 | `SI` vs `DC` | 소문자 `cR`(기한) vs 대문자 `CR`(손님이 끊음) | 없음 |
| 응답 쓰기 | `send_timeout` 60s, 쓰기 사이 간격 | 손님 쪽 쓰기 기한 따로 없음 | `timeout client` | `write_timeout` 없음, `write_idle_timeout` 1m |
| 끌 때 | QUIT 은 정상 종료, `worker_shutdown_timeout` 0(상한 없음) | `--drain-time-s` 600, HCM `drain_timeout` 5s | `hard-stop-after` 상한 없음 | `grace_period` 0 = 끝까지 기다림, `shutdown_delay` |

쿠버네티스는 SIGTERM 뒤 `terminationGracePeriodSeconds` 기본 30초가 지나면 강제로 죽인다. Go 1.25.1 `Server.Shutdown` 은 context 기한이 지나면 `ctx.Err()` 를 돌려준다(server.go L3159-3178).

## 선택지

### 1. 몸통 읽기 (J1)

- A. 전체 시계 하나(`ReadTimeout`): 코드가 가장 적다. 정상인데 큰 업로드도 잘린다.
- B. 틈 시계(읽기 사이 N초): 큰 업로드를 살린다. 2초마다 1바이트씩 흘리는 손님(slow POST 공격)은 틈이 짧아 못 잡는다.
- C. 틈 시계 + 최소 속도: 멈춘 손님과 흘리는 손님을 둘 다 잡는다. 코드가 가장 많다.

어느 쪽이든 끊긴 이유는 "느린 손님(408)"과 "떠난 손님(499)"으로 나눠 기록한다.

### 2. 응답 쓰기 (J2)

- A. `WriteTimeout` 을 업스트림 기한보다 길게(예: 20초): 한 줄 수정. 업스트림 기한을 바꾸면 같이 바꿔야 한다.
- B. `WriteTimeout` 을 끄고 쓰기 틈 시계: 업스트림을 기다리는 동안은 시계가 돌지 않아 두 기한이 엮이지 않는다. 코드가 는다.
- C. `WriteTimeout` 을 끄고 업스트림 기한에 맡김: 가장 단순하다. 손님 쪽 쓰기를 따로 지키는 장치가 없다.

### 3. 끌 때 기다리는 시간 (J4)

- A. 고정값(업스트림 15초보다 길고 쿠버네티스 30초보다 짧게): 예측하기 쉽다. 업스트림 기한과 같이 관리해야 한다.
- B. 끝까지 기다림: 요청을 끊지 않는다. 실제 상한은 쿠버네티스가 30초에 강제로 죽이는 시점이 되고, 그때는 아무 기록도 남지 않는다.

어느 쪽이든 기다림이 시간 안에 끝나지 않으면 그 사실을 로그에 남기고 exit 1 로 끝낸다.

## 결정

**1은 C 를 고른다.** 학습자의 말: "두 가지를 모두 해결할 수 있기 때문에 선택했다. 잡을 수 있으면 둘 다 잡는 게 낫다." 읽기 틈 10초, 최소 속도 초당 1KB 로 둔다. 몸통을 읽기 시작하고 10초가 지난 뒤부터 평균 속도를 잰다. 그래서 작은 JSON 은 영향을 받지 않고, J1 의 손님(초당 0.5바이트)은 10초 안팎에서 끊긴다. 몸통 읽기는 `r.Body` 를 감싼 마디를 거치게 하고, 그 마디가 읽을 때마다 `http.ResponseController.SetReadDeadline` 으로 틈 시계를 다시 건다. 같은 마디가 처음 난 읽기 오류를 기록한다. 기한 초과(`errors.Is(err, os.ErrDeadlineExceeded)`)나 최소 속도 미달이면 408 과 새 flag `CT`(client timeout)로, 그 밖의 읽기 오류는 지금처럼 `DC` 로 남긴다. 핫픽스의 `ReadTimeout` 10초는 뺀다. `ReadHeaderTimeout` 2초는 그대로 둔다.

**2는 B 를 고른다.** 학습자의 말: "모든 걸 해결해 주는 선택지는 없다. 코드가 늘어나더라도 문제를 해결해 주는 쪽이 가장 낫다고 본다. 코드가 좀 복잡해도 관리할 수 있는 만큼 관리하는 게 좋다." 핫픽스의 `WriteTimeout` 15초는 빼고, 쓰기 틈 10초를 둔다. 응답을 쓰는 마디가 쓸 때마다 `SetWriteDeadline` 으로 시계를 다시 건다.

**3은 A 를 고른다.** 학습자의 말: "예측하기 쉬운 게 가장 무난하다. B 를 하더라도 완벽한 해결책은 아니다." 드레인은 20초로 둔다(업스트림 15초 < 20초 < 쿠버네티스 30초). `Shutdown` 이 오류를 돌려주면 그때 처리 중이던 요청 수(`inflight`)와 함께 로그를 남기고 exit 1 로 끝낸다.

`http.TimeoutHandler` 는 쓰지 않는다. 핸들러 전체에 시계 하나를 거는 방식이라 결정 1·2 의 틈 시계가 되지 못하고, 응답을 다 모았다가 보내서 스트리밍을 막으며, 기한에 걸리면 503 으로 답해 느린 손님과 느린 백엔드를 구분하지 못한다.

## 결과

- 손님 쪽 서버 설정은 함수 하나(`newFrontServer`)로 모아 `httptest` 로 시험한다. 시험은 세 장면을 하나씩 다시 만든다. 느리게 흘리는 몸통은 408·`CT`, 도중에 끊은 손님은 `DC`, 업스트림 기한을 넘긴 응답은 손님에게 504 로 도착, 드레인 기한을 넘기면 `Shutdown` 오류를 알린다.
- 마지막 재검사에서 J1 의 느린 POST 는 10초 안팎에 408 `CT` 로, J2 의 느린 검색은 15초에 504 로, J4 의 배포는 끊긴 등록 0건으로 바뀌어야 한다.
- 업스트림 기한(ADR 0012)을 늘리면 드레인 20초도 다시 본다. 드레인은 늘 업스트림 기한보다 길어야 한다.
- 알려진 한계: 프록시가 하나뿐이라 꺼진 동안의 `refused` 는 그대로 남는다. caddy `shutdown_delay` 처럼 부하 분산기가 이 서버를 빼 갈 시간을 주는 일은 다음 단계 후보로 둔다.
- 알려진 한계: 최소 속도는 평균으로 잰다. 처음에 빨리 보내고 나중에 멈추는 손님은 틈 시계가 잡는다.

## 원문 근거

2026-11-01 에 태그로 고정한 원문을 읽었다.

- nginx `release-1.31.8`: `src/http/ngx_http_core_module.c` L3936-3937(`client_body_timeout` 60s), L3977(`send_timeout` 60s), `src/http/ngx_http_request_body.c` L262·L282(408), `src/http/ngx_http_request.h` L138(499), `src/core/nginx.c` L1194·`src/core/ngx_cycle.c` L1437-1451(`worker_shutdown_timeout` 0 이면 상한 없음). 문서: nginx.org `ngx_http_core_module.html#client_body_timeout`·`#send_timeout`.
- envoy `v1.40.2`: `api/envoy/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto` L590-656(`stream_idle_timeout` 5m, `request_timeout` 꺼짐), `docs/root/configuration/advanced/substitution_formatter.rst` L567(DC)·L579(SI), `docs/root/operations/cli.rst` L297-306(`--drain-time-s` 600).
- haproxy `v3.4.0`: `doc/configuration.txt` L14936-14975(`timeout http-request`), L14739-14777(`timeout client`), L30112-30145(종료 flag `cR`·`CR`·`cH`), L2798-2815(`hard-stop-after`).
- caddy `v2.11.7`: `modules/caddyhttp/server.go` L61-111(`read_idle_timeout`·`read_min_rate`·`write_idle_timeout`), `modules/caddyhttp/app.go` L136-155(`grace_period`·`shutdown_delay`), L927-940(기본값).
- kubernetes `k8s.io/api` `v0.37.2`: `core/v1/types.go` L3987·L4411-4419(`terminationGracePeriodSeconds` 30).
- Go `go1.25.1`: `src/net/http/server.go` 1029-1031·3033-3038(`WriteTimeout` 시작 시점), L3159-3178(`Shutdown` 이 `ctx.Err()` 를 돌려줌).

## 관련 노트

- 09-01.Go HTTP 서버는 타임아웃을 정하고 핸들러에 의존성을 주입하며 httptest 로 시험한다.md
- 09-02.미들웨어는 핸들러를 감싸고 ServeMux 는 경로를 나누며 HTTP2 서버 푸시는 브라우저가 걷어냈다.md
