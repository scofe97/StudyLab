# 한산할 때 장비 등록만 502 가 나고 백엔드에는 기록이 없다 (I2)

- 장: ch08
- 날짜: 2026-10-28
- 앞 장면과의 연결: I1·I3 과 같은 v0 프록시(Go `http.DefaultTransport` 그대로). I1·I3 이 부하가 몰릴 때의 얼굴이었다면, I2 는 부하가 거의 없을 때 나는 얼굴이다. 원인은 풀에 세워 둔 빈 연결의 보관 시간(`IdleConnTimeout` 90s)이 백엔드의 보관 시간보다 길다는 데 있다.

## 신고

> "한산한 시간대에도 하루 수십 건씩 장비 등록(POST)에서만 502 가 납니다. 같은 시간에 단건 조회(GET)는 0건입니다. 백엔드 로그를 뒤져 보면 실패한 그 등록 요청은 아예 들어온 기록이 없습니다. 부하가 몰릴 때가 아니라 오히려 조용할 때 납니다."

### 창이 몰래 켠 장애

진단이 끝난 뒤 학습자에게 공개했다.

| 조건 | 값 | 흉내 낸 것 |
|---|---|---|
| 백엔드 `idle_close` (`PUT /_faults?idle_close=1s`, 11:29:13) | 1초 | 서버의 빈 연결 보관 시간(Go `http.Server.IdleTimeout`, nginx `keepalive_timeout`)이 프록시의 90초보다 짧은 상황. 현실 값(수 초~수십 초)을 시간 압축으로 1초까지 당겼다 |
| `tc netem delay` (루프백, 9090 포트만) | 100ms | 프록시↔백엔드 네트워크 지연. 서버가 보낸 FIN 이 프록시에 닿기 전인 틈을 넓혀, 하루 수십 건짜리 경합을 10분 안에 보이게 했다 |

## 진단 과정

| 순서 | 본 신호(명령·지표) | 세운 가설 | 남김 / 버림과 이유 |
|---|---|---|---|
| 1 | npgwatch 1초 그래프가 거의 초록 | 거의 실패 없이 된다 (학습자) | 버림. 프록시 카운터는 10분에 `reasons` UC 44 / 요청 1,813 이다. 40건 중 1건꼴이라 1초 그래프로는 안 보인다 |
| 2 | 프록시 로그 `WRN upstream error error=EOF flags=UC` | — | 같은 시각의 `INF … status=502` 접근 로그 줄에 `method`·`reused`·`idle` 이 있다. 창이 npgwatch `/api/state` 로 13:05:40~13:07:26 줄을 직접 모았다 |
| 3 | 502 9건이 모두 `method=POST reused=true idle=807~973ms`. 같은 구간 재사용 GET 121건은 실패 0. 성공 줄의 `idle` 은 대부분 0~700ms | 쓰던 연결을 다시 쓸 때 POST 만 실패한다 (학습자) | 남김. "1초 가까이 쉰 연결" 이 경계선이다 |
| 4 | 프록시가 받은 요청 1,813 − 백엔드가 받은 요청 1,770 = 43 ≈ UC 44 | — | 실패한 POST 는 백엔드에 닿지 않았다. 신고의 "백엔드에 기록이 없다" 와 맞는다 |
| 5 | 프록시 `IdleConnTimeout` 은 90s | 서버가 `close()` 로 먼저 닫았고 프록시는 열려 있다, CLOSE_WAIT 인가 (학습자) | 남김 + 보정. 닫은 쪽은 서버가 맞다. 다만 Go 는 풀의 연결마다 `readLoop` 로 읽기를 걸어 두어 FIN 이 오면 바로 닫고 풀에서 뺀다(CLOSE_WAIT 는 잠깐). 실패는 FIN 이 오는 중에 POST 를 먼저 보낸 경합이다 |
| 6 | 같은 신호 | 서버에도 빈 연결 보관 시간 제어가 따로 있다 (학습자) | 남김. `http.Server.IdleTimeout`, nginx `keepalive_timeout`. 양쪽 보관 시간이 다르다 |
| 7 | GET 0 / POST 9 | 차이를 모르겠다 (학습자) | 창 설명. Go Transport 는 쓰던 연결이 이렇게 끊기면 다시 보내도 되는 요청만 새 연결로 한 번 더 보낸다(`shouldRetryRequest`). GET 은 조용히 살아남고 POST 는 에러가 그대로 502 가 된다 |
| 8 | `isReplayable` 대조 | 멱등 메서드라도 안 보내는 경우가 있고, 다시 보내는 건 조회 계열이거나 `Idempotency-Key` 헤더가 있을 때다 (학습자) | 남김. PUT·DELETE 는 RFC 9110 상 멱등인데도 Go 는 다시 보내지 않는다. 메서드와 무관하게 한 바이트도 못 쓴 경우(`nothingWrittenError`)는 본문을 되감을 수 있으면 다시 보낸다 |

### 학습자의 원인 정리 (원문)

> 사유: 프록시는 90초동안 보관하는데 서버는 1초쯤 쉰 연결을 먼저 닫아버렸다 (양쪽이 다른 보관시간을 사용)
>
> GET/POST의 차이 -> GET은 죽은 연결에 대해 다시 재전송을 요청한다 (멱등성이 보장된다) POST는 멱등성이 보장이 안됨

창의 보정 한 가지: "멱등성이 보장된다" 는 Go 의 기준보다 넓다. Go 가 다시 보내는 것은 GET·HEAD·OPTIONS·TRACE 와 `Idempotency-Key`(`X-Idempotency-Key`) 헤더가 붙은 요청뿐이고, PUT·DELETE 는 빠진다.

### 경합의 시간 축

```
백엔드:  응답 보냄(t0) ──── 1초 쉼 ────→ close() ──FIN(100ms)──→
프록시:      응답 받음(t0+100) → 풀에서 쉼 ──→ POST 보냄 ──(100ms)──→ 백엔드는 이미 닫음
                                         └ 프록시 기준 idle 800~1000ms 에 보낸 요청만 이 틈에 걸린다
프록시:  응답 대신 EOF → UC → 손님에게 502
```

- 프록시 기준 idle 을 x 라 하면, 요청이 백엔드에 닿는 시각은 t0+200+x 다. 이것이 t0+1000 보다 늦으면(x > 800) 백엔드는 이미 닫았다.
- FIN 이 프록시에 닿는 시각은 t0+1100 이다. x < 1000 이면 프록시는 아직 FIN 을 못 받아 연결이 살아 있는 줄 안다.
- 관찰된 9건의 idle 807~973ms 가 이 구간 안에 있다.

## 실행 전 예측

ADR 0012 재확인(2026-10-28)은 I1·I3 과 묶어 창이 화면 버튼으로 돌렸고, I2 는 따로 예측을 받지 않았다. ADR 0013 재확인 때 예측을 받는다.

## 실행 (사용자가 친 명령, OrbStack `ubuntu`)

```bash
./bin/npgwatch-linux -lab -backend-addr 192.168.139.208:9090 -tw-port 9090
# 웹 화면에서 백엔드 → 프록시 → "I2" 장면 (11:27:22, 12:57 두 번)
# I2 장면 = 단건 조회 GET -rate 1 -poisson 10분 + 장비 등록 POST -rate 1 -poisson 10분
```

창이 몰래 친 명령(공개함):

```bash
sudo tc qdisc add dev lo root handle 1: prio
sudo tc qdisc add dev lo parent 1:3 handle 30: netem delay 100ms
sudo tc filter add dev lo parent 1: protocol ip u32 match ip dport 9090 0xffff flowid 1:3
sudo tc filter add dev lo parent 1: protocol ip u32 match ip sport 9090 0xffff flowid 1:3
curl -X PUT 'http://192.168.139.208:9090/_faults?idle_close=1s'
# 정리(13:43): idle_close=0s, sudo tc qdisc del dev lo root
```

백엔드는 eth0 주소에 떠 있지만, 같은 호스트의 자기 주소로 가는 패킷은 루프백 장치를 지나므로 `lo` 에 건 netem 이 프록시↔백엔드 구간에만 걸린다.

## 관찰

12:57 실행 중 13:05:40~13:07:26 에 모은 프록시 접근 로그 273줄:

| 시각 | method | reused | idle (ms) | dur (ms) |
|---|---|---|---|---|
| 13:05:08 | POST | true | 956 | 45 |
| 13:05:32 | POST | true | 807 | 196 |
| 13:06:00 | POST | true | 973 | 32 |
| 13:06:21 | POST | true | 841 | 158 |
| 13:06:34 | POST | true | 919 | 86 |
| 13:06:46 | POST | true | 952 | 50 |
| 13:06:48 | POST | true | 907 | 105 |
| 13:06:59 | POST | true | 843 | 161 |
| 13:07:21 | POST | true | 930 | 71 |

- 같은 구간 성공: 재사용 GET 121건, 재사용 POST 113건. 성공 응답의 `dur` 은 약 210ms(netem 왕복 200ms + 처리), 새 연결은 약 415ms(맺기 왕복이 하나 더).
- 13:43 누적(두 번 실행): 프록시 요청 2,405, `reasons` UC 56. 백엔드 요청 2,350(GET 1,199, POST 1,151), 2xx 2,350, 5xx 0. 차이 55 ≈ UC 56. 프록시가 받은 POST 는 2,405 − GET 1,199 = 1,206 이다(GET 은 모두 백엔드에 닿았다고 본다. 저장한 자료로는 GET 502 를 셀 수 없다). 1,151 + 56 = 1,207 로 세면 백엔드에 닿고도 UC 가 된 1건(차이 55 와 UC 56 의 어긋남)을 두 번 센다.

## 전후 수치

| 지표 | 개선 전 (v0) | 개선 후 |
|---|---|---|
| POST 502(UC) / 프록시가 받은 POST (10분 두 번) | 56 / 1,206 (약 4.6%) | 약 21 / 318 (재확인 장면 약 4분, v0 재확인 약 20 / 329) |
| GET 502 / GET | 0 / 1,199 | 세지 못함(메서드별 5xx 없음) |
| 502 의 idle 범위 | 807~973ms | 보지 않음 |
| 백엔드에 닿지 않은 요청 | 55 | 보지 않음 |

ADR 0012 재확인(2026-10-28, npgwatch `-lab` I2 장면, 리눅스 VM, 약 4분씩)은 같은 장면으로 v0 도 다시 돌렸다. 바뀌지 않았다: v0 재확인 프록시 UC 약 20 / POST 329, ADR 0012 뒤 UC 약 21 / POST 318. 장면의 백엔드 `idle_close=1s` 가 프록시 4초보다 짧아 ADR 0012 결정 3(프록시가 먼저 닫음)의 전제가 깨진 상태다. ADR 0012 결과에 "백엔드가 4초보다 짧은 유휴 기한을 쓰면 I2 가 돌아온다"로 적은 한계 그대로다. 결정 3 이 흔한 5초 백엔드에서 I2 를 없애는지는 이 장면으로 보일 수 없고, 1초로 압축한 시간을 4초·5초 비율로 다시 짜는 실험이 따로 필요하다(미정). 저장 자료가 메서드별 5xx 를 나누지 않아 GET 502 수는 재확인에서 세지 못했다. 비교 화면 `img/ch08-retest-i2.jpg`.

## 포스트모템

- 고친 것(ADR 0012 결정 3): 프록시의 빈 연결 보관을 4초로 줄여 흔한 5초 백엔드보다 먼저 닫는다. 이 장면(백엔드 1초)에서는 전제가 맞지 않아 UC 가 그대로였다. 고침이 통하는지는 아직 보이지 못했다.
- 배운 것: 빈 연결은 먼저 닫는 쪽이 안전하다. 상대보다 짧게 두려면 상대의 유휴 기한을 알아야 하고, 모르면 POST 를 다시 보내도 되는지(멱등성)가 남은 방어선이다.
- 남은 것: 백엔드 5초·프록시 4초 비율을 지키며 압축한 새 장면(미정), 메서드별 5xx 계측, POST 재시도는 ADR 0013.

- 근본 원인(진단까지): v0 프록시는 Go 기본 Transport 를 그대로 써서 빈 연결을 90초 보관한다. 백엔드는 1초 쉰 연결을 먼저 닫았다. 백엔드가 닫은 직후 FIN 이 프록시에 닿기 전에 그 연결로 보낸 요청은 백엔드에 닿지 않고 EOF 가 된다. Go 는 GET 은 새 연결로 조용히 다시 보내 손님에게 숨기고, POST 는 다시 보내지 않아 502(UC)로 드러난다.
- 이어질 결정: 프록시의 빈 연결 보관 시간을 업스트림보다 짧게 두는 일(ADR 0012, 기한·풀), POST 를 언제 다시 보내도 되는지(ADR 0013, 재시도·멱등성).

## 알려진 한계

- GET 이 실제로 다시 보내졌다는 것은 직접 보지 않았다. 접근 로그는 마지막 `GotConn` 의 `reused`·`idle` 만 남긴다(`internal/proxy/trace.go` 훅이 덮어쓴다). 다시 보낸 횟수를 세는 계측은 ADR 0013 재확인 때 붙인다.
- 성공한 재사용 GET 8건과 POST 3건의 `idle` 이 1초를 넘는다(최대 3,272ms, `dur` 약 210ms). 백엔드가 1초에 닫았다면 살아 있을 수 없는 연결이다. 원인은 확인하지 않았다.
- 백엔드 `conns_new` 2,922 와 프록시 `conns_new` 302 의 차이가 크다. npgwatch 가 1초마다 백엔드 `/_stats` 를 읽는 연결이 섞였을 것으로 보이나 확인하지 않았다. netem 도 그 연결에 걸린다.
- 실패 빈도는 압축한 값(1초, 100ms)에서 나온 것이다. 현실 빈도는 왕복 지연과 업스트림 보관 시간, 요청 간격에 따라 달라진다.

## 원문 근거

- Go 1.25.1 `net/http/transport.go:808-852` `shouldRetryRequest` — 새 연결이면 안 보냄(`:821`), `nothingWrittenError` 면 본문을 되감을 수 있을 때 보냄(`:832`), `!req.isReplayable()` 이면 안 보냄(`:836`), `errServerClosedIdle` 이면 보냄(`:846`).
- 같은 패키지 `request.go:1534` `isReplayable` — 본문이 없거나 `GetBody` 가 있을 때 GET·HEAD·OPTIONS·TRACE, 또는 `Idempotency-Key`·`X-Idempotency-Key` 헤더(golang/go#19943).
- `transport.go:2431` `readLoopPeekFailLocked` — 풀에서 쉬는 연결이 EOF 를 받으면 `pc.closeLocked(errServerClosedIdle)`.
- `transport.go:1009-1013` `errServerClosedIdle` 주석 — "is not seen by users for idempotent requests".
- `server.go:3040-3044` `http.Server.IdleTimeout` — "the maximum amount of time to wait for the next request when keep-alives are enabled".
- `cmd/fakeinv/main.go` `idleTimer` — `idle_close` 손잡이. `http.StateIdle` 이 되면 그 시간 뒤 `c.Close()`.

## 이어 읽을 노트

- 08-02.Go HTTP 클라이언트는 응답 본문을 닫아야 연결을 재사용하고 context 로 요청을 끊는다.md
