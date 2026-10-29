#!/usr/bin/env bash
# 179 포트 BGP 메시지 캡처.
#
# macOS Docker Desktop 은 VM 안에서 돌아 호스트 tcpdump 로 브리지가 안 보인다.
# 그래서 사이드카 컨테이너를 r1 의 네트워크 네임스페이스에 붙여 캡처한다.
# 01-04 §2 의 "관측 지점을 고른다" 가 여기서도 그대로 적용된다.
set -uo pipefail
MODE="${1:-keepalive}"
SEC="${2:-14}"
IMG=nicolaka/netshoot:latest

docker ps --format '{{.Names}}' | grep -qx r1 || { echo "r1 미기동 — 'bash up.sh' 먼저"; exit 1; }

case "$MODE" in
  keepalive)
    echo "== KEEPALIVE 관찰 (${SEC}초) — timers 3/9 이므로 3초마다 19바이트 =="
    docker run --rm --net=container:r1 "$IMG" \
      timeout "$SEC" tcpdump -i eth0 -n 'tcp port 179' 2>/dev/null
    ;;
  open)
    echo "== 세션 수립 관찰 (${SEC}초) — 캡처 중 clear 로 OPEN 을 유발한다 =="
    docker run --rm --net=container:r1 "$IMG" \
      timeout "$SEC" tcpdump -i eth0 -n -v 'tcp port 179' 2>/dev/null > /tmp/bgp_open.txt &
    cap=$!
    sleep 2
    docker exec r1 vtysh -c 'clear bgp *' >/dev/null 2>&1
    wait $cap 2>/dev/null
    echo "-- OPEN 메시지 (AS 번호·Holdtime·Router ID 가 평문) --"
    grep -B1 -A2 'Open Message' /tmp/bgp_open.txt | head -20
    echo
    echo "-- 3-way handshake 횟수 (세션당 TCP 연결 1개) --"
    grep -c 'Flags \[S\]' /tmp/bgp_open.txt
    echo "전체 출력: /tmp/bgp_open.txt"
    ;;
  *) echo "사용법: bash capture.sh [keepalive|open] [초]"; exit 1 ;;
esac
