#!/usr/bin/env bash
# 기준 리비전(기본 origin/main, 없으면 HEAD)과 비교해 바뀐 Bazel 타깃을 출력한다.
# bazel-contrib/target-determinator v0.34.0 릴리스 바이너리를 감싼다. 없으면 한 번 내려받는다.
# 사용: tools/changed-targets.sh [기준 리비전]   (시험까지 돌리려면: bazel test $(tools/changed-targets.sh))
set -euo pipefail

VERSION="v0.34.0"
root="$(git rev-parse --show-toplevel)"
base="${1:-}"
if [ -z "$base" ]; then
  if git rev-parse --verify -q origin/main >/dev/null; then base="origin/main"; else base="HEAD"; fi
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
esac

bin="${TARGET_DETERMINATOR:-${XDG_CACHE_HOME:-$HOME/.cache}/runners-lab/target-determinator-$VERSION}"
if [ ! -x "$bin" ]; then
  mkdir -p "$(dirname "$bin")"
  curl -fsSL -o "$bin" "https://github.com/bazel-contrib/target-determinator/releases/download/$VERSION/target-determinator.$os.$arch"
  chmod +x "$bin"
fi

cd "$root"
exec "$bin" -bazel bazel "$base"
