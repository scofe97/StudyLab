# 0019. 커스텀 Caddy 모듈은 따로 둔 go.mod 에 두고 xcaddy 로 빌드한다

- 상태: 채택
- 날짜: 2026-11-05
- 장: ch10

## 맥락

리허설 E6 에서 Caddy 에 우리가 만든 모듈을 넣는다. Caddy 는 설정으로 바꿀 수 없는 동작을 모듈(Go 패키지)로 넓히는 구조이고, 모듈은 Caddy 와 같은 바이너리에 링크되어야 한다. 그래서 모듈 코드를 저장소 어디에 두고 어떤 `go.mod` 로 묶을지 정해야 한다.

저장소 규칙과 부딪치는 지점이 여기다. `AGENTS.md` 는 루트 `go.mod` 하나로 모든 의존성 버전을 관리하는 단일 버전 규칙(One-Version Rule)을 따른다고 적고 있고, 이 문단에는 예외가 적혀 있지 않다. 모듈 코드를 루트에 두면 규칙을 그대로 지키지만, Caddy 와 그 의존을 루트 `go.mod` 가 끌어와야 한다.

이 비용을 추측하지 않고 사본에서 재 보았다. 저장소 사본의 루트 `go.mod` 에 `github.com/caddyserver/caddy/v2 v2.11.7` 을 넣고 Go 와 Bazel 을 각각 돌렸다.

- Go: `go build ./...` 와 `go test ./net/npg/... -count=1` 이 통과했다. 루트 `go 1.25.1` 은 `go 1.26.0` 으로 올랐고 require 는 22줄에서 174줄이 됐다. 기존 의존도 함께 올랐다(예: pflag v1.0.5 → v1.0.10).
- Bazel: `bazel run //:gazelle` 은 통과했지만, caddy 를 import 하는 타깃의 빌드는 두 단계로 실패했다. 먼저 `MODULE.bazel` 의 `use_repo` 에 caddy 가 없어서 실패했고, `bazel mod tidy` 로 고치자 외부 모듈 `cel-go v0.32.0` 에서 `no such target '@@gazelle++go_deps+dev_cel_cel_go//interpreter:interpreter'` 로 실패했다. caddy 의 `modules/caddyhttp` 가 cel-go 에 의존하는데, gazelle 이 만든 그 외부 모듈의 BUILD 파일에 `interpreter` 타깃이 없다.
- 이 실패는 해당 타깃에만 머물지 않는다. `bazel test //net/npg/...` 는 분석 단계에서 중단되어 시험이 0개로 끝났다. CI 가 `bazel test //...` 를 쓰므로 같은 일이 CI 에서 일어난다. 문제의 타깃을 제외하면 나머지 시험 16개는 통과했다.

즉 단일 버전 규칙을 지키면 Go 쪽 비용은 크지 않고, Bazel 쪽 비용이 CI 를 막는다.

### 개념. xcaddy 가 하는 일

Caddy 문서(Extending Caddy)는 모듈을 "In a new Go module" 에서 시작하라고 안내한다. 그리고 xcaddy 가 하는 일을 이렇게 설명한다. xcaddy 는 Caddy 와 플러그인을 모두 require 하는 새 Go 모듈을 만들고(로컬 개발 버전을 쓰도록 적절한 replace 를 넣어), 그 플러그인이 컴파일에 들어가도록 빈 import 를 더한다. 즉 모듈 코드 자체는 Caddy 를 require 하는 하나의 Go 모듈이면 되고, 최종 바이너리를 조립하는 모듈은 xcaddy 가 그때그때 만든다.

## 선택지

### A. 루트 go.mod 에 둔다

- 단일 버전 규칙을 그대로 지킨다. 루트에서 `go test ./...` 로 모듈도 함께 시험된다.
- 위 사본 점검대로 Go 1.26 상승, 의존 174줄, 기존 의존 버전 상승이 저장소 전체에 퍼진다.
- Bazel 에서 caddy 를 import 하는 타깃이 빌드되지 않아 CI 가 분석 단계에서 실패한다.

### A-1. A 에 cel-go 의 `gazelle_override` 를 더한다

- `MODULE.bazel` 에서 cel-go 의 gazelle 지시어를 주어 `interpreter` 타깃 문제를 풀 수 있을 것으로 보인다.
- 이번에는 검증하지 않았다. 풀리더라도 caddy 의 나머지 외부 의존 약 150개가 같은 방식으로 더 깨질 수 있고, 저장소 전체가 Go 1.26 과 올라간 의존을 떠안는 문제는 남는다.

### A-2. A 에 모듈 패키지를 gazelle 에서 제외한다

- 모듈 폴더를 gazelle 에서 빼면 `BUILD.bazel` 이 생기지 않아 Bazel 이 이 코드를 보지 않는다. CI 는 통과할 것이다.
- 그런데 루트 `go.mod` 에는 caddy 가 남아 Go 1.26 상승과 의존 174줄은 그대로다. 비용만 지고 Bazel 에서는 시험도 못 하는 절충이라 얻는 것이 적다.

### B. 폴더에 따로 go.mod 를 둔다

- 모듈 코드는 `net/npg/deploy/caddy/caddymod/` 에 두고 그 폴더에서 `go mod init npg.local/deploy/caddymod` 로 자기 `go.mod` 를 가진다. 루트 `go.mod`·`go.sum`·`MODULE.bazel` 은 바뀌지 않는다.
- 루트 `BUILD.bazel` 에 `# gazelle:exclude net/npg/deploy/caddy/caddymod` 를 두어 gazelle 이 이 폴더를 건드리지 않게 한다.
- 최종 바이너리는 `xcaddy build v2.11.7 --with npg.local/deploy/caddymod=./caddymod` 로 만든다.
- 루트 `go test ./...` 와 CI 의 `bazel test //...` 가 이 모듈을 시험하지 않는다. 시험은 그 폴더에서 따로 돌려야 한다.
- xcaddy 가 필요하다.

## 결정

B 로 정한다. 학습자가 A 를 먼저 골랐다가 위 사본 점검 결과와 아래 근거를 보고 B 로 바꿨다(2026-11-05). 근거는 네 가지다.

1. Go 공식 위키의 Modules FAQ(Should I have multiple modules in a single repository?)는 기본을 이렇게 말한다. 여러 모듈을 한 저장소에 두는 구성은 "almost always easier and simpler to manage a single-module repository" 라고 하고, Russ Cox 의 말을 빌려 "one repo = one module" 을 일반 관례로 든다. 그러면서도 따로 `go.mod` 를 두는 것이 합리적인 경우를 두 가지 든다. 그중 하나가 작은 패키지에 복잡한 의존성을 가진 예제(위키의 예는 kubernetes 를 쓰는 예제)가 붙을 때, 그 예제 디렉터리에 자기 `go.mod` 를 두는 것이다. Caddy 모듈은 이 모양과 같다. 우리 쪽 코드는 작고, 끌고 오는 의존은 Caddy 전체(약 150개)다.
2. Caddy 문서(Extending Caddy)는 모듈을 새 Go 모듈로 만들라고 하고, xcaddy 가 Caddy 와 플러그인을 require 하는 새 Go 모듈을 만들어 조립한다고 설명한다. 모듈을 따로 둔 `go.mod` 에 두는 것이 이 도구가 전제하는 모양이다.
3. 단일 버전 규칙의 목적은 한 바이너리에 같이 링크되는 코드가 서로 다른 버전의 같은 라이브러리를 요구하는 다이아몬드 충돌을 막는 것이다. Caddy 바이너리는 루트 모듈의 다른 코드와 같이 링크되지 않는다. 모듈과 Caddy 만 들어가는 별도 바이너리라서 이 규칙이 막으려는 충돌이 생길 자리가 없다.
4. 저장소 안에 선례가 있다. 참조 구현(`chapters/ch07-10-service/edge/caddymod/`)이 같은 방식으로 따로 `go.mod`(go 1.26.0)를 두고, 루트 `BUILD.bazel` 이 그 폴더를 gazelle 에서 제외한다. 이번 모듈도 같은 규약을 따른다.

세부는 다음과 같이 정한다.

- 모듈 경로는 `npg.local/deploy/caddymod` 다. 공개 저장소 주소가 아니라서 `go get` 으로 받을 수 없고, xcaddy 의 `--with 경로=./caddymod` 가 로컬 폴더로 대체한다.
- `go.mod` 는 `go 1.26.0` 이고 `github.com/caddyserver/caddy/v2 v2.11.7` 을 require 한다. 모듈 코드를 쓰기 전에는 `go mod tidy` 를 돌리지 않는다. 코드가 없으면 tidy 가 caddy 를 지우기 때문이다.
- 빌드 스크립트는 `net/npg/deploy/caddy/build.sh` 다. `bin/caddy`(우리 모듈 포함)와 `bin/caddy-stock`(표준 모듈만)을 만들고, `bin/` 은 `net/npg/.gitignore` 로 무시된다.

## 결과

- 쉬워지는 것: 루트 `go.mod`·`go.sum`·`MODULE.bazel` 이 바뀌지 않는다. Go 1.25.1 과 기존 의존 버전이 그대로이고, 저장소 전체 CI 가 Caddy 의존의 영향을 받지 않는다.
- 어려워지는 것: 루트 `go test ./...` 와 CI 의 `bazel test //...` 가 이 모듈을 시험하지 않는다. 시험은 `net/npg/deploy/caddy/caddymod/` 에서 `go test ./...` 로 따로 돌린다. 필요해지면 `arch/gateway` 처럼 `sh_test`(태그 `manual`)로 감싸 Bazel 에서 이름을 지정해 돌릴 수 있다. 이번에는 하지 않는다.
- 규칙과의 관계: `AGENTS.md` 의 단일 버전 규칙 문단에는 이 예외가 적혀 있지 않다. 이 문서가 예외의 근거이고, `AGENTS.md` 는 고치지 않았다. 참조 구현의 제외 한 줄도 같은 상태다.
- 새로 필요한 도구: xcaddy(v0.4.7 로 설치). 사본 점검에서 xcaddy 없이 `go build` 로 `modules/standard` 를 넣은 Caddy 를 만드는 방법도 동작했지만, 그 방법은 루트 `go.mod` 가 caddy 를 require 해야 해서 이번 결정과 맞지 않는다.
- 다시 볼 조건: 모듈이 여럿으로 늘거나 다른 lab 이 같은 Caddy 모듈을 가져다 쓰게 되면 `libs/` 승격과 위치를 다시 따진다. A-1 의 `gazelle_override` 가 실제로 통하는지는 확인하지 않았으므로, 단일 `go.mod` 로 돌아가려면 그 검증이 먼저다.

## 원문 근거

2026-11-05 에 원문을 받아 문장을 찾아 확인했다.

- Go 위키 Modules(go.dev/wiki/Modules) 「Should I have multiple modules in a single repository?」: "it is almost always easier and simpler to manage a single-module repository rather than multiple modules in an existing repository", Russ Cox 인용 "you probably want to adopt the usual convention that one repo = one module", 따로 `go.mod` 를 두는 경우의 예 "usage examples where the examples themselves have a complex set of dependencies (e.g., perhaps you have a small package but include an example of using your package with kubernetes)" 와 "an example or _example directory with its own go.mod".
- Caddy 문서 Extending Caddy(caddyserver.com/docs/extending-caddy): "In a new Go module, paste the following template into a Go file", "xcaddy is simply making a new Go module that requires both Caddy and your plugin (with an appropriate replace to use your local development version)".
- 참조 구현(저장소 안): `chapters/ch07-10-service/edge/build.sh`·`edge/caddymod/go.mod`, 루트 `BUILD.bazel` 의 `gazelle:exclude`. `deploy/caddy/build.sh` 는 `edge/build.sh` 에서 `MODULE` 값만 `npg.local/deploy/caddymod` 로 바꿔 옮겼다.

## 관련 노트

- 10-01.Caddy 는 JSON 설정을 관리 API 로 실시간 바꾸고 모듈과 어댑터로 넓힌다.md
