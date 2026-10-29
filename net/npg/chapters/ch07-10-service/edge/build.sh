#!/usr/bin/env bash
# xcaddy 로 Caddy 바이너리 두 개를 만든다.
#   bin/caddy        — 표준 모듈 + 이 폴더의 restrict_prefix 모듈
#   bin/caddy-stock  — 표준 모듈만 (E6 에서 비교용)
#
# 사용: ./build.sh            두 개 다
#       ./build.sh custom     bin/caddy 만
set -euo pipefail
cd "$(dirname "$0")"

CADDY_VERSION="${CADDY_VERSION:-v2.11.7}"
XCADDY="${XCADDY:-$(go env GOPATH)/bin/xcaddy}"
MODULE="npg.local/edge/caddymod"

if [[ ! -x "$XCADDY" ]]; then
  echo "xcaddy 가 없습니다: go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.7" >&2
  exit 1
fi
mkdir -p bin

# --with 모듈=로컬경로 는 생성한 main 모듈의 go.mod 에 replace 를 넣는다
"$XCADDY" build "$CADDY_VERSION" --with "$MODULE=./caddymod" --output bin/caddy
if [[ "${1:-all}" != custom ]]; then
  "$XCADDY" build "$CADDY_VERSION" --output bin/caddy-stock
fi

bin/caddy version
bin/caddy list-modules | grep -n restrict_prefix || true
