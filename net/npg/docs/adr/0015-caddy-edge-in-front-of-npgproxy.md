# 0015. 10장은 Caddy 를 npgproxy 앞단에 두고, 백엔드는 fakeinv 에 손님 주소를 보여 주는 엔드포인트만 더한다

- 상태: 채택
- 날짜: 2026-11-02
- 장: ch10

## 맥락

처음 계획의 10장 모양은 `운영자 ─HTTPS─▶ Caddy ─▶ api(9장) ─▶ inventory 업스트림(8장)` 이었다. ADR 0011 로 8·9장이 api 서비스 대신 리버스 프록시 npgproxy 를 만들면서, 10장에서 Caddy 가 npgproxy 와 어떤 관계에 설지 다시 정해야 한다. ADR 0011 은 api 서비스를 언제 넣을지도 미뤘고, 9장(2026-11-01)이 다시 10장으로 미뤘다.

학습자는 이 장의 기준을 이렇게 밝혔다. "웹서버 자체를 제품으로 만들어 배포하는 것보다 실무 흐름을 이해하는 것이 1원칙이다." 그래서 범위는 "실무에서 요청이 어떤 층을 지나고, 층마다 무엇을 믿는가"를 끝까지 볼 수 있는 만큼으로 잡는다.

프록시가 두 층이 되면 맨 뒤 서버가 보는 상대 주소는 바로 앞 프록시의 주소다. 진짜 손님 주소는 `X-Forwarded-For`(XFF) 헤더에만 남는데, 이 헤더는 손님이 직접 채워 보낼 수 있다. 실제 프록시는 이 문제를 "바로 앞 주소를 믿을 때만 그 헤더를 이어 쓴다"로 푼다.

- caddy: `reverse_proxy` 는 들어온 `X-Forwarded-*` 값을 기본으로 무시하고, Caddy 가 손님이 처음 붙는 서버가 아닐 때(앞에 CDN 이 있을 때 등) `trusted_proxies` 대역을 적으라고 안내한다.
- envoy: 기본은 바로 앞 연결의 주소를 손님 주소로 삼고, "edge Envoy 앞에 신뢰하는 프록시가 하나 이상 있는 환경"에서 `xff_num_trusted_hops` 로 XFF 의 오른쪽 주소를 몇 개 더 믿을지 정한다.
- nginx: `ngx_http_realip_module` 의 `set_real_ip_from` 이 "올바른 대체 주소를 보낸다고 알려진 신뢰 주소"를 정한다.

지금 npgproxy 는 들어온 `X-Forwarded-*` 를 모두 버리고(`httputil.ReverseProxy` 가 `Rewrite` 를 부르기 전에 지운다) 자기가 본 손님 주소로 새로 적는다(`internal/proxy/proxy.go` 의 `pr.SetXForwarded()`). 손님이 직접 붙던 8·9장에서는 위조를 막는 올바른 기본값이었지만, 앞에 Caddy 가 서면 백엔드는 모든 손님을 Caddy 주소 하나로 보게 된다.

## 선택지

### 제품 모양

#### A. Caddy 를 npgproxy 앞단에 둔다

`운영자 ─HTTPS─▶ Caddy ─▶ npgproxy ─▶ 백엔드`

- 장점: 실무에서 흔한 두 층 배치(클라우드 부하 분산기나 CDN 뒤의 nginx·envoy)를 그대로 겪는다. 이 장의 판단 둘이 모두 실제 설정과 코드로 생긴다. 라우트 순서는 Caddy 설정에, 신뢰 대역은 Caddy 설정과 npgproxy 코드 양쪽에 생긴다. 10-02 에서 틀렸던 위조 XFF 를 장애로 겪는다.
- 단점: 요청이 프록시를 두 번 거쳐 연결 하나(Caddy→npgproxy)와 장애 원인을 찾을 층 하나가 는다. npgproxy 자체 기능은 조금만 는다.

#### B. Caddy 를 npgproxy 의 비교 대상으로 둔다

같은 장면을 Caddy 와 npgproxy 에 각각 돌리고, 라우트 여러 개·헬스 체크를 npgproxy 에 들인다.

- 장점: npgproxy 가 Caddy 기능을 따라잡아 제품이 커진다.
- 단점: 범위가 크고, 이 장의 Caddy 는 관찰만 한다. 두 층을 지날 때 생기는 신뢰 문제를 겪지 않는다.

### 백엔드

#### 1. fakeinv 를 그대로 두고, 받은 주소를 보여 주는 엔드포인트만 더한다

- 장점: 흐름을 끝까지 눈으로 본다. 범위가 작아 이 장의 판단에 집중한다. 8·9장의 장애 손잡이(`/_faults`)를 그대로 쓴다. fakeinv 는 이미 장비 조회·검색·등록(`GET /v1/devices/{id}`, `GET /v1/devices`, `POST /v1/devices`)을 흉내 내므로, 옛 api 서비스가 할 요청 모양은 지금도 있다.
- 단점: 앱이 손님 주소로 무언가를 결정하는 단계는 없다. 차단은 Caddy 나 npgproxy 가 맡는다.

#### 2. api 서비스(장비 등록 API)를 새로 넣는다

- 장점: 앱이 손님 주소를 직접 읽어 쓰는 단계까지 실무와 같아진다.
- 단점: 장비 등록 로직은 10장 주제와 관계가 없어 범위가 커진다. 신뢰 판단이 세 곳(Caddy, npgproxy, api)으로 늘지만, 새로 배우는 판단은 npgproxy 에서와 같다.

## 결정

제품 모양은 A, 백엔드는 1 을 고른다. 학습자의 1원칙이 "실무 흐름 이해"이고, A·1 조합이 그 흐름을 가장 작은 범위로 끝까지 보여 주기 때문이다.

- Caddy 는 운영자 HTTPS 를 받아 npgproxy 로 넘기는 앞단이다. 라우트 순서와 `trusted_proxies` 는 이 장의 다음 판단(ADR 0016·0017 예정)에서 정한다.
- npgproxy 는 앞단 Caddy 가 보낸 XFF 를 어디까지 믿을지 정해야 한다. 지금의 "모두 버리고 새로 적기"를 유지할지, 신뢰 대역에서 온 요청만 이어 쓸지는 `trusted_proxies` 판단에서 함께 정한다.
- fakeinv 에 받은 요청의 직전 홉 주소(`RemoteAddr`), 받은 XFF, 받은 `X-Forwarded-Proto`·`X-Forwarded-Host` 를 JSON 으로 돌려주는 `/_whoami` 를 더한다. 장애 손잡이와 같은 밑줄 이름 규칙을 따른다. `cmd/*` 이므로 창이 쓴다.
- api 서비스는 넣지 않는다.

## 결과

- 쉬워지는 것: 요청 하나가 `운영자 → Caddy → npgproxy → fakeinv` 를 지나며 층마다 손님 주소가 어떻게 바뀌는지 `/_whoami` 와 두 프록시의 접근 로그로 맞대 볼 수 있다. 신뢰 판단을 caddy·envoy·nginx 설정과 일대일로 비교할 수 있다.
- 어려워지는 것: 장애가 나면 층이 하나 더 있어, 어느 층의 설정 때문인지 먼저 가려야 한다. 이 장의 리허설이 이 가리기를 연습한다.
- 학습용으로 단순화한 것(프로덕션이면 더 필요한 것): 손님·Caddy·npgproxy·fakeinv 가 한 기계(`127.0.0.1`)에 있어 신뢰 대역이 루프백 하나로 줄어든다. 실제 배치에서는 층마다 다른 주소 대역에 두고 그 대역만 믿으며, 방화벽으로 앞단을 건너뛴 직접 접근을 막아야 한다. 앱이 손님 주소로 차단·요청 제한을 하는 단계는 없다.
- 다시 볼 조건: 손님 주소로 결정을 내리는 앱 기능(요청 제한, 감사 로그)이 필요해질 때 api 서비스를 다시 본다. 그때는 신뢰 대역 설정을 앱에도 같은 값으로 둬야 한다.

## 원문 근거

2026-11-02 에 태그나 커밋으로 고정한 원문을 읽었다.

- caddy 문서 `caddyserver/website` 커밋 `a49be19`: `src/docs/markdown/caddyfile/directives/reverse_proxy.md` L504(XFF 를 설정하거나 덧붙임), L508(들어온 `X-Forwarded-*` 값은 위조를 막으려 기본으로 무시), L510(Caddy 가 처음 붙는 서버가 아닐 때 `trusted_proxies`), L512(`servers > trusted_proxies` 전역 옵션을 권장).
- envoy `v1.40.2`: `docs/root/configuration/http/http_conn_man/headers.rst` L310-313(기본 신뢰 손님 주소), L315-326(edge Envoy 앞에 신뢰 프록시가 있을 때 `xff_num_trusted_hops`).
- nginx 1.31.8 문서: nginx.org `ngx_http_realip_module.html#set_real_ip_from`("Defines trusted addresses that are known to send correct replacement addresses").
- Go `go1.25.1`: `src/net/http/httputil/reverseproxy.go` L434-437(`Rewrite` 를 부르기 전에 `Forwarded`·`X-Forwarded-For`·`X-Forwarded-Host`·`X-Forwarded-Proto` 를 지움), L70-90(`SetXForwarded` 는 나가는 요청에 XFF 가 있으면 손님 주소를 덧붙이고, 들어온 XFF 를 이어 쓰려면 `Rewrite` 안에서 먼저 복사하라는 문서와 예시).

## 관련 노트

- 10-01.Caddy 는 JSON 설정을 관리 API 로 실시간 바꾸고 모듈과 어댑터로 넓힌다.md
- 10-02.Caddy 를 앞단에 두고 백엔드로 리버스 프록시하며 정적 파일과 자동 HTTPS 를 붙인다.md
- ADR 0011(리버스 프록시 제품), 이슈 #3
