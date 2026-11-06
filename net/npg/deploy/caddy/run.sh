#!/usr/bin/env bash
# 10장 앞단 Caddy 를 띄우고 걷는다. Caddy 의 데이터(로컬 CA 포함)와 로그는 deploy/caddy/.run/ 아래에 둔다.
# npgproxy(:28080)·fakeinv(:29090) 는 이 스크립트가 띄우지 않는다. 9장 화면(npgwatch)이 띄운 것을 그대로 쓴다.
#
#   ./run.sh up [설정]        Caddy 를 띄운다 (설정 기본값 caddy.json)
#   ./run.sh down             Caddy 를 내린다
#   ./run.sh status           Caddy 상태와 쓰는 포트
#   ./run.sh load 설정        관리 API /load 로 설정 전체를 바꾼다
#   ./run.sh admin 메서드 경로 [JSON파일]   관리 API 호출
#
# 환경 변수: CADDY_BIN (기본: PATH 의 caddy)
set -euo pipefail
cd "$(dirname "$0")"
RUN="$PWD/.run"
CADDY_BIN="${CADDY_BIN:-caddy}"
ADMIN="127.0.0.1:2019"
export XDG_DATA_HOME="$RUN/data" XDG_CONFIG_HOME="$RUN/config"
mkdir -p "$RUN/logs"

log() { printf '[run] %s\n' "$*" >&2; }
pid() { cat "$RUN/caddy.pid" 2>/dev/null || true; }
alive() { local p; p="$(pid)"; [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null; }

up() {
  local cfg="${1:-caddy.json}" i
  if alive; then log "caddy 는 이미 떠 있습니다 (pid $(pid))"; return; fi
  nohup "$CADDY_BIN" run --config "$cfg" >>"$RUN/logs/caddy.log" 2>&1 &
  echo $! >"$RUN/caddy.pid"
  for i in $(seq 1 100); do nc -z 127.0.0.1 8000 2>/dev/null && break; sleep 0.1; done
  nc -z 127.0.0.1 8000 2>/dev/null || { tail -20 "$RUN/logs/caddy.log"; return 1; }
  log "caddy 시작 (pid $(pid)) — 로그 .run/logs/caddy.log"
}

down() {
  local p i; p="$(pid)"
  if [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null; then
    kill -TERM "$p"
    for i in $(seq 1 100); do kill -0 "$p" 2>/dev/null || break; sleep 0.1; done
    kill -0 "$p" 2>/dev/null && { log "10초 안에 끝나지 않아 SIGKILL"; kill -KILL "$p"; }
    log "caddy 종료"
  fi
  rm -f "$RUN/caddy.pid"
}

admin() {
  local method="$1" path="$2" file="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -w '%{stderr}[HTTP %{http_code}]\n')
  [[ -n "$file" ]] && args+=(--data-binary "@$file")
  curl "${args[@]}" "http://$ADMIN$path"
}

status() {
  if alive; then echo "caddy  pid $(pid)"; else echo "caddy  -"; fi
  echo "--- LISTEN"
  lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR==1 || /:(2019|8000|8088|8443|28080|29090) /' || true
}

cmd="${1:-}"; shift || true
case "$cmd" in
  up) up "$@"; status ;;
  down) down; status ;;
  status) status ;;
  load) admin POST /load "$1" ;;
  admin) admin "$@" ;;
  *) sed -n '2,12p' "$0"; exit 1 ;;
esac
