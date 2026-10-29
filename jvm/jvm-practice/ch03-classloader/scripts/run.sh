#!/usr/bin/env bash
# 04-01 톰캣 클래스 로더 실습 — 빌드 + 실행 러너
# 순수 Java + URLClassLoader 로 노트의 4가지 현상을 직접 재현한다.
#
# 사용법:
#   ./scripts/run.sh build       # 전체 컴파일
#   ./scripts/run.sh exp1        # 이름+로더=타입 (ClassCastException)
#   ./scripts/run.sh exp2        # 클래스패스 격리 + NoSuchMethodError
#   ./scripts/run.sh exp3        # 부모위임 깨기 + TCCL/SPI
#   ./scripts/run.sh exp4 leak   # ClassLoader Leak → Metaspace OOM
#   ./scripts/run.sh exp4 noleak # 누수 없음 → 안정
#   ./scripts/run.sh all         # build + exp1~3 순차 실행
set -euo pipefail

# --- JDK 21 고정 (현재 기본 PATH 가 Java 25 라서 명시) ---
JAVA21="/Users/user/Library/Java/JavaVirtualMachines/temurin-21.0.3/Contents/Home"
if [[ ! -d "$JAVA21" ]]; then
  echo "Java 21 경로를 찾을 수 없습니다: $JAVA21" >&2
  echo "/usr/libexec/java_home -v 21 로 경로를 확인해 스크립트를 고치세요." >&2
  exit 1
fi
export JAVA_HOME="$JAVA21"
JAVAC="$JAVA_HOME/bin/javac"
JAVA="$JAVA_HOME/bin/java"

# 스크립트 위치 기준으로 프로젝트 루트 이동
cd "$(dirname "$0")/.."
ROOT="$(pwd)"

build() {
  echo "== build (JDK: $("$JAVA" -version 2>&1 | head -1)) =="
  rm -rf out && mkdir -p out/{app,v1,v2,plugin,provider}

  # 1) 앱/부모 계층: Plugin·Codec 인터페이스 + 실험 드라이버
  "$JAVAC" -d out/app \
    src/com/acme/api/Plugin.java \
    src/com/acme/spi/Codec.java
  # 드라이버는 app 클래스(인터페이스)에 의존하므로 out/app 을 클래스패스에 둔다
  "$JAVAC" -cp out/app -d out/app \
    src/com/acme/lab/ChildFirstClassLoader.java \
    src/com/acme/lab/Exp1_TypeIdentity.java \
    src/com/acme/lab/Exp2_ClasspathIsolation.java \
    src/com/acme/lab/Exp3_TcclSpi.java \
    src/com/acme/lab/Exp4_ClassLoaderLeak.java

  # 2) 같은 FQCN 의 두 버전을 별도 디렉터리로
  "$JAVAC" -d out/v1 libsrc/v1/com/acme/Greeter.java
  "$JAVAC" -d out/v2 libsrc/v2/com/acme/Greeter.java

  # 3) 플러그인 구현(Plugin 인터페이스 필요 → out/app 참조)
  "$JAVAC" -cp out/app -d out/plugin libsrc/plugin/com/acme/plugin/HelloPlugin.java

  # 4) SPI 구현 + META-INF/services
  "$JAVAC" -cp out/app -d out/provider libsrc/provider/com/acme/provider/Base64Codec.java
  mkdir -p out/provider/META-INF/services
  cp libsrc/provider/META-INF/services/com.acme.spi.Codec out/provider/META-INF/services/

  echo "build 완료 → $ROOT/out"
}

need_build() { [[ -d out/app ]] || build; }

case "${1:-all}" in
  build) build ;;
  exp1)  need_build; echo "== 실험 1: 이름+로더=타입 =="; "$JAVA" -cp out/app com.acme.lab.Exp1_TypeIdentity ;;
  exp2)  need_build; echo "== 실험 2: 클래스패스 격리 =="; "$JAVA" -cp out/app com.acme.lab.Exp2_ClasspathIsolation ;;
  exp3)  need_build; echo "== 실험 3: TCCL/SPI =="; "$JAVA" -cp out/app com.acme.lab.Exp3_TcclSpi ;;
  exp4)
    need_build
    mode="${2:-leak}"
    echo "== 실험 4: ClassLoader Leak ($mode) — MaxMetaspaceSize=32m =="
    # 누수 모드면 OOM 으로 끝나는 게 정상이므로 set -e 에서 실패해도 메시지 출력
    "$JAVA" -XX:MaxMetaspaceSize=32m -cp out/app com.acme.lab.Exp4_ClassLoaderLeak "$mode" || {
      echo ""
      echo "→ (위 OutOfMemoryError: Metaspace 가 leak 모드의 기대 결과입니다)"
    }
    ;;
  all)
    build
    echo; "$JAVA" -cp out/app com.acme.lab.Exp1_TypeIdentity
    echo; "$JAVA" -cp out/app com.acme.lab.Exp2_ClasspathIsolation
    echo; "$JAVA" -cp out/app com.acme.lab.Exp3_TcclSpi
    echo; echo "(실험 4 는 시간이 걸리므로 ./scripts/run.sh exp4 leak 으로 따로 실행)"
    ;;
  *) echo "알 수 없는 명령: $1" >&2; exit 2 ;;
esac
