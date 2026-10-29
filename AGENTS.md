# 코드 및 모노레포 컨벤션 — runners-lab

이 문서는 `scofe97/runners-lab` 모노레포에서 사람과 AI 에이전트가 코드를 작성하거나 구조를 바꿀 때 지켜야 할 규칙을 정의한 정본입니다.

## 저장소 목적

- runners-lab 은 개념 학습을 거친 뒤 직접 구현하고 검증하는 Phase 3 실습 코드를 모으는 공개 모노레포로, 분산된 실습 환경을 단일 빌드 시스템 아래 통합해 관리합니다.
- 학습·정독·설계 문서는 별도 저장소인 `runners-high` 의 `write/` 트리에 두고 각 lab 의 `README.md` 와 상호 링크하여, 실습 코드베이스를 가볍게 유지하면서 개념 정리와 실행 코드 사이의 양방향 탐색을 보장합니다.

## 최상위 도메인 축

모노레포 루트는 도메인 계층에 따라 8개 축으로 분류합니다.

| 디렉토리 | 설명 |
|---|---|
| `net/` | 네트워크 소켓, 프로토콜 구현, CNI, 패킷 분석, 터널링 등 네트워크 계층 실습을 둔다. |
| `k8s/` | 쿠버네티스 애플리케이션 배포, 인-클러스터 제어, 오케스트레이션 실습을 둔다. |
| `jvm/` | Java 가상 머신 내부 구조, 바이트코드, 메모리 모델 및 GC 실습을 둔다. |
| `arch/` | API 게이트웨이, 연동 허브, 프록시 등 분산 시스템 아키텍처 패턴 실습을 둔다. |
| `ai/` | LLM 프롬프트 인프라, AI 모델 연동 및 평가 실습을 둔다. |
| `libs/` | 두 개 이상의 lab 이 실제로 공유하는 공용 라이브러리를 둔다. |
| `tools/` | 민감정보 검사, 변경 영향 분석, 스캐폴딩 등 저장소 관리 도구를 둔다. |
| `third_party/` | 외부 참조 라이브러리 및 소스 클론 안내 문서(`README.md`)만 둔다. |

### 도메인 선택 기준

- 새 lab 을 어느 도메인에 둘지 모호할 때는 주된 관측 대상과 관심사 계층에 맞춰 의존성 경계가 흐려지지 않도록 "그 lab 이 가장 많이 다루는 계층"을 기준으로 선택합니다.

## 이름 규칙

- 모노레포 내부의 lab 디렉토리는 저장소 이름(`runners-lab`)과의 중복을 피하기 위해 `-lab` 접미사를 떼고 kebab-case 로 짓습니다.
- 책 실습 lab 은 개념 학습 노트 디렉토리와 실습 코드 디렉토리 사이의 대응 관계를 명확히 유지하도록 `runners-high` 의 `write/` 내 책 폴더 약어와 일치시켜 짓습니다.
- 이름 변환 예시:
  - `gonet-lab` → `net/gonet`
  - `k8s_in_action` → `k8s/k8s-in-action`
  - `cntd-control-plane-lab` → `net/cntd-control-plane`
  - `gateway-lab` → `arch/gateway`

## 빌드 규칙

- 모든 패키지(디렉토리)에 `BUILD.bazel` 을 둡니다. 그래야 Bazel 이 패키지 경계를 명확히 인식하여 빌드 그래프와 캐시를 세밀하게 계산할 수 있습니다.
- Go 코드는 저장소 루트의 `go.mod` 하나로 모든 의존성 버전을 관리하는 단일 버전 규칙(One-Version Rule)을 따릅니다. 서브모듈마다 라이브러리 버전이 갈라져 발생하는 다이아몬드 의존성 문제를 방지하기 위해서입니다.
- Go 의 `BUILD.bazel` 파일은 `bazel run //:gazelle` 로 자동 생성하고 갱신합니다. 수작업 관리로 생길 수 있는 타깃 누락과 휴먼 에러를 방지하려는 목적입니다.
- 타깃의 `visibility` 기본값은 lab 간 불필요한 결합을 막고 독립적인 실험 환경을 격리하기 위해 자기 lab 내부(`//<domain>/<lab>:__subpackages__` 또는 private)로 제한합니다.
- 코드를 `libs/` 로 승격하는 조건은 두 개 이상의 lab 이 실제로 import 하여 사용할 때로 한정합니다. 사전 추측만으로 공용 라이브러리를 만들면 조기 추상화와 불필요한 의존성이 생길 수 있기 때문입니다.
- Java 빌드 방식:
  - `arch/gateway`, `arch/hub` 는 Spring Boot 4.1.1 / Java 25 앱이라 Gradle 을 그대로 씁니다. Bazel 에서는 `sh_test`(태그 `manual`, `requires-network`)가 격리된 복사본에서 `./gradlew test` 를 호출합니다. boot jar 를 만드는 `rules_spring` 이 Bazel 9 에서 패치 없이는 돌지 않고, 앱이 두 개뿐이라 네이티브 전환의 캐시 이득이 크지 않기 때문입니다. `manual` 태그가 붙어 있어 `bazel test //...` 에는 포함되지 않으며, `bazel test //arch/gateway:gradle_test` 처럼 이름을 지정해 돌립니다.
  - `jvm/jvm-practice` 는 Bazel 네이티브(`java_library`, `java_junit5_test`, Maven 의존은 `rules_jvm_external`)로 빌드합니다. 장별로 작은 순수 Java 모듈이라 파일 단위 증분 캐시와 `rdeps` 이득이 크고 boot jar 가 필요 없기 때문입니다. `jvm/jvm-practice/tsj/` 의 Maven 프로젝트는 그대로 옮겨 두었으며 BUILD 를 두지 않았습니다.
  - Java 규칙은 항상 `@rules_java//java:defs.bzl` 에서 load 해야 합니다. `.bazelrc` 의 JDK 플래그 네 줄이 하나라도 빠지면 내장 JDK 로 컴파일됩니다.

- 코드가 없는 lab(`ai/flats`, `ai/infra-ai`, `k8s/k8s-in-action`, `k8s/networking-and-kubernetes`, `net/paw`, `net/cntd-control-plane`)의 `BUILD.bazel` 은 `//tools:defs.bzl` 의 `lab_files()` 하나만 둡니다. `tools/changed-targets.sh` 가 쓰는 target-determinator 가 공백이나 한글처럼 라벨로 쓸 수 없는 파일명을 파싱하지 못하므로, 이 매크로가 해당 라벨들을 미리 제외해 줍니다.
- `k8s/networking-and-kubernetes` 의 `*.go` 둘은 같은 디렉터리의 단독 실행용 `main` 이라 `//go:build ignore` 를 붙였고, `go run <파일>` 로 따로 돌립니다.
- `k8s/k8s-in-action` 책 예제 앱 셋과 `ai/infra-ai`, `net/paw` 의 Go 코드는 루트 `go.mod` 로 합쳐 함께 빌드합니다. 책 예제의 컴파일된 실행 파일(`quiz-api`, `node-agent`)은 이력에서 빼고 `.gitignore` 에 넣었습니다.
- 저장소에 `.gitattributes` 가 없어 줄 끝은 `core.autocrlf` 설정을 따릅니다. 작업 트리에 CRLF 파일이 남으면 git worktree 와 해시가 달라져 `tools/changed-targets.sh` 가 무관한 타깃을 내므로 항상 LF 로 둡니다.

## lab 폴더 필수 파일

각 lab 폴더는 다음 두 파일을 반드시 포함해야 합니다.

1. `README.md`: 실습 목적(무엇을 실습하고 관측하는지), 관련 write 문서 링크(`runners-high` 경로), 실행 명령(빌드·실행·검증 방법)을 기술합니다. 저장소 방문자나 에이전트가 다른 자료 없이도 실습의 맥락과 재현 방법을 즉시 파악할 수 있도록 돕습니다.
2. `BUILD.bazel`: 해당 lab 내 소스들을 빌드하고 테스트하는 타깃을 정의합니다. 그래야 모노레포의 통합 빌드 및 테스트 파이프라인에 즉시 편입되어 지속적인 검증을 거치게 됩니다.

## 공개 저장소 규칙

이 저장소는 GitHub 에 public 으로 공개됩니다.

- 회사명, 고객사명, 인명, 사내 내부 주소(도메인, 내부 IP 등)는 코드, 문서, 주석, 커밋 메시지 또는 예시에 일절 포함하지 않습니다. 오픈소스 공개 환경에서 내부 기밀과 조직 고유 정보가 유출되는 상황을 원천 차단하기 위한 조치입니다.
- 커밋 전 `tools/sensitive-check.sh` 와 git pre-commit 훅을 통해 민감정보를 검사합니다. 작업자의 부주의로 인한 민감정보 커밋을 커밋 시점에 자동으로 차단하기 위해서입니다.
- 민감정보 deny 목록은 저장소 외부 경로인 `~/.config/runners-lab/deny.txt` 에 보관하며, deny 목록의 내용 자체를 저장소에 포함하지 않습니다. deny 목록 파일 자체가 사내 명칭 등 민감정보를 포함하고 있어 파일 자체가 유출원이 될 수 있기 때문입니다.
- 외부 코드는 저장소 내부에 직접 벤더링하지 않습니다. `third_party/` 에는 외부 원본 저장소 클론 위치와 참조 방법을 안내하는 `README.md` 만 둡니다. 외부 소스로 인한 저장소 용량 비대화와 라이선스 오염을 막고 외부 변경사항을 깨끗하게 관리하려는 목적입니다.

## 변경 영향

- 변경 시에는 바뀐 타깃만 선별하여 테스트하는 `tools/changed-targets.sh` 를 사용합니다. 모노레포 전체 테스트로 인한 빌드 시간 낭비를 막고 코드 변경에 대한 피드백 루프를 빠르게 유지하기 위해서입니다.
- CI(`.github/workflows/ci.yml`)는 `bazel-contrib/setup-bazel` 0.19.1 을 커밋 SHA 로 고정해 쓰고 있습니다. 타깃이 수백 개 이하라 전체 시험 시간이 짧고 `disk-cache` 는 GitHub 캐시 10GB 한도를 쉽게 넘기므로, `bazelisk-cache` 와 `repository-cache` 는 켜고 `disk-cache` 는 끈 채 `bazel test //...` 를 실행합니다. `tools/changed-targets.sh` 는 CI 에는 쓰지 않고 로컬에서 바뀐 타깃만 돌릴 때 활용하도록 구성했습니다.
