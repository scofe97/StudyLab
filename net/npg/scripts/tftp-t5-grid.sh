#!/usr/bin/env bash
# T5 격자 측정: (지연, 손실, 서버 타임아웃, 서버 재시도) 칸마다 서버를 띄우고 클라이언트를 N 번 돌려
# 성공률과 전송 시간을 CSV 한 줄로 낸다. tftp-netns.sh up 이 먼저 되어 있어야 한다.
#
# 사용 (맥에서, 저장소 루트):
#   orb -m ubuntu sudo env DELAYS="10ms" LOSSES="10%" TIMEOUTS="200ms 1s" RETRIES="3 10" N=5 \
#     ./scripts/tftp-t5-grid.sh > work/tftp/t5.csv
#
# 클라이언트는 -timeout 3s -retries 3 으로 서버보다 오래 버텨서 서버 쪽 값의 영향만 본다.
# 클라이언트는 마지막 ACK 뒤에 한 타임아웃(여기서는 3초)을 더 기다리므로 칸마다 N x 3초가 더 걸린다.
# 측정 시간은 클라이언트 로그의 elapsed_ms(마지막 DATA 를 받은 순간까지)이고 이 기다림은 빠진다.
set -euo pipefail
cd "$(dirname "$0")/.."

DELAYS=${DELAYS:-"10ms"}
LOSSES=${LOSSES:-"10%"}
TIMEOUTS=${TIMEOUTS:-"200ms 1s"}
RETRIES=${RETRIES:-"3 10"}
N=${N:-5}
FILE=${FILE:-boot-32k.bin}
ROOT=${ROOT:-work/tftp/root}
SRV_BIN=${SRV_BIN:-./bin/tftpd-linux}
CLI_BIN=${CLI_BIN:-./bin/tftpget-linux}
SERVER_LOG=${SERVER_LOG:-work/tftp/t5-server.log}
CLI_TIMEOUT=${CLI_TIMEOUT:-3s}
CLI_RETRIES=${CLI_RETRIES:-3}

strip() { sed 's/\x1b\[[0-9;]*m//g'; }

echo "delay,loss,timeout,retries,n,ok,success_pct,p50_ms,max_ms,client_timeouts"
: >"$SERVER_LOG"

for delay in $DELAYS; do
  for loss in $LOSSES; do
    ./scripts/tftp-netns.sh set "$delay" "$loss" >&2
    for to in $TIMEOUTS; do
      for rt in $RETRIES; do
        echo "### delay=$delay loss=$loss timeout=$to retries=$rt" >>"$SERVER_LOG"
        ip netns exec tftp-srv "$SRV_BIN" -root "$ROOT" -listen 10.69.0.1:6969 \
          -timeout "$to" -retries "$rt" 2>&1 | strip >>"$SERVER_LOG" &
        pipe_pid=$!
        sleep 0.5

        ok=0 cto=0 times=()
        for ((i = 1; i <= N; i++)); do
          out=$(ip netns exec tftp-cli timeout 120 "$CLI_BIN" -server 10.69.0.1:6969 \
            -timeout "$CLI_TIMEOUT" -retries "$CLI_RETRIES" -o - "$FILE" 2>&1 | strip) && rc=0 || rc=$?
          line=$(grep '받기 끝' <<<"$out" | tail -1)
          t=$(sed -n 's/.*timeouts=\([0-9]*\).*/\1/p' <<<"$line")
          cto=$((cto + ${t:-0}))
          if grep -q ' INF 받기 끝' <<<"$out"; then
            ok=$((ok + 1))
            times+=("$(sed -n 's/.*elapsed_ms=\([0-9.]*\).*/\1/p' <<<"$line")")
          else
            echo "fail: delay=$delay loss=$loss timeout=$to retries=$rt: $(grep -o 'error="[^"]*"' <<<"$line" | head -1)" >&2
          fi
        done

        # 서버를 내린다 (파이프의 첫 프로세스까지 정리)
        pkill -f "tftpd-linux -root $ROOT -listen 10.69.0.1:6969 -timeout $to -retries $rt" || true
        wait "$pipe_pid" 2>/dev/null || true

        p50="" max=""
        if ((ok > 0)); then
          sorted=$(printf '%s\n' "${times[@]}" | sort -n)
          p50=$(awk -v n="$ok" 'NR==int((n+1)/2){print; exit}' <<<"$sorted")
          max=$(tail -1 <<<"$sorted")
        fi
        pct=$(awk -v ok="$ok" -v n="$N" 'BEGIN{printf "%.0f", ok*100/n}')
        echo "$delay,$loss,$to,$rt,$N,$ok,$pct,$p50,$max,$cto"
      done
    done
  done
done
