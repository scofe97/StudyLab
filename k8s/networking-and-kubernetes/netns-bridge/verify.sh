#!/usr/bin/env bash
# 02-04 의 관측 지점을 한 번에 훑는다. 각 출력이 문서의 어느 절에 대응하는지 주석으로 표시.
set -uo pipefail
PEER=${PEER:-192.168.139.238}

echo "=== §1 깔려 있는 배선 ==="
ip -d link show eth0 | sed -n '1p;3p'
ip route
ip route get "$PEER"

echo; echo "=== §2 만든 배선 — 기본 netns 에 셋, 안쪽 끝은 안 보임 ==="
ip -br link | grep -E 'veth|br0'
bridge link
sudo ip netns exec ns1 ip -br link

echo; echo "=== §2 브리지는 홉이 아니다 (ttl=64 기대) ==="
sudo ip netns exec ns1 ping -c2 10.10.1.12

echo; echo "=== §3 크로스호스트 (ttl=63 기대 — 라우터 한 번) ==="
sudo ip netns exec ns1 ping -c2 "$PEER"

echo; echo "=== §5 conntrack 튜플 두 줄 ==="
sudo conntrack -L 2>/dev/null

echo; echo "=== §5 규칙은 첫 패킷에만 발화 (pkts 를 보라) ==="
sudo iptables -t nat -L POSTROUTING -n -v
