#!/usr/bin/env bash
# up.sh 가 만든 것을 되돌린다.
# 주의: netns 를 지워도 바깥쪽 veth 끝이 남는 경우가 있어 명시적으로 지운다(2026-08-30 실측).
set -uo pipefail

sudo iptables -t nat -F OUTPUT 2>/dev/null || true
sudo iptables -t nat -D POSTROUTING -s 10.10.1.0/24 -o eth0 -j MASQUERADE 2>/dev/null || true
sudo iptables -D FORWARD -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || true
sudo ip route del 10.10.9.9/32 2>/dev/null || true
for n in 1 2 3 4; do
  sudo ip netns del "ns$n" 2>/dev/null || true
  sudo ip link del "veth$n-br" 2>/dev/null || true
done
sudo ip link del br0 2>/dev/null || true
sudo conntrack -F 2>/dev/null || true

echo "원복 완료 — 남은 것 확인:"
ip -br link | grep -E 'veth|br0' || echo "  (없음)"
