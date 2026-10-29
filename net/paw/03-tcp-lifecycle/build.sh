#!/usr/bin/env bash
# Go 판을 리눅스 arm64 바이너리로 빌드해 bin/ 에 둔다. 컨테이너에는 Go 를 넣지 않는다.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p bin
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/server ./src-go/server
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/client ./src-go/client
ls -la bin
