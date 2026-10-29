#!/usr/bin/env bash
# 묶음 C — 확률 사다리 1/4 → 1/3 → 1/2 → 없음 이 고르게 나뉘는지 센다.
# 세는 것은 백엔드가 아니라 iptables 규칙의 pkts 카운터다.
# 먼저: ./up.sh 4
set -uo pipefail
TRIES=${1:-100}
VIP=10.10.9.9

echo "== 백엔드 넷을 띄운다 =="
for n in 1 2 3 4; do sudo ip netns exec "ns$n" sh -c "nohup nc -lk -p 8080 >/dev/null 2>&1 &"; done
sleep 1

echo "== 가짜 서비스 주소로 가는 길 =="
sudo ip route add "$VIP/32" dev br0 2>/dev/null || true

echo "== 사다리 — 분모가 4 → 3 → 2 로 줄고 마지막은 확률 없이 무조건 =="
i=0
for p in 0.25 0.3333 0.5 -; do
  i=$((i+1))
  if [ "$p" = "-" ]; then
    sudo iptables -t nat -A OUTPUT -d "$VIP" -p tcp --dport 80 -j DNAT --to-destination "10.10.1.1$i:8080"
  else
    sudo iptables -t nat -A OUTPUT -d "$VIP" -p tcp --dport 80 \
      -m statistic --mode random --probability "$p" -j DNAT --to-destination "10.10.1.1$i:8080"
  fi
done

echo "== $TRIES 번 보낸다 =="
sudo iptables -t nat -Z OUTPUT
for _ in $(seq "$TRIES"); do nc -z -w1 "$VIP" 80 >/dev/null 2>&1; done

echo; echo "== 분포 — pkts 합이 $TRIES 이고 policy 는 0 이어야 한다 =="
sudo iptables -t nat -L OUTPUT -n -v --line-numbers
