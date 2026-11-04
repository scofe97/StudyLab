#!/usr/bin/env bash
# results/*.json 을 viz/data.js 한 파일로 묶는다. file:// 로 연 index.html 은 fetch 가 막혀 script 태그로 읽는다.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
{ printf 'window.RESULTS = '; jq -s -c 'sort_by(.variant,.scenario,.c)' "$here"/results/*.json; printf ';\n'; } >"$here/viz/data.js"
echo "viz/data.js: $(jq -s length "$here"/results/*.json) rows"
