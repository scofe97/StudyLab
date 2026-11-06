# E6 공통 차단 규칙을 Caddy 모듈로 만든다

- 장: ch10
- 날짜: 2026-11-05 ~ 2026-11-06
- 앞 장면과의 연결: E2 에서 `/docs/*` 정적 갈래(`file_server`)를 npg 갈래 앞에 붙였다. 정적 폴더가 생기면서 그 안에 실수로 들어간 파일이 밖으로 나갈 길도 함께 생겼다.

## 신고

"정적 문서 사이트에서 `.env` 파일이 내려받혔다. 서버마다 막는 설정을 복사하지 말고 공통 모듈 하나로 만들어 달라."

## 진단 과정

| 순서 | 본 신호(명령·지표) | 세운 가설 | 남김 / 버림과 이유 |
|---|---|---|---|
| 1 | `curl /docs/.env` → 200, 본문 `DB_PASSWORD=not-a-real-secret` | `file_server` 는 점으로 시작하는 파일을 따로 가리지 않는다 | 남김. 재현됨 |
| 2 | 서버마다 `not path` 매처를 복사하는 안 | 복사본이 늘면 한 곳만 빠져도 샌다 | 버림. 요청이 공통 모듈이다 |
| 3 | 모듈 코드 위치: 루트 `go.mod`(A) vs 폴더에 따로 `go.mod`(B) | A 는 Bazel 이 cel-go 외부 모듈 BUILD 오류로 caddy import 타깃을 못 빌드한다 | B 를 남김. [ADR 0019](../adr/0019-custom-caddy-module-separate-go-mod.md) |

## 실행 전 예측

학습자 원문 그대로 적는다.

- 개념 문답 — 틀린 접두사(`"a/b"`)를 Validate 와 ServeHTTP 중 어디서 거절하나: "Validate에서 검사하는게 좋겠지? 괜히 소켓 생성까지 안가고"
- 예측 1 — `list-modules` 차이: "별차이 없을거 같음" / 크기: "우리 모듈이 새로 끌고 들어오는 코드는 말 그대로 모듈에 있는 코드 아냐?, Caddy가 이미 들고있던건 import로 쓰고있던 공통 라이브러리들? -> 그러면 특이케이스가 아니라면 그냥 순수하게 코드함수만 들어간 크기가 있게쎈?"
- 예측 2 — 기본 접두사 `"."` 에서 `/.well-known/security.txt`, `/%2eenv`, `/docs/v1.2/`: 첫째 막힘, "/.env가 되기떄문에 막히는거구나"(둘째 막힘), 셋째 통과
- 예측 3 — Validate 가 거절하는 설정(`[".env", "/admin"]`)을 관리 API 로 올리면: "에러 났으면 에러난 설정은 버리고 이전껄 그대로 읽습니다 즉 3번에서 막힌다는거 아닌가?"
- 예측 4·5 — 생략. 학습자 목표("네트워크 핵심을 배우는게 1차 목표")에 비추어 Caddy·Go 설계 쪽 장면이라 돌리지 않았다.

## 실행

모듈 코드는 학습자가 조각 1·2(등록·구조체·Provision·Validate)를 입력하고 나머지는 창이 썼다. 빌드·시험·실행은 학습자 결정("검증도 너가한다")에 따라 창이 돌렸다.

```bash
cd deploy/caddy/caddymod
go mod tidy
go vet ./...
go test ./... -count=1 -race
cd ..
./build.sh
bin/caddy-stock list-modules --skip-standard
bin/caddy list-modules --skip-standard
go version -m bin/caddy
size -m bin/caddy-stock
size -m bin/caddy

./run.sh down
CADDY_BIN=bin/caddy ./run.sh up scenarios/e6-base.json
./run.sh admin PUT /id/docs_route/handle/2 scenarios/e6-restrict.json
./run.sh load scenarios/e6-base.json
./run.sh admin PUT /id/docs_route/handle/2 scenarios/e6-restrict-bad.json
./run.sh admin GET /id/docs_route/handle
```

- `scenarios/e6-base.json`: `caddy.json` 의 npg 갈래 앞에 E2 의 `docs_route` 를 끼운 설정
- `handle/2`: `docs_route` 의 핸들러 배열(`headers`, `rewrite`, `file_server`)에서 `file_server` 바로 앞 자리
- 예측 3 은 `/docs/.env` 를 0.05초 간격으로 60번 부르는 도중에 PUT 했다

## 관찰

**모듈 표와 크기 (예측 1).** `list-modules` 는 표준 137줄이 같고, `bin/caddy` 에만 `http.handlers.restrict_prefix` 와 `Non-standard modules: 1` 이 더 있다. 파일 크기 차이는 160 B 다. 섹션으로 보면 `__text` +3,696 B, `__rodata` 두 곳 +1,120 B, `__go_buildinfo` +160 B 이고 `Segment __TEXT` 는 40,566,784 B 로 같다. caddyhttp·zap·net/http 는 표준 Caddy 에 이미 들어 있어 새로 링크된 것은 모듈 함수뿐이고, 늘어난 코드는 16 KB 페이지 여백 안에 들어갔다. `go version -m` 에는 `dep npg.local/deploy/caddymod` 과 replace 대상인 로컬 절대 경로가 남는다. `-trimpath` 로 빌드해도 이 경로는 지워지지 않는다.

**경계 경로 (예측 2).**

| 경로 | 모듈 없음 | 기본 `restrict_prefix` |
|---|---|---|
| `/docs/.env` | 200 | 404 |
| `/docs/.secrets/token.txt` | 200 | 404 |
| `/docs/.well-known/security.txt` | 200 | 404 |
| `/docs/%2eenv` | 200 | 404 |
| `/docs/v1.2/` | 200 | 200 |

`url.Parse("/%2eenv")` 는 `Path="/.env"`, `RawPath="/%2eenv"` 를 준다. 모듈은 풀린 `r.URL.Path` 를 보므로 인코딩으로 피해 가지 못한다. 대신 `/a%2fb` 도 `Path="/a/b"` 로 풀려 조각 경계가 바뀐다.

**거절되는 설정 (예측 3).**

```
[HTTP 500]
{"error":"loading new config: loading http app module: provision http: server edge: setting up route handlers: route 0: loading handler modules: position 2: loading module 'restrict_prefix': http.handlers.restrict_prefix: invalid configuration: prefixes[1] \"/admin\": 접두사는 세그먼트 하나를 가리키므로 '/' 를 넣을 수 없다"}
```

직후 핸들러 목록은 `["headers","rewrite","file_server"]` 로 옛 설정 그대로였고, `/config/` 에 `restrict_prefix` 는 0번 나왔다. 그 사이 손님 요청 60개는 모두 200 이었다. Caddy pid 는 바뀌지 않았다.

## 채점

| 문항 | 결과 | 빈 곳 |
|---|---|---|
| Validate vs ServeHTTP | 결론 맞음, 이유 절반 | `"a/b"` 는 어떤 조각과도 안 맞아 **조용히 아무것도 막지 않는다**. 손님은 고칠 수 없는 사고를 겪고, 운영자는 설정을 올리는 순간 오류를 받아야 고칠 수 있다. 소켓 생성 전이라는 점은 핵심이 아니다 |
| 예측 1 | 둘 다 맞음 | 파일 크기만 보면 페이지 정렬 때문에 160 B 로 작게 보인다 |
| 예측 2 | 셋 다 맞음 | 기본값 `"."` 은 `.well-known` 까지 막는다 |
| 예측 3 | 설정·손님 요청 맞음 | 응답 코드(500)와 본문 모양은 예측하지 않았다 |

## 전후 수치

| 지표 | 개선 전 | 개선 후 |
|---|---|---|
| `/docs/.env` 응답 | 200 (가짜 비밀 본문) | 404 |
| 바이너리 크기 | 50,013,058 B | 50,013,218 B (+160 B) |
| 거절된 설정 반영 중 손님 요청 실패 | — | 0 / 60 |

## 포스트모템

- 타임라인: 2026-11-05 모듈 위치 A/B 결정(B), 뼈대·`build.sh`. 2026-11-06 모듈 코드, 시험, xcaddy 빌드, 예측 1~3 실행.
- 근본 원인: `file_server` 는 이름이 점으로 시작하는 파일을 따로 가리지 않는다. 정적 폴더에 무엇이 들어갈지는 배포하는 사람에게 달려 있다.
- 조치: `http.handlers.restrict_prefix` 모듈. 접두사 여럿을 받고, 비면 Provision 이 `"."` 로 채우고, Validate 가 빈 문자열과 `/` 가 든 접두사를 거절한다. 막을 때는 응답을 직접 쓰지 않고 `caddyhttp.Error(404, …)` 를 돌려준다.
- 재발 방지:
  - Validate 가 거절하면 Caddy 는 옛 설정을 지킨다. 그런데 옛 설정이 새는 설정이면 손님은 계속 비밀을 받아 간다(예측 3 의 60/60). 배포 자동화는 관리 API 응답이 200 이 아니면 배포 실패로 멈춰야 한다.
  - 기본값 `"."` 은 `security.txt`(RFC 9116)나 외부 ACME 클라이언트의 확인 파일까지 막는다. Caddy 자신의 HTTP-01 확인은 사용자 핸들러보다 먼저 처리되므로 깨지지 않는다(`modules/caddyhttp/server.go` 702행 `HandleHTTPChallenge`). 운영에서는 막을 접두사를 `.env`, `.git`, `.secrets` 처럼 좁혀 적는다.
  - 모듈은 L7 차단이다. 막힌 요청도 TCP·TLS 핸드셰이크와 요청 줄 수신을 다 마친 뒤에 거절된다. 경로를 볼 수 있는 대신 그만큼 비용을 치른다.

## 원문 근거

- Caddy v2.11.7 `modules/caddyhttp/server.go` 702행 — "guarantee ACME HTTP challenges; handle them separately from any user-defined handlers"
- Caddy 문서 Extending Caddy — https://caddyserver.com/docs/extending-caddy (모듈은 새 Go 모듈에 둔다, Provision → Validate 순서)
- Caddy 문서 Command Line, `caddy list-modules` — https://caddyserver.com/docs/command-line#caddy-list-modules
- Go `net/url` — `URL.Path` 는 풀린 값, `RawPath` 는 인코딩이 기본과 다를 때만 채운다
- RFC 9116 `security.txt` — https://www.rfc-editor.org/rfc/rfc9116

## 이어 읽을 노트

- runners-high `write/02_os/book/npg_network-programming-with-go/10-01` §2 「Caddy 바이너리와 빌드」, §5 restrict_prefix
- runners-high `write/01_language/book/lgo_learning-go/10-02` 「init 함수는 되도록 피합니다」, `11-04` §1 「바이너리 안의 빌드 정보」
