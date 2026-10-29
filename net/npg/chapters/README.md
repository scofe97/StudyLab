# npg-practice

『Network Programming with Go』(Adam Woodbeck, 2021) 장별 실습입니다. 노트는 `~/runners-high/write/02_os/book/npg_network-programming-with-go/` 에 있고, 여기는 그 노트의 주장이 정말 그런지 돌려 보는 자리입니다.

## 무엇을 확인하는 랩인가

책의 Listing 을 그대로 옮기는 것이 목표가 아닙니다. 학습 세션(Phase 1 예측)에서 예측이 빗나간 지점을 실험 하나씩으로 확인합니다. 실험마다 코드를 먼저 읽고 결과를 예측한 뒤 직접 실행합니다.

| 장 | 폴더 | 실험 |
|---|---|---|
| 3 | `ch03/` | 1 context 로 연결 시도 끊기 · 2 팬아웃과 select · 3 지난 데드라인과 바쁜 루프 · 4 하트비트와 데드라인 · 5 (선택) 읽지 않는 수신자 |
| 6 | `ch06-tftp/` | TFTP 실무판(PXE·펌웨어 배포). T1 받은 펌웨어가 장비에서 깨짐 · T2 40MB 이미지만 받다가 멈춤 · T3 원격 지사 부팅이 수십 분 · T4 같은 블록이 두 번씩 나감 · T5 타임아웃·재시도 값 정하기 · T6 보안 점검 지적 · T7 랙 하나를 한꺼번에 재부팅 |
| 7~10 | `ch07-10-service/` | 사내 API 서비스 한 벌. S1~S11 은 Go 서비스 쪽(업스트림 클라이언트·서버 타임아웃·드레인·Unix 소켓 관리 채널·준비 알림), `edge/` 의 E1~E7 은 Caddy 앞단(관리 API·라우트 순서·신뢰 프록시·헬스 체크·내부 HTTPS·커스텀 모듈) |

6장과 7~10장 폴더는 장애 장면 단위입니다. 장면마다 README 에 증상·실행 전 예측·돌리는 법·볼 것·이어 읽을 노트가 있고, 결과는 직접 돌린 뒤 적습니다.

```bash
cd ~/study/npg-practice
go test ./ch03 -run TestDialContext -v -count=1
```

## 책과 다르게 한 것

- 책은 Go 1.12~1.14 기준이고, 여기는 go1.25.1 (darwin/arm64) 에서 돌립니다. 소켓 상태(`ss`) 관측이 필요한 실험만 OrbStack `ubuntu` 에서 돌립니다.
- 원문 테스트는 대부분 성공·실패만 단언합니다. 여기서는 예측과 대조하려고 에러의 타입·판별 결과·걸린 시간을 `t.Logf` 로 함께 남깁니다.
- 테스트 본문은 `// Given`, `// When`, `// Then` 으로 나눕니다.

## 실행 기록 — 3장 (2026-10-06, go1.25.1 darwin/arm64)

| 실험 | 파일·테스트 | 결과 |
|---|---|---|
| 1 context 로 연결 시도 끊기 | `dial_context_test.go` `TestDialContextDeadline`·`TestDialContextCancel` | 데드라인: 5.001s, `i/o timeout`, `Is(context.DeadlineExceeded)`·`Timeout()` true. 취소: 0s — 고루틴이 출발하기 전에 `cancel()` 이 먼저 불려 `Control` 이 돌지 않음 |
| 1-b 연결 도중 취소 | `TestDialContextCancelMidway` | 101ms 에 취소, 1.001s 에 반환 — `Control` 안의 잠은 취소를 보지 않는다(협조적 취소) |
| 2 팬아웃과 select | `fanout_test.go` `TestDialContextCancelFanOut`·`TestDialFanOutWithoutSelect` | 성공 1·취소 9 (취소가 나머지 핸드셰이크보다 먼저), select 없는 판도 10/10 끝남 |
| 2-c·2-d 취소 100ms 지연 | `…Delayed` | 미실행(학습자 선택). 별도 확인값: 성공 10/10 — `Accept` 를 한 번만 불러도 핸드셰이크는 커널이 끝낸다 |
| 3 지난 데드라인과 바쁜 루프 | `deadline_test.go` `TestDeadlineSetOnce`·`TestDeadlineResetEachRead` | 한 번만: 1초에 19,936,237회 / 매번: 5회. 둘 다 `Is(os.ErrDeadlineExceeded)` true |
| 4 하트비트 · 5 읽지 않는 수신자 | — | 미실행(학습자 선택) |
