#!/usr/bin/env bash
# DHCP 캡처를 정렬된 표로 읽는다.
#   ./look-dhcp.sh pcap/a1-server.pcap
#   ./look-dhcp.sh pcap/a1-server.pcap 'dhcp.option.dhcp == 3'
# 열: 번호 · 시간 · 목적지MAC · 출발IP · 도착IP · 프로토콜 · 종류(1찾기 2제안 3선택 4DECLINE 5확정 6NAK 7RELEASE)
#     · ciaddr · yiaddr · giaddr · 옵션54(고른 서버) · 옵션50(요청 주소) · 트랜잭션ID
set -euo pipefail
cd "$(dirname "$0")"
file="${1:?사용: ./look-dhcp.sh <pcap 파일> [디스플레이 필터]}"
filter="${2:-dhcp or icmp or arp}"
tshark -r "$file" -Y "$filter" -T fields -E header=y \
  -e frame.number -e frame.time_relative -e eth.dst -e ip.src -e ip.dst -e _ws.col.Protocol \
  -e dhcp.option.dhcp -e dhcp.ip.client -e dhcp.ip.your -e dhcp.ip.relay \
  -e dhcp.option.dhcp_server_id -e dhcp.option.requested_ip_address -e dhcp.id \
  | awk -F'\t' -v OFS='\t' '{ for (i = 1; i <= NF; i++) if ($i == "") $i = "-" } 1' \
  | column -t -s $'\t'
# 빈 칸을 - 로 채우는 이유: column -t 는 빈 필드를 건너뛰어 뒤 열이 앞으로 당겨진다 (04-tls 에서 발견).
