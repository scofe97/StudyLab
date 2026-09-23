# concepts — 이 프로젝트에 적용한 개념 정리

과정 기록이 아닙니다. **프로젝트를 만들며 적용하려 했던 개념을 나중에 다시 보기 위해** 남깁니다. 그래서 축이 작업 단계가 아니라 개념입니다.

## 작성 규약

`writing-method` 스킬을 따릅니다. 합니다체, H2 첫 블록에 `>` 한 줄 요약, 절은 정의가 아니라 문제로 열기, 작성 후 7부 센서 실행.



## 두 종류가 있습니다

| 종류 | 언제 쓰나 | 무엇을 담나 |
|---|---|---|
| **개념 문서** | 모르는 용어·개념이 나올 때 **그때 바로** | 그 개념이 무엇이고 이 프로젝트의 어디에 걸리는가 |
| **정리 문서** | 1국면 여덟 단계를 **마친 뒤** | 실제로 내린 설계 판단과 어디서 걸렸는가 |

개념 문서를 미루면 학습이 막힙니다 — 낱말에서 막히면 그건 회상 훈련이 아니라 정보 부족입니다. 반대로 정리 문서를 만들기 전에 쓰면 책에서 읽은 것을 옮기게 되고, 만든 뒤에 써야 "실제로 어디서 걸렸는가"가 들어갑니다.



## 이후 이관 (미확정)

정리가 끝나면 `~/runners-high/write/` 로 옮기는 것을 검토합니다. 옮길 때는 그쪽 카테고리 규약과 프론트매터를 따라야 하므로 `writing-method` 5부(파일 배치)를 그때 함께 읽습니다.



## 개념 문서 (작성됨)

| 문서 | 담긴 개념 |
|---|---|
| [terms-request-path.md](network/terms-request-path.md) | dial 과 **소켓 API**(socket·bind·listen·accept·connect) · `net.OpError` 의 `Op` 값 · handshake(3-way·FIN·RST) · keepalive 와 커넥션 풀 · TTFB · 리다이렉트 · 프록시 터널 · discard 포트 9 |
| [go-http-client.md](network/go-http-client.md) | Client·Transport·RoundTripper 3층 · DefaultTransport 전역 문제 · Clone · httptrace 훅 16개 · keep-alive 두 종류 · 함수형 옵션 |
| [error-as-value.md](error-as-value.md) | error 인터페이스 · sentinel 과 타입 · `%w` 래핑 · `errors.Is`/`As` · 실측한 체인 넷 · 값 receiver 함정 · 타임아웃이 단계를 안 말하는 문제 |
| [go-concurrency-race.md](go-concurrency-race.md) | goroutine · 데이터 레이스 3조건 · `sync.Mutex` 와 `defer` 관용구 · 읽기도 잡아야 하는 이유 · 락의 성능 비용 · `-race` 탐지기 |
| [go-nil-and-pointer.md](go-nil-and-pointer.md) | zero value · 포인터로 부재 표현 · omitempty 와의 관계 · 판단 기준 셋 · 다른 언어와 비교 |

네트워크 계열 3개(`terms-request-path.md`, `network-signal-to-error-value.md`, `go-http-client.md`)는 `network/` 하위로 묶여 있습니다.



## 정리 문서 목록 (8단계 후)

`direction.md` 가 "go-roadmap 기준으로 이 프로젝트가 걸치는 곳은 넷"이라고 지목한 2·4·7·11단계를 축으로, 프로젝트 고유 판단 셋을 더합니다.

| 문서 | 다루는 개념 | 어느 단계에서 걸리는가 |
|---|---|---|
| `network-signal-to-error-value.md` | 네트워크에서 일어난 일이 errno 를 거쳐 Go 에러 값이 되기까지. 관측 층위와 그 한계 | 5단계 (go-roadmap 11단계) |
| `httptrace-stage-boundary.md` | 표준 클라이언트 안에서 단계 경계를 관측하는 법. 훅이 안 불리는 경우들 | 3·4단계 |
| `context-and-timeout.md` | 데드라인 합성. 커널 타임아웃과 호출자 타임아웃의 구분 | 3·6단계 (go-roadmap 4단계) |
| `hooks-and-concurrency.md` | 훅이 임의 goroutine 에서 불린다는 사실과 그 결과. `-race` | 4단계 (go-roadmap 7단계) |
| `contract-package-design.md` | `types/` 를 유일한 계약으로 두는 설계. 의존 방향과 `make lint` 강제 | 1·2단계 |

목록은 확정이 아닙니다. 만들면서 실제로 걸린 개념이 생기면 늘리고, 안 걸린 것은 뺍니다.



## 현재 재료

| 파일 | 상태 |
|---|---|
| `_draft-network-signal-to-error-value.md` | 초안. 대화에서 나온 표를 모아 둔 것이라 아직 writing-method 규약 미적용. 정식 작성 때 재료로 쓰고 교체합니다 |

`_` 접두사는 아직 정식 문서가 아니라는 표시입니다.
