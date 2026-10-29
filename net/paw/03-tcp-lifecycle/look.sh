#!/usr/bin/env bash
# 캡처를 정렬된 표로 읽는다.
#   ./look.sh pcap/a3-close20.pcap
#   ./look.sh pcap/a3-close20.pcap 'tcp.flags.fin == 1'    # 둘째 인자는 디스플레이 필터
set -euo pipefail
cd "$(dirname "$0")"

file="${1:?사용: ./look.sh <pcap 파일> [디스플레이 필터]}"
filter="${2:-}"

args=(-r "$file")
[ -n "$filter" ] && args+=(-Y "$filter")

tshark "${args[@]}" -T fields -E header=y \
  -e frame.number -e frame.time_relative -e tcp.srcport -e tcp.dstport \
  -e tcp.flags.str -e tcp.seq_raw -e tcp.ack_raw -e tcp.len | column -t
