#!/usr/bin/env bash
# edge 실습의 프로세스를 띄우고 걷는다. 모든 상태는 edge/.run/ 아래에 둔다.
#
#   ./run.sh up [설정]        upstream·api 두 개·Caddy 를 띄운다 (설정 기본값 caddy.json)
#   ./run.sh down             모두 내리고 소켓 파일을 지운다
#   ./run.sh status           떠 있는 프로세스·포트·소켓 파일
#   ./run.sh admin 메서드 경로 [JSON파일]   관리 API 호출 (TCP 든 Unix 소켓이든)
#   ./run.sh load 설정        관리 API /load 로 설정 전체를 바꾼다
#   ./run.sh api-stop N | api-start N | api-restart N    N=1 은 :8081, N=2 는 :8082
#   ./run.sh caddy-stop | caddy-start [설정] | caddy-restart [설정]
#
# 환경 변수
#   CADDY_BIN     기본 bin/caddy (E6 에서 bin/caddy-stock 으로 바꿔 본다)
#   API_TRUSTED   api 의 -trusted-proxies 값. 기본 127.0.0.1/32, 빈 값도 그대로 넘긴다
#   API_EXTRA     api 에 덧붙일 플래그 (낱말 단위로 나눈다)
set -euo pipefail
cd "$(dirname "$0")"
EDGE="$PWD"
ROOT="$(cd ../.. && pwd)"          # chapters 폴더 (루트 모듈 안)
RUN="$EDGE/.run"
CADDY_BIN="${CADDY_BIN:-bin/caddy}"
API_TRUSTED="${API_TRUSTED-127.0.0.1/32}"
ADMIN_TCP="127.0.0.1:2019"
ADMIN_SOCK="$RUN/caddy-admin.sock"

# Caddy 의 데이터(로컬 CA 포함)와 autosave 설정을 홈이 아니라 .run 아래에 둔다
export XDG_DATA_HOME="$RUN/data" XDG_CONFIG_HOME="$RUN/config"

mkdir -p "$RUN/pids" "$RUN/logs" "$RUN/bin"

log() { printf '[run] %s\n' "$*" >&2; }

pid_of() { cat "$RUN/pids/$1.pid" 2>/dev/null || true; }
alive() { local p; p="$(pid_of "$1")"; [[ -n "$p" ]] && kill -0 "$p" 2>/dev/null; }

wait_port() {
  local port="$1" i
  for i in $(seq 1 100); do
    nc -z 127.0.0.1 "$port" 2>/dev/null && return 0
    sleep 0.1
  done
  log "포트 $port 가 열리지 않았습니다"; return 1
}

# start 이름 로그파일 명령... — 백그라운드로 띄우고 PID 를 적는다
start() {
  local name="$1"; shift
  if alive "$name"; then log "$name 은 이미 떠 있습니다 (pid $(pid_of "$name"))"; return; fi
  nohup "$@" >>"$RUN/logs/$name.log" 2>&1 &
  echo $! >"$RUN/pids/$name.pid"
  log "$name 시작 (pid $!) — 로그 .run/logs/$name.log"
}

# stop 이름 — SIGTERM 을 보내고 끝날 때까지 기다린다 (10초 넘으면 SIGKILL)
stop() {
  local name="$1" p i
  p="$(pid_of "$name")"
  if [[ -z "$p" ]] || ! kill -0 "$p" 2>/dev/null; then rm -f "$RUN/pids/$name.pid"; return; fi
  kill -TERM "$p"
  for i in $(seq 1 100); do kill -0 "$p" 2>/dev/null || break; sleep 0.1; done
  if kill -0 "$p" 2>/dev/null; then log "$name 이 10초 안에 끝나지 않아 SIGKILL"; kill -KILL "$p"; fi
  rm -f "$RUN/pids/$name.pid"
  log "$name 종료"
}

# 백엔드 바이너리를 고른다. service 창의 cmd/api 가 빌드되면 그것을, 아니면 임시 백엔드를 쓴다.
build_backends() {
  if [[ -d "$ROOT/ch07-10-service/cmd/api" ]] &&
     (cd "$ROOT" && go build -o "$RUN/bin/api" ./ch07-10-service/cmd/api) 2>"$RUN/logs/build-api.log"; then
    echo real >"$RUN/backend-kind"
  else
    (cd "$ROOT" && go build -o "$RUN/bin/api" ./ch07-10-service/edge/cmd/fakeapi)
    echo fake >"$RUN/backend-kind"
  fi
  rm -f "$RUN/bin/upstream"
  if [[ -d "$ROOT/ch07-10-service/cmd/upstream" ]]; then
    (cd "$ROOT" && go build -o "$RUN/bin/upstream" ./ch07-10-service/cmd/upstream) 2>"$RUN/logs/build-upstream.log" ||
      rm -f "$RUN/bin/upstream"
  fi
  (cd "$ROOT" && go build -o "$RUN/bin/load" ./ch07-10-service/edge/cmd/load)
  log "백엔드: $(cat "$RUN/backend-kind") (upstream: $([[ -x "$RUN/bin/upstream" ]] && echo real || echo 없음))"
}

api_start() {
  local n="$1" extra=()
  # API_EXTRA 의 낱말을 api 플래그로 덧붙인다 (예: API_EXTRA='-deny-ips 127.0.0.1/32')
  [[ -n "${API_EXTRA:-}" ]] && read -r -a extra <<<"$API_EXTRA"
  start "api$n" "$RUN/bin/api" -addr "127.0.0.1:808$n" -upstream http://127.0.0.1:9090 \
    -trusted-proxies "$API_TRUSTED" -admin-socket "$RUN/api$n-admin.sock" ${extra[@]+"${extra[@]}"}
  wait_port "808$n"
}

caddy_start() {
  local cfg="${1:-caddy.json}"
  start caddy "$CADDY_BIN" run --config "$cfg"
  wait_port 8000 || { tail -20 "$RUN/logs/caddy.log"; return 1; }
}

admin() {
  local method="$1" path="$2" file="${3:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -w '%{stderr}[HTTP %{http_code}]\n')
  [[ -n "$file" ]] && args+=(--data-binary "@$file")
  # 관리 소켓 파일이 있으면 먼저 그쪽으로, 접속이 안 되면(curl 종료 코드 7) TCP 로 보낸다
  if [[ -S "$ADMIN_SOCK" ]]; then
    local rc=0
    curl "${args[@]}" --unix-socket "$ADMIN_SOCK" "http://localhost$path" 2>"$RUN/admin-sock.err" || rc=$?
    [[ $rc -ne 7 ]] && { cat "$RUN/admin-sock.err" >&2; return $rc; }
    log "관리 소켓에 접속하지 못해 TCP $ADMIN_TCP 로 보냅니다: $(cat "$RUN/admin-sock.err")"
  fi
  curl "${args[@]}" "http://$ADMIN_TCP$path"
}

status() {
  local name
  for name in upstream api1 api2 caddy; do
    if alive "$name"; then echo "$name  pid $(pid_of "$name")"; else echo "$name  -"; fi
  done
  echo "--- LISTEN"
  lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR==1 || /:(2019|8000|8081|8082|8088|8443|9090) /' || true
  echo "--- 소켓 파일"
  find "$RUN" -type s 2>/dev/null || true
}

cmd="${1:-}"; shift || true
case "$cmd" in
  up)
    build_backends
    if [[ -x "$RUN/bin/upstream" ]]; then
      start upstream "$RUN/bin/upstream" -addr 127.0.0.1:9090; wait_port 9090
    fi
    api_start 1; api_start 2
    caddy_start "${1:-caddy.json}"
    status ;;
  down)
    for name in caddy api1 api2 upstream; do stop "$name"; done
    find "$RUN" -type s -delete 2>/dev/null || true
    status ;;
  status) status ;;
  admin) admin "$@" ;;
  load) admin POST /load "$1" ;;
  api-stop) stop "api$1" ;;
  api-start) api_start "$1" ;;
  api-restart) stop "api$1"; api_start "$1" ;;
  caddy-stop) stop caddy ;;
  caddy-start) caddy_start "${1:-caddy.json}" ;;
  caddy-restart) stop caddy; caddy_start "${1:-caddy.json}" ;;
  *) sed -n '2,15p' "$0"; exit 1 ;;
esac
