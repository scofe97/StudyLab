#!/usr/bin/env bash
# 아무 캡처나 원하는 필드만 정렬된 표로 읽는다. 빈 칸은 - 로 채운다(column -t 열 밀림 방지).
#   ./look.sh <pcap> '<디스플레이 필터 또는 ->' 필드1 필드2 ...
#   ./look.sh pcap/c4-client.pcap http frame.number tcp.stream http.time
set -euo pipefail
cd "$(dirname "$0")"
file="${1:?사용: ./look.sh <pcap> <필터|-> <필드...>}"; filter="$2"; shift 2
args=(-r "$file"); [ "$filter" != "-" ] && args+=(-Y "$filter")
for f in "$@"; do args+=(-e "$f"); done
tshark "${args[@]}" -T fields -E header=y \
  | awk -F'\t' -v OFS='\t' '{ for (i = 1; i <= NF; i++) if ($i == "") $i = "-" } 1' \
  | column -t -s $'\t'
