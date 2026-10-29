# ch07-10-service — 사내 API 서비스 한 벌

『Network Programming with Go』 7장(Unix 도메인 소켓)·8장(HTTP 클라이언트)·9장(HTTP 서비스)을 서비스 하나로 합친 실습입니다. 사내에서 「환율 조회 API」 하나를 운영한다고 보고, 외부 제공자가 느려지거나 끊기거나 배포가 겹칠 때 우리 서비스가 어떻게 보이는지를 장애 재현 형식으로 돌려 봅니다.

앞단(Caddy) 실습은 [`edge/README.md`](edge/README.md) 에 있습니다. 같은 api·upstream 을 Caddy 뒤에 두고 이어 갑니다.

## 이 프로젝트로 배우는 것

1. 업스트림 호출 하나에 걸리는 시간을 시도당 기한·전체 기한·`ResponseHeaderTimeout`·호스트당 연결 수 상한으로 나눠 묶고, 장애 때 어느 기한이 먼저 걸리는지 `httptrace` 로 짚을 수 있습니다.
2. 응답 본문을 다루는 방식과 응답 모양에 따라 `Transport` 가 연결을 다시 쓰는지를 `httptrace`·`ss`·fd 수로 가려낼 수 있습니다.
3. 재시도가 장애를 키우는 경로를 숫자로 재고, 백오프·지터·재시도 예산·멱등성 키로 막는 설정을 고르며, 요청을 다시 보내도 되는지 가를 때 멱등성이 어떤 기준이 되는지 설명할 수 있습니다.
4. 서버 타임아웃 네 필드, `http.TimeoutHandler`, 드레인 뒤 `Shutdown` 이 각각 연결과 요청의 어느 구간을 지키는지 재현으로 확인할 수 있습니다.
5. 신뢰하는 프록시 대역을 정해 `X-Forwarded-For` 에서 클라이언트 IP 를 계산하고, 위조로 차단을 우회하는 경로를 막을 수 있습니다.
6. ETag 재검증 캐시에서 신선도 판단과 재검증을 나눠 설명하고, 업스트림 호출 수와 전송 바이트가 어떻게 바뀌는지 잴 수 있습니다.
7. Unix 소켓 위에 관리 채널을 열고 파일 권한·`SO_PEERCRED` 인가·소켓 파일 생애·unixgram 준비 알림을 운영 절차로 다룰 수 있습니다.

| 시나리오 | 목표 | 노트 |
|---|---|---|
| S1 외부 API 가 느려지자 p99 가 같이 무너졌다 | 1 | 08-02 §3 |
| S2 업스트림 연결 수가 요청 수만큼 늘어난다 | 2 | 08-02 §2 |
| S3 장애가 끝났는데도 업스트림이 일어나지 못한다 | 3 | 08-01 §2·§3 ★ |
| S4 배포할 때마다 502 가 몇 건 난다 | 4 | 09-01 §2 |
| S5 연결 수가 한도에 붙었는데 요청은 거의 없다 | 4 | 09-01 §2 |
| S6 3초 넘는 요청은 503 인데 서버 로그는 200 | 4 | 09-02 §1·§2 |
| S7 IP 차단을 우회당했다 | 5 | 09-02 §1 |
| S8 업스트림 호출량을 줄여라 | 6 | 08-01 §3 ★ |
| S9 관리 소켓으로 아무나 서비스를 멈출 수 있었다 | 7 | 07-02 ★ |
| S10 재시작했더니 관리 소켓을 열지 못한다 | 7 | 07-01 §2 ★ |
| S11 감독 프로세스가 서비스 준비를 모른다 | 7 | 07-01 §4 |

★ 는 Phase 1 에서 예측이 빗나갔던 자리입니다. 시나리오마다 「실행 전에 적을 예측」 을 먼저 적고 돌립니다. 답은 이 문서와 코드 주석에 없습니다.

### 이미 해 본 것 — 여기서는 다시 하지 않습니다

- idle 연결·FD 고갈·goroutine 적체: [gonet-lab Phase 1](<~/runners-high/write/02_os/project/gonet-lab/01-01.조용한 연결이 서버를 멈춘다 - goroutine·FD·idle timeout.md>)
- accept 대기열 넘침·TIME-WAIT 누적과 재사용: [gonet-lab Phase 4](<~/runners-high/write/02_os/project/gonet-lab/04-01.무응답은 증거가 아니다 - CONNECT·SYN·UDP 포트 스캔.md>), [실습 기록](<~/runners-high/write/02_os/project/gonet-lab/04-02.포트 스캐너 실습 기록 - 손잡이·방화벽 4종·재시도·고속 스캔.md>)
- keep-alive 로 둘째 요청 앞 SYN 이 없는 것, `http.time`: [paw 5장 실습](<~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/05-04.실습 — 이름과 요청을 직접 잡기.md>)
- TLS 핸드셰이크 읽기·복호화: [paw 4장](<~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-04.실습 — TLS 핸드셰이크를 직접 잡기.md>)
- 재시도 간격·상한 설계 자체는 회사 학습(메일 발송 outbox)에서 다뤘습니다. S3 은 HTTP 와 Go 에만 있는 부분, 곧 멱등성 판단과 재시도가 연결 풀과 맞물리는 부분에 집중합니다.

## 구성

```text
                 ┌────────────── 관리 채널 (Unix 소켓 위 HTTP) ──────────────┐
                 │  cmd/adminctl ── unix:/tmp/npgsvc/api-8081.sock ──┐       │
                 │                                                   ▼       │
cmd/load ──TCP──▶ cmd/api :8081  ── internal/upclient ──TCP──▶ cmd/upstream :9090
cmd/slowclient    (internal/api)                                (internal/upstream, 장애 손잡이)
                   │
cmd/supervisor ◀── unixgram NOTIFY_SOCKET "READY=1"
```

| 경로 | 하는 일 |
|---|---|
| `cmd/upstream`, `internal/upstream` | 외부 환율 제공자를 흉내 냅니다. `GET /v1/rates/{base}`, `POST /v1/conversions`, 장애 손잡이 `PUT·GET·DELETE /_faults`, 통계 `GET·DELETE /_stats`, 1초마다 보고 줄 |
| `cmd/api`, `internal/api` | 우리 서비스입니다. 미들웨어(recover → 요청 ID → 클라이언트 IP → 접근 로그 → 처리 중 계수 → IP 차단 → `/debug/` 접두 차단 → `TimeoutHandler`), `ServeMux` 패턴, 서버 타임아웃, 연결 수 상한 리스너, 드레인 뒤 `Shutdown` |
| `internal/upclient` | api 의 업스트림 클라이언트입니다. `Transport` 설정, 시도당·전체 기한, 본문 정리 방식, 재시도(백오프·지터·예산·`Retry-After`), 멱등성 키, ETag 재검증 캐시, `httptrace` 계측 |
| `internal/clientip` | 신뢰 프록시 대역 안에서 온 요청만 `X-Forwarded-For` 를 읽어 클라이언트 IP 를 정합니다 |
| `internal/admin`, `cmd/adminctl` | 관리 소켓 서버와 클라이언트입니다. 소켓 파일 권한·그룹, 남은 소켓 파일 처리, `SO_PEERCRED` 인가(Linux), 명령 `status`·`drain`·`log-level`·`peer`·`gc`·`watch` |
| `internal/notify`, `cmd/supervisor` | systemd `sd_notify` 방식 준비 알림입니다. 감독 프로세스가 unixgram 소켓을 만들고 자식에게 `NOTIFY_SOCKET` 으로 넘깁니다 |
| `cmd/load` | 부하 도구입니다. 닫힌 루프(`-c`)와 열린 루프(`-rate`), 1초마다 상태 분포·p50/p90/p99, 에러 종류(timeout·refused·reset·eof·other) |
| `cmd/slowclient` | 헤더를 한 바이트씩 보내거나(`header`) 요청 하나 뒤 가만히 있는(`idle`) 연결을 여러 개 엽니다 |
| `experiments/uds` | 곁가지 실험(스트림 세 번 쓰기의 읽힘 단위, unixpacket·unixgram 작은 버퍼) |

### edge 창과 맞춘 계약

edge 창의 Caddy 설정이 이 다섯 가지를 믿습니다. 바꾸지 않습니다.

- `api -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 -trusted-proxies 127.0.0.1/32 -admin-socket <경로>` (두 번째 인스턴스는 `-addr 127.0.0.1:8082`). `-trusted-proxies` 를 비운 판의 `client_ip` 도 계약에 듭니다(S7 에서 확인합니다).
- `GET /healthz` — 200 `ok` 또는 503 `draining`
- `GET /whoami` — `{"remote_addr", "client_ip", "x_forwarded_for"}`
- `GET /api/v1/rates/{base}` — 업스트림을 부릅니다
- `upstream -addr 127.0.0.1:9090`

## 준비

macOS 에서 대부분 돌고, `ss`·`SO_PEERCRED`·`127.0.0.2` 가 필요한 시나리오(S2·S5·S7·S9·S10 일부)는 OrbStack `ubuntu` 에서 돌립니다. OrbStack 안의 홈은 `/home/<사용자>` 이라 `~/study` 가 없습니다. 저장소는 `/Users/$USER/study/runners-lab/net/npg/chapters` 경로로 들어갑니다.

```bash
cd ~/study/runners-lab/net/npg/chapters            # OrbStack 에서는 cd /Users/$USER/study/runners-lab/net/npg/chapters
go build -o ch07-10-service/bin/$(go env GOOS)/ ./ch07-10-service/cmd/...
B=ch07-10-service/bin/$(go env GOOS)
mkdir -p /tmp/npgsvc
```

신호를 다루는 시나리오(S4·S10·S11)는 `go run` 이 아니라 빌드한 바이너리로 돌립니다. `kill` 이 `go run` 이 아니라 api 프로세스에 닿아야 하기 때문입니다.

창은 넷을 씁니다. T1 upstream, T2 api, T3 부하, T4 관측입니다. 관측에 자주 쓰는 명령은 이렇습니다.

```bash
curl -s 127.0.0.1:9090/_stats                 # 업스트림 누적 통계 (DELETE 로 0 으로)
curl -s 127.0.0.1:8081/debug/vars             # api 상태: goroutines·fds·server_conns·upstream 계수·설정
$B/adminctl -socket /tmp/npgsvc/api-8081.sock status       # 같은 내용을 관리 소켓으로
$B/adminctl -socket /tmp/npgsvc/api-8081.sock log-level debug   # 업스트림 시도마다 httptrace 줄이 찍힌다
ss -Htan state established '( dport = :9090 )' | wc -l     # (Linux) api → upstream 연결 수
ss -Htan state time-wait  '( dport = :9090 )' | wc -l
```

`adminctl` 두 줄은 api 를 `-admin-socket /tmp/npgsvc/api-8081.sock` 으로 띄웠을 때만 붙습니다. `/debug/vars` 의 `upstream` 계수 가운데 `retries` 는 `upclient` 의 재시도 루프에서, `got_conn_extra` 는 httptrace `GotConn` 콜백에서 셉니다. 셈하는 코드는 `internal/upclient` 에 있습니다.

### api 플래그

`-profile` 은 플래그 묶음입니다. 명령줄에 직접 준 플래그가 묶음보다 앞섭니다. `-h` 에 보이는 기본값은 `guarded` 묶음의 값입니다.

| 묶음 | 값 |
|---|---|
| `guarded` (기본) | 서버 `read-header-timeout 2s`·`read-timeout 10s`·`write-timeout 15s`·`idle-timeout 60s`, `handler-timeout 3s`, `graceful`·`drain-delay 5s`·`shutdown-timeout 15s` / 클라이언트 `dial-timeout 500ms`·`response-header-timeout 1s`·`attempt-timeout 1s`·`total-timeout 2.5s`·`max-conns-per-host 64`·`max-idle-per-host 64`, `retries 2`·`backoff exp 50ms~500ms`·`jitter full`·`retry-budget 0.2`·`honor-retry-after`, `idempotency-key`, `cache etag` |
| `bare` | 위의 시간·개수 값을 모두 0(Go 기본 동작), `graceful=false`(SIGTERM 을 받지 않는다), 재시도·캐시·멱등성 키 끔 |

| 묶음 밖 플래그 | 뜻 |
|---|---|
| `-max-conns N` | 동시에 열린 연결이 N 개면 `Accept` 를 부르지 않고 기다립니다 |
| `-log-position outer\|inner` | 접근 로그 미들웨어를 `TimeoutHandler` 바깥 또는 안쪽에 둡니다 |
| `-propagate-ctx` | 업스트림 호출에 요청 context 를 넘깁니다. false 면 `context.WithoutCancel` 로 취소를 떼어 냅니다 |
| `-body-mode drain\|close\|leak` | 응답 본문을 읽은 뒤 끝까지 비우고 닫기 / 닫기만 / 닫지 않기 |
| `-retry-post`, `-idempotency-key` | POST 재시도 여부, 업스트림 POST 에 `Idempotency-Key` 를 붙일지 |
| `-xff-pick rightmost\|leftmost`, `-deny-ips`, `-debug-allow` | `X-Forwarded-For` 읽는 방향, 403 으로 막을 대역, `/debug/` 를 열어 줄 대역 |
| `-admin-perm`·`-admin-group`·`-admin-stale fail\|remove\|probe`·`-admin-allow-groups`·`-admin-peercred rawconn\|file\|file-noclose`·`-admin-authz-by uid-groups\|gid` | 관리 소켓 |
| `-warmup USD,EUR`, `-notify-local`, `-notify-ack-wait` | 시작할 때 미리 받아 둘 통화, 준비 알림 보낼 때 묶을 경로, 답장 기다리는 시간 |

### upstream 장애 손잡이

`curl -X PUT 'http://127.0.0.1:9090/_faults?키=값&키=값'` 로 켜고 `curl -X DELETE http://127.0.0.1:9090/_faults` 로 되돌립니다. 시작할 때 켜려면 `-faults '키=값&…'` 을 줍니다.

| 키 | 뜻 (기본값) |
|---|---|
| `latency` | 요청마다 작업자를 붙잡는 처리 시간 (10ms) |
| `workers`, `queue_limit`, `honor_cancel` | 동시 처리 상한, 대기 상한(넘치면 바로 에러), 작업자를 얻은 뒤 클라이언트가 떠났으면 건너뛰기 (0, 0, false) |
| `header_stall` | 헤더를 보낸 뒤 본문 전에 멈추는 시간 (0) |
| `body_cut` | 본문 절반만 보내고 연결 끊기 (false) |
| `error_rate`, `error_status`, `retry_after` | 에러 비율, 상태 코드, `Retry-After` 초 (0, 503, 0) |
| `reset_rate`, `reset_nth`, `reset_when` | 응답 없이 RST 로 끊을 비율, 연결마다 N 번째 요청에서 끊기, 끊는 시점 `before`(처리 전)·`after`(처리·기록 뒤) (0, 0, before) |
| `chunked`, `trailing`, `body_size` | 청크 인코딩, JSON 뒤 공백 바이트 수, `padding` 필드 바이트 수 (false, 0, 0) |
| `etag`, `max_age`, `rotate` | ETag·If-None-Match, `Cache-Control: max-age`, 환율이 바뀌는 주기 (true, 0, 30s) |

## 시나리오

### S1. 외부 API 가 느려지자 우리 API 의 p99 가 같이 무너졌다

**증상** — 환율 제공사 응답이 2초로 늘어난 10분 동안 `/api/v1/rates` 의 p99 가 함께 치솟았고, api 의 고루틴 수와 업스트림 연결 수가 계속 늘었다.

**실행 전에 적을 예측**

1. `bare` 판에서 업스트림 지연을 2초로 올리면 초당 100 요청의 p50·p99 는 각각 얼마쯤이 될까요? api 의 `goroutines` 와 업스트림 `conns_open` 은 장애 동안 어떤 모양으로 바뀔까요?
2. `guarded` 판에서 같은 장애가 나면 클라이언트는 어떤 상태 코드를 받고, 걸린 시간은 어느 값 근처에 모일까요? `attempt-timeout 1s`·`total-timeout 2.5s`·`response-header-timeout 1s`·`handler-timeout 3s` 가운데 무엇이 먼저 걸릴까요?
3. `-max-conns-per-host 4` 를 더하면 api 쪽 연결 수와 업스트림이 본 `conns_open` 은 각각 어떻게 될까요? 연결을 기다리는 시간은 위 기한 가운데 어디에 들어갈까요?
4. 지연 대신 `header_stall=2s`(헤더는 바로, 본문만 늦게)를 걸면 `guarded` 판의 상태 분포는 어떻게 될까요?

**돌리는 법** — 캐시는 끕니다. 캐시가 켜진 판은 S8 에서 따로 봅니다.

```bash
T1$ $B/upstream -addr 127.0.0.1:9090
T2$ $B/api -profile bare -cache off -admin-socket /tmp/npgsvc/api-8081.sock -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090
T3$ $B/load -url http://127.0.0.1:8081/api/v1/rates/USD -rate 100 -d 30s -timeout 60s
T4$ sleep 8; curl -X PUT '127.0.0.1:9090/_faults?latency=2s'; sleep 15; curl -X DELETE 127.0.0.1:9090/_faults
    # 장애 동안 몇 번: curl -s 127.0.0.1:8081/debug/vars | grep -E '"goroutines"|attempt_timeouts|conn_new|conn_waits'
# T2 를 바꿔 되풀이: (기본 guarded) -cache off  /  -cache off -max-conns-per-host 4
# 4번: T4 의 latency=2s 대신 header_stall=2s
```

**볼 것** — 부하 도구의 1초 줄(상태 분포·p50·p99·inflight)이 장애 전·중·후에 어떻게 바뀌는지, `/debug/vars` 의 `goroutines`·`upstream.attempt_timeouts`·`conn_new`·`conn_waits`, 업스트림 보고 줄의 `inflight`·`conns_open`·`conns_new` 입니다. `$B/adminctl -socket /tmp/npgsvc/api-8081.sock log-level debug` 를 켜면 시도마다 `conn_wait`(연결을 얻기까지)·`connect`·`ttfb`(요청을 다 쓰고 첫 바이트까지)·`err` 가 찍혀 어느 구간에서 시간이 갔는지 가를 수 있습니다. 3번은 클라이언트가 본 연결 수와 업스트림이 본 연결 수를 나란히 놓고 봅니다.

**이어 읽을 노트** — [08-02 §3 타임아웃과 취소](<~/runners-high/write/02_os/book/npg_network-programming-with-go/08-02.Go HTTP 클라이언트는 응답 본문을 닫아야 연결을 재사용하고 context 로 요청을 끊는다.md>), Go 문서 [02-02 Client.Timeout·Transport 타임아웃·httptrace](<~/runners-high/write/01_language/docs/go/net-http/02-02.Client.Timeout·Transport 타임아웃·httptrace.md>) §1·§2, [02-01 Client·Transport](<~/runners-high/write/01_language/docs/go/net-http/02-01.Client·Transport.md>) §1(연결 풀 제어 필드)

### S2. 업스트림 연결 수가 요청 수만큼 늘어난다

**증상** — 배포 뒤 제공사 방화벽의 세션 수가 우리 요청 수와 같이 오르고, api 호스트의 fd 사용량이 시간이 갈수록 늘었다.

**실행 전에 적을 예측**

1. 본문 정리 방식 셋(`drain`·`close`·`leak`) × 응답 모양 셋(기본 `Content-Length` · `trailing=65536` · `chunked=true`) 아홉 칸에 연결이 재사용되는지(○/×)를 먼저 채웁니다. JSON 디코더가 어디까지 읽는지가 칸마다 어떻게 다를까요?
2. 재사용되지 않는 칸에서 `fds`·`goroutines` 는 요청 수와 어떤 관계로 늘까요, 아니면 늘지 않을까요?
3. 재사용되지 않는 칸에서 TIME-WAIT 는 api 쪽(목적지 9090)과 upstream 쪽(출발지 9090) 가운데 어디에 쌓일까요?
4. `-propagate-ctx=false` 를 빼고(요청 context 를 넘기고) 같은 표를 다시 채우면 `leak` 줄이 달라질까요? `guarded` 의 `attempt-timeout` 은 여기에 어떤 영향을 줄까요?
5. `-max-idle-per-host` 를 빼서 Go 기본값을 쓰고 `-c 20` 으로 돌리면 `drain` 칸의 `conn_new` 는 어떻게 될까요?

**돌리는 법** — OrbStack 에서 돌립니다. 칸 하나에 api 를 새로 띄웁니다.

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'latency=0'      # 또는 'latency=0&trailing=65536', 'latency=0&chunked=true'
T2$ $B/api -profile bare -propagate-ctx=false -max-idle-per-host 64 -body-mode drain \
        -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090                # drain · close · leak
T3$ $B/load -url http://127.0.0.1:8081/api/v1/rates/USD -c 20 -d 6s -report 0
T4$ ss -Htan state established '( dport = :9090 )' | wc -l
    ss -Htan state time-wait '( dport = :9090 )' | wc -l; ss -Htan state time-wait '( sport = :9090 )' | wc -l
    curl -s 127.0.0.1:8081/debug/vars | grep -E '"(goroutines|fds|conn_new|conn_reused|conn_put_idle)"'
```

같은 축소판이 테스트로도 있습니다: `go test ./ch07-10-service/internal/upclient -run TestBodyModes -v -count=1`.

**볼 것** — `conn_new` 와 `conn_reused` 의 비율, `conn_put_idle`(httptrace `PutIdleConn` 이 에러 없이 불린 횟수, 곧 연결이 풀로 돌아간 횟수), 업스트림 `_stats` 의 `conns_new`, `ss` 의 established·time-wait 수, `fds`·`goroutines` 입니다. 3번은 `sudo tcpdump -nn -i lo 'tcp port 9090 and tcp[tcpflags] & (tcp-rst|tcp-fin) != 0'` 로 닫는 쪽과 플래그도 봅니다. TIME-WAIT 의 원리는 gonet Phase 4 에서 다뤘습니다.

**이어 읽을 노트** — [08-02 §2 응답 본문을 끝까지 읽고 닫아야](<~/runners-high/write/02_os/book/npg_network-programming-with-go/08-02.Go HTTP 클라이언트는 응답 본문을 닫아야 연결을 재사용하고 context 로 요청을 끊는다.md>), Go 문서 [02-01 §2 Body 를 완전히 소비해야](<~/runners-high/write/01_language/docs/go/net-http/02-01.Client·Transport.md>)(bodyEOFSignal 과 readLoop 의 회수 조건)

### S3. 장애가 끝났는데도 업스트림이 일어나지 못한다

**증상** — 제공사의 10초짜리 지연 장애가 끝난 뒤에도 503·타임아웃이 한참 이어졌고, 제공사는 그 사이 우리 트래픽이 평소의 몇 배였다고 알려 왔다. 같은 기간 환전 요청 몇 건이 두 번 기록됐다.

**실행 전에 적을 예측** — 앞 판(GET)

1. 업스트림은 작업자 8개·처리 10ms(초당 약 800건)이고 평소 부하는 초당 400건입니다. 처리 시간을 10초 동안 100ms 로 늘렸다가 되돌리면, `storm` 판(즉시 재시도 4번, 예산 없음)에서 장애를 푼 뒤 업스트림의 초당 요청 수와 `queued` 는 어떻게 될까요?
2. `calm` 판(지수 백오프 + full 지터 + 재시도 예산 0.1)에서는 장애 중 업스트림이 받는 초당 요청 수가 `storm` 과 얼마나 다르고, 장애를 푼 뒤 몇 초 만에 200 으로 돌아올까요?
3. 클라이언트 쪽은 그대로 두고 업스트림만 `honor_cancel=true` 로 바꾸면 `storm` 판의 회복은 어떻게 달라질까요? 왜 업스트림 쪽 손잡이 하나가 클라이언트 재시도 폭풍에 영향을 줄까요?
4. 시도당 기한 300ms 가 업스트림 대기열 안의 요청에 어떤 일을 하는지, 업스트림 쪽 `worked_after_cancel` 로 확인할 수 있을까요?

**돌리는 법** — 앞 판(OrbStack 권장, 부하가 큽니다)

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -faults 'workers=8&latency=10ms'           # 3번: 'workers=8&latency=10ms&honor_cancel=true'
T2$ $B/api -profile bare -cache off -max-idle-per-host 512 -attempt-timeout 300ms -total-timeout 2s -retries 4 \
        -backoff none -retry-budget 0 -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090          # storm
    # calm: … -retries 4 -backoff exp -backoff-base 50ms -backoff-max 1s -jitter full -retry-budget 0.1
T3$ $B/load -url http://127.0.0.1:8081/api/v1/rates/USD -rate 400 -d 60s -timeout 3s
T4$ sleep 5; curl -X PUT '127.0.0.1:9090/_faults?latency=100ms'; sleep 10; curl -X PUT '127.0.0.1:9090/_faults?latency=10ms'
    # 끝나고: curl -s 127.0.0.1:8081/debug/vars | grep -E '"(calls|attempts|retries|retry_denied|attempt_timeouts)"'
```

**실행 전에 적을 예측** — 뒤 판(POST·멱등성) ★ 08-01

5. 업스트림이 연결마다 세 번째 요청을 응답 없이 RST 로 끊습니다(`reset_nth=3`). api 의 재시도 정책을 끈 판(`-retry-post=false`)에서, 업스트림 POST 에 `Idempotency-Key` 를 붙이지 않을 때와 붙일 때 클라이언트가 받는 502 는 각각 몇 퍼센트쯤일까요? 둘이 다르다면 누가 다시 보냈을까요?
6. 끊는 시점을 `reset_when=after`(업스트림이 환전을 기록한 뒤 끊기)로 바꾸면, 위 네 조합(키 유무 × `-retry-post` 유무) 가운데 같은 환전이 두 번 기록되는 조합은 어디일까요? 클라이언트가 받은 2xx 수와 업스트림 `conversions_created` 를 나란히 놓으면 무엇이 보일까요?
7. 같은 실험을 GET 으로 하면(`go test … -run TestResetOnSecondRequestLog`) 키가 없어도 결과가 POST 키 있음 판과 비슷할까요? 기준은 RFC 9110 의 무엇일까요?

**돌리는 법** — 뒤 판

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'latency=5ms&reset_nth=3&reset_when=before'   # 6번: reset_when=after
T2$ $B/api -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 -idempotency-key=false               # 또는 =true, 각각 -retry-post 를 더해서
T3$ $B/load -method POST -body '{"from":"USD","to":"KRW","amount":10}' -url http://127.0.0.1:8081/api/v1/conversions -c 2 -d 3s -report 0
T4$ curl -s 127.0.0.1:9090/_stats | grep -E '"(requests|resets|conversions_created|conversions_replayed)"'
    curl -s 127.0.0.1:8081/debug/vars | grep -E '"(got_conn_extra|retries|retry_denied)"'
```

**볼 것** — 업스트림 보고 줄의 `req`(초당 요청)·상태 분포·`queued`·`conns_new`, 부하 도구의 상태 분포와 p99, api 의 `calls`·`attempts`·`retries`·`retry_denied` 입니다. `attempts ÷ calls` 가 증폭 배수입니다. 뒤 판은 `got_conn_extra`·`retries`·업스트림 `requests` 를 나란히 봅니다.

**이어 읽을 노트** — [08-01 §2 메서드마다 약속이 다르다](<~/runners-high/write/02_os/book/npg_network-programming-with-go/08-01.HTTP 는 URL 로 자원을 가리키고 메서드로 요청하며 상태 코드로 답한다.md>)(안전성·멱등성, 멱등성 키), §3 상태 코드별 대응, Go 문서 [02-01 Client·Transport](<~/runners-high/write/01_language/docs/go/net-http/02-01.Client·Transport.md>), `go doc net/http.Transport`

### S4. 배포할 때마다 502 가 몇 건 난다

**증상** — 롤링 배포 때마다 앞단에서 502 가 몇 건씩 났다. 앞단 없이 api 만 보면 배포 순간 클라이언트 쪽 에러가 보인다.

**실행 전에 적을 예측**

1. 업스트림 지연 2초, 작업자 20개 닫힌 루프 부하 중에 api 에 SIGTERM 을 보냅니다. `bare` 판에서 그 순간 처리 중이던 20건은 클라이언트에게 무엇으로 보일까요? 부하 도구의 에러 종류 가운데 어느 칸에 들어갈까요?
2. `guarded` 판(`-drain-delay 2s`)에서는 SIGTERM 부터 종료까지 `/healthz` 가 어떤 순서로 바뀌고, 처리 중 20건과 드레인 동안 들어온 요청은 어떻게 끝날까요?
3. `-drain-delay 0 -shutdown-timeout 300ms` 이면 `Shutdown` 은 무엇을 돌려주고, 처리 중이던 요청은 어떻게 될까요?
4. 드레인 중에도 이미 열린 keep-alive 연결로는 요청이 계속 들어올까요? `-drain-close-keepalive` 를 켜면 무엇이 달라질까요?

**돌리는 법** — 업스트림 기한은 지연보다 길게 풀어 둡니다.

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'latency=2s'
T2$ $B/api -profile bare -cache off -max-idle-per-host 64 -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 & echo $!
    # guarded: $B/api -cache off -attempt-timeout 5s -total-timeout 5s -response-header-timeout 5s -handler-timeout 10s -drain-delay 2s …
    # 3번:    위 guarded 에서 -drain-delay 0 -shutdown-timeout 300ms
T3$ $B/load -url http://127.0.0.1:8081/api/v1/rates/USD -c 20 -d 10s -timeout 20s
T4$ while sleep 0.25; do echo "$(date +%T.%N | cut -c1-12) $(curl -s -o /dev/null -m 1 -w '%{http_code}' 127.0.0.1:8081/healthz)"; done
    # T3 를 시작하고 3초 뒤: kill -TERM <api pid>
```

**볼 것** — 부하 도구의 `== total` 줄(200 수와 에러 종류), `== example` 줄, T4 의 상태 코드가 바뀌는 시각, api 로그의 `drain start`·`draining inflight=`·`shutdown done err= inflight=` 입니다. Caddy 를 앞에 두고 인스턴스 두 개로 롤링하는 판은 [edge](edge/README.md) 에서 이어 갑니다.

**이어 읽을 노트** — Go 문서 [03-02 §2 Shutdown](<~/runners-high/write/01_language/docs/go/net-http/03-02.Server 타임아웃·Shutdown.md>), [09-01 §2](<~/runners-high/write/02_os/book/npg_network-programming-with-go/09-01.Go HTTP 서버는 타임아웃을 정하고 핸들러에 의존성을 주입하며 httptest 로 시험한다.md>)

### S5. 연결 수가 한도에 붙었는데 요청은 거의 없다

**증상** — 연결 수 상한(100)에 붙었다는 경보가 떴는데 초당 요청 수는 평소의 1% 였다. 헬스 체크가 실패해 로드밸런서가 인스턴스를 뺐다.

**실행 전에 적을 예측**

1. `bare` 판(`ReadHeaderTimeout`·`ReadTimeout` 0)에 헤더를 1초에 한 바이트씩 보내는 연결 100개를 붙이면, 정상 클라이언트의 `/healthz` 요청은 어떻게 될까요? 그 정상 클라이언트의 TCP 연결은 커널에서 어떤 상태일까요?
2. 그때 `curl 127.0.0.1:8081/debug/vars` 로 상태를 볼 수 있을까요? 관리 소켓(`adminctl status`)은요?
3. `guarded` 판(`read-header-timeout 2s`)에서 느린 연결은 끊길까요, 끊긴다면 몇 초 뒤일까요? 바이트가 1초마다 올 때 2초 기한은 바이트마다 다시 잴까요, 한 번만 잴까요?
4. 느린 헤더 연결은 `server_conns` 의 `new`·`active`·`idle` 가운데 어느 상태로 보일까요?
5. `-mode idle`(요청 하나를 마친 뒤 가만히 있는 keep-alive 연결)이면 `bare` 와 `-idle-timeout 5s` 판이 어떻게 갈릴까요? `IdleTimeout` 이 0 이면 Go 는 무엇을 대신 쓸까요?

**돌리는 법** — OrbStack 권장(`ss`).

```bash
T2$ $B/api -profile bare -max-conns 100 -admin-socket /tmp/npgsvc/api-8081.sock -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090
    # guarded: 같은 줄에서 -profile bare 를 뺀다 / 5번: -mode idle 과 함께 -profile bare, 그리고 -idle-timeout 5s
T3$ $B/slowclient -addr 127.0.0.1:8081 -n 100 -mode header -every 1s -d 20s
T3'$ sleep 2; $B/load -url http://127.0.0.1:8081/healthz -c 5 -d 12s -timeout 2s
T4$ curl -s -m 2 127.0.0.1:8081/debug/vars | grep -A4 -E '"listener"|"server_conns"'
    $B/adminctl -socket /tmp/npgsvc/api-8081.sock status | grep -A4 -E '"listener"|"server_conns"'
    ss -Htan state established '( sport = :8081 )' | wc -l
```

**볼 것** — `slowclient` 의 1초 줄(`open`·`closed_by_server`)과 `== lifetime`·`== reason`, 부하 도구의 상태 분포, 관리 소켓으로 본 `listener.in_use`·`accept_waiting` 과 `server_conns`, `ss` 의 established 수와 `listener.in_use` 를 나란히 놓은 값입니다. accept 대기열 자체는 gonet Phase 4 에서 다뤘습니다.

**이어 읽을 노트** — [09-01 §2 느린 클라이언트가 서버 자원을 붙잡지 못하게](<~/runners-high/write/02_os/book/npg_network-programming-with-go/09-01.Go HTTP 서버는 타임아웃을 정하고 핸들러에 의존성을 주입하며 httptest 로 시험한다.md>), Go 문서 [03-02 §1 서버 타임아웃](<~/runners-high/write/01_language/docs/go/net-http/03-02.Server 타임아웃·Shutdown.md>), [03-01 §1](<~/runners-high/write/01_language/docs/go/net-http/03-01.Server·ServeMux·Handler.md>)(ConnState)

### S6. 3초 넘는 요청은 클라이언트가 503 을 받는데 서버 로그는 200

**증상** — 고객은 503 `handler timeout` 을 받았다고 하는데 접근 로그에는 같은 요청 ID 가 200 으로 남아 있고, 그 시각 업스트림 호출량은 줄지 않았다.

**실행 전에 적을 예측**

1. 업스트림 지연 4초, `handler-timeout 3s` 에서 접근 로그를 `TimeoutHandler` 안쪽(`-log-position inner`)에 둔 판과 바깥(`outer`)에 둔 판 가운데, 증상의 503·200 은 어느 판에서 날까요? 그때 클라이언트가 기다린 시간과 접근 로그의 `dur`·`write_err` 는 무엇일까요?
2. 같은 판에서 업스트림 호출은 3초에 끊길까요, 4초까지 갈까요? `-propagate-ctx=false` 와 `true` 에서 각각 답합니다.
3. 접근 로그를 바깥(`-log-position outer`)으로 옮기면 로그 줄이 찍히는 시각과 `status` 가 어떻게 바뀔까요?
4. `-propagate-ctx=true` 로 업스트림 호출을 끊었을 때 업스트림 쪽 일은 멈출까요? 업스트림 `worked_after_cancel` 로 무엇을 알 수 있을까요?

**돌리는 법** — 재시도·클라이언트 기한은 꺼서 `TimeoutHandler` 만 남깁니다.

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'latency=4s'
T2$ $B/api -handler-timeout 3s -attempt-timeout 0 -total-timeout 0 -response-header-timeout 0 -retries 0 -cache off -log-level debug \
        -log-position inner -propagate-ctx=false -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090
    # 바꿔 가며: -log-position outer -propagate-ctx=false  /  -log-position inner -propagate-ctx=true
T3$ curl -s -w '\n%{http_code} %{time_total}s\n' 127.0.0.1:8081/api/v1/rates/USD
T4$ sleep 2; curl -s 127.0.0.1:9090/_stats | grep -E '"(requests|worked_after_cancel)"'; curl -s -X DELETE 127.0.0.1:9090/_stats >/dev/null
```

**볼 것** — T2 로그의 `msg=access`(`log_at`·`status`·`dur`·`write_err`·`ctx_err`)와 `msg="upstream attempt"`(`total`·`err`) 두 줄이 찍히는 순서와 시각, curl 의 상태와 시간, 업스트림 통계입니다.

**이어 읽을 노트** — [09-02 §1·§2 TimeoutHandler](<~/runners-high/write/02_os/book/npg_network-programming-with-go/09-02.미들웨어는 핸들러를 감싸고 ServeMux 는 경로를 나누며 HTTP2 서버 푸시는 브라우저가 걷어냈다.md>), Go 문서 [03-01 §1 Handler 와 ResponseWriter 수명주기](<~/runners-high/write/01_language/docs/go/net-http/03-01.Server·ServeMux·Handler.md>), [03-02 §2 클라이언트 취소 전파](<~/runners-high/write/01_language/docs/go/net-http/03-02.Server 타임아웃·Shutdown.md>)

### S7. IP 차단을 우회당했다

**증상** — 남용으로 차단한 IP(127.0.0.2 로 흉내 냄)에서 여전히 요청이 들어왔고, 그 IP 가 `/debug/vars` 까지 읽어 갔다. 요청에는 `X-Forwarded-For` 가 붙어 있었다.

**실행 전에 적을 예측** — 아래 표의 빈칸(`client_ip`, 상태 코드)을 먼저 채웁니다.

| 요청 \ `-trusted-proxies` | `""` | `127.0.0.0/8` | `127.0.0.1/32` |
|---|---|---|---|
| 차단된 IP 가 그냥 `/whoami` | | | |
| 차단된 IP 가 `X-Forwarded-For: 198.51.100.1` 을 붙여 `/whoami` | | | |
| 차단된 IP 가 `X-Forwarded-For: 127.0.0.1` 을 붙여 `/debug/vars` | | | |
| 프록시(127.0.0.1)가 차단된 IP 를 대신해 `X-Forwarded-For: 127.0.0.2` 로 `/whoami` | | | |

1. 넷째 줄에서 신뢰 대역을 비운 판은 차단을 지킬까요?
2. `-xff-pick leftmost` 로 바꾸면 위 표 가운데 어느 칸이 바뀔까요? 프록시가 이미 붙어 있던 `X-Forwarded-For` 뒤에 자기가 본 주소를 덧붙인다면 어느 쪽 끝이 믿을 만할까요?

**돌리는 법** — OrbStack 에서 돌립니다(Linux 는 127.0.0.0/8 전체가 루프백이라 `curl --interface 127.0.0.2` 로 출발 주소를 바꿀 수 있습니다).

```bash
T2$ $B/api -deny-ips 127.0.0.2 -trusted-proxies '' -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090
T3$ curl -s --interface 127.0.0.2 -w ' %{http_code}\n' 127.0.0.1:8081/whoami
    curl -s --interface 127.0.0.2 -H 'X-Forwarded-For: 198.51.100.1' -w ' %{http_code}\n' 127.0.0.1:8081/whoami
    curl -s --interface 127.0.0.2 -H 'X-Forwarded-For: 127.0.0.1' -o /dev/null -w '%{http_code}\n' 127.0.0.1:8081/debug/vars
    curl -s -H 'X-Forwarded-For: 127.0.0.2' -w ' %{http_code}\n' 127.0.0.1:8081/whoami
# T2 의 -trusted-proxies 를 127.0.0.0/8, 127.0.0.1/32 로 바꿔 되풀이
```

계산 규칙만 따로 보려면 `go test ./ch07-10-service/internal/clientip -v -count=1` 이 조합 표를 찍습니다. Caddy 가 실제로 붙이는 `X-Forwarded-For` 로 같은 표를 채우는 판은 [edge](edge/README.md) 에서 이어 갑니다.

**볼 것** — `/whoami` 의 `remote_addr`(TCP 상대)와 `client_ip`(계산값)가 같은지 다른지, 403 이 나는 칸입니다. 코드는 `internal/clientip/clientip.go` 의 `ClientIP` 하나입니다.

**이어 읽을 노트** — [09-02 §1 미들웨어로 조기 차단](<~/runners-high/write/02_os/book/npg_network-programming-with-go/09-02.미들웨어는 핸들러를 감싸고 ServeMux 는 경로를 나누며 HTTP2 서버 푸시는 브라우저가 걷어냈다.md>), §3 접두어로 가리기

### S8. 업스트림 호출량을 줄여라

**증상** — 제공사가 호출량 과금을 시작했다. 환율은 10초에 한 번 바뀌는데 우리는 요청마다 20KB 본문을 받아 오고 있었다.

**실행 전에 적을 예측** ★ 08-01

1. 캐시 두 방식(`off`·`etag`) × 업스트림 `max_age` 두 값(0·5) 네 판에서, 15초 동안 작업자 5개가 보내는 요청에 대해 업스트림 `requests` 와 `body_bytes` 는 각각 어느 크기쯤일까요? 표로 채웁니다.
2. `etag`·`max_age=0` 판의 업스트림 요청 수와 `body_bytes` 는 `off` 판과 비교해 각각 어떨까요?
3. `etag`·`max_age=5` 판에서 15초 동안 업스트림에 가는 요청은 몇 번쯤일까요? 그 가운데 304 는 몇 번일까요?
4. 막 뜬 api 에 작업자 5개가 동시에 첫 요청을 보내면 `cache_miss` 는 몇이 될까요? `rotate=10s` 경계에서는요?
5. 304 응답 하나의 크기와 걸리는 왕복은 200 과 비교해 어떨까요? `curl -si -H 'If-None-Match: <ETag>' 127.0.0.1:9090/v1/rates/USD` 로 확인합니다.

**돌리는 법**

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'body_size=20000&max_age=0&rotate=10s'     # max_age=5 로 한 번 더
T2$ $B/api -cache off -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090                        # -cache etag 로 한 번 더
T3$ $B/load -url http://127.0.0.1:8081/api/v1/rates/USD -c 5 -d 15s -report 0
T4$ curl -s 127.0.0.1:9090/_stats | grep -E '"(requests|not_modified|body_bytes)"'
    curl -s 127.0.0.1:8081/debug/vars | grep -E '"cache_'
    curl -si 127.0.0.1:9090/v1/rates/USD | head -8        # ETag 를 본 뒤 If-None-Match 로 다시
```

**볼 것** — 업스트림 `requests`·`not_modified`·`body_bytes`, api `cache_miss`·`cache_fresh`·`cache_revalidated`, 부하 도구의 `sent` 입니다. 응답 헤더 `X-Upstream-Cache` 로 요청 하나하나의 캐시 결과도 볼 수 있습니다.

**이어 읽을 노트** — [08-01 §3 304 와 클라이언트 대응](<~/runners-high/write/02_os/book/npg_network-programming-with-go/08-01.HTTP 는 URL 로 자원을 가리키고 메서드로 요청하며 상태 코드로 답한다.md>), RFC 9111 §4.2(신선도)·§4.3(재검증)

### S9. 관리 소켓으로 아무나 서비스를 멈출 수 있었다

**증상** — 운영 서버의 다른 계정(배치 작업용)이 실수로 관리 소켓에 `drain` 을 보내 서비스가 내려갔다. 소켓 파일 권한은 `srw-rw-rw-` 였다.

**실행 전에 적을 예측** ★ 07-02

1. 권한 `0666`·`0660` 두 판에서 `nobody` 가 `adminctl peer` 를 부르면 각각 어떻게 될까요? 실패한다면 HTTP 403 일까요, 그 전에 다른 에러일까요?
2. 소켓은 `0666` 인데 들어 있는 디렉터리를 `chmod 700` 하면 `nobody` 는 붙을 수 있을까요?
3. `-admin-allow-groups sudo` 에 판정 기준 둘(`uid-groups`·`gid`)을 걸고, 네 호출자(나 · `sudo -g sudo` 로 그룹만 바꾼 나 · `nobody` · `setpriv` 로 GID 만 sudo 로 바꾼 `nobody`)가 `status` 를 부를 때 결과 여덟 칸을 채웁니다. 어느 기준이 무엇을 믿는 셈일까요?
4. 자격 증명을 읽는 방법 셋(`rawconn`·`file`·`file-noclose`)으로 `adminctl status` 를 50번 부르면 `/proc/<pid>/fd` 개수는 각각 어떻게 될까요? 그 뒤 `adminctl gc` 를 부르면요?
5. 허용 그룹을 하나가 아니라 여럿 둘 수 있게 집합(map)으로 둔 이유는 무엇일까요?

**돌리는 법** — OrbStack 에서 돌립니다(`SO_PEERCRED` 는 Linux 전용, macOS 에서는 허용 그룹을 걸면 시작을 거절합니다).

```bash
S=/tmp/npgsvc/api-8081.sock
T2$ $B/api -admin-socket $S -admin-perm 0666 -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 & echo $!
T3$ stat -c '%A %U:%G' $S
    $B/adminctl -socket $S peer; sudo -u nobody $B/adminctl -socket $S peer
    # 2번: chmod 700 /tmp/npgsvc 뒤 다시, 끝나면 chmod 755 /tmp/npgsvc
# 3번: T2 에 -admin-allow-groups sudo -admin-authz-by uid-groups (또는 gid)
    $B/adminctl -socket $S status
    sudo -g sudo $B/adminctl -socket $S status
    sudo -u nobody $B/adminctl -socket $S status
    sudo setpriv --reuid=nobody --regid=sudo --clear-groups $B/adminctl -socket $S status
# 4번: T2 에 -admin-peercred rawconn (또는 file, file-noclose)
    ls /proc/<pid>/fd | wc -l; for i in $(seq 50); do $B/adminctl -socket $S status >/dev/null; done; ls /proc/<pid>/fd | wc -l
    $B/adminctl -socket $S gc; ls /proc/<pid>/fd | wc -l
```

`drain` 은 api 를 내리므로 마지막에 한 번만 시험합니다. 같은 축소판이 테스트로도 있습니다: `go test ./ch07-10-service/internal/admin -run 'TestAuthzSettingApplied|TestPeerCredMethodsFDLog' -v -count=1`(Linux).

**볼 것** — `adminctl` 의 에러 문자열(어느 단계에서 막혔는지), 403 본문의 `decision`(커널이 알려 준 `pid`·`uid`·`gid`, 사용자 DB 의 `user_groups`, `reason`), api 로그의 `msg="admin request"`, fd 개수입니다.

**이어 읽을 노트** — [07-02 피어 자격 증명으로 접속한 프로세스를 인증하는 서비스](<~/runners-high/write/02_os/book/npg_network-programming-with-go/07-02.피어 자격 증명으로 접속한 프로세스를 인증하는 서비스.md>) §1(File() 이 필요한 이유)·§2(그룹 집합)·§4(sudo -g 시험, 파일 권한과 자격 증명의 차이), [07-01 §2 파일 소유권과 권한](<~/runners-high/write/02_os/book/npg_network-programming-with-go/07-01.Unix 도메인 소켓은 파일 경로로 묶이고 세 가지 타입이 경계를 다르게 다룬다.md>)

### S10. 재시작했더니 관리 소켓을 열지 못한다

**증상** — 배포 스크립트가 응답 없는 api 를 `kill -9` 로 내리고 다시 띄웠더니 관리 소켓을 열지 못해 시작하지 못했다. 급히 시작 스크립트에 `rm -f` 를 넣었더니, 이번에는 같은 서버에서 api 두 개가 같은 소켓 경로를 두고 엉켰다.

**실행 전에 적을 예측** ★ 07-01

1. `kill -9` 뒤 소켓 파일은 남을까요? 같은 경로에 `-admin-stale fail` 로 다시 띄우면 시작 로그에 무엇이 찍힐까요? 그렇게 되는 이유는 무엇일까요?
2. `-admin-stale probe` 는 남은 파일에 `Dial` 을 해 봅니다. 듣는 프로세스가 없을 때와 있을 때 `Dial` 은 각각 무엇을 돌려줄까요? 다른 계정이 만든, 내가 쓸 권한이 없는 남은 파일이면요?
3. api A 가 듣는 중에 api B 를 같은 소켓 경로·`-admin-stale remove` 로 띄우면, 그 뒤 `adminctl status` 는 누구의 `pid` 를 돌려줄까요? 그다음 B 를 SIGTERM 으로 내리면 A 의 관리 채널은 어떻게 될까요?
4. `adminctl watch` 로 연결 하나를 붙여 둔 채 소켓 파일을 `rm` 하면, 그 watch 와 새로 띄운 `adminctl status` 는 각각 어떻게 될까요?
5. `-profile bare`(SIGTERM 을 받지 않음)로 띄운 api 를 SIGTERM 으로 내리면 소켓 파일은 남을까요?

**돌리는 법** — macOS 에서도 돌지만 결과를 비교하려면 OrbStack 에서 돌립니다.

```bash
S=/tmp/npgsvc/api-8081.sock
$B/api -admin-socket $S -admin-stale fail -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 & A=$!
sleep 0.5; kill -9 $A; ls -l $S
$B/api -admin-socket $S -admin-stale fail -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090      # 1번
$B/api -admin-socket $S -admin-stale probe -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 & A=$!   # 2번
$B/api -admin-socket $S -admin-stale probe -addr 127.0.0.1:8082 -upstream http://127.0.0.1:9090      # 2번, 살아 있을 때
$B/api -admin-socket $S -admin-stale remove -addr 127.0.0.1:8082 -upstream http://127.0.0.1:9090 & B2=$!  # 3번
$B/adminctl -socket $S status | grep '"pid"'; kill -TERM $B2; sleep 6; ls $S; $B/adminctl -socket $S status
kill -TERM $A
# 4번: api 를 새로 띄우고
$B/adminctl -socket $S -n 8 watch &  sleep 2.5; rm $S; $B/adminctl -socket $S status
```

**볼 것** — `ls -l` 로 본 소켓 파일의 있고 없음, 시작 로그의 `admin socket exists`·`admin listening` 줄과 에러 문자열, `status` 의 `pid`, `watch` 줄의 `reused`·`conn`·`pid` 입니다. 코드는 `internal/admin/admin.go` 의 `prepareSocketPath` 입니다. 3번 결과는 `go doc net.UnixListener.SetUnlinkOnClose` 와 견줍니다.

**이어 읽을 노트** — [07-01 §2 소켓 파일의 생성과 바인딩 충돌, 리스너별 정리 규칙](<~/runners-high/write/02_os/book/npg_network-programming-with-go/07-01.Unix 도메인 소켓은 파일 경로로 묶이고 세 가지 타입이 경계를 다르게 다룬다.md>), [07-02 §3 소켓 생애주기 관리](<~/runners-high/write/02_os/book/npg_network-programming-with-go/07-02.피어 자격 증명으로 접속한 프로세스를 인증하는 서비스.md>)

### S11. 감독 프로세스가 서비스 준비를 모른다

**증상** — 감독 프로세스가 api 를 띄우자마자 트래픽을 보냈더니 첫 몇 초 동안 요청이 실패했다. api 는 시작할 때 환율을 미리 받아 두느라(워밍업) 몇 초 걸린다.

**실행 전에 적을 예측**

1. 업스트림 지연 800ms, `-warmup USD,EUR` 로 띄우면 감독 프로세스 로그에서 `started`, `/healthz` 첫 200, `READY=1` 셋은 어떤 순서·간격으로 찍힐까요? 워밍업 동안 `/healthz` 를 부르면 어떻게 될까요?
2. 감독 프로세스가 답장(`-ack`)을 보내도록 하고, api 가 보내는 쪽 소켓을 경로에 묶지 않은 판(`-notify-local` 없음)과 묶은 판에서 답장은 각각 어떻게 될까요? 감독 프로세스 로그의 `from=` 은 무엇으로 찍힐까요?
3. systemd 는 `READY=1` 에 답장하지 않습니다. 그런데도 `MAINPID=` 같은 정보를 같이 보내는 이유는 무엇일까요?
4. 감독 프로세스를 SIGTERM 으로 내리면 api 쪽에서 어떤 알림이 오고, 감독 프로세스가 만든 소켓 파일은 남을까요?

**돌리는 법**

```bash
T1$ $B/upstream -addr 127.0.0.1:9090 -report 0 -faults 'latency=800ms'
T2$ $B/supervisor -socket /tmp/npgsvc/notify.sock -health http://127.0.0.1:8081/healthz -- \
        $B/api -warmup USD,EUR -drain-delay 1s -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090
    # 2번: $B/supervisor -ack … -- $B/api -notify-ack-wait 1s …                                  (묶지 않음)
    #      $B/supervisor -ack … -- $B/api -notify-ack-wait 1s -notify-local /tmp/npgsvc/api-notify.sock …   (묶음)
    # 4번: 다른 창에서 kill -TERM <supervisor pid>, 끝난 뒤 ls /tmp/npgsvc
```

**볼 것** — 감독 프로세스 줄의 `+시간`, `notify from=… msg=…`, `reply=ok`·`reply_err=`, api 로그의 `warmup`·`notify READY=1 ack= err=` 입니다.

**이어 읽을 노트** — [07-01 §4 unixgram 과 보내는 쪽 소켓 파일](<~/runners-high/write/02_os/book/npg_network-programming-with-go/07-01.Unix 도메인 소켓은 파일 경로로 묶이고 세 가지 타입이 경계를 다르게 다룬다.md>), `man sd_notify`

## 곁가지 실험

서비스와 상관없이 07-01 의 Phase 3 후보 둘을 테스트로 남겼습니다. 결과는 `t.Logf` 로만 찍습니다.

```bash
go test ./ch07-10-service/experiments/uds -run TestStreamThreeWrites -v -count=1          # 스트림에 "ping" 세 번 → Read 마다 몇 바이트?
orb -m ubuntu bash -lc 'cd /Users/$USER/study/runners-lab/net/npg/chapters && go test ./ch07-10-service/experiments/uds -run TestUnixpacket -v -count=1'
# 64바이트 메시지 둘을 16바이트 버퍼로 두 번 읽는다 — unixpacket 과 unixgram 의 둘째 읽기에 무엇이 올까?
```

## 테스트

```bash
cd ~/study/runners-lab/net/npg/chapters
go vet ./ch07-10-service/... && go test ./ch07-10-service/... -count=1
orb -m ubuntu bash -lc 'cd /Users/$USER/study/runners-lab/net/npg/chapters && go test ./ch07-10-service/... -count=1'
```

테스트의 단언은 edge 계약(`/healthz`·`/whoami`·`/api/v1/rates`), 미들웨어 계약(요청 ID, recover, `/debug/` 접두 차단), 설정 적용(소켓 권한, 허용 그룹을 켰을 때와 껐을 때, 남은 소켓 파일을 `probe` 로 다루는 절차)만 겁니다. 연결 재사용·시도 수·캐시 결과·fd 수처럼 시나리오가 묻는 값은 `-v` 로 돌려 `t.Logf` 줄로만 봅니다.

## 다루지 않는 것

| 노트의 목표 | 이유 |
|---|---|
| 07-01 목표 1 — TCP 루프백과 Unix 소켓의 성능 비교 | 지연·처리량 벤치마크는 맥북·OrbStack 의 잡음이 커서 결론을 낼 수 없습니다. 차이의 원리(커널이 상대 수신 큐로 바로 옮김)는 노트로 갈음합니다 |
| 07-01 목표 3·5 — 스트림 에코, unixpacket 작은 버퍼 읽기 | 서비스 장면이 없어 곁가지 실험으로만 둡니다 |
| 08-01 목표 1·3·4·5 — URL 다섯 부분, nc 로 본 요청 바이트, 상태 코드 범주, keep-alive 왕복 | 기초이거나 paw 5장에서 캡처로 확인했습니다 |
| 08-02 목표 1·5 — `http.Head`·`Date` 시각 오차, `req.Close`·`DisableKeepAlives`, multipart | 실무 장애 장면과 거리가 멉니다. `-disable-keepalives` 플래그만 남겨 두어 S2 의 비교 대조군으로 쓸 수 있습니다 |
| 09-01 목표 3 — `ServeTLS` | TLS 종단은 edge 창의 Caddy 가 맡고, 핸드셰이크 읽기는 paw 4장에서 했습니다 |
| 09-01 목표 4 — `httptest` | 시나리오가 아니라 이 폴더의 테스트 코드(`internal/*/…_test.go`)가 그 예입니다 |
| 09-02 목표 3 — `StripPrefix`·`FileServer` 정적 파일 | 정적 파일은 edge 창의 Caddy 가 맡습니다. 접두 차단 자체는 `/debug/` 로 S7 에 들어 있습니다 |
| 09-02 목표 4 — `ServeMux` 우선순위·슬래시 리다이렉트 | 라우팅은 go1.22 패턴으로 쓰기만 하고 장애 장면을 만들지 않았습니다 |
| 09-02 목표 5 — HTTP/2 서버 푸시 | 브라우저가 걷어낸 기능이라 운영 장면이 없습니다 |
