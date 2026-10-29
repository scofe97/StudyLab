---
title: context 와 타임아웃 예산
tags: [netpath-lab, go, context, timeout, cancellation]
status: draft
updated: 2026-09-14
---

# context 와 타임아웃 예산
---

## 왜 프로브에 예산이 필요한가

> 재는 일에는 끝이 있어야 합니다. 답이 오지 않는 주소에 걸면 커널은 **2분 남짓** 재전송을 계속합니다.

리눅스 기본값이 `net.ipv4.tcp_syn_retries = 6` 이라, 아무 답이 없는 주소에 `connect()` 를 걸면 그 여섯 번을 다 소진한 뒤에야 `ETIMEDOUT` 이 옵니다. 진단 도구가 한 대상에 2분을 쓰면 쓸모가 없습니다.

그래서 **우리가 먼저 포기할 시각**을 정합니다. 그 시각을 나르는 것이 `context` 입니다.



## context 는 요청 범위의 신호를 나르는 통로입니다

> 취소 · 데드라인 · 값 셋을 한 인자로 실어 호출 사슬 아래로 내려보냅니다.

```go
func Do(ctx context.Context, target string, cfg Config) (*types.Result, error)
```

Go 에서 I/O 를 하는 공개 함수는 `context.Context` 를 **첫 인자**로 받는 것이 관례입니다. 우리가 그 안에서 부르는 `http.Client.Do` 도, 그 안의 `net.Dialer.DialContext` 도 같은 값을 받아 씁니다. **한 번 건네면 사슬 끝까지 따라갑니다.**

```go
ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
defer cancel()
```

`cancel` 을 `defer` 로 부르는 게 관례입니다. 안 부르면 타이머와 그에 딸린 goroutine 이 데드라인까지 살아 있어 **누수**가 됩니다. 이미 끝난 context 에 `cancel` 을 불러도 아무 일도 안 일어나므로, 부르는 쪽이 안전합니다.

### 데드라인은 짧은 쪽이 이깁니다

`context.WithTimeout` 은 부모의 데드라인을 존중합니다. 부모가 1초 뒤에 끝나는데 자식에게 10초를 주면, **자식도 1초에 끝납니다.**

```go
parent, _ := context.WithTimeout(context.Background(), 1*time.Second)
child, _  := context.WithTimeout(parent, 10*time.Second)   // 실제로는 1초
```

그래서 호출자가 준 예산을 우리가 초과할 걱정은 안 해도 됩니다. **문제는 반대 방향입니다** — 남은 예산이 이미 없는데 요청을 보내는 경우입니다.



## `0` 은 "제한 없음" 을 뜻하는 자리가 있습니다

> `utils.RemainingTimeout` 이 음수를 그대로 돌려주는 이유입니다.

```go
func RemainingTimeout(ctx context.Context, want time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return want
	}
	if left := time.Until(deadline); left < want {
		return left
	}
	return want
}
```

호출자의 남은 시간과 우리가 원하는 예산 중 **짧은 쪽**을 돌려줍니다. 여기서 남은 시간이 이미 지났으면 음수가 나오는데, **그것을 0 으로 다듬지 않습니다.**

```go
d := net.Dialer{Timeout: 0}   // 0 은 "타임아웃 없음" 이다. 무제한 대기
```

이미 시간이 끝났는데 `0` 을 넘기면 **무제한으로 기다리게 됩니다.** 정반대 결과입니다. 그래서 음수를 그대로 주고 호출자가 확인하게 합니다.

```go
if left := utils.RemainingTimeout(ctx, cfg.Timeout); left <= 0 {
	return nil, fmt.Errorf("데드라인이 이미 지났다: %v 남음", left)
}
```

이 검사가 없으면 요청을 보내 봐야 곧바로 `context deadline exceeded` 로 끝나고, 그 결과가 **대상의 문제처럼 보고됩니다.** 실제로는 호출자의 예산이 끝난 것인데요.

`omitempty` 가 `0` 을 "값 없음" 으로 읽어 필드를 지운 것과 같은 종류의 함정입니다. **`0` 이 특별한 뜻을 갖는 API 는 흔합니다.**



## 취소와 타임아웃은 다른 사건입니다

> 둘 다 `ctx.Err()` 로 오지만, 하나는 우리 사정이고 하나는 대상의 사정입니다.

| | 언제 | 무엇을 뜻하나 | ErrorKind |
|---|---|---|---|
| `context.DeadlineExceeded` | 정한 시각이 지남 | 대상이 시간 안에 답하지 않았다 | 단계별 `*_timeout` |
| `context.Canceled` | 누군가 `cancel()` 을 부름 | **우리가 그만뒀다** | `canceled` |

사용자가 `Ctrl+C` 를 눌러 프로브를 멈췄다고 합시다. 이건 **측정 실패가 아닙니다.** 대상은 아무 잘못이 없고 우리가 그만둔 것입니다. `tcp_timeout` 으로 보고하면 없는 문제를 대상에게 뒤집어씌우게 됩니다.

```go
if errors.Is(err, context.Canceled) {
	return types.ErrCanceled          // 단계별 timeout 으로 보내지 않는다
}
if errors.Is(err, context.DeadlineExceeded) {
	return timeoutFor(stage)
}
```

순서가 중요합니다. `context.Canceled` 를 먼저 봐야 합니다 — 취소된 context 도 `net.Error.Timeout()` 검사에 걸릴 여지가 있어서, 뒤에 두면 타임아웃으로 분류될 수 있습니다.



## 타임아웃 에러는 어느 단계였는지 말해 주지 않습니다

> 이 국면에서 가장 비싼 교훈입니다.

실제 체인을 벗겨 보면 두 겹뿐입니다.

```text
*url.Error → context.deadlineExceededError
```

`*net.OpError` 조차 없어서 **`Op: "dial"` 같은 단서가 어디에도 없습니다.** DNS 조회 중이었는지 연결 중이었는지 응답을 기다리던 중이었는지, 에러만 보면 알 수 없습니다.

반면 거부당한 경우는 네 겹이고 그 안에 단서가 있습니다.

```text
*url.Error → *net.OpError{Op:"dial"} → *os.SyscallError{Syscall:"connect"} → syscall.Errno(61)
```

그래서 타임아웃일 때는 **훅 기록에서 단계를 찾습니다.** 시작은 했는데 끝나지 않은 단계가 그 자리입니다.

```go
func (s stageSpan) hung() bool { return s.started() && !s.finished() }
```

이 판정이 없으면 `tcp_timeout` 이 `http_timeout` 으로 보고되고, 진단이 **"애플리케이션이 느리다"** 를 가리킵니다. 실제로는 패킷이 도착조차 못 한 것입니다.



## 테스트에서는 `t.Context()` 를 씁니다

> Go 1.24 부터 `*testing.T` 가 자기 context 를 줍니다. 테스트가 끝나면 자동으로 취소됩니다.

```go
got, err := Do(t.Context(), srv.URL, Config{Timeout: 5 * time.Second})
```

`context.Background()` 를 쓰면 테스트가 실패하거나 패닉으로 끝났을 때 **거기서 시작한 작업이 살아남을 수 있습니다.** `t.Context()` 는 정리 단계에서 취소되므로 그 누수가 없습니다.

그 위에 프로브 예산을 얹는 것도 그대로 됩니다 — 위에서 본 대로 **짧은 쪽이 이기기** 때문입니다.
