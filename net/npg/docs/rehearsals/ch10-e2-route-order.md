# 새 라우트를 넣었더니 API 가 404 (E2)

- 장: ch10
- 날짜: 2026-11-04
- 앞 장면과의 연결: C1 에서 관리 API 로 설정 한 칸을 고치는 법을 익혔다. 이 장면은 그 방법으로 라우트를 끼워 넣다가 생기는 사고를 본다. E5 의 `e5-internal-tls.json` 이 떠 있는 상태에서 시작한다.

## 신고

> "사내 문서 사이트를 붙이는 라우트를 넣은 뒤부터 `/v1/devices` 호출이 404 다. npgproxy 로그에는 요청이 한 줄도 없다."

이 장면은 Caddy 설정 문법을 맞추는 연습이 아니다. 요청을 나눠 주는 시스템이 공통으로 만나는 두 문제, **규칙 목록에서 하나를 고르는 방식**과 **이름 공간 붙이기**를 Caddy 로 눈에 보이게 하는 장면이다. 개념 설명과 결정은 [ADR 0016](../adr/0016-edge-route-order-and-host-split.md) 에 있다.

### 이 장면의 설정

- 떠 있는 설정: `deploy/caddy/scenarios/e5-internal-tls.json`. 엣지 `127.0.0.1:8000` 에 매처 없는 `npg_route` 하나(`X-Edge-Route: npg` 도장 → npgproxy `127.0.0.1:28080`)
- 끼워 넣는 docs 라우트(참조 구현 `chapters/ch07-10-service/edge/scenarios/` 에서 복사)
  - `e2-docs-route.json`: 매처 없음, `X-Edge-Route: docs` 도장 → `file_server` (`root: site/docs`)
  - `e2-docs-route-matched.json`: 위와 같고 매처 `/docs/*` 만 더함
  - `e2-debug-logging.json`: 로그 수준 DEBUG
- 고친 판(창 작성): `e2-docs-route-fixed.json`. 매처 `/docs/*`, 도장 → `rewrite strip_path_prefix "/docs"` → `file_server`
- 정적 파일: `deploy/caddy/site/docs/index.html`
- fakeinv 가 아는 주소는 `/v1/devices`, `/v1/devices/{id}`, `/_faults`, `/_stats` 뿐이고 나머지는 404 다(`cmd/fakeinv/main.go` L436-440). 그래서 npg 도장에 404 가 찍히면 "엣지는 맞게 넘겼고 맨 뒤가 모르는 주소"라는 뜻이다.

## Caddy JSON 읽는 법

이 장면의 설정을 읽으려면 Caddy JSON 의 뼈대만 알면 된다. 엣지 부분만 뽑으면 이렇다.

```
{
  "admin": { "listen": "127.0.0.1:2019" }        ← 관리 API 주소 (C1)
  "apps": {                                       ← 앱 묶음: http, tls, pki(E5 내부 CA)
    "http": {
      "servers": {
        "edge": {                                 ← 서버 하나. 이름은 마음대로
          "listen": ["127.0.0.1:8000"],
          "routes": [                             ← 라우트 목록. 적힌 순서가 검사 순서
            {
              "@id": "npg_route",                 ← 별명. 관리 API 로 /id/npg_route 처럼 바로 찾을 때만 씀
              "match": [ ... ],                   ← 매처. 없으면 모든 요청이 맞음
              "handle": [                         ← 맞았을 때 실행할 핸들러, 위에서부터
                { "handler": "headers", ... },
                { "handler": "reverse_proxy",
                  "upstreams": [ { "dial": "127.0.0.1:28080" } ] }
              ]
            }
          ]
        }
      }
    }
  }
}
```

- **`routes`** 는 배열이라 적힌 순서대로 본다. E2 의 사고는 전부 이 순서에서 난다.
- **`match`** 는 그 라우트를 탈 조건이다. 한 집합 안의 조건은 모두 맞아야 하고(AND), 집합이 여러 개면 하나만 맞아도 된다(OR). 비어 있으면 모든 요청이 맞는다.
- **`handle`** 의 핸들러는 두 부류다. `headers`(헤더 바꾸기), `rewrite`(주소 고치기)는 일을 하고 다음으로 넘긴다. `file_server`, `reverse_proxy` 는 응답을 직접 써서 사슬을 거기서 끝낸다.
- **`upstreams`** 는 `reverse_proxy` 가 요청을 넘길 뒤쪽 서버 목록이고, `dial` 은 TCP 로 연결할 주소다. 여러 개를 적으면 나눠 보낸다(E1·E4).
- **`{http.reverse_proxy.upstream.hostport}`** 같은 중괄호는 자리표시자다. 요청마다 실제 값으로 바뀐다.
- **`@id`** 는 동작에 영향이 없는 별명이다. C1 에서 쓴 `H=/id/npg_route/handle/0/response/set` 이 이 별명으로 긴 경로를 줄인 것이다.

`"handler": "reverse_proxy"` 의 문자열은 이름표가 아니라 부품 선택이다. Caddy 안의 Go 모듈은 `http.handlers.reverse_proxy`, `http.matchers.path` 같은 이름으로 등록부에 올라가 있고, JSON 의 이 값이 어느 구현을 꺼내 끼울지 고른다. `database/sql` 이 `sql.Open("postgres", …)` 의 문자열로 드라이버를 고르는 것과 같은 "인터페이스 + 이름 등록부" 설계다. 그래서 Caddy JSON 은 외울 문법이 아니라 부품을 어떤 순서로 끼웠는지 적은 조립도로 읽으면 된다. E6 에서 만들 블록 모듈도 이 등록부에 자기 이름을 올린다.

## 진단 과정

| 순서 | 본 신호(명령·지표) | 세운 가설 | 남김 / 버림과 이유 |
|---|---|---|---|
| 1 | 변형 1(매처 없는 docs 를 0번에): `/v1/devices` 404 docs, `/` 200 docs, `/docs/` 404 docs | `/v1/devices` 는 npg 200, `/`·`/docs/` 는 docs 200 (학습자) | 버림. 매처 없는 0번 docs 가 모든 요청을 받았다. `file_server` 는 응답을 쓰고 끝내므로 뒤의 npg 까지 가지 않는다 |
| 2 | 디버그 로그 `sanitized path join`: `site/docs/v1/devices`, `site/docs`, `site/docs/docs` | (창) 서랍 이름에 주소를 그대로 붙인다 | 남김. `/` 만 `site/docs` + `index.html` 로 파일이 있었다. 앞부분을 떼지 않는다 |
| 3 | 변형 2(같은 docs 를 맨 뒤에): 셋 다 npg, 200 / 404 / 404 | npg 가 다 받고 `/v1/devices` 만 200 (학습자) | 남김. 매처 없는 npg 가 앞에서 모든 요청을 가져가 docs 는 한 번도 불리지 않았다(가려진 규칙). 순서만으로는 둘 다 살릴 수 없다 |
| 4 | 변형 3(매처 `/docs/*` 를 단 docs 를 0번에): `/v1/devices` 200 npg, `/` 404 npg, `/docs/` 404 docs | 404, 404, 200 (학습자) | 버림(`/` 만 맞음). 매처가 API 를 살렸다. `/docs/` 는 맞는 라우트에 갔지만 `site/docs/docs/` 를 찾아 404 다 |
| 5 | 고친 판(매처 + `strip_path_prefix "/docs"`): `/v1/devices` 200 npg, `/` 404 npg, `/docs/` 200 docs | (예측 없이 실행) | 남김. `/docs/` → `/` → `site/docs/` + `index.html` |

## 실행 전 예측

2026-11-04 에 적었고 실행 뒤 고치지 않았다.

1. 변형 1: "저 device가 Npgproxy가 받고 fakeinv에서 저 요청을 받고 200을 호출하는거 아닌가?" (그 앞에 "예측 1에서는 시작부터 달라서 못받을거 같고 이후 /와 Docs는 앞에 값이 동일하니 받을 수 있지 않을까요?" → 정리하면 `/v1/devices` npg 200, `/` docs 200, `/docs/` docs 200)
2. 변형 2: "npg_route가 먼저 오면 받는 창구가 모두 npg가되고 200 나머지 404 이렇게되나?"
3. 변형 3: "저렇게 되면 모두 caddy로 진행되고 404, 404, 200으로 나오지 않을가요" (순서는 `/v1/devices`, `/`, `/docs/`. 디스크 경로 예측은 없음)
4. 고친 판: 예측 없이 바로 실행했다.

## 실행 (사용자가 친 명령)

```bash
# 세 손님에게 보내고 상태 코드와 도장만 보는 함수
probe() {
  for u in /v1/devices / /docs/; do
    printf '%-14s ' "$u"
    curl -s \
      -o /dev/null \
      -D - \
      "http://127.0.0.1:8000$u" | grep -iE '^HTTP|x-edge-route' | tr -d '\r' | paste -sd' ' -
  done
}

# 변형 1 — 매처 없는 docs 를 맨 앞(0번)에 끼움
./run.sh load scenarios/e5-internal-tls.json
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e2-docs-route.json
probe

# 원리 확인 — file_server 가 만든 디스크 경로
./run.sh admin PUT /config/logging scenarios/e2-debug-logging.json
probe
grep 'sanitized path join' .run/logs/caddy.log | tail -3

# 변형 2 — 같은 docs 를 맨 뒤에 덧붙임
./run.sh load scenarios/e5-internal-tls.json
./run.sh admin POST /config/apps/http/servers/edge/routes scenarios/e2-docs-route.json
probe

# 변형 3 — 매처 /docs/* 를 단 docs 를 맨 앞에
./run.sh load scenarios/e5-internal-tls.json
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e2-docs-route-matched.json
probe

# 고친 판 — 매처 + 앞부분 떼기
./run.sh load scenarios/e5-internal-tls.json
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e2-docs-route-fixed.json
probe
```

배열 칸 주소(`routes/0`)에 PUT 하면 그 자리에 끼워 넣고 원래 있던 것은 한 칸 뒤로 밀린다. 배열 주소(`routes`)에 POST 하면 맨 끝에 덧붙인다. `load` 는 설정 전체를 파일 내용으로 바꾸므로 변형마다 처음 상태로 돌아간다(디버그 로그 수준도 함께 원래대로 돌아간다).

## 관찰

- 변형 1: `/v1/devices` 404 docs, `/` 200 docs, `/docs/` 404 docs. npgproxy 로 간 요청이 없다.
- 디버그 로그 `sanitized path join` 의 `result`: `/v1/devices` → `site/docs/v1/devices`, `/` → `site/docs`, `/docs/` → `site/docs/docs`.
- 변형 2: `/v1/devices` 200 npg, `/` 404 npg, `/docs/` 404 npg. docs 도장은 한 번도 찍히지 않았다.
- 변형 3: `/v1/devices` 200 npg, `/` 404 npg, `/docs/` 404 docs.
- 고친 판: `/v1/devices` 200 npg, `/` 404 npg, `/docs/` 200 docs.
- 붙여 넣은 출력에 `probe` 정의 전 실행으로 `command not found` 가 한 번 있었다. PUT 은 이미 성공했으므로 PUT 을 다시 보내지 않고 `probe` 만 정의해 다시 실행했다(다시 보내면 docs 가 하나 더 끼어든다).

## 전후 수치

| 판 | `/v1/devices` | `/` | `/docs/` |
|---|---|---|---|
| 처음(npg 하나) | 200 npg | 404 npg | 404 npg |
| 변형 1 docs(매처 없음) 맨 앞 | **404 docs** | 200 docs | 404 docs |
| 변형 2 docs(매처 없음) 맨 뒤 | 200 npg | 404 npg | 404 npg |
| 변형 3 docs(`/docs/*`) 맨 앞 | 200 npg | 404 npg | 404 docs |
| 고친 판 | 200 npg | 404 npg | **200 docs** |

| 요청 | 변형 3 디스크 경로 | 고친 판 디스크 경로 |
|---|---|---|
| `/docs/` | `site/docs/docs/` (없음) | `site/docs/` + `index.html` |

## 포스트모템

- 타임라인: npg 하나 → docs 를 맨 앞에(API 가 깨짐) → 디버그 로그로 디스크 경로 확인 → docs 를 맨 뒤에(docs 가 가려짐) → 매처를 달아 맨 앞에(API 는 살고 docs 는 서랍 칸이 틀림) → 매처 + 앞부분 떼기(둘 다 정상) → 파일 설정으로 돌아옴.
- 근본 원인
  - Caddy 의 라우트 목록은 적힌 순서대로 도는 한 줄의 미들웨어 사슬이고, 응답을 쓰는 핸들러를 가진 처음 맞는 라우트가 이긴다. 매처 없는 라우트는 모든 요청과 맞으므로 그보다 뒤의 라우트를 모두 가린다(ADR 0016 개념 1).
  - `file_server` 는 `root` 와 요청 경로를 그대로 이어 붙인다. 하위 경로에 붙인 사이트는 앞부분을 따로 떼야 한다(ADR 0016 개념 2).
- 조치: [ADR 0016](../adr/0016-edge-route-order-and-host-split.md)
  - 매처 있는 라우트를 앞에, 매처 없는 라우트는 맨 끝에 하나만 둔다.
  - 두 번째 서비스는 기본으로 호스트 이름으로 나눈다. 하위 경로가 꼭 필요하면 매처와 앞부분 떼기를 함께 둔다.
- 진단 장치: 라우트마다 `X-Edge-Route` 도장을 달아 두면 어느 라우트가 답했는지 응답 헤더만 보고 가린다. 상태 코드만 보면 변형 1 의 404(docs 가 막음)와 변형 2 의 404(npg 로 갔지만 fakeinv 가 모름)를 구별할 수 없다.
- 재발 방지: 제품 설정(`deploy/caddy/caddy.json`)은 바꾸지 않았다. 규칙은 ADR 0016 으로 남긴다.

## 원문 근거

- caddy v2.11.7
  - `modules/caddyhttp/routes.go` L45-49(핸들러 사슬, 어느 핸들러든 사슬을 멈출 수 있음), L256-324 `wrapRoute`(매처가 안 맞으면 다음 라우트로, 맞으면 이 라우트의 핸들러를 앞에 끼움)
  - `modules/caddyhttp/headers/headers.go` L111, `modules/caddyhttp/rewrite/rewrite.go` L152(다음으로 넘김)
  - `modules/caddyhttp/fileserver/staticfiles.go` L338·L347(`root` 와 요청 경로로 파일 경로), L783-788(파일이 없으면 `pass_thru` 가 아닌 한 404 로 끝냄)
  - `modules/caddyhttp/caddyhttp.go` L252-273 `SanitizedPathJoin`
  - `modules/caddyhttp/rewrite/rewrite.go` L78 `StripPathPrefix`
- Go go1.25.1 `src/net/http/server.go` L2551-2553(가장 구체적인 패턴), `src/net/http/example_test.go` L66-68(`StripPrefix`)

## 이어 읽을 노트

- 10-02.Caddy 를 앞단에 두고 백엔드로 리버스 프록시하며 정적 파일과 자동 HTTPS 를 붙인다.md (§2 라우트와 매처, 면접 2)
- 10-01.Caddy 는 JSON 설정을 관리 API 로 실시간 바꾸고 모듈과 어댑터로 넓힌다.md (§5 미들웨어가 빠져나가는 길)
- C1 리허설 [ch10-c1-admin-api-control.md](ch10-c1-admin-api-control.md) (관리 API 로 한 칸 고치기)
