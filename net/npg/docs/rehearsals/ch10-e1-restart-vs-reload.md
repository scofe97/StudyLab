# 재시작과 설정 교체 (E1)

- 장: ch10
- 날짜: 2026-11-04
- 앞 장면과의 연결: C1 에서 관리 API 기초를 익힌 뒤, 부하가 들어오는 상태에서 프로세스 재시작(문 닫고 공사)과 관리 API 를 통한 무중단 설정 교체(간판만 바꿔 달기)의 차이를 관측한다.

## 신고

> "설정을 바꾸려고 프록시를 재시작했더니 몇 초 동안 수만 건의 연결 거부가 쏟아졌다. 반면 관리 API 로 교체했을 때는 아무런 오류도 없었다."

이 장면은 Caddy 재시작 방법의 우열을 따지는 단순한 팁이 아니다. 듣는 소켓(listening socket)과 이미 맺힌 연결(established, keep-alive)은 완전히 다른 것이며, 재시작이 만드는 공백과 관리 API 교체가 무중단을 만드는 원리를 눈으로 확인하는 장면이다.

### 이 장면의 설정

- 경로: 손님 npgload → Caddy `127.0.0.1:8000` → npgproxy `127.0.0.1:28080` → fakeinv `127.0.0.1:29090`. Caddy 설정 `deploy/caddy/scenarios/e5-internal-tls.json`.
- 화면: `npgwatch -lab` 대시보드(크롬 `http://127.0.0.1:28090`)의 「손님 → Caddy」 행. 장면 「E1: 엣지 경유 단건 조회 동시 10, 60초」(`npgload -url http://127.0.0.1:8000/v1/devices/7 -c 10 -d 60s`), 버튼 `↻ 재시작(문 닫고 공사)`(run.sh down = SIGTERM, 2초 쉼, up)·`⇄ 설정만 바꾸기(관리 API /load)`(run.sh load 같은 설정), 그래프 「손님 요청 결과 / 초 (쌓은 막대)」, 동작 구간은 회색 띠.

## 진단 과정

| 단계 | 한 일 | 알게 된 것 |
|---|---|---|
| 대조군 (재시작) | 부하 도중 `↻ 재시작(문 닫고 공사)` 실행 (down = SIGTERM, 2초 쉼, up) | 약 3.0초 동안 초당 수만 건(최대 97,183/s)의 `refused` 가 발생했다. 듣는 소켓이 닫혀 비어 있는 동안 새 연결 시도가 커널에서 즉시 RST 로 거절되었기 때문이다. npgproxy 의 표지(UC 등)나 eof·reset 은 발생하지 않았다. |
| 변형 (설정 교체) | 부하 도중 `⇄ 설정만 바꾸기(관리 API /load)` 실행 (같은 설정 /load) | 교체 구간(1회 97ms, 2회 50ms) 동안 실패가 전혀 없이(`refused` 0) 초당 약 1,500건 성공을 유지했다. Caddy 가 새 서버를 먼저 띄우고 SO_REUSEPORT 로 듣는 소켓 공백을 없앴기 때문이다. |

## 실행 전 예측

2026-11-04 에 적었고 실행 뒤 고치지 않았다.

- 대조군:
  > 과거에 연결되어있다 … 새로다시 가동된 상태에서는 새로운 소켓끼리 다시 매핑될거니 이슈없이 진행될거 같습니다
- 보충:
  > 내려가있는동안 연결 맺으려면 모두 실패하지? (이건 8~9에서도 본거같음) // 운영체제는 UC로 받지 않을까요
- 변형:
  > 회색띠가 생기면 쌓은 막대는 연결거부로 계속 생기는거 아냐?
- 마지막 질문(맺힌 keep-alive 는 옛·새 설정 중 누가 처리하나)에 대한 답:
  > load를 누른 순간 이미 맺어진 소켓은 그대로 쓰고, 다른 소켓으로 연결해주는거 아닌가?

## 실행 (사용자가 친 명령)

`npgwatch -lab` 웹 대시보드(`http://127.0.0.1:28090`)에서 다음 순서로 버튼을 눌렀다.

1. 대조군 (재시작):
   - 백엔드 시작
   - 프록시 시작
   - 장면 E1 (`npgload -url http://127.0.0.1:8000/v1/devices/7 -c 10 -d 60s`) 선택
   - ▶ 부하 시작
   - 몇 초 뒤 `↻ 재시작(문 닫고 공사)` 한 번 (run.sh down = SIGTERM, 2초 쉼, up)
2. 변형 (설정 교체):
   - 같은 부하(`▶ 부하 시작`) 상태 유지
   - `⇄ 설정만 바꾸기(관리 API /load)` 실행 (run.sh load 같은 설정)

## 관찰

- 대조군 (재시작):
  > 회색때 refused가 발생했습니다. (내가 말한 UT,UF이런건 아예 안찍힘), 성공은 금방 돌아온거 같다 (그래프로보면 찰나라 잘 모르겠음)
  - 회색 띠가 나타난 재시작 구간(약 3.0초) 동안 대량의 `refused` 가 발생했다.
  - npgproxy 의 Envoy 호환 응답 표지(UC·UT·UF)는 전혀 찍히지 않았다.
  - 성공 막대는 재시작 완료 후 곧바로 원래 수치(초당 약 1,200건대)로 복귀했다. 그래프 세로축이 9만대로 늘어나 성공 막대가 바닥에 붙어 찰나처럼 보였다.
  - `eof` 와 `reset` 은 0건이었다.
- 변형 (설정 교체):
  > 설정만 바꾸는건 그냥 0.1초만에 끝나고 아무런 실패없이 넘어가는거 같음
  - 교체 동작은 1회 97ms, 2회 50ms 만에 완료되었다.
  - 60초 내내 초당 약 1,500건의 성공을 유지했고, `refused`·`eof`·`reset` 모두 0건으로 실패 없이 넘어갔다.

## 전후 수치

대시보드 `/api/points` 의 1초 값:

| 판 | 구간 길이 | 초당 성공 | refused |
|---|---|---|---|
| 재시작 전 | - | 1,234 | 0 |
| 재시작 1초째 | 약 3.0초 중 | 779 | 21,298 |
| 2초째 | | 0 | 97,183 |
| 3초째 | | 0 | 83,572 |
| 4초째 | | 810 | 22,502 |
| 재시작 뒤 | - | 1,269 | 0 |
| /load 1회 | 97ms | 약 1,500 (60초 내내) | 0 |
| /load 2회 | 50ms | 약 1,500 | 0 |

- `eof` 와 `reset` 은 두 판 모두 0이다.
- npgproxy 의 UC·UT·UF 도 0이다.

## 포스트모템

- 판정과 이유:
  1. **refused 가 첫 구간(손님 → Caddy)에서 난 이유와 표지**:
     - `refused` 는 "그 포트에 듣는 소켓이 없음"이다. 손님의 SYN 패킷에 커널이 RST 로 응답하고 손님은 `ECONNREFUSED` 를 받는다. `refused` 는 듣는 소켓이 비어 있던 시간을 그대로 재는 자다.
     - 요청이 Caddy 에 닿지도 못하고 첫 구간(손님 → Caddy)에서 끊어졌으므로, 뒷단인 npgproxy 까지 전달되지 않았다. 따라서 npgproxy 의 표지(UC·UT·UF, Envoy `%RESPONSE_FLAGS%` 이름을 따름, `internal/proxy/reason.go` L12)는 찍힐 수 없다. 표지는 자기가 맡은 구간의 일만 적는다. 운영체제가 주는 것은 `ECONNREFUSED` 이고 npgload 가 이를 `refused` 로 적는다. → 보충 예측의 "UC" 는 틀림.
  2. **eof·reset 이 없던 이유**:
     - `run.sh down` 은 `SIGTERM` 이고 Caddy 는 우아하게 닫는다(Go `http.Server.Shutdown`: 듣는 소켓 먼저 닫고, 쉬는 연결 닫고, 처리 중 요청이 끝나길 기다림).
     - 손님 Go 클라이언트는 다시 쓰려던 연결이 응답 전에 닫히면 GET 을 새 연결로 조용히 다시 보낸다. 그 새 연결이 refused 가 됐다. → 대조군 예측 "기존 연결은 이슈 없음, 내려간 동안 새 연결 실패"는 맞음.
  3. **refused 가 초당 수만 건인 이유**:
     - 거절은 같은 기계 커널이 즉시 RST 를 주므로 한 건이 0.1ms 안팎이다.
     - 동시 10 닫힌 루프라 리틀의 법칙 $L = \lambda W$ 로 $\lambda = 10 / 0.0001\text{s} \approx 10\text{만/s}$ 가 된다. 평소 정상 처리 시에는 $10 / 8\text{ms} \approx 1,250\text{/s}$ (실측 1,234)다.
     - 빠른 실패가 요청 수를 부풀린다(재시도 폭주의 시작). 그래프 세로축이 9만대로 늘어나 성공 막대가 바닥에 붙어 "찰나"처럼 보였다. `-c 10` 은 초당 10건이 아니라 동시 10건이다.
  4. **/load 에 refused 가 없던 이유**:
     - Caddy 는 새 설정 서버를 옛 설정이 돌고 있는 동안 먼저 띄우고, Unix 에서는 `SO_REUSEPORT` 로 같은 포트에 소켓을 하나 더 연다.
     - 듣는 소켓이 비는 순간이 없다. → 변형 예측 "연결 거부가 계속" 은 틀림.
     - 비유: 재시작은 "문 닫고 공사", /load 는 "옆에 새 문을 먼저 달고 옛 문을 뗀다".
  5. **맺힌 keep-alive 는 누가 처리하나**:
     - 연결은 그것을 받아 준 옛 서버(옛 설정) 몫이라 새 서버로 옮겨 가지 않는다.
     - 처리 중 요청은 옛 설정이 마저 답하고, 쉬는 연결은 옛 서버가 닫는다.
     - 손님이 새 연결을 열면 새 소켓이 받아 새 설정이 처리한다. → 학습자 답은 반쯤 맞음("다른 소켓으로 연결"은 맞고, "그대로 쓴다"는 다음 요청까지는 아님).
     - 이 판에서는 옛·새 설정이 같아 어느 쪽이 답했는지 화면으로 가르지 않았다(원문 근거로 판정).
  6. **한계**:
     - 한 맥북의 루프백이라 RST 가 즉시 온다. 원격 네트워크 환경이면 refused 한 건이 왕복 시간(RTT)만큼 걸려 초당 요청 수가 훨씬 작게 관측된다.

## 원문 근거

- caddy `v2.11.7` `listeners.go` L115-121: "Caddy's listeners can overlap each other … during config changes, the new config is started while the old config is still running … on Unix, SO_REUSEPORT is set except on Unix sockets"
- caddy `v2.11.7` `listen_unix.go` L216 `reusePort` 가 SO_REUSEPORT 를 켬
- caddy `v2.11.7` `modules/caddyhttp/app.go` L718 `Stop`, L791 `server.server.Shutdown(ctx)`, L136-142 `GracePeriod`
- Go `go1.25.1` `src/net/http/server.go` L3160-3162: "Shutdown works by first closing all open listeners, then closing all idle connections, and then waiting indefinitely for connections to return to idle and then shut down."
- npgload 에러 분류 `cmd/npgload/main.go` L80-105 `errKind`

## 이어 읽을 노트

- 10-02.Caddy 를 앞단에 두고 백엔드로 리버스 프록시하며 정적 파일과 자동 HTTPS 를 붙인다.md
- [ch10-e3-forwarded-for-trust.md](ch10-e3-forwarded-for-trust.md)
- [ch10-e5-internal-tls.md](ch10-e5-internal-tls.md)
- [ADR 0015](../adr/0015-caddy-edge-in-front-of-npgproxy.md)
- 이슈 #3
