#!/usr/bin/env bash
# T5 — 지연·손실 조합마다 서버의 timeout·retries 값을 바꿔 가며 tftpbench 를 돌리고 CSV 를 출력한다.
# netns.sh up 이 먼저 되어 있어야 한다. 끝나면 netem 을 걷는다 (네임스페이스는 남긴다).
#
# 사용 (macOS 에서):
#   orb -m ubuntu sudo ./scripts/t5-grid.sh > work/t5.csv
#   orb -m ubuntu sudo env DELAYS="25ms" LOSSES="10%" N=20 ./scripts/t5-grid.sh > work/t5-one.csv
#
# 칸마다: netem 설정 → tftp-srv 에서 tftpd 를 띄움 → tftp-cli 에서 tftpbench → tftpd 종료.
# 서버 로그는 work/t5-server.log 에 이어 붙는다.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN=bin/linux
FILE=${FILE:-boot-32k.bin}
N=${N:-5}
# 방향마다 거는 지연이다. 왕복에는 두 번 들어간다.
DELAYS=${DELAYS:-"10ms 50ms"}
LOSSES=${LOSSES:-"0% 5% 20%"}
TIMEOUTS=${TIMEOUTS:-"200ms 1s"}
RETRIES=${RETRIES:-"3 10"}
# 클라이언트는 서버보다 오래 버티게 둔다 (서버 쪽 값의 효과를 보려는 것이므로)
CLI_TIMEOUT=${CLI_TIMEOUT:-3s}
CLI_RETRIES=${CLI_RETRIES:-3}

if ! ip netns list | grep -qw tftp-srv; then
  echo "run: sudo ./scripts/netns.sh up" >&2
  exit 1
fi

header=--header
for d in $DELAYS; do
  for l in $LOSSES; do
    ./scripts/netns.sh set "$d" "$l" >&2
    for t in $TIMEOUTS; do
      for r in $RETRIES; do
        ip netns exec tftp-srv $BIN/tftpd -root work/root -listen 10.69.0.1:6969 \
          -timeout "$t" -retries "$r" 2>>work/t5-server.log &
        pid=$!
        sleep 0.3
        ip netns exec tftp-cli $BIN/tftpbench -server 10.69.0.1:6969 -file "$FILE" -orig "work/root/$FILE" \
          -n "$N" -timeout "$CLI_TIMEOUT" -retries "$CLI_RETRIES" \
          -label "delay=$d;loss=$l;srv_timeout=$t;srv_retries=$r" $header
        header=
        kill "$pid"
        wait "$pid" 2>/dev/null || true
      done
    done
  done
done
./scripts/netns.sh clear >&2
