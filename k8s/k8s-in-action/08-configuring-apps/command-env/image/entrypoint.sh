#!/bin/sh
set -eu

port="${1:-8080}"
printf 'entrypoint=/entrypoint.sh port=%s image_only=%s\n' "$port" "$IMAGE_ONLY"
exec sleep 3600
