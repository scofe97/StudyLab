# 0016. 엣지 라우트는 팻말 있는 것을 앞에 두고 팻말 없는 것은 맨 끝에 하나만 두며, 두 번째 서비스는 이름(호스트)으로 나눈다

- 상태: 채택
- 날짜: 2026-11-04
- 장: ch10

## 맥락

ADR 0015 는 Caddy 를 npgproxy 앞단에 두고 라우트 순서는 다음 판단으로 미뤘다. 지금 엣지에는 매처 없는 `npg_route` 하나만 있다. 여기에 사내 문서 사이트 같은 두 번째 서비스를 붙이는 순간 두 가지 문제가 생긴다는 것을 리허설 E2([ch10-e2-route-order.md](../rehearsals/ch10-e2-route-order.md))에서 직접 봤다. 매처 없는 docs 라우트를 맨 앞에 넣자 API 가 404 가 됐고, 맨 뒤에 넣자 docs 가 한 번도 불리지 않았다. 매처 `/docs/*` 를 달아 맨 앞에 넣자 API 는 살았지만 `/docs/` 는 디스크의 `site/docs/docs/` 를 찾다가 404 가 났다.

이 두 문제는 Caddy 만의 사정이 아니라 요청을 나눠 주는 모든 시스템이 만나는 일반 문제다.

### 개념 1. 규칙 목록에서 하나를 고르는 방식

요청 하나에 규칙 여러 개가 맞을 때 시스템이 고르는 방식은 크게 둘이다.

| 방식 | 뜻 | 쓰는 곳 |
|---|---|---|
| 첫 일치 (first match) | 위에서부터 보고 처음 맞는 규칙이 이긴다 | 방화벽 규칙, ACL, `switch` 문, Caddy JSON `routes` |
| 가장 구체적인 일치 (most specific) | 맞는 규칙 중 가장 좁은 규칙이 이긴다 | IP 라우팅 표(longest prefix match), Go `http.ServeMux`, Caddyfile 의 `handle` 정렬 |

첫 일치는 단순하고 빠르지만 **순서가 곧 의미**다. 넓은 규칙을 좁은 규칙보다 위에 두면 아래 규칙은 영원히 불리지 않는다(가려진 규칙, shadowed rule). 방화벽에서 "모두 허용"을 맨 위에 두면 아래 차단 규칙이 다 무력해지는 사고와 같은 모양이다. 가장 구체적인 일치는 순서를 신경 쓰지 않아도 되지만, 시스템이 "무엇이 더 좁은가"를 판단할 수 있는 규칙에서만 된다. Go 표준 라이브러리는 이 방식을 문서로 못 박았다: "If two or more patterns match a request, then the most specific pattern takes precedence."

Caddy JSON 은 엄밀히 말하면 "첫 일치에서 멈춤"이 아니라 **라우트 전체가 한 줄의 미들웨어 사슬**이다. 매처가 안 맞는 라우트는 건너뛰고, 맞는 라우트는 핸들러를 위에서부터 차례로 실행한다. `headers`·`rewrite` 처럼 일을 하고 다음으로 넘기는 핸들러를 지나면 그다음 라우트로도 이어진다. 사슬을 끊는 것은 응답을 직접 쓰는 `file_server`·`reverse_proxy` 같은 핸들러다(`file_server` 는 파일이 없어도 `pass_thru` 를 켜지 않으면 404 로 끝낸다). 그래서 실제 효과는 "응답을 쓰는 핸들러를 가진, 처음 맞는 라우트가 이긴다"이고, 순서가 곧 의미라는 첫 일치의 성질을 그대로 가진다.

### 개념 2. 이름 공간 붙이기 (namespace mapping)

URL 주소 `/docs/guide.html` 과 디스크 경로 `site/docs/guide.html` 은 서로 다른 이름 공간이다. 한 이름 공간을 다른 이름 공간의 한 지점에 붙이면, 붙인 자리 이름(`/docs`)을 떼고 나머지만 넘기는 번역이 필요하다. 리눅스에서 USB 를 `/mnt/usb` 에 mount 하면 USB 안에서는 `/mnt/usb` 가 보이지 않는 것과 같은 원리다.

Caddy 의 `file_server` 는 이 번역을 하지 않는다. 디스크 경로를 `root` 와 요청 경로를 그대로 이어 붙여 만든다(`SanitizedPathJoin`). 이 함수는 `..` 로 서랍 밖에 나가는 것만 막고 앞부분을 떼지 않는다. 떼는 일은 따로 `rewrite` 의 `strip_path_prefix`(Caddyfile 로는 `handle_path`)가 한다. Go 도 같은 문제를 같은 방법으로 푼다. 표준 라이브러리 예제가 `http.Handle("/tmpfiles/", http.StripPrefix("/tmpfiles/", http.FileServer(http.Dir("/tmp"))))` 이다. 그래서 이 판단은 Go 로 서버를 직접 짤 11장 이후에도 그대로 쓰인다.

하위 경로로 붙인 앱에는 번역이 한 겹 더 필요하다. 앱이 자기가 `/docs` 아래에 붙은 줄 모르고 HTML 에 `/style.css` 같은 절대 경로 링크를 쓰면, 브라우저의 그 요청은 `/docs/*` 매처와 맞지 않아 엣지의 다른 라우트(여기서는 npg)로 간다. E2 에서 `/` 가 npg 로 간 것과 같은 길이다.

## 선택지

### A. 경로로 나눈다 (`api.corp.internal/docs/...`)

같은 이름 아래 하위 경로로 붙인다.

- 이름(DNS)과 인증서가 하나로 끝난다.
- 브라우저 입장에서 API 와 docs 가 같은 출처(origin: scheme·host·port 가 같음)라 쿠키를 같이 쓰고 CORS 가 필요 없다.
- 두 서비스의 규칙이 한 이름 공간 안에서 겹치므로 순서 규칙(개념 1)을 계속 지켜야 한다.
- 앞부분 떼기(개념 2)를 엣지에서 해야 하고, 앱도 자기 기준 경로를 알아야 링크가 깨지지 않는다.

### B. 이름으로 나눈다 (`docs.corp.internal`)

서비스마다 다른 호스트 이름을 주고, 엣지는 `host` 매처로 고른다.

- 두 서비스의 규칙이 Host 로 갈려 서로 겹치지 않는다. 한 서비스의 라우트 순서 실수가 다른 서비스를 가리지 않는다.
- 앱은 자기 이름의 `/` 에 붙으므로 앞부분 떼기와 기준 경로 설정이 필요 없다.
- 이름(DNS)과 인증서가 하나 더 필요하다. 이 실습의 내부 CA(E5)는 이름만 적으면 인증서를 자동으로 발급한다.
- 출처가 달라지므로 docs 페이지가 브라우저에서 API 를 직접 부르면 CORS 설정이 필요하다.

## 결정

창이 B 를 추천했고 학습자가 받아들였다(2026-11-04, "ADR0016은 추천대로 진행"). 이유는 B 가 개념 1·2 의 문제를 규칙을 잘 지켜서 피하는 게 아니라 **문제가 생기는 자리 자체를 없애기** 때문이다.

1. 엣지 라우트 순서 규칙. 매처가 있는 라우트를 앞에 두고, 매처 없는 라우트(지금은 `npg_route`)는 맨 끝에 하나만 둔다. 매처 없는 라우트가 둘이면 뒤의 것은 반드시 가려진다.
2. 두 번째 서비스는 기본으로 호스트 이름으로 나눈다(B).
3. 하위 경로(A)는 앱이 기준 경로 설정을 지원하고 같은 출처가 꼭 필요할 때만 쓴다. 그때는 매처와 앞부분 떼기를 함께 둔다(JSON 은 `match: path` + `rewrite strip_path_prefix`, Caddyfile 은 `handle_path`).
4. 제품 설정 `deploy/caddy/caddy.json` 은 바꾸지 않는다. 10장에는 붙일 두 번째 서비스가 없고, E2 의 docs 라우트는 리허설 재료(`deploy/caddy/scenarios/e2-*.json`)로만 둔다.

## 결과

- 쉬워지는 것: 서비스가 늘어도 각 서비스의 라우트 순서를 그 서비스 안에서만 보면 된다. 앱이 하위 경로를 몰라도 된다.
- 어려워지는 것: 서비스마다 이름과 인증서가 늘고, 브라우저에서 서비스끼리 부르면 CORS 를 맞춰야 한다.
- 학습용으로 단순화한 것: 이름 풀이는 curl 의 `--resolve` 로 대신하고 실제 DNS 레코드는 만들지 않는다. 공개 인증서라면 이름마다 발급·갱신 대상이 늘어난다.
- 다시 볼 조건: 이름을 하나만 쓸 수 있는 환경(외부 이름 하나만 열 수 있음 등)이거나, 두 서비스가 같은 출처의 쿠키를 꼭 나눠야 하면 A 로 다시 본다.

## 원문 근거

2026-11-04 에 태그로 고정한 원문을 읽었다.

- caddy `v2.11.7`
  - `modules/caddyhttp/routes.go` L45-49(핸들러는 위에서 아래로 사슬처럼 이어지고 어느 핸들러든 사슬을 멈출 수 있음), L256-324 `wrapRoute`(매처가 안 맞으면 다음으로 넘김, 맞으면 이 라우트의 핸들러를 다음 라우트 앞에 끼움)
  - `modules/caddyhttp/headers/headers.go` L111, `modules/caddyhttp/rewrite/rewrite.go` L152(일을 하고 `next.ServeHTTP` 로 넘김)
  - `modules/caddyhttp/fileserver/staticfiles.go` L162 `pass_thru`, L783-788(파일이 없을 때 `pass_thru` 가 아니면 다음으로 넘기지 않음), L338·L347(`root` 와 요청 경로로 파일 경로를 만듦)
  - `modules/caddyhttp/caddyhttp.go` L252-273 `SanitizedPathJoin`(경로 정리와 이어 붙이기만, 앞부분을 떼지 않음)
  - `modules/caddyhttp/rewrite/rewrite.go` L78 `StripPathPrefix`
  - `modules/caddyhttp/matchers.go` L238(`http.matchers.host`), L404(`http.matchers.path`)
- Go `go1.25.1`
  - `src/net/http/server.go` L2551-2553(`ServeMux` 는 가장 구체적인 패턴이 이김)
  - `src/net/http/example_test.go` L66-68(하위 경로에 파일 서버를 붙일 때 `StripPrefix` 로 앞부분을 뗌)

## 관련 노트

- 10-02.Caddy 를 앞단에 두고 백엔드로 리버스 프록시하며 정적 파일과 자동 HTTPS 를 붙인다.md (§2 라우트와 매처, 면접 2)
- 10-01.Caddy 는 JSON 설정을 관리 API 로 실시간 바꾸고 모듈과 어댑터로 넓힌다.md (§5 미들웨어가 빠져나가는 길)
- 리허설 [ch10-e2-route-order.md](../rehearsals/ch10-e2-route-order.md), ADR 0015, 이슈 #3
