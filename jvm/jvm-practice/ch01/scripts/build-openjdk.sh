#!/usr/bin/env bash
# 《JVM 밑바닥까지 파헤치기》§1.6.4 OpenJDK 빌드 명령 박제.
# 책은 OpenJDK 12 기준이지만 현재 실습은 OpenJDK 21 LTS 권장.
# 실제 빌드는 디스크 10GB+, 시간 30분+ 소요. dry-run 기본.
#
# 사용법:
#   ./build-openjdk.sh                                  # 무엇을 할지 출력만
#   ./build-openjdk.sh -y --boot-jdk /path/to/jdk-20    # 실제 clone + configure + make 실행
#
# 환경:
#   --boot-jdk <path>   부트스트랩 JDK 위치 (필수, 빌드 대상보다 한 단계 낮은 LTS 권장)
#   --src-dir <path>    OpenJDK 소스를 받을 디렉터리 (기본: ./openjdk-src)
#   --branch <name>     체크아웃할 브랜치/태그 (기본: jdk21u의 master)

set -euo pipefail

DRY_RUN=1
BOOT_JDK=""
SRC_DIR="$(pwd)/openjdk-src"
BRANCH="master"
REPO="https://github.com/openjdk/jdk21u.git"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -y) DRY_RUN=0; shift ;;
        --boot-jdk) BOOT_JDK="$2"; shift 2 ;;
        --src-dir)  SRC_DIR="$2"; shift 2 ;;
        --branch)   BRANCH="$2"; shift 2 ;;
        *) echo "알 수 없는 인자: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$BOOT_JDK" ]]; then
    echo "--boot-jdk 인자가 필요하다 (예: /Users/.../JavaVirtualMachines/jdk-20/Contents/Home)" >&2
    exit 1
fi

CONFIGURE_OPTS=(
    "--with-boot-jdk=$BOOT_JDK"
    "--with-debug-level=slowdebug"
    "--with-jvm-variants=server"
    "--enable-ccache"
)

echo "== 빌드 계획 =="
echo "  저장소:     $REPO"
echo "  브랜치/태그: $BRANCH"
echo "  소스 위치:  $SRC_DIR"
echo "  부트 JDK:   $BOOT_JDK"
echo "  configure 옵션:"
printf '    %s\n' "${CONFIGURE_OPTS[@]}"
echo "  make 타깃:  images"

if [[ "$DRY_RUN" -eq 1 ]]; then
    echo
    echo "dry-run 모드. 실제 빌드는 -y 인자를 붙여 다시 실행한다."
    exit 0
fi

# 1. 소스 받기
if [[ ! -d "$SRC_DIR/.git" ]]; then
    git clone "$REPO" "$SRC_DIR"
fi
cd "$SRC_DIR"
git fetch --tags
git checkout "$BRANCH"

# 2. configure
bash configure "${CONFIGURE_OPTS[@]}"

# 3. make images
make images

# 4. 결과 확인
BUILD_DIR=$(ls -d build/*/ | head -1)
BUILT_JAVA="${BUILD_DIR}images/jdk/bin/java"
if [[ -x "$BUILT_JAVA" ]]; then
    echo
    echo "== 빌드 완료 =="
    "$BUILT_JAVA" -version
    echo "위치: $BUILT_JAVA"
else
    echo "빌드 결과 java 실행 파일을 찾지 못했다." >&2
    exit 1
fi
