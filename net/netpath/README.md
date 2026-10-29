# NetPath Lab

Pod 에서 목적지까지 요청을 보내고 `DNS → TCP → HTTP` 각 단계의 지연과 실패를 관측해, **어디서 왜 실패했는지** 설명하는 도구. TLS 단계는 1.5국면에서 더한다.

## 상태

**1국면(측정 모델) 완료.** 여덟 단계를 다 밟았고 Done Line 여섯 줄이 실측으로 통과했습니다.

```bash
make build                              # bin/netpath
./bin/netpath http://example.com        # waterfall
./bin/netpath --json http://127.0.0.1:9 # Result JSON (한 줄에 하나)
make check                              # vet + gofmt + 코어 경계 + test -race
```

| 단계 | 파일 | 무엇 |
|---|---|---|
| 1 | `types/error.go` | `ErrorKind` 13개 |
| 2 | `types/result.go` | `Stage`·`StageStatus`·`StageResult`·`Result` |
| 3 | `utils/http.go` | `NewTransport`(재사용 차단)·`RemainingTimeout` |
| 4 | `probe/trace.go` | 훅 수집기, `hung()` 판정 |
| 5 | `probe/classify.go` | `errors.Is`/`As` 분류 |
| 6 | `probe/probe.go` | 조립, 에러 귀속 |
| 7 | `types/render.go` | waterfall |
| 8 | `cmd/netpath/main.go` | 플래그·종료 코드 |

**TLS 는 [1.5국면](docs/direction.md)으로 분리했습니다.** 1국면은 평문 경로만 잽니다. `https://` 대상도 잴 수 있지만 handshake 시간이 어느 단계에도 안 잡히므로, 그 사실을 화면이 직접 말합니다.

### 알려진 한계

- **간헐적 실패를 스크립트가 못 잡습니다.** `--count 3` 에서 한 번만 실패하면 종료 코드가 0 입니다(`ping` 과 같은 규칙). 사람은 요약 줄로 보지만 스크립트는 놓칩니다. 필요해지면 `--fail-on-any` 를 더합니다.
- **`https://` 의 TLS 시간이 빠집니다.** 1.5국면에서 채웁니다.

### 마치고 할 것

- [x] [gosuda/gosuda-template-go 의 AGENTS.md](https://github.com/gosuda/gosuda-template-go/blob/main/AGENTS.md) 대조 완료. 그쪽은 조직 차원의 Go 코딩 규약이고 우리 `AGENTS.md`(=`portal-tunnel` 계열)는 프로젝트 행동 규칙이라 보완 관계다. 반영한 것 — `make build` 에 `CGO_ENABLED=0`, `make cover`·`make vuln` 추가, 테스트를 `t.Context()` 로. 미뤄 둔 것 — `ErrorKind` 상수의 `Err*` 접두사(Go 관례상 `Err*` 는 `error` 값인데 우리는 문자열 분류다). 1.5국면에서 TLS 상수를 더할 때 함께 판단한다.
- `docs/concepts/` 에 정리 문서를 쓴다. 계획은 [concepts/README.md](docs/concepts/README.md).

### 진행 방식

구술로 요구사항을 받아 코드를 씁니다. 단계마다 **설명 → 판단·요구사항 → 코드** 순서를 지킵니다. 모르는 개념이 나오면 `docs/concepts/` 에 그때 바로 문서를 씁니다.

## 목표 출력

```text
http://example.com

DNS       14ms  성공: 172.66.147.243, 104.20.23.154, 2606:4700:10::ac42:93f3
TCP       34ms  성공: 172.66.147.243:80
HTTP      47ms  성공 (200, HTTP/1.1)
                Total 96ms
```

실패하면 멈춘 지점과 그 이후 실행되지 않은 단계가 함께 보입니다.

```text
http://127.0.0.1:9

DNS          -  실행되지 않음
TCP      0.2ms  실패: tcp_refused (dial tcp 127.0.0.1:9: connect: connection refused)
HTTP         -  실행되지 않음
                Total 0.2ms

실패 단계: tcp (tcp_refused)
```

끝나지 않은 단계는 숫자가 아니라 `-` 입니다. dial 이 타임아웃되면 `ConnectDone` 훅이 발화하지 않아 시간이 없는데, `0.0ms` 로 찍으면 3초를 기다린 사실이 사라집니다.

## 왜 단계를 나누는가

"연결이 안 된다" 는 진단이 아니다. 같은 실패처럼 보여도 신호가 다르다.

| 신호 | 뜻 | Kubernetes 에서 흔한 원인 |
|---|---|---|
| `tcp_refused` (즉시) | 호스트가 RST 응답. 경로는 살아 있고 포트에 아무것도 없음 | Pod 는 떴는데 앱이 포트를 안 열었다 |
| `tcp_timeout` (전체 소진) | 아무 응답 없음. 패킷이 조용히 버려짐 | NetworkPolicy DROP |

이 둘을 "연결 실패" 하나로 뭉치면 진단이 반대 방향을 가리킨다. 그래서 단계와 오류 종류를 값으로 구분해 기록한다.

## 구조

```
types/    Result 스키마와 오류 코드 — 유일한 안정 계약
utils/    상태 없는 공용 변환 (Transport 옵션, 타임아웃 합성)
probe/    코어 측정 — Kubernetes·OTel 을 모른다
cmd/      CLI
docs/     phase1-guide.md (작업 순서), result-schema.md (JSON 계약)
```

`probe/` 가 외부 기술을 모르는 상태를 유지하는 게 핵심이다. `make lint` 가 이 경계를 강제한다.

## 문서

| 문서 | 내용 |
|---|---|
| [`docs/direction.md`](docs/direction.md) | **프로젝트 방향성** — 국면 다섯, 각 자리에 끼울 수 있는 확장, 옆 프로젝트와의 경계 |
| [`docs/phase1-guide.md`](docs/phase1-guide.md) | 1국면 작업 순서, 함정 두 가지, Done Line |
| [`docs/result-schema.md`](docs/result-schema.md) | JSON 계약 — 2국면이 이 위에 붙는다 |
| [`docs/concepts/`](docs/concepts/README.md) | 만들며 정리한 개념 — 요청 경로 용어, Go HTTP 클라이언트 내부, 포인터와 nil, 에러를 값으로 |
| [`AGENTS.md`](AGENTS.md) | 행동 규칙 |
