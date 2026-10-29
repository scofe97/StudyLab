#!/usr/bin/env bash
# 《JVM 밑바닥까지 파헤치기》§1.6.3 빌드 도구 설치 명령 박제 (Ubuntu 기준).
# 실제 실행은 사용자 확인 후. dry-run 모드를 기본으로 두고 -y 인자로 명시적으로 실행한다.
#
# 사용법:
#   ./install-build-deps-ubuntu.sh           # 무엇이 설치되는지 출력만
#   ./install-build-deps-ubuntu.sh -y        # 실제 apt-get install 실행

set -euo pipefail

PACKAGES=(
    build-essential
    libfreetype6-dev
    libcups2-dev
    libx11-dev libxext-dev libxrender-dev libxrandr-dev
    libxtst-dev libxt-dev
    libasound2-dev
    libfontconfig1-dev
    libffi-dev
    autoconf
    zip unzip
)

echo "설치 대상 패키지 (${#PACKAGES[@]}개):"
printf '  - %s\n' "${PACKAGES[@]}"

if [[ "${1:-}" != "-y" ]]; then
    echo
    echo "dry-run 모드. 실제 설치는 -y 인자를 붙여 다시 실행한다."
    exit 0
fi

if ! command -v apt-get >/dev/null 2>&1; then
    echo "이 스크립트는 Ubuntu/Debian (apt-get) 환경 전용이다." >&2
    exit 1
fi

sudo apt-get update
sudo apt-get install -y "${PACKAGES[@]}"
echo "빌드 도구 설치 완료."
