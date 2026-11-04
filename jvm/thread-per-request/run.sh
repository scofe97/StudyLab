#!/usr/bin/env bash
# 사용: ./run.sh <변형|all> <시나리오|all>
#   변형:    spring-platform | spring-platform-small | spring-virtual | go            (A·B·C)
#            spring-platform-blocking | spring-platform-emitter | spring-virtual-blocking | go   (D)
#   시나리오: A(작은 요청 대량) | B(느린 요청) | C(큰 응답) | D(SSE 연결 유지) | all
# 결과는 results/<변형>-<시나리오>-c<동시수>.json 에 쓴다. 부하는 localhost 로만 보낸다.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
variant="${1:-all}"
scenario="${2:-all}"
dur="${DUR:-10s}"
warm="${WARMUP:-2s}"
port=18080
out="$here/results"
bin="$here/.bin"
mkdir -p "$out" "$bin"

ulimit -n 10240 2>/dev/null || echo "경고: ulimit -n 을 올리지 못했다 ($(ulimit -n))" >&2

# --- 빌드 ---
(cd "$here/../.." && go build -o "$bin/goserver" ./jvm/thread-per-request/goserver \
  && go build -o "$bin/loadgen" ./jvm/thread-per-request/loadgen)
jar="$here/spring/build/libs/thread-per-request-0.0.1-SNAPSHOT.jar"
if [ ! -f "$jar" ]; then
  (cd "$here/spring" && ./gradlew bootJar --no-daemon --console=plain -q)
fi

pid=""
start_server() {
  case "$1" in
    spring-platform)       java -jar "$jar" --server.port=$port >"$bin/server.log" 2>&1 & ;;
    spring-platform-small) java -jar "$jar" --server.port=$port --server.tomcat.threads.max=20 >"$bin/server.log" 2>&1 & ;;
    spring-platform-blocking) java -jar "$jar" --server.port=$port --tpr.sse.mode=blocking >"$bin/server.log" 2>&1 & ;;
    spring-platform-emitter)  java -jar "$jar" --server.port=$port --tpr.sse.mode=emitter >"$bin/server.log" 2>&1 & ;;
    spring-virtual-blocking)  java -jar "$jar" --server.port=$port --tpr.sse.mode=blocking --spring.threads.virtual.enabled=true >"$bin/server.log" 2>&1 & ;;
    spring-virtual)        java -jar "$jar" --server.port=$port --spring.threads.virtual.enabled=true >"$bin/server.log" 2>&1 & ;;
    go)                    "$bin/goserver" -port $port >"$bin/server.log" 2>&1 & ;;
    *) echo "알 수 없는 변형: $1" >&2; exit 2 ;;
  esac
  pid=$!
  for _ in $(seq 1 60); do
    curl -fs -m 1 "http://localhost:$port/small" >/dev/null 2>&1 && return 0
    sleep 0.5
  done
  echo "서버가 뜨지 않았다: $1" >&2; exit 1
}
stop_server() {
  [ -n "$pid" ] && kill "$pid" 2>/dev/null && wait "$pid" 2>/dev/null || true
  pid=""
}
trap stop_server EXIT

run_one() { # 변형 시나리오 경로 c
  local v="$1" s="$2" path="$3" c="$4"
  local res="$out/$v-$s-c$c.json"
  # 부하 중간에 스레드·RSS 를 한 번 잰다.
  "$bin/loadgen" -url "http://localhost:$port$path" -c "$c" -d "$dur" -warmup "$warm" >"$bin/load.json" &
  local lg=$!
  sleep "$(awk -v w="${warm%s}" -v d="${dur%s}" 'BEGIN{print w + d/2}')"
  local os_threads rss_kb inapp
  os_threads=$(ps -M -p "$pid" | tail -n +2 | wc -l | tr -d ' ')
  rss_kb=$(ps -o rss= -p "$pid" | tr -d ' ')
  inapp=$(curl -s -m 3 "http://localhost:$port/stats" | jq -r '.threads' 2>/dev/null || true)
  wait "$lg"
  jq -c --arg v "$v" --arg s "$s" --argjson t "$os_threads" --argjson r "$rss_kb" --arg a "${inapp:-}" \
    '. + {variant:$v, scenario:$s, os_threads:$t, rss_mb:($r/1024|round), inapp_threads:(if $a=="" then null else ($a|tonumber) end)}' \
    "$bin/load.json" >"$res"
  jq -r '"\(.variant) \(.scenario) c=\(.c): \(.rps|round)/s p50=\(.p50_ms)ms p99=\(.p99_ms)ms err=\(.errors) threads=\(.os_threads) rss=\(.rss_mb)MB"' "$res"
}

scenarios() { # 시나리오 → "라벨 경로 c..." 줄들
  case "$1" in
    A) for c in 10 100 1000; do echo "A /small $c"; done ;;
    B) for c in 10 100 1000; do echo "B /slow?ms=200 $c"; done ;;
    C) for c in 10 100; do echo "C /big?kb=1024 $c"; done ;;
  esac
}

# D(SSE): 연결 N 개를 ramp 동안 열고 유지한다. 서버는 실행마다 새로 띄워 RSS 기준선을 같게 한다.
sse_ns=(1000 5000)
sse_ramp=4s
sse_settle=1s
sse_dur="${SSE_DUR:-15s}"
sse_interval=1000

run_sse() { # 변형 N
  local v="$1" n="$2"
  local res="$out/$v-D-c$n.json"
  # 예열하지 않는다. start_server 가 /small 을 한 번 불러 본 직후가 기준선이다.
  # 예열로 힙을 키워 두면 GC 가 줄여서 "연결 뒤 RSS < 기준선" 이 되어 연결당 증가분이 음수가 된다(첫 실행에서 겪음).
  local rss_before_kb
  rss_before_kb=$(ps -o rss= -p "$pid" | tr -d ' ')
  "$bin/loadgen" -sse -url "http://localhost:$port/sse?interval=$sse_interval" -conns "$n" \
    -ramp "$sse_ramp" -warmup "$sse_settle" -d "$sse_dur" >"$bin/load.json" &
  local lg=$!
  sleep "$(awk -v r="${sse_ramp%s}" -v w="${sse_settle%s}" -v d="${sse_dur%s}" 'BEGIN{print r + w + d/2}')"
  local os_threads rss_kb inapp
  os_threads=$(ps -M -p "$pid" | tail -n +2 | wc -l | tr -d ' ')
  rss_kb=$(ps -o rss= -p "$pid" | tr -d ' ')
  inapp=$(curl -s -m 2 "http://localhost:$port/stats" | jq -r '.threads' 2>/dev/null || true)
  wait "$lg"
  jq -c --arg v "$v" --argjson t "$os_threads" --argjson r "$rss_kb" --argjson b "$rss_before_kb" --arg a "${inapp:-}" \
    '. + {variant:$v, scenario:"D", os_threads:$t, rss_mb:($r/1024|round), rss_before_mb:($b/1024|round),
          rss_per_conn_kb:(if .ok_conns>0 then (($r-$b)/.ok_conns*10|round/10) else null end),
          inapp_threads:(if $a=="" then null else ($a|tonumber) end)}' \
    "$bin/load.json" >"$res"
  jq -r '"\(.variant) D c=\(.c): ok=\(.ok_conns) stalled=\(.stalled_conns) notopened=\(.not_opened) failed=\(.failed_conns) ev/s=\(.events_per_sec|round) p50=\(.lat_p50_ms)ms p99=\(.lat_p99_ms)ms threads=\(.os_threads) rss=\(.rss_mb)MB(+\(.rss_mb-.rss_before_mb)) per-conn=\(.rss_per_conn_kb)KB"' "$res"
}

# 변형이 시나리오에 쓰이는지
applies() {
  case "$2" in
    D) case "$1" in spring-platform-blocking|spring-platform-emitter|spring-virtual-blocking|go) return 0 ;; esac ;;
    *) case "$1" in spring-platform|spring-platform-small|spring-virtual|go) return 0 ;; esac ;;
  esac
  return 1
}

variants=(spring-platform spring-platform-small spring-virtual spring-platform-blocking spring-platform-emitter spring-virtual-blocking go)
[ "$variant" != all ] && variants=("$variant")
scs=(A B C D)
[ "$scenario" != all ] && scs=("$scenario")

for v in "${variants[@]}"; do
  # A·B·C: 서버를 한 번 띄워 연달아 돌린다.
  rest=()
  for s in "${scs[@]}"; do [ "$s" != D ] && applies "$v" "$s" && rest+=("$s"); done
  if [ "${#rest[@]}" -gt 0 ]; then
    start_server "$v"
    # JVM 이 막 뜬 직후의 느린 구간을 지나가도록 짧게 예열한다.
    "$bin/loadgen" -url "http://localhost:$port/small" -c 10 -d 3s -warmup 0s >/dev/null
    for s in "${rest[@]}"; do
      while read -r label path c; do
        run_one "$v" "$label" "$path" "$c"
      done < <(scenarios "$s")
    done
    stop_server
  fi
  # D: 연결 수마다 서버를 새로 띄운다.
  for s in "${scs[@]}"; do
    if [ "$s" = D ] && applies "$v" D; then
      for n in "${sse_ns[@]}"; do
        start_server "$v"
        run_sse "$v" "$n"
        stop_server
      done
    fi
  done
done
