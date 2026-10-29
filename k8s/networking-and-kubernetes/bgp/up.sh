#!/usr/bin/env bash
# BGP 실습 토폴로지 기동 — FRR 3대
#
#   AS 65001                      AS 65002
#   ┌──────────────────┐          ┌────────┐
#   │  r1 ──iBGP── r2  │──eBGP────│   r3   │
#   │ .0.2        .0.3 │          │  .0.4  │
#   └──────────────────┘          └────────┘
#
# r2 는 r3 와 직접 피어링하지 않는다. 그래야 iBGP 로 배운 경로가
# 어떻게 전파되는지(그리고 왜 안 되는지)가 드러난다.
set -euo pipefail
cd "$(dirname "$0")"
IMG=quay.io/frrouting/frr:10.2.1
NET=bgplab

docker network inspect "$NET" >/dev/null 2>&1 || \
  docker network create --subnet 172.30.0.0/24 "$NET" >/dev/null
echo "네트워크 $NET (172.30.0.0/24)"

i=2
for r in r1 r2 r3; do
  docker rm -f "$r" >/dev/null 2>&1 || true
  docker run -d --name "$r" --hostname "$r" \
    --network "$NET" --ip "172.30.0.$i" \
    --privileged \
    -v "$PWD/$r/frr.conf:/etc/frr/frr.conf:ro" \
    -v "$PWD/$r/daemons:/etc/frr/daemons:ro" \
    -v "$PWD/$r/vtysh.conf:/etc/frr/vtysh.conf:ro" \
    "$IMG" >/dev/null
  echo "  $r  -> 172.30.0.$i"
  i=$((i+1))
done

echo "세션 수립 대기 (timers 3/9)..."
sleep 12
echo "완료. 'bash verify.sh' 로 확인."
