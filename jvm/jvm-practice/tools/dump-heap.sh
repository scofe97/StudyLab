#!/usr/bin/env bash
# 실행 중인 자바 프로세스의 힙 덤프를 jcmd 로 떠낸다.
# 사용법: ./tools/dump-heap.sh <pid> [<out-file>]
# 예: ./tools/dump-heap.sh 12345 /tmp/heap.hprof

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "사용법: $0 <pid> [<out-file>]" >&2
    echo "현재 자바 프로세스:" >&2
    jcmd >&2 || true
    exit 1
fi

PID="$1"
OUT_FILE="${2:-build/heap-${PID}-$(date +%Y%m%d-%H%M%S).hprof}"
mkdir -p "$(dirname "$OUT_FILE")"

jcmd "$PID" GC.heap_dump "$OUT_FILE"
echo "힙 덤프: ${OUT_FILE}"
