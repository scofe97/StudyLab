#!/usr/bin/env bash
# TLS 캡처를 정렬된 표로 읽는다.
#   ./look-tls.sh pcap/a1-handshake.pcap
#   ./look-tls.sh pcap/a1-handshake.pcap 'tls.handshake.type == 1'
set -euo pipefail
cd "$(dirname "$0")"

file="${1:?사용: ./look-tls.sh <pcap 파일> [디스플레이 필터]}"
filter="${2:-}"

args=(-r "$file")
[ -n "$filter" ] && args+=(-Y "$filter")

tshark "${args[@]}" -T fields -E header=y \
  -e frame.number -e frame.time_relative -e tcp.srcport -e tcp.dstport \
  -e tls.record.content_type -e tls.handshake.type -e tls.record.length \
  -e tls.alert_message.level -e tls.alert_message.desc \
  -e tls.handshake.extensions_server_name -e tls.handshake.ciphersuite \
  | awk -F'\t' -v OFS='\t' '{ for (i = 1; i <= NF; i++) if ($i == "") $i = "-" } 1' \
  | column -t -s $'\t'
# 빈 칸을 - 로 채우는 이유: column -t 는 빈 필드를 건너뛰어 뒤 열이 앞으로 당겨진다.
# 암호화된 레코드는 핸드셰이크 종류가 비는데, 그러면 레코드 길이가 종류 열 밑에 찍혀 잘못 읽힌다(2026-09-20 발견).
