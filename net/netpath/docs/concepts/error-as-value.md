---
title: 에러를 값으로 분류하기
tags: [netpath-lab, go, errors, classification]
status: draft
updated: 2026-09-13
---

# 에러를 값으로 분류하기
---

## 왜 메시지로 분기하면 안 되는가

> 실패 원인을 알아내려면 에러를 들여다봐야 합니다. 그런데 **에러 메시지는 어느 패키지의 계약도 아닙니다.**

닫힌 포트에 접속하면 이런 메시지가 옵니다.

```text
dial tcp 127.0.0.1:9: connect: connection refused
```

여기서 `"connection refused"` 를 찾아 분기하고 싶어집니다. 당장은 동작합니다. 그런데 이 문자열은 아무도 유지하겠다고 약속한 적이 없습니다. OS 마다 다르고, Go 버전이 올라가며 표현이 바뀔 수 있고, 로케일이 개입할 여지도 있습니다.

**더 나쁜 것은 바뀌어도 컴파일이 깨지지 않는다는 점입니다.** 테스트가 실제 요청을 하지 않으면 통과하고, 운영에서 조용히 모든 실패가 `other` 로 떨어집니다. 그러면 진단 도구가 진단을 멈춘 채로 계속 돕니다.

`syscall.ECONNREFUSED` 는 상수입니다. 이름이 바뀌면 **컴파일이 깨져서** 즉시 알게 됩니다. 조용히 틀리는 것과 시끄럽게 틀리는 것의 차이입니다.



## Go 의 에러는 값입니다

> `error` 는 메서드 하나짜리 인터페이스입니다. 그래서 에러는 특별한 제어 흐름이 아니라 **돌려받아 검사하는 값**입니다.

```go
type error interface {
	Error() string
}
```

이 한 줄이 전부입니다. 예외를 던지고 받는 문법이 따로 없고, 함수가 두 번째 반환값으로 값을 돌려줍니다. 그래서 에러를 식별하는 방법도 값을 다루는 두 가지 방법으로 갈립니다.

**sentinel** — 미리 만들어 둔 특정 값과 비교합니다.

```go
var ErrNotFound = errors.New("not found")   // 패키지가 내놓는 식별자
if errors.Is(err, ErrNotFound) { ... }
```

`io.EOF`, `context.Canceled`, `syscall.ECONNREFUSED` 가 이 부류입니다. **"무엇이 일어났는가"만 알면 되는** 경우에 씁니다.

**타입** — 구조체로 만들어 필드에 정보를 담습니다.

```go
type DNSError struct {
	IsNotFound  bool
	IsTimeout   bool
	IsTemporary bool
}
var e *net.DNSError
if errors.As(err, &e) { ... e.IsNotFound ... }
```

`*net.DNSError`, `x509.HostnameError` 가 이 부류입니다. **필드를 들여다봐야** 하는 경우에 씁니다.

둘 중 무엇을 쓸지는 만드는 쪽이 정합니다. 받는 쪽은 그 선택에 맞춰 `Is` 나 `As` 를 골라야 합니다.



## 에러는 감싸여 옵니다

> Go 1.13 부터 에러가 다른 에러를 품을 수 있습니다. 그래서 직접 비교는 거의 항상 실패합니다.

`fmt.Errorf` 의 `%w` 가 감쌉니다.

```go
fmt.Errorf("설정 읽기: %w", err)   // 체인이 이어진다
fmt.Errorf("설정 읽기: %v", err)   // 문자열로 굳는다. 안쪽을 잃는다
```

`%w` 로 감싼 에러는 `Unwrap() error` 를 갖게 되고, 그 메서드를 따라가면 안쪽이 나옵니다. 표준 라이브러리도 이렇게 층을 쌓습니다.

실제로 무엇이 오는지 재어 봤습니다(2026-09-13, Go 1.25.1).

```Bash
# [ 닫힌 포트 ]
  *url.Error         Get "http://127.0.0.1:9": dial tcp ...: connect: connection refused
  *net.OpError       dial tcp 127.0.0.1:9: connect: connection refused
  *os.SyscallError   connect: connection refused
  syscall.Errno      connection refused

# [ 없는 도메인 ]
  *url.Error         Get "https://nope.invalid": dial tcp: lookup nope.invalid: no such host
  *net.OpError       dial tcp: lookup nope.invalid: no such host
  *net.DNSError      lookup nope.invalid: no such host

# [ 자체 서명 인증서 ]
  *url.Error                          tls: failed to verify certificate: x509: ...
  *tls.CertificateVerificationError   tls: failed to verify certificate: x509: ...
  x509.UnknownAuthorityError          x509: certificate signed by unknown authority

# [ dial 타임아웃 ]
  *url.Error                        Get "http://10.255.255.1:8080": context deadline exceeded
  context.deadlineExceededError     context deadline exceeded
```

**최상위가 전부 `*url.Error` 입니다.** HTTP 클라이언트가 자기 층에서 한 번 감싸기 때문입니다. 그래서 이런 코드는 하나도 잡지 못합니다.

```go
if err == syscall.ECONNREFUSED { }          // 절대 참이 안 된다
if e, ok := err.(*net.DNSError); ok { }     // 절대 참이 안 된다
```

체인을 따라가는 함수를 써야 합니다.

| | 무엇을 찾나 | 쓰는 자리 |
|---|---|---|
| `errors.Is(err, target)` | 체인에 `target` 과 같은 **값**이 있나 | sentinel. `syscall.ECONNREFUSED`, `context.DeadlineExceeded` |
| `errors.As(err, &target)` | 체인에 그 **타입**이 있나. 찾으면 대입한다 | 타입. 필드를 봐야 할 때 |

`errors.Is` 는 체인의 각 단계에서 `==` 비교를 하고, 에러가 `Is(error) bool` 메서드를 갖고 있으면 그 판정을 존중합니다. `syscall.Errno` 가 그 메서드를 구현하고 있어서 `errors.Is(err, syscall.ECONNREFUSED)` 가 네 겹을 지나 잡힙니다.



## `errors.As` 의 타입은 정확히 맞아야 합니다

> 표준 라이브러리 안에서도 어떤 에러는 포인터로, 어떤 에러는 값으로 옵니다. 틀리면 **조용히 못 잡습니다.**

실제로 이 자리에서 한 번 틀렸습니다. `x509` 에러를 포인터로 찾았는데 분류가 안 됐습니다. (x509 분류 자체는 1국면에서 빠져 1.5국면으로 갔지만, 함정은 그대로 유효합니다.)

```go
var authErr *x509.UnknownAuthorityError    // 포인터
errors.As(err, &authErr)                   // false — 못 잡는다

var authErr x509.UnknownAuthorityError     // 값
errors.As(err, &authErr)                   // true
```

이유는 `Error()` 메서드의 receiver 입니다.

```go
func (h HostnameError) Error() string            ← 값 receiver. 값으로 온다
func (e CertificateInvalidError) Error() string  ← 값 receiver
func (e UnknownAuthorityError) Error() string    ← 값 receiver
func (e *CertificateVerificationError) Error()   ← 포인터 receiver. 포인터로 온다
```

값 receiver 로 `Error()` 를 구현하면 그 타입의 **값**이 `error` 인터페이스를 만족합니다. 그러니 체인에 들어 있는 것도 값이고, `errors.As` 는 정확히 그 타입을 찾습니다.

정리하면 이렇게 갈립니다.

| 포인터로 잡는 것 | 값으로 잡는 것 |
|---|---|
| `*net.DNSError`, `*net.OpError`, `*os.SyscallError` | `x509.HostnameError` |
| `*url.Error`, `*tls.CertificateVerificationError` | `x509.CertificateInvalidError`, `x509.UnknownAuthorityError` |

**이 실수는 컴파일도 되고 실행도 됩니다.** 분류만 조용히 빗나갑니다. 실제 요청으로 테스트하지 않으면 못 찾습니다 — 손으로 만든 가짜 에러는 내가 가정한 모양을 그대로 갖고 있어서 통과합니다.



## 타임아웃은 단계를 말해 주지 않습니다

> 가장 중요한 발견입니다. dial 타임아웃의 에러 체인에는 **`*net.OpError` 조차 없습니다.**

```text
*url.Error → context.deadlineExceededError
```

두 겹뿐입니다. "TCP 단계에서 멈췄다"는 정보가 어디에도 없습니다. 에러만 보면 DNS 에서 멈춘 것인지 TCP 인지 HTTP 인지 알 길이 없습니다.

그래서 분류 함수가 단계를 **인자로 받습니다.**

```go
func classify(stage types.Stage, err error) types.ErrorKind
```

단계는 누가 아는가. 훅 수집기가 압니다. 시작은 했는데 끝나지 않은 단계가 바로 그 자리입니다.

```go
func (s stageSpan) hung() bool { return s.started() && !s.finished() }
```

dial 이 타임아웃되면 `ConnectStart` 는 불리고 `ConnectDone` 은 불리지 않습니다(실측). 그 상태가 `hung()` 이고, 거기에 에러를 귀속시킵니다. **이 판정이 없으면 `tcp_timeout` 이 `http_timeout` 으로 보고되고, 진단이 "애플리케이션이 느리다"를 가리킵니다.** 실제로는 패킷이 도착조차 못 한 것입니다.

`duration` 대신 start·end 시각을 따로 저장한 이유가 여기서 회수됩니다. duration 하나로 접으면 "안 끝남"과 "0ms 에 끝남"이 같은 값이 되어 이 판정을 할 수 없습니다.



## 분류를 늘리는 기준

> 이름을 늘리는 것과 진단이 좋아지는 것은 다른 일입니다.

셋을 다 만족해야 늘립니다.

**하나, 값으로 구분할 근거가 있는가.** `errors.Is`/`As` 로 갈려야 합니다. 메시지를 뒤져서 늘리면 처음 문제로 돌아갑니다.

**둘, 그 구분이 다른 조치로 이어지는가.** 이게 진짜 기준입니다. `tcp_refused` 와 `tcp_timeout` 을 가르는 이유는 조치가 반대이기 때문입니다 — 앞은 앱의 포트를, 뒤는 네트워크 정책을 봅니다. 조치가 같으면 이름만 늘고 진단은 그대로입니다.

**셋, 실제로 마주쳤는가.** 그래서 `_other` 가 있습니다. **늘리기 전 대기실**입니다. 원문 메시지가 `Result.Error` 필드에 사람용으로 남으니, 무엇이 자주 오는지 보고 나서 승격하면 됩니다. 미리 만들면 쓰이지 않는 상수가 쌓입니다.

지금 대기실에 들어 있는 대표적인 것이 `ECONNRESET` 입니다. 연결이 세워진 **뒤에** 끊긴 경우인데, `tcp_refused`(연결 시점의 RST)와는 다른 사건입니다. Kubernetes 에서 사이드카가 먼저 죽거나 LB idle timeout 이 지났을 때 흔히 나옵니다. 실제로 얼마나 오는지 보고 승격을 판단합니다.
