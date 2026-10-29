#!/usr/bin/env bash
# GC 로그를 켜고 Gradle task 를 실행한다.
# 사용법: ./tools/run-with-gc-log.sh <gradle-task> [<log-file>]
# 예: ./tools/run-with-gc-log.sh :ch03:run /tmp/gc.log

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "사용법: $0 <gradle-task> [<log-file>]" >&2
    exit 1
fi

TASK="$1"
LOG_FILE="${2:-build/gc-$(date +%Y%m%d-%H%M%S).log}"
mkdir -p "$(dirname "$LOG_FILE")"

JVM_ARGS=(
    "-Xlog:gc*=info:file=${LOG_FILE}:time,level,tags"
    "-XX:+HeapDumpOnOutOfMemoryError"
    "-XX:HeapDumpPath=${LOG_FILE%.log}.hprof"
)

cd "$(dirname "$0")/.."
./gradlew "$TASK" --no-daemon \
    -Dorg.gradle.jvmargs="${JVM_ARGS[*]}" \
    -PrunJvmArgs="${JVM_ARGS[*]}"

echo "GC 로그: ${LOG_FILE}"
