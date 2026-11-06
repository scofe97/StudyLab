# 손님이 보낸 X-Forwarded-For 가 맨 뒤에 안 온다 (E3)

- 장: ch10
- 날짜: 2026-11-04
- 앞 장면과의 연결: E2 뒤, `e5-internal-tls.json` 이 떠 있는 상태에서 시작한다.

## 신고

> "앞에 LB 를 두자 백엔드 로그의 손님 주소가 전부 127.0.0.1 이다."

이 장면은 Caddy 설정을 맞추는 연습이 아니다. 주장과 사실의 차이, 신뢰 경계, 그리고 위치로 믿기의 전제를 보는 장면이다. 개념 설명과 결정은 [ADR 0017](../adr/0017-forwarded-for-trust-only-named-hops.md) 에 있다.

### 이 장면의 설정

- 떠 있는 설정: `deploy/caddy/scenarios/e5-internal-tls.json`. 엣지 `127.0.0.1:8000` 에 매처 없는 `npg_route` 하나가 npgproxy `127.0.0.1:28080` 로 연결한다.
- 맨 뒤 백엔드: fakeinv 가 `GET /_whoami` 로 받은 요청 그대로 `remote_addr`·`x_forwarded_for`·`x_forwarded_proto`·`x_forwarded_host` 를 JSON 으로 돌려준다.
- 끼워 넣는 Caddy 설정: `scenarios/e3-server-trust-loopback.json`(참조 구현에서 복사). Caddy 서버에 `trusted_proxies` 로 `127.0.0.1/32` 를 지정한다.
- npgproxy 새 플래그: `-trusted-proxies`. 바로 앞 홉이 이 대역에 들 때만 들어온 `X-Forwarded-For` 헤더를 이어 쓴다.

## 진단 과정

| 단계 | 한 일 | 알게 된 것 |
|---|---|---|
| 판 a (기본) | 기본 설정에서 손님이 `X-Forwarded-For: 203.0.113.195` 헤더를 실어 `:8000/_whoami` 호출 | fakeinv 가 받은 `x_forwarded_for` 는 `["127.0.0.1"]` 이다. Caddy 와 npgproxy 가 둘 다 앞 홉을 믿지 않아 가짜 헤더를 지우고 직전 홉 주소만 적었다. fakeinv 의 `remote_addr` 는 바로 앞 홉인 npgproxy 다. |
| 판 b (Caddy 가 127.0.0.1 을 믿음) | Caddy 에 `e3-server-trust-loopback.json` 을 올려 `127.0.0.1/32` 를 믿게 한 뒤 판 a 와 같은 요청 호출 | fakeinv 가 받는 값은 판 a 와 같다. 신뢰는 홉마다 따로 정하므로 Caddy 가 믿어도 뒤의 npgproxy 가 지우면 맨 뒤까지 전달되지 않는다. |
| 판 b′ (접근 로그로 Caddy 안의 client_ip 확인) | Caddy 접근 로그에서 `remote_ip` 와 `client_ip` 확인 | Caddy 접근 로그에서 `remote_ip` 는 사실인 `127.0.0.1` 이고, `client_ip` 는 손님이 보낸 XFF 맨 왼쪽 값인 `203.0.113.195` 다. Caddy 안에서는 믿는 프록시 판정이 동작했다. |
| 결정 B 와 코드 | npgproxy 에 이름을 댄 바로 앞 홉일 때만 XFF 를 이어 쓰는 결정 B 를 채택하고 코드 작성 | Go `httputil.ReverseProxy` 의 `Rewrite` 경로가 먼저 지우는 메커니즘을 제공하므로, 정책으로 앞 홉 검사와 XFF 복원을 `SetXForwarded` 앞에 배치했다. 기본값은 아무도 믿지 않는 빈 목록이다. |
| 6단계 우회 | npgproxy 에 `-trusted-proxies 127.0.0.1/32` 를 주고 curl 로 `:28080/_whoami` 에 직접 `X-Forwarded-For: 6.6.6.6` 전송 | fakeinv 가 `["6.6.6.6, 127.0.0.1"]` 을 받았다. 같은 기계에서는 손님도 `127.0.0.1` 이라 믿는 홉으로 판정되어 지어낸 주소가 맨 앞에 남는다. 위치 기반 신뢰의 전제가 깨지면 위조가 통한다. |

## 실행 전 예측

2026-11-04 에 적었고 실행 뒤 고치지 않았다.

1. 판 a:
   > remote_addr에는 caddy의 주소가 나오고, xff는 잘 모르겠습니다 (아무것도 신뢰안한다면 쓴것도 그냥 지우고 다른거 쓸거 같은데)
2. 판 b:
   > 어차피 npgproxy가 무조건 지우는데 의미가 있는가? 앞하고 같음
3. 판 b′:
   > 127.0.0.1을 믿으면 그 손님이 실제로 127.0.0.1로 들어왔다해도 XFF값을 믿어서 client_ip를 그대로 쓴다? (remote_ip에는 원본이 남고)
4. 6단계:
   > 6.6.6.6이 찍히 않을까요? 지금 신뢰되는 기준에 로컬 호스트가 있지 않나요? 방금 추가한 코드가 npgproxy코드잖아

## 실행 (사용자가 친 명령)

```bash
# 판 a
curl -s -H 'X-Forwarded-For: 203.0.113.195' http://127.0.0.1:8000/_whoami

# 판 b
./run.sh admin PUT /config/apps/http/servers/edge/trusted_proxies scenarios/e3-server-trust-loopback.json
curl -s -H 'X-Forwarded-For: 203.0.113.195' http://127.0.0.1:8000/_whoami

# 판 b′
# Caddy 접근 로그 확인 (.run/logs/caddy.log)

# 6단계
go run ./cmd/npgproxy \
  -addr 127.0.0.1:28080 \
  -admin 127.0.0.1:28081 \
  -upstream http://127.0.0.1:29090 \
  -trusted-proxies 127.0.0.1/32
curl -s -H 'X-Forwarded-For: 6.6.6.6' http://127.0.0.1:28080/_whoami
```

1. 판 a 는 기본 설정 상태에서 가짜 `X-Forwarded-For` 헤더를 붙여 엣지(`:8000`)로 보낸다.
2. 판 b 는 관리 API 로 Caddy 서버에 loopback 신뢰 설정을 올린 뒤 판 a 와 같은 curl 명령을 보낸다.
3. 판 b′ 는 Caddy 접근 로그(`.run/logs/` 아래)에서 `remote_ip` 와 `client_ip` 를 확인한다.
4. 6단계는 npgproxy 에 `-trusted-proxies 127.0.0.1/32` 플래그를 주어 재시작한 뒤, npgproxy 포트(`:28080`)로 직접 `X-Forwarded-For: 6.6.6.6` 을 붙여 보낸다.

## 관찰

- 판 a:
  ```json
  {"remote_addr":"127.0.0.1:57153","x_forwarded_for":["127.0.0.1"],"x_forwarded_host":"127.0.0.1:8000","x_forwarded_proto":"http"}
  ```
  fakeinv 의 `remote_addr` 는 바로 앞 홉인 npgproxy 다. 가짜 `203.0.113.195` 는 사라졌다. Caddy 가 손님 것을 지우고 `127.0.0.1` 을 적었고, npgproxy 도 앞 홉을 믿지 않아 지우고 새로 적었다.
- 판 b:
  ```json
  {"remote_addr":"127.0.0.1:57297","x_forwarded_for":["127.0.0.1"],"x_forwarded_host":"127.0.0.1:8000","x_forwarded_proto":"http"}
  ```
  fakeinv 가 받은 결과는 판 a 와 완전히 같다. Caddy 는 앞 홉을 믿었지만 뒤에 선 npgproxy 가 신뢰 목록이 없어 다시 지우고 Caddy 주소만 적었기 때문이다.
- 판 b′:
  ```json
  {"remote_ip":"127.0.0.1","client_ip":"203.0.113.195"}
  ```
  Caddy 접근 로그의 `remote_ip` 는 실제 연결 주소인 `127.0.0.1` 이 남았다. Caddy 서버는 `127.0.0.1` 을 믿는 프록시로 보아 손님이 보낸 XFF 맨 왼쪽 유효 주소인 `203.0.113.195` 를 `client_ip` 로 기록했다.
- 6단계:
  ```json
  {"remote_addr":"127.0.0.1:59882","x_forwarded_for":["6.6.6.6, 127.0.0.1"],"x_forwarded_host":"127.0.0.1:28080","x_forwarded_proto":"http"}
  ```
  손님이 지어낸 `6.6.6.6` 이 fakeinv 의 `x_forwarded_for` 맨 앞에 남았다. npgproxy 가 바로 앞 홉(`127.0.0.1`)을 믿는 프록시로 판정하여 손님이 보낸 헤더를 복원한 뒤 직전 홉 주소를 덧붙였기 때문이다.

## 전후 수치

| 판 | fakeinv 가 받은 `x_forwarded_for` | Caddy 접근 로그 `client_ip` |
|---|---|---|
| a | `["127.0.0.1"]` | - |
| b | `["127.0.0.1"]` | - |
| b′ (판 b 와 같은 설정) | - | `203.0.113.195` |
| 코드 뒤 직접 붙기 | `["6.6.6.6, 127.0.0.1"]` | - |

## 포스트모템

- 근본 원인
  - `X-Forwarded-For` 는 보내는 쪽이 임의로 적을 수 있는 주장이고, TCP 연결의 `RemoteAddr` 만이 3방향 핸드셰이크로 증명된 사실이다.
  - 신뢰 경계는 홉마다 따로 정해지며 다음 홉으로 번지지 않는다. 앞단 프록시가 믿더라도 뒷단 프록시가 지우면 백엔드까지 전달되지 않는다.
  - IP 대역으로 믿는 위치 기반 신뢰는 그 주소에 믿는 프록시만 설 수 있다는 네트워크 격리가 전제되어야 한다. 한 로컬 기계 안에서는 손님도 `127.0.0.1` 이므로 위치만으로는 상대를 가를 수 없다.
- 조치: [ADR 0017](../adr/0017-forwarded-for-trust-only-named-hops.md)
  - npgproxy 의 믿는 대역 기본값은 빈 목록으로 두어 플래그를 생략해도 아무도 믿지 않는다.
  - 믿는 대역은 그 포트에 앞 프록시만 닿는다는 네트워크 경계와 짝일 때만 켠다.
  - 이 실습의 배포 설정은 비워 두고, `127.0.0.1/32` 는 리허설에서만 켠다.
  - Caddy 엣지는 아무도 믿지 않는 기본 설정을 유지한다.
  - 앞 홉의 신원을 증명하는 mTLS 는 10장에서 구현하지 않는다.
- 재발 방지:
  - 단위 시험 `internal/proxy/trust_test.go` 가 `ParseTrustedProxies`, `Trusts`, 프록시 전송 시의 XFF 처리 순서와 기본값을 검증한다. 들어온 XFF 복원이 `SetXForwarded` 앞에 오는지와 기본값이 빈 목록인지가 시험으로 보호된다.
- 창의 실수:
  - 판 b′ 준비 과정에서 Caddy 접근 로그를 켜기 위해 `scenarios/e3-access-log.json` 을 관리 API 에 PUT 하려다 `409 key already exists` 오류가 났다. `e5-internal-tls.json` 에 이미 `"logs": {}` 가 선언되어 있었는데, 기존 설정을 확인하지 않고 새 시나리오 파일을 만들어 안내한 실수였다. 동작에는 영향이 없었고 해당 파일은 삭제했다.

## 원문 근거

- [ADR 0017](../adr/0017-forwarded-for-trust-only-named-hops.md) 의 `## 원문 근거` 에 적힌 Go `net/http/httputil` 과 Caddy 소스 코드를 바탕으로 검증했다.

## 이어 읽을 노트

- [ADR 0017](../adr/0017-forwarded-for-trust-only-named-hops.md) (npgproxy 는 이름을 댄 바로 앞 홉일 때만 X-Forwarded-For 를 이어 쓰고, 기본은 아무도 믿지 않는다)
- [ADR 0015](../adr/0015-caddy-edge-in-front-of-npgproxy.md) (Caddy 를 npgproxy 앞단에 두고 whoami 로 맨 뒤를 관측한다)
- [ch10-e5-internal-tls.md](ch10-e5-internal-tls.md) (내부 CA, mTLS 로 이어짐)
- 이슈 #3
