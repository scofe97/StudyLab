# 사내 도메인에 HTTPS 를 붙여라 (E5)

- 장: ch10
- 날짜: 2026-11-03 (예측) ~ 2026-11-04 (실행)
- 앞 장면과의 연결: 10장 첫 장면이다. ADR 0015 로 npgproxy 앞에 Caddy 를 세웠고, 이 장면에서 그 앞단에 HTTPS 를 붙인다.

## 신고

> "사내 도메인 `api.corp.internal` 에 HTTPS 를 붙여 주세요."

공인 도메인이 아니어서 Let's Encrypt 는 쓸 수 없다. 공개 CA 는 공개 DNS 로 찾을 수 없는 이름의 제어권을 확인할 수 없기 때문이다.

### 이 장면의 설정

`deploy/caddy/scenarios/e5-internal-tls.json` 은 기본 설정(`caddy.json`, `127.0.0.1:8000` → npgproxy `:28080`)에 다음을 더한다.

- HTTPS 서버 `127.0.0.1:8443`, 호스트 매처 `api.corp.internal`
- `tls.automation.policies` 에 내부 발급자(`internal`, CA `local`)
- `http_port` 8088, `https_port` 8443 (관리자 권한 없이 돌리려고 80·443 을 쓰지 않음)
- `pki.certificate_authorities.local.install_trust: false` (macOS 키체인에 넣지 않음)

`e5-short-lifetime.json` 은 호스트 `short.corp.internal`, 잎 수명 `3m`, `automation.renew_interval: "20s"` 로 바꾼 판이다.

## 진단 과정

| 순서 | 본 신호(명령·지표) | 세운 가설 | 남김 / 버림과 이유 |
|---|---|---|---|
| 1 | `--cacert intermediate.crt` 로도 curl 200 | 인증서가 있으니 통과한다 (학습자) | 버림. `openssl verify -CAfile intermediate.crt` 는 `error 2 at 1 depth` 로 실패하고 `-partial_chain` 을 붙여야 OK. curl 이 부분 체인을 기본으로 받아들여서 통과했다 |
| 2 | HTTP 8088 요청에 `308`, `Location: https://api.corp.internal/v1/devices` | 포트만 8443 으로 바뀐 주소, 301 (학습자 예측) | 버림. Caddy 는 설정한 `https_port` 와 같은 포트는 주소에 붙이지 않는다(바깥은 443 이고 안쪽으로 넘겨 준다고 본다). 상태는 메서드를 지키는 308 |
| 3 | 3분 잎이 09:48:07 에 만료됐는데 serial 이 그대로, 만료된 잎을 6분 동안 내밈 | `load` 로 올린 `renew_interval: 20s` 가 적용되지 않았다 (창) | 남김. 인증서 캐시와 점검 타이머는 프로세스에 하나이고, 타이머는 첫 `up`(기본 10분) 때 한 번만 만든다. 다음 점검 09:54:12 에 `remaining: -364.68` 로 갱신된 것으로 확인 |
| 4 | `down` → `up e5-short-lifetime.json` 뒤 1분 40초마다 교체 | 1/3 규칙대로 2분째에 바뀐다 (학습자 예측) | 버림. 남은 수명이 점검 간격 5배(100초) 아래면 비상 갱신하는 규칙이 1/3 규칙보다 먼저 걸렸다. 로그 `emergency renewal window; expiration imminent` |
| 5 | `.run/data` 를 지우고 `up` → 새 루트 지문, 옛 루트로는 `unable to get local issuer certificate` | 루트가 없으니 Caddy 가 에러를 낸다 (학습자 예측) | 버림. 저장소에 루트가 "없음"이면 경고 없이 새로 만든다. 서버는 멀쩡하고 옛 루트를 심은 손님만 깨진다 |

## 실행 전 예측

2026-11-03 에 적었고 실행 뒤 고치지 않았다.

1. 루트·중간·잎 수명: "루트가 90일이면 중간이나 잎은 그보다는 적게 하지 않을까?"
2. curl 세 경우(아무것도 / 루트 / 중간): "가는 인증서 없으니 실패, 나·다는 인증서 있으니 성공"
3. `http://api.corp.internal:8088/v1/devices` 의 `Location`: "포트만 8443 으로 바뀐 주소로 가지 않을까요? 상태코드는 301?"
4. 3분 잎이 바뀌는 때: "2분까지는 멀쩡하고 1/3 남은 시점에 바뀌니 2분 이후에 교체 → 다시 4분 때 바뀌어서 2번 바뀌지 않을까요"
   - 4-2 같은 이름의 기본 수명 인증서가 저장소에 이미 있었다면: "예측대로 되는 거 아닌가? 새로 발급받는데 3분이 적용 안 됨?" (두 갈래)
5. `.run/data` 를 지우고 다시 띄우면: "루트 인증서를 지워버림 — 결국 인증서 못 써서 에러가 나겠지? 옛 루트를 믿게 해 둔 노트북들도 그 인증서 보고 해야 하는 거 아닌가?"

## 실행 (사용자가 친 명령)

```bash
cd net/npg/deploy/caddy
./run.sh up                                   # 실행 0: 기본 설정으로 Caddy → npgproxy 경로 확인
curl -si http://127.0.0.1:8000/v1/devices | head -12

./run.sh load scenarios/e5-internal-tls.json
CA=.run/data/caddy/pki/authorities/local
curl -s -o /dev/null -w '%{http_code} %{errormsg}\n' --resolve api.corp.internal:8443:127.0.0.1 https://api.corp.internal:8443/v1/devices
curl ... --cacert $CA/root.crt ...
curl ... --cacert $CA/intermediate.crt ...
curl -sI --resolve api.corp.internal:8088:127.0.0.1 http://api.corp.internal:8088/v1/devices

for f in $CA/root.crt $CA/intermediate.crt; do openssl x509 -in $f -noout -subject -issuer -dates; done
openssl s_client -connect 127.0.0.1:8443 -servername api.corp.internal </dev/null 2>/dev/null | openssl x509 > .run/leaf.pem
openssl verify -CAfile $CA/intermediate.crt .run/leaf.pem
openssl verify -partial_chain -CAfile $CA/intermediate.crt .run/leaf.pem
openssl verify -CAfile $CA/root.crt -untrusted $CA/intermediate.crt -attime $(date -v+13H +%s) .run/leaf.pem

./run.sh load scenarios/e5-short-lifetime.json
scenarios/e5-watch-serial.sh short.corp.internal 10 30
./run.sh down && ./run.sh up scenarios/e5-short-lifetime.json
scenarios/e5-watch-serial.sh short.corp.internal 10 30

cp $CA/root.crt .run/old-root.crt
./run.sh down && rm -rf .run/data && ./run.sh up scenarios/e5-internal-tls.json
curl ... --cacert .run/old-root.crt ...
curl ... --cacert $CA/root.crt ...
```

## 관찰

- 실행 0: `Via: 1.1 Caddy`, `X-Edge-Route: npg`, `X-Upstream: 127.0.0.1:28080`, 본문은 fakeinv. 요청이 Caddy → npgproxy → fakeinv 를 지났다.
- 설정을 올린 순간(00:06:44 GMT) 루트·중간·잎이 한꺼번에 생겼다. 아무도 CLI 로 만들지 않았고 Caddy 로그 `pki.ca.local`, `tls.obtain` 에 남았다.
- curl: 아무것도 없이 `000 SSL certificate problem: unable to get local issuer certificate`, 루트 200, 중간 200. `000` 은 TLS 에서 끊겨 HTTP 응답이 없다는 curl 의 표시다.
- 리다이렉트: `HTTP/1.1 308 Permanent Redirect`, `Location: https://api.corp.internal/v1/devices`.
- 수명: 루트 3600일(2036-09-12), 중간 7일(2026-11-11), 잎 12시간. 잎의 subject 는 비어 있고 이름은 SAN(`DNS:api.corp.internal`)에만 있다.
- `-attime` 으로 13시간 뒤를 검증하면 `error 10 at 0 depth lookup: certificate has expired`.
- 3분 잎(`load` 로 올림): 09:45:07 발급, 09:48:07 만료, 09:54:12 갱신(`remaining: -364.68`). 그사이 만료된 잎을 그대로 내밈.
- 3분 잎(재시작): 저장소의 유효한 잎을 재발급 없이 씀. 교체 10:05:35, 10:07:15, 10:08:55.
- 저장소 삭제: 새 루트(지문 `86:7E:0B:29…`). 옛 루트 `000 … unable to get local issuer certificate`, 새 루트 200.

## 전후 수치

| 지표 | `load` 로 3분 설정 | 재시작으로 3분 설정 |
|---|---|---|
| 점검 간격 | 10분(첫 `up` 때 값) | 20초 |
| 만료된 잎을 내민 시간 | 6분 5초 | 0 |
| 잎 교체 간격 | 점검 시각에 맞춰 10분 | 1분 40초 (비상 갱신 창) |

## 포스트모템

- 타임라인: 09:04 `up`(기본 설정) → 09:45 `load` 3분 설정 → 09:48 잎 만료 → 09:54 갱신 → 10:04 재시작 → 1분 40초마다 갱신 → 저장소 삭제 → 새 루트.
- 근본 원인
  - 갱신 지연: Caddy 의 인증서 캐시는 설정을 바꿔도 인증서를 다시 받지 않으려고 프로세스에 하나만 두고, 그 점검 타이머는 캐시를 만들 때 한 번만 맞춘다. `load` 는 옵션 값만 바꾸고 타이머는 다시 만들지 않는다.
  - 루트 교체: 저장소에 루트가 없으면 처음 켠 날과 구분하지 않고 새 루트를 만든다.
  - 부분 체인: 같은 신뢰 설정도 클라이언트(curl, openssl, 브라우저, Go)에 따라 결과가 다르다.
- 조치
  - 점검 간격(`automation.renew_interval`)을 바꾸면 `load` 가 아니라 재시작한다.
  - 손님 기기에는 중간이 아니라 루트를 심는다. 중간은 7일짜리이고, 남은 수명 20% 에서 새 키로 바뀐다.
  - 저장소(`.run/data`, 컨테이너라면 `/data`)는 지워지지 않는 곳에 두고 백업한다. 루트를 오래 지켜야 하면 회사 CA 의 루트·중간을 설정(`pki…root`)으로 넣는다.
- 재발 방지: 이 장면은 코드가 아니라 운영 규칙이라 시험 대신 위 조치를 ADR 없이 기록으로 남긴다. "load 로 안 바뀌는 설정"은 C1 장면에서 이어서 본다.

### 공개 도메인이었다면 (실행하지 않고 따짐)

80 을 막고 443 만 열면 HTTP-01 은 못 쓰고 TLS-ALPN-01 과 DNS-01 은 쓸 수 있다(학습자 답, 맞음). DNS-01 은 80·443 을 쓰지 않고 공개 DNS 의 TXT 레코드로 확인하므로, Caddy 가 DNS 업체 API 로 레코드를 쓸 수 있어야 하고 그 업체 모듈을 넣은 빌드가 필요하다. 와일드카드(`*.example.com`)는 Let's Encrypt 에서 DNS-01 로만 받는다.

## 원문 근거

- caddy v2.11.7
  - `modules/caddyhttp/autohttps.go` L544-575 `makeRedirRoute` (설정한 HTTPS 포트는 주소에 붙이지 않음, 308)
  - `modules/caddypki/ca.go` L475-478 (루트 3600일, 중간 7일, CA 갱신 비율 0.2), L277-296 `loadOrGenRoot`
  - `modules/caddypki/certificates.go` `generateRoot`·`generateIntermediate`
  - `modules/caddypki/maintain.go` (중간을 새 키로 갱신)
  - `modules/caddytls/internalissuer.go` L200 (잎 12시간)
  - `modules/caddytls/tls.go` L218-224 (캐시가 있으면 `SetOptions` 만)
- certmagic v0.25.6
  - `maintain.go` L58-60 (점검 타이머를 한 번만 만듦), L1009 (기본 10분)
  - `certificates.go` L170 (1/3 규칙), L176-186 (남은 시간이 점검 간격 5배 아래면 비상 갱신)
- libcurl `CURLOPT_SSL_OPTIONS` 의 `CURLSSLOPT_NO_PARTIALCHAIN` (부분 체인을 기본으로 받아들임). 실습 curl 8.7.1(LibreSSL), openssl 3.6.4.

## 이어 읽을 노트

- 10-02.Caddy 를 앞단에 두고 백엔드로 리버스 프록시하며 정적 파일과 자동 HTTPS 를 붙인다.md §4
- 11-01.TLS 클라이언트는 CA 서명으로 서버를 확인하고 서버는 리스너를 감싸며 피닝은 믿을 인증서를 좁힌다.md (체인 검증)
- 11-02.상호 TLS 는 직접 만든 인증서로 클라이언트도 검증하고 서버는 접속 주소를 인증서 이름과 맞춘다.md §2 (인증서를 Go 로 직접 만들기)
