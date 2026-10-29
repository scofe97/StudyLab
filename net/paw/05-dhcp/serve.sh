#!/usr/bin/env bash
# 컨테이너 안의 dnsmasq 를 깨끗한 상태로 다시 띄운다 (임대 기록·로그 삭제 후 시작).
#   ./serve.sh dhcp-server dhcp-a.conf
#   ./serve.sh dhcp-server dhcp-a.conf --no-ping
#   ./serve.sh dhcp-server stop
set -euo pipefail
c="${1:?사용: ./serve.sh <컨테이너> <conf 파일|stop> [dnsmasq 옵션]}"; conf="${2:?conf 파일 또는 stop}"; shift 2
# 임대 기록을 지우지 않으면 앞 단계에서 준 주소를 기억해 같은 주소를 곧바로 다시 준다
docker exec "$c" sh -c 'pkill dnsmasq 2>/dev/null; sleep 0.3; rm -f /tmp/dnsmasq.log /var/lib/misc/dnsmasq.leases'
[ "$conf" = stop ] && { echo "$c: dnsmasq 중지"; exit 0; }
docker exec -d "$c" sh -c "dnsmasq -k -C /conf/$conf --log-facility=/tmp/dnsmasq.log $*"
sleep 0.5
docker exec "$c" sh -c 'grep -E "IP range|DHCPv6|relay|started" /tmp/dnsmasq.log || true'
