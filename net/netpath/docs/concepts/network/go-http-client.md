---
title: Go HTTP 클라이언트의 내부와 httptrace
tags: [netpath-lab, go, http, httptrace]
status: draft
updated: 2026-09-13
---

# Go HTTP 클라이언트의 내부와 httptrace
---

## 왜 내부를 알아야 하는가

> `netpath` 는 표준 클라이언트를 **바꾸지 않고** 그 안의 단계 경계를 관측합니다. 훅이 어디서 불리는지 모르면 무엇이 측정되고 무엇이 빠지는지 알 수 없습니다.

직접 소켓을 열어 HTTP 를 손으로 짜면 단계를 나누기는 쉽습니다. 그런데 그러면 실제 서비스가 쓰는 경로와 다른 것을 재게 됩니다. 리다이렉트 처리, HTTP/2 협상, 프록시 규칙이 전부 빠지기 때문입니다. **실제와 같은 경로를 재려면 표준 클라이언트를 써야 하고, 그 안을 보려면 구조를 알아야 합니다.**



## 세 층으로 되어 있습니다

> `Client` 가 정책을 정하고, `Transport` 가 실제로 보내고, `RoundTripper` 가 그 둘을 잇는 약속입니다.

```text
  http.Client          정책 — 타임아웃, 리다이렉트 따라가기, 쿠키
       |
       | 요청을 넘긴다
       v
  http.RoundTripper    인터페이스 — "요청 하나를 응답 하나로 바꾼다"
       |
       | 기본 구현
       v
  http.Transport       실제 일 — dial, TLS, 커넥션 풀, HTTP/1·2 프로토콜
       |
       v
  net.Dialer  →  소켓  →  네트워크
```

`RoundTripper` 는 메서드 하나뿐인 인터페이스입니다.

```go
type RoundTripper interface {
	RoundTrip(*Request) (*Response, error)
}
```

이 한 줄이 경계입니다. `Client` 는 이 인터페이스만 알고 `Transport` 의 내부를 모릅니다. 그래서 테스트에서 가짜 구현을 끼울 수 있고, 프록시·캐시·계측 계층을 감쌀 수 있습니다.

일이 어디서 벌어지는지가 중요합니다. **우리가 재려는 dial·TLS·커넥션 풀은 전부 `Transport` 안**입니다. `Client` 를 아무리 설정해도 그 안은 안 보입니다. 그래서 `Transport` 를 우리가 만들어 넘기고, `httptrace` 로 안을 들여다봅니다.



## `http.DefaultTransport` 는 전역 변수입니다

> 편의를 위해 패키지 수준에 하나 만들어 둔 것이고, **프로세스 전체가 같은 것을 공유합니다.**

실제 선언이 이렇습니다.

```go
var DefaultTransport RoundTripper = &Transport{
	Proxy: ProxyFromEnvironment,
	DialContext: defaultTransportDialContext(&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}),
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}
```

`var` 이고 포인터입니다. 그래서 필드를 고치면 **그 프로세스에서 이 Transport 를 쓰는 모든 코드가 영향을 받습니다.**

```go
// 하면 안 되는 것
t := http.DefaultTransport.(*http.Transport)
t.DisableKeepAlives = true   // 이 프로세스 전체의 keepalive 가 꺼진다
```

`http.DefaultClient` 가 이것을 쓰고, `http.Get` 같은 패키지 함수도 이것을 씁니다. 우리가 의존하는 라이브러리가 `http.Get` 으로 무언가를 받아 온다면 그 요청까지 매번 새 연결을 세우게 됩니다. **측정 도구가 자기 프로세스의 다른 부분을 느리게 만드는** 셈이고, 더 나쁜 것은 그 사실이 어디에도 드러나지 않는 점입니다.

전역 변수를 고쳐 쓰는 문제의 전형입니다. 고친 쪽은 자기 코드만 본다고 생각하는데, 실제 영향 범위는 프로세스 전체입니다.



## `Clone()` — 설정은 물려받고 상태는 내 것으로

> 기본 설정을 그대로 쓰고 싶지만 전역을 건드리고 싶지 않을 때 쓰는 메서드입니다.

```go
t := http.DefaultTransport.(*http.Transport).Clone()
t.DisableKeepAlives = true   // 이제 우리 것만 바뀐다
```

`Clone()` 은 **설정 필드를 복사한 새 `*Transport` 를 돌려줍니다.** 프록시 규칙, 타임아웃, HTTP/2 설정 같은 것이 그대로 넘어옵니다. 처음부터 `&http.Transport{}` 를 만들면 이 값들이 전부 zero value 라서, 프록시 설정이 빠지고 HTTP/2 협상도 안 됩니다.

중요한 것은 **커넥션 풀은 복사되지 않는다**는 점입니다. 새 Transport 는 빈 풀로 시작합니다. 설정만 물려받고 상태는 자기 것을 갖습니다.

"클래스의 인스턴스를 만드는 것 같다"고 하셨는데 방향이 맞습니다. 다만 정확히는 **기존 인스턴스의 설정을 베껴 새 인스턴스를 만드는 것**입니다. Java 로 치면 `new Transport()` 가 아니라 복사 생성자에 가깝습니다.

그리고 `Clone()` 이 물려받는 것 중 하나를 알아 둘 만합니다. `Proxy: ProxyFromEnvironment` 는 **`HTTP_PROXY`·`HTTPS_PROXY`·`NO_PROXY` 환경변수를 읽습니다.** 프록시가 설정된 환경에서 측정하면 우리가 재는 TCP·TLS 가 프록시까지의 것이 됩니다. 사내망에서 측정할 때 결과가 이상하면 이 환경변수를 먼저 봐야 합니다.



## `httptrace` — 표준 클라이언트 안을 들여다보는 창

> Go 1.7(2016)에 들어온 표준 패키지입니다. 요청 하나가 지나가는 동안 단계마다 콜백을 불러 줍니다.

동작 방식이 특이합니다. Transport 를 설정하는 게 아니라 **`context` 에 콜백 묶음을 실어 보냅니다.**

```go
trace := &httptrace.ClientTrace{
	DNSStart:     func(info httptrace.DNSStartInfo) { /* ... */ },
	DNSDone:      func(info httptrace.DNSDoneInfo)  { /* ... */ },
	ConnectStart: func(network, addr string)        { /* ... */ },
	ConnectDone:  func(network, addr string, err error) { /* ... */ },
}

ctx := httptrace.WithClientTrace(req.Context(), trace)
req = req.WithContext(ctx)
```

`Transport` 가 요청을 처리하면서 context 에서 이 묶음을 꺼내 해당 시점에 함수를 부릅니다. **요청 하나에 딱 붙는 계측**이라, 같은 Transport 로 다른 요청을 보내도 서로 섞이지 않습니다. `context` 를 "요청 범위 값 전달"에 쓰는 대표 사례입니다.

`net/http/httptrace` 는 2016년에 들어온 뒤 훅 구성이 거의 그대로입니다. `direction.md` 가 코어를 여기에만 기대게 한 이유이기도 합니다 — 10년째 안 바뀐 API 위에 세우면 낡을 일이 적습니다.

### 훅은 열여섯 개, 우리가 쓸 것은 여덟 개

`go doc net/http/httptrace.ClientTrace` 로 확인한 전체 목록입니다.

| 훅 | 언제 | 우리가 쓰나 |
|---|---|---|
| `GetConn` | 연결을 구하기 시작 | |
| `GotConn` | 연결을 얻음 (재사용 여부가 여기 실린다) | **쓴다** |
| `PutIdleConn` | 연결을 풀에 반납 | |
| `DNSStart` · `DNSDone` | 이름 조회 시작·끝 | **쓴다** |
| `ConnectStart` · `ConnectDone` | dial 시작·끝 | **쓴다** |
| `TLSHandshakeStart` · `TLSHandshakeDone` | TLS handshake 시작·끝 | 1.5국면 |
| `WroteHeaderField` · `WroteHeaders` | 요청 헤더 전송 | |
| `WroteRequest` | 요청 전송 완료 | 확장 후보 (TTFB 분리) |
| `GotFirstResponseByte` | 응답 첫 바이트 도착 | **쓴다** |
| `Got100Continue` · `Wait100Continue` · `Got1xxResponse` | 1xx 중간 응답 | |

`GotConn` 이 주는 정보에 `Reused bool` 이 있습니다. 재사용된 연결인지 클라이언트가 직접 알려주는 것이라, 우리 `Result.TCP.Reused` 가 이 값을 그대로 받습니다.

### 훅이 불리지 않는 경우들

이게 함정의 뿌리입니다. **불리지 않은 것과 0ms 에 끝난 것은 다른 사실**인데, 훅만 보면 둘이 같아 보입니다.

| 상황 | 안 불리는 훅 | 왜 |
|---|---|---|
| 커넥션 재사용 | `ConnectStart/Done`, `TLSHandshake*` | dial 과 handshake 를 건너뛴다 |
| IP 를 직접 지정 | `DNSStart/Done` | 조회할 이름이 없다 |
| 평문 HTTP | `TLSHandshake*` | TLS 를 안 한다 |
| **dial 이 타임아웃** | `ConnectDone` | 시작은 했는데 끝나지 않았다 |

마지막 줄이 6단계의 함정입니다. 문서만 보면 추측이라 **직접 재어 확인했습니다**(2026-09-13, Go 1.25.1).

```text
[ DROP 되는 주소, --timeout 3s ]          [ 닫힌 포트 127.0.0.1:9 ]

0.000s  GetConn                           0.000s  GetConn
0.001s  ConnectStart                      0.000s  ConnectStart
          (3초 침묵)                       0.001s  ConnectDone(err=connection refused)
3.001s  Do() 반환: context deadline        0.001s  Do() 반환: connection refused
                 exceeded
              ↑
      ConnectDone 이 없다
```

**refused 는 `ConnectDone` 에 에러가 실려 오고, timeout 은 `ConnectDone` 자체가 없습니다.** 이것이 두 실패를 코드에서 가르는 기계적 차이입니다.

정상 요청은 훅이 이 순서로 옵니다.

```text
0.000s  GetConn(example.com:443)
0.000s  DNSStart              0.013s  DNSDone([172.66.147.243, 104.20.23.154, 2606:4700:10::6814:179a, ...])
0.013s  ConnectStart          0.029s  ConnectDone(err=<nil>)
0.029s  TLSHandshakeStart     0.059s  TLSHandshakeDone(err=<nil>)
0.059s  GotConn(reused=false)
0.059s  WroteRequest          0.086s  GotFirstResponseByte        0.086s  200 OK
```

두 가지를 더 알 수 있습니다. **`GotConn` 이 TLS 뒤에 옵니다** — `TLSHandshakeDone` 과 같은 시각입니다. 그러니 `GotConn` 을 TCP 단계의 끝으로 쓰면 TLS 시간까지 삼킵니다. TCP 의 끝은 `ConnectDone` 이고, `GotConn` 은 `Reused` 와 주소를 얻는 용도입니다.

그리고 **DNS 가 주소 넷을 줬는데 `ConnectStart` 는 한 번만 불렸습니다.** 첫 주소가 성공했기 때문입니다. 그 주소가 죽어 있으면 여러 번 불립니다.

앞 표의 마지막 줄을 다시 보면, `ConnectStart` 는 불렸고 `ConnectDone` 은 안 불렸습니다. 그래서 TCP 단계에 에러가 안 남고, 에러가 HTTP 단계까지 흘러가 `http_timeout` 으로 분류됩니다.

해결의 실마리는 `trace.go` 가 **duration 이 아니라 start·end 시각을 따로 저장**하는 데 있습니다. start 는 있는데 end 가 없는 단계, 즉 "시작했는데 끝나지 않은 단계"를 찾아 그곳에 에러를 귀속시킵니다.

그리고 `ConnectStart` 는 **여러 번 불릴 수 있습니다.** DNS 가 주소를 여럿 돌려주면 Go 가 차례로 시도하기 때문입니다. 첫 IP 가 죽어 있으면 두 번, 세 번 불립니다. 한 번만 온다고 가정한 코드는 여기서 틀립니다.



## 같은 이름 다른 것 — keep-alive 가 둘입니다

> `DefaultTransport` 안에 `DisableKeepAlives` 와 `Dialer.KeepAlive` 가 따로 있습니다. 이름이 닮았지만 다른 층의 기능입니다.

| | 무엇인가 | 어느 층 |
|---|---|---|
| `DisableKeepAlives` | HTTP 연결을 **재사용할지** | HTTP (애플리케이션) |
| `net.Dialer.KeepAlive` | 유휴 TCP 연결에 **살아 있냐고 물어보는 주기** | TCP (전송) |

`Dialer.KeepAlive = 30 * time.Second` 는 연결이 30초간 조용하면 빈 패킷을 보내 상대가 살아 있는지 확인한다는 뜻입니다(`SO_KEEPALIVE`). 죽은 연결을 빨리 알아채려는 장치입니다.

우리가 끄는 것은 **앞의 것**입니다. 연결 재사용을 막아 매번 dial 과 handshake 가 일어나게 합니다. 뒤의 것은 그대로 둡니다 — 매번 새로 연결하니 유휴 연결이 생기지 않아 무관합니다.



## 함수형 옵션 — 3단계에서 쓸 형태

> 설정 가능한 것을 만들 때 Go 에서 자주 쓰는 형태입니다. 인자를 계속 늘리지 않고 함수로 받습니다.

```go
type TransportOption func(*http.Transport)

func WithInsecure() TransportOption {
	return func(t *http.Transport) {
		t.TLSClientConfig.InsecureSkipVerify = true
	}
}

func NewTransport(opts ...TransportOption) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableKeepAlives = true          // 측정을 위해 항상 끈다
	for _, opt := range opts {
		opt(t)
	}
	return t
}
```

쓰는 쪽이 이렇게 됩니다.

```go
t1 := NewTransport()                          // 기본
t2 := NewTransport(WithInsecure())            // 인증서 검증 없이
t3 := NewTransport(WithInsecure(), WithHTTP1()) // 둘 다
```

**왜 이 형태인가.** 설정이 늘어날 때 기존 호출을 안 고쳐도 됩니다. 인자 목록으로 받으면 `NewTransport(false, true, 30*time.Second)` 처럼 되어 읽을 수 없고, 하나 추가할 때마다 모든 호출 지점을 고쳐야 합니다. 설정 구조체를 받는 방법도 있지만, 그러면 "기본값을 원한다"와 "그 필드에 zero value 를 원한다"가 구분되지 않습니다. `omitempty` 에서 본 것과 같은 문제입니다.

**항상 켜야 하는 것은 옵션으로 두지 않습니다.** `DisableKeepAlives` 는 옵션이 아니라 `NewTransport` 안에 고정합니다. 끄면 측정이 사라지므로 선택지로 두면 안 됩니다. 옵션은 **정당한 선택이 둘 이상 있을 때만** 만듭니다.
