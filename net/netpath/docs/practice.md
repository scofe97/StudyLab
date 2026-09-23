---
title: netpath 실습 — 현상에서 코드로, Go 를 익히며
tags: [netpath-lab, practice, go]
status: draft
updated: 2026-09-17
---

# netpath 실습 — 현상에서 코드로, Go 를 익히며
---

## 이 실습이 하는 일

> 실패 상황을 **직접 만들어** 도구에 물리고, 그 출력이 나온 **코드 경로를 역추적**합니다. 그 과정에서 쓰인 Go 문법과 관용구를 그 자리에서 익힙니다.

목표가 둘입니다 — netpath 가 어떻게 판단하는지 이해하는 것과, Go 에 익숙해지는 것. 그래서 각 회차가 세 겹입니다.

| 겹 | 하는 일 |
|---|---|
| **현상** | 상황을 만들어 도구에 물리고 출력을 본다 |
| **코드** | 그 출력이 나온 자리를 코드에서 찾는다 |
| **Go** | 그 자리에 쓰인 문법·관용구를 익힌다 |

### 진행 규약 — 예측이 먼저입니다

명령을 보기 전에 **결과를 먼저 예측해 적습니다.** 예측 없이 실행하면 화면에 뜬 것을 그냥 받아들이게 되고, 그건 관찰이지 학습이 아닙니다. 빗나간 예측이 가장 좋은 재료입니다.

한 번에 **한 회차씩** 갑니다. 해석까지 마쳐야 다음으로 넘어갑니다. 용어가 막히면 그 자리에서 물어보세요 — 모르고 넘어가면 뒤가 전부 막힙니다.

```bash
cd ~/study/netpath-lab && make build
```



## 회차 1 — 포트에 아무도 없을 때

> 가장 단순한 실패입니다. 여기서 `Result` 라는 자료구조 전체와 Go 의 구조체·JSON 태그를 봅니다.

### 현상

포트 9 는 discard 포트라 아무것도 듣고 있지 않다는 것이 사실상 보장됩니다.

```bash
./bin/netpath http://127.0.0.1:9
./bin/netpath --json http://127.0.0.1:9 | jq
```

볼 것 — DNS 단계에 무엇이 찍혔나요? 왜 그럴까요? (`127.0.0.1` 은 이름이 아니라 주소입니다)

### 코드

출력을 만든 자리를 찾습니다.

| 무엇 | 어디 |
|---|---|
| `tcp_refused` 라는 이름이 붙는 자리 | `probe/classify.go` |
| 단계별 상태(`ok`/`failed`/`skipped`)를 정하는 자리 | `probe/probe.go` 의 `assemble` |
| 화면에 그리는 자리 | `types/render.go` 의 `Waterfall` |

### Go

`types/result.go` 를 엽니다. 이 파일은 Go 의 자료구조 표현이 압축돼 있습니다.

```go
type Stage string          // ← string 인데 왜 새 타입을 만들까?

type StageResult struct {
    Stage  Stage  `json:"stage"`        // ← 백틱 안의 것은 무엇인가?
    Error  string `json:"error,omitempty"`  // ← omitempty 는?
}

type Result struct {
    DNS *DNSInfo `json:"dns,omitempty"` // ← 왜 포인터(*)인가?
}
```

익힐 것 — **named type**(`type Stage string`), **struct tag**(백틱), **포인터와 값의 차이**.



## 회차 2 — 패킷이 조용히 버려질 때

> 같은 "연결 실패" 인데 앞 회차와 다른 이름이 붙습니다. 그 차이가 이 도구의 존재 이유입니다.

### 현상

```bash
./bin/netpath --timeout 2s http://10.255.255.1:8080
```

볼 것 — TCP 단계의 **시간 자리**에 무엇이 찍혔나요? 회차 1 과 비교하세요.

### 코드

`probe/trace.go` 의 `stageSpan` 이 왜 `duration` 하나가 아니라 `start`·`end` 둘을 갖는지, 그리고 `hung()` 이 무엇을 판정하는지 봅니다. `probe/probe.go` 의 `blame()` 이 이 사실을 씁니다.

### Go

```go
func (s stageSpan) hung() bool { return s.started() && !s.finished() }
```

익힐 것 — **메서드 리시버**(`func (s stageSpan)`), **값 리시버 vs 포인터 리시버**, 짧은 함수의 한 줄 본문.



## 회차 3 — 이름이 없을 때

> 실패가 TCP 가 아니라 DNS 에서 납니다. 단계가 앞으로 당겨지면 뒤 단계가 어떻게 되는지 봅니다.

### 현상

```bash
./bin/netpath http://없는도메인.invalid
```

### 코드

`classify.go` 의 DNS 분기가 `errors.As` 를 쓰는 이유 — `*net.DNSError` 가 `IsNotFound`·`IsTimeout` 플래그를 들고 있기 때문입니다.

### Go

```go
var dnsErr *net.DNSError
if errors.As(err, &dnsErr) { ... }
```

익힐 것 — **`errors.Is` 와 `errors.As` 의 차이**, **에러 래핑(`%w`)**, 포인터 변수 선언과 `&`.



## 회차 4 — 서버가 응답은 하는데 실패일 때

> 지금까지는 경로가 끊긴 실패였습니다. 이번엔 **경로는 살아 있고 내용이 실패**입니다.

### 현상

로컬에 서버를 하나 띄웁니다. 이 파일을 직접 쓰는 것이 이번 회차의 Go 실습입니다.

```bash
mkdir -p /tmp/srv && cd /tmp/srv
# main.go 를 작성한다 (아래 Go 절 참고)
go run main.go &
cd ~/study/netpath-lab
./bin/netpath http://127.0.0.1:8080/500
```

### 코드

`assemble` 의 이 분기를 찾습니다.

```go
case s.stage == types.StageHTTP && resp != nil && resp.StatusCode >= 400:
```

왜 "경로는 살았고 내용이 실패" 를 별도 상태로 두는지 생각해 보세요.

### Go

작은 HTTP 서버를 직접 씁니다.

```go
package main

import "net/http"

func main() {
    http.HandleFunc("/500", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(500)
    })
    http.ListenAndServe(":8080", nil)
}
```

익힐 것 — **함수를 값으로 넘기기**(익명 함수), `http.HandlerFunc` 의 시그니처, 패키지 레벨 함수 호출.



## 회차 5 — 서버가 느릴 때

> 타임아웃이 어느 단계에 귀속되는지 봅니다. 회차 2 와 같은 `timeout` 인데 단계가 다릅니다.

### 현상

회차 4 의 서버에 느린 핸들러를 더해 물려봅니다.

### 코드

`utils.RemainingTimeout` 과 `probe.Do` 의 `context.WithTimeout` 이 어떻게 맞물리는지, `classify` 가 타임아웃을 어느 단계로 보내는지(`timeoutFor`) 추적합니다.

### Go

```go
ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
defer cancel()
```

익힐 것 — **`context` 의 취소 전파**, **`defer`**, 다중 반환값(`ctx, cancel = ...`).



## 회차 6 — 정상 경로

> 마지막에 성공을 봅니다. 실패를 먼저 본 뒤라 성공 출력의 각 숫자가 어디서 왔는지 이미 알고 있을 것입니다.

### 현상

```bash
./bin/netpath http://example.com
./bin/netpath --json http://example.com | jq
```

볼 것 — `Total` 이 세 단계의 합과 다릅니다. 차이가 어디서 나올까요?

### 코드

`Waterfall` 의 `Total` 계산과 `Result.Total` 이 어떻게 재어지는지(`probe.Do` 의 `time.Since`) 봅니다.

### Go

```go
var b strings.Builder
fmt.Fprintf(&b, "%s\n\n", r.Target)
```

익힐 것 — **`strings.Builder`** 를 쓰는 이유(문자열 `+=` 와의 차이), `fmt.Fprintf` 의 `io.Writer` 인자.
