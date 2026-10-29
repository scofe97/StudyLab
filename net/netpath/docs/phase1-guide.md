# Phase 1 작업 순서

직접 짜면서 배우기 위한 순서다. 각 단계마다 빌드가 되고 테스트가 도는 상태를 유지한다.

## 순서

### 1. `types/error.go` — 오류 분류 상수

`ErrorKind` 타입과 상수 집합. 단계별 접두사(`dns_`, `tcp_`, `http_`)를 붙인다. `tls_` 는 1.5국면에서 더한다.

가장 중요한 건 **`tcp_refused` 와 `tcp_timeout` 을 따로 두는 것**이다. 이 도구의 존재 이유다.

- `refused` — 호스트가 RST 로 답했다. 경로는 살아 있고 포트에 아무것도 없다. 즉시 돌아온다.
- `timeout` — 아무 응답이 없다. 패킷 필터가 조용히 버렸다. 타임아웃을 전부 소진한다.

### 2. `types/result.go` — 계약

`Stage`, `StageStatus`, `StageResult`, `Result`. JSON 태그까지 여기서 확정한다 (`docs/result-schema.md` 참고). Phase 2 의 OpenTelemetry exporter 가 이 구조만 읽으므로, 여기가 흔들리면 뒤가 전부 흔들린다.

`StatusSkipped` 를 반드시 둔다. "실행되지 않음"과 "0ms 만에 성공"은 다른 사실이다.

### 3. `utils/http.go` — Transport 조립

함수형 옵션(`TransportOption func(*http.Transport)`)으로 만든다.

**여기에 함정이 하나 있다.** Go 의 `http.Transport` 는 커넥션을 풀에 재사용하는데, 재사용된 커넥션은 dial 과 handshake 를 건너뛴다. 그러면 httptrace 의 `ConnectStart`·`TLSHandshakeStart` 훅이 **아예 호출되지 않는다.** 두 번째 프로브가 "TCP 0ms" 가 아니라 "TCP 실행되지 않음" 으로 나온다.

`DisableKeepAlives` 를 켜고, `http.DefaultTransport` 를 직접 쓰지 말고 `Clone()` 해서 쓴다.

`RemainingTimeout(ctx, want)` 도 여기 둔다. 단계 예산이 전체 데드라인보다 길면 안 된다 — 안 그러면 "TCP 타임아웃" 이라고 보고하는데 실제로는 호출자의 데드라인이 끝난 것이다.

### 4. `probe/trace.go` — httptrace 수집

훅은 transport 가 쓰는 아무 goroutine 에서나 불린다. **전 필드를 mutex 로 보호한다.** `go test -race` 로 확인.

duration 이 아니라 start/end 시각을 저장한다. 끝나지 않은 단계를 구분해야 하기 때문이다.

훅 목록: `DNSStart/DNSDone`, `ConnectStart/ConnectDone`, `GotConn`, `GotFirstResponseByte`. TLS 훅은 1.5국면에서 더한다.

### 5. `probe/classify.go` — 오류 분류

`errors.Is` / `errors.As` 만 쓴다. **메시지 문자열은 절대 매칭하지 않는다.** 메시지는 어느 패키지의 계약도 아니라서 Go 버전이 바뀌면 조용히 깨진다.

쓸 것: `*net.DNSError` 의 `IsNotFound`/`IsTimeout`, `syscall.ECONNREFUSED`, `syscall.EHOSTUNREACH`, `syscall.ENETUNREACH`, `*net.AddrError`, `net.Error.Timeout()`. x509 계열은 1.5국면.

### 6. `probe/probe.go` — 실행과 조립

`Do(ctx, target, cfg) (*types.Result, error)`.

에러 반환 규칙을 정한다. **"TCP 가 거부됐다" 는 측정 결과이지 측정 실패가 아니다.** Result 안에 담는다. `error` 를 반환하는 건 프로브를 아예 실행할 수 없을 때(잘못된 URL 등)뿐이다.

단계를 순서대로 걸으면서 첫 실패 이후는 전부 `StatusSkipped` 로 만든다.

**여기서 두 번째 함정.** dial 이 타임아웃되면 `ConnectDone` 훅이 **발화하지 않는다.** 그래서 `connErr` 이 비어 있고, 에러가 HTTP 단계로 흘러가 `http_timeout` 으로 잘못 분류된다. "시작했는데 끝나지 않은 단계" 를 찾아서 거기에 에러를 귀속시켜야 한다.

### 7. `types/render.go` — waterfall 출력

`StatusSkipped` 는 duration 없이 "실행되지 않음" 으로 찍는다.

### 8. `cmd/netpath/main.go` — 플래그 연결

`--json`, `--count`, `--timeout`. 종료 코드는 0 정상 / 1 단계 실패 / 2 실행 불가. `--insecure`·`--http1.1` 은 1.5국면.

## Done Line

- [ ] 정상 대상에 waterfall 3단계가 나온다
- [ ] 닫힌 포트 → `tcp_refused`, 이후 단계 "실행되지 않음"
- [ ] 없는 도메인 → `dns_not_found`
- [ ] 드롭되는 주소 → `tcp_timeout` (**`http_timeout` 아님**)
- [ ] `--count 2` 로 두 번 재도 같은 단계 수가 나온다
- [ ] `make check` 통과 (vet + gofmt + 코어 경계 + `test -race`)

## 검증에 쓸 대상

| 목적 | 대상 |
|---|---|
| 정상 | `http://example.com` |
| refused | `http://127.0.0.1:9` |
| NXDOMAIN | `https://nope.invalid` |
| timeout | `http://10.255.255.1:8080 --timeout 3s` |
| 포트 범위 초과 | `http://127.0.0.1:99999` |

## 막히면

`go doc net/http/httptrace.ClientTrace` 로 훅 시그니처를 본다. 표준 라이브러리만 쓰므로 답은 전부 `go doc` 안에 있다.

패턴 참고는 `~/study/portal-tunnel` 의 `utils/http.go`(함수형 옵션·Transport Clone), `utils/network.go`(잔여 타임아웃 계산). `utils/tls.go`(인증서 체인 추출)는 1.5국면에서 본다.
