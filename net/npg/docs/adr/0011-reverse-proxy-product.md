# 0011. 8장부터 npg-lab 은 리버스 프록시를 만들고, 8장은 그 업스트림 층을 맡는다

- 상태: 채택
- 날짜: 2026-10-23
- 장: ch08

## 맥락

처음 계획(README 「제품 모양」)에서는 우리 `api` 서비스가 inventory 업스트림을 HTTP 클라이언트로 부르는 것이 8장이었다. 9장에서 그 api 를 HTTP 서버로 세우고, 10장에서 Caddy 를 앞단에 두는 순서였다. 사용자는 7장을 건너뛰기로 하면서(이슈 #5) 목표를 이렇게 다시 밝혔다. "8장부터 해서 실제 Envoy, Nginx 같은 웹서버처럼 굴려 보고 싶다."

실제 프록시는 요청을 받는 층과 요청을 보내는 층을 나눈다(원문 확인).

- envoy v1.40.1 용어 문서: "Downstream: A downstream host connects to Envoy, sends requests, and receives responses. Upstream: An upstream host receives connections and requests from Envoy". 받는 쪽은 listener 와 HTTP connection manager 가, 보내는 쪽은 cluster(연결 풀, `connect_timeout` 기본 5s, circuit breaker)와 route 의 `timeout`·`retry_policy` 가 맡는다.
- nginx 1.31.8: 받는 쪽은 `server {}`·`location {}` 이다. 보내는 쪽은 `ngx_http_upstream_module` 의 `upstream {}`("used to define groups of servers that can be referenced by the proxy_pass … directives")와 `proxy_connect_timeout`·`proxy_read_timeout`·`proxy_next_upstream` 이다.
- 책의 장 구성도 이 경계와 맞는다. 8장은 HTTP 클라이언트, 9장은 HTTP 서버다.

두 안 모두 8장에서 짜는 핵심 코드(`Transport` 설정, 기한, 재시도, 응답 본문 정리)는 거의 같다. 달라지는 것은 그 코드를 부르는 쪽이다. 특히 재시도 판단이 달라진다. 앱은 자기가 만든 요청의 의미를 알지만, 프록시는 남이 보낸 요청을 메서드와 헤더만 보고 판단해야 한다.

## 선택지

### A. 계획 그대로: api 서비스 안의 업스트림 클라이언트

- 장점: 처음 계획과 이슈 #6 문장을 그대로 쓴다. 요청의 의미를 앱이 알아서 멱등성 키를 앱이 직접 만들 수 있다.
- 단점: 사용자가 밝힌 목표(envoy·nginx 같은 웹서버)와 거리가 있다. 남의 요청을 대신 보낼 때의 판단(본문을 다시 보낼 재료, 비멱등 요청을 다시 보내지 않는 기본값)을 다루지 않는다.

### B. npg-lab 리버스 프록시: 8장은 업스트림 층, 9장은 다운스트림 층

- 장점: envoy cluster·nginx upstream 과 같은 자리를 직접 만든다. nginx 의 `non_idempotent`, haproxy 의 `retry-on` 경고("replay protection mechanism" 이 없으면 위험)처럼 실제 프록시가 고민하는 판단을 그대로 마주한다. 9장(서버 타임아웃·드레인)과 10장(Caddy)이 같은 프록시의 다른 층이나 비교 대상으로 자연스럽게 이어진다.
- 단점: 계획 문서와 README 의 제품 모양을 고쳐야 한다. 요청을 그대로 넘기는 일(헤더·본문 전달)이 새로 들어와 8장 범위가 조금 커진다.

## 결정

B 를 고른다. 사용자가 밝힌 목표와 실제 프록시의 층 구분이 일치하기 때문이다.

- npg-lab 에 리버스 프록시 실행 파일을 더한다. 8장은 라우트 하나(업스트림 하나)짜리 최소 프록시와 그 업스트림 층(연결 풀, 기한, 재시도, 멱등성 판단)을 만든다.
- 리허설에서 프록시 뒤에 세울 가짜 업스트림(장애 손잡이가 있는 inventory 흉내)을 만든다.
- 다운스트림 층의 서버 타임아웃과 드레인은 9장으로 넘긴다. 8장의 최소 프록시는 받는 쪽 설정을 최소한으로 둔다.
- 처음 계획의 api 서비스는 프록시 뒤에 서는 백엔드 가운데 하나가 된다. 언제 만들지는 9장에서 다시 정한다.

## 결과

- 쉬워지는 것: 8장 판단(기한 넷, 재시도·멱등성)을 envoy·nginx·haproxy 설정과 일대일로 비교할 수 있다.
- 어려워지는 것: 요청 전달을 표준 `httputil.ReverseProxy` 로 할지, 직접 짤지 정해야 한다(구현 중 판단, 새 ADR). 재시도하려면 요청 본문을 다시 보낼 재료가 있어야 해서, 본문을 담아 둘지와 그 상한도 정해야 한다.
- 학습용으로 단순화한 것(프로덕션이면 더 필요한 것): 라우트 여러 개, 업스트림 여러 대의 부하 분산과 헬스 체크, 설정 파일 다시 읽기, circuit breaker, 지표 노출. 이 장의 프록시는 업스트림 한 대만 본다.
- 다시 볼 조건: 9장에서 다운스트림 층을 세울 때와, 10장에서 Caddy 와 역할을 나눌 때.

## 관련 노트

- 08-01.HTTP 는 URL 로 자원을 가리키고 메서드로 요청하며 상태 코드로 답한다.md
- 08-02.Go HTTP 클라이언트는 응답 본문을 닫아야 연결을 재사용하고 context 로 요청을 끊는다.md
- 이슈 #5 (7장 보류 결정), 이슈 #6
