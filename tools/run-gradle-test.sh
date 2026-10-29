#!/usr/bin/env bash
# Gradle 프로젝트를 Bazel 이 이해하지 않고, 격리된 복사본에서 ./gradlew test 를 호출한다.
# 인자: 프로젝트 경로(저장소 루트 기준, 예: arch/gateway)
set -euo pipefail
rel="${1:?프로젝트 경로가 필요하다}"
runfiles="${RUNFILES_DIR:-${TEST_SRCDIR:?}}/${TEST_WORKSPACE:-_main}"
# runfiles 는 읽기 전용 symlink 라 build/ 를 쓰지 못한다. 실제 파일로 복사해 돌린다.
work="${TEST_TMPDIR:-$(mktemp -d)}/proj"
mkdir -p "$work"
cp -RL "$runfiles/$rel"/. "$work"/
rm -f "$work/BUILD.bazel"
cd "$work"
export GRADLE_USER_HOME="${TEST_TMPDIR:-$work}/gradle-home"
if [ -z "${JAVA_HOME:-}" ] && [ -x /usr/libexec/java_home ]; then
  JAVA_HOME="$(/usr/libexec/java_home -v 25)"
  export JAVA_HOME
fi
./gradlew test --no-daemon --console=plain
