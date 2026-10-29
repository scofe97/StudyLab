# Result JSON 스키마

`netpath --json` 의 출력이자 이 도구의 유일한 안정 계약이다. Phase 2 의 OpenTelemetry exporter 는 이 구조만 읽는다. 정의는 `types/result.go`.

## 최상위

| 필드 | 타입 | 의미 |
|---|---|---|
| `target` | string | 정규화된 대상 URL |
| `started_at` | RFC3339 | 측정 시작 시각 |
| `stages` | array | 단계별 결과, 실행 순서 |
| `dns` / `tcp` / `http` | object | 단계별 상세, 해당 단계가 정보를 남겼을 때만 |
| `total_ns` | int | 전체 소요 시간 (ns) |
| `failure_stage` | string | 첫 실패 단계, 성공 시 생략 |
| `error_kind` | string | 첫 실패의 분류, 성공 시 생략 |

`total_ns` 는 각 단계의 합이 아니다. 요청 write·리다이렉트 처리·정리 시간이 함께 들어간다.

## stages[]

| 필드 | 타입 | 의미 |
|---|---|---|
| `stage` | string | `dns` \| `tcp` \| `http` |
| `status` | string | `ok` \| `failed` \| `skipped` |
| `duration_ns` | int | 소요 시간 (ns) |
| `error_kind` | string | 분류된 원인, 성공 시 생략 |
| `error` | string | 원문 메시지 — **사람용. 절대 이 값으로 분기하지 않는다** |

`duration_ns` 가 0이면 측정값이 없다는 뜻이지 "0ms 에 끝났다" 가 아니다. 차트에 그리면 안 된다. 두 경우가 여기 해당한다 — `status` 가 `skipped` 인 단계, 그리고 시작했으나 끝나지 않아 `failed` 로 끝난 단계(dial 타임아웃). 앞 단계가 실패해 실행되지 않았거나(TCP 실패 후 HTTP), 애초에 해당 없음(IP 를 직접 지정했을 때의 DNS)이다.

## error_kind 값

정의는 `types/error.go`. 단계별로 접두사가 다르다.

| 단계 | 값 |
|---|---|
| DNS | `dns_not_found` (NXDOMAIN), `dns_timeout`, `dns_other` |
| TCP | `tcp_refused`, `tcp_timeout`, `tcp_unreachable`, `tcp_other` |
| HTTP | `http_status` (4xx/5xx), `http_timeout`, `http_protocol` |
| 공통 | `canceled`, `invalid_target`, `other` |

### `tcp_refused` 와 `tcp_timeout` 을 절대 합치지 않는 이유

이 도구의 존재 이유다.

- **`tcp_refused`** — 호스트가 RST 로 응답했다. 경로는 살아 있고 그 포트에 아무것도 듣고 있지 않다. 보통 즉시(1ms 미만) 돌아온다.
- **`tcp_timeout`** — 아무 응답도 오지 않았다. 패킷 필터가 조용히 버린 것이다. 타임아웃 전체를 소진한다.

Kubernetes 에서 전자는 대개 "Pod 는 뜬 상태, 앱이 포트를 안 열었다", 후자는 "NetworkPolicy 가 DROP" 이다. 둘을 하나의 "연결 실패" 로 뭉치면 진단이 반대 방향을 가리킨다.

## `tcp.attempts` — 성공했지만 정상이 아닌 경우

연결을 시도한 횟수다. DNS 가 주소를 여럿 돌려주면 Go 가 차례로 시도하므로, 첫 주소가 죽어 있으면 두 번째, 세 번째로 넘어간다.

```
ConnectStart(172.66.147.243:443)   →  ConnectDone(err=refused)    1번째
ConnectStart(104.20.23.154:443)    →  ConnectDone(err=<nil>)      2번째 — 성공
```

이때 단계는 `ok` 이고 `error_kind` 는 비어 있다. 결과만 보면 정상이다. 그런데 `attempts` 가 2 이상이면 **앞의 주소들이 죽어 있었다**는 뜻이다. Kubernetes 에서 EndpointSlice 에 죽은 Pod IP 가 남아 있으면 이 상태가 된다 — 매번 성공하지만 매번 느리다.

`error_kind` 는 **실패의 원인**만 담는다. 성공했지만 정상이 아닌 것은 이런 필드가 담는다. 같은 축의 다른 예가 DNS 의 search domain 순회(`ndots`)인데, 그건 아직 필드가 없다.

`0` 은 "시도하지 않았다"이며 재사용된 연결을 뜻한다. 의미가 있는 값이라 생략하지 않는다.

## TLS 는 1.5국면이 맡는다

1국면은 **평문 경로만** 잰다. `tls` 단계와 `tls_*` 오류 분류, `TLSInfo` 필드는 1.5국면에서 이 스키마에 더해진다.

더할 때 지킬 것이 하나 있다. `tls` 는 **이 프로세스가 대상과 직접 수행한 애플리케이션 TLS** 다. 서비스 메시가 경로에 있으면 프록시 사이에 별도의 mTLS 가 존재하는데, 그건 다른 것이며 5국면에서 별도 필드를 갖는다. 두 TLS 를 하나로 합치면 "TLS 성공" 이 무엇을 뜻하는지 알 수 없게 된다.

## 예시

```json
{
  "target": "http://example.com/",
  "started_at": "2026-09-10T10:00:00.000000+09:00",
  "stages": [
    { "stage": "dns",  "status": "ok", "duration_ns": 3000000 },
    { "stage": "tcp",  "status": "ok", "duration_ns": 12000000 },
    { "stage": "http", "status": "ok", "duration_ns": 40000000 }
  ],
  "dns":  { "addresses": ["93.184.216.34"] },
  "tcp":  { "remote_addr": "93.184.216.34:443", "local_addr": "192.168.0.2:52814", "reused": false, "attempts": 1 },
  "http": { "status_code": 200, "proto": "HTTP/2.0" },
  "total_ns": 101000000
}
```

## 호환성

Phase 2 가 이 스키마 위에 붙으므로 필드 이름과 `error_kind` 값은 함부로 바꾸지 않는다. 추가는 안전하고, 이름 변경과 삭제는 exporter 를 깨뜨린다.
