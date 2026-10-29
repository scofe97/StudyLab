#!/usr/bin/env bash
# javap 로 클래스 파일을 자세히 역어셈블한다.
# 사용법: ./tools/decompile.sh <class-file-path>
# 예: ./tools/decompile.sh ch06/build/classes/java/main/org/runners/jvm/ch06/Foo.class

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "사용법: $0 <class-file-path>" >&2
    exit 1
fi

CLASS_FILE="$1"

if [[ ! -f "$CLASS_FILE" ]]; then
    echo "파일이 없다: $CLASS_FILE" >&2
    exit 1
fi

# -c: 바이트코드, -p: private 포함, -v: verbose(상수 풀 포함), -l: 줄 번호와 로컬 변수
javap -c -p -v -l "$CLASS_FILE"
