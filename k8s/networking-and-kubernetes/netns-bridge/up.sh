#!/usr/bin/env bash
# 02-04 실습 전체 세팅을 한 번에. 백엔드 수는 인자로 (기본 2, 묶음 C 는 4).
#   ./up.sh        → ns1·ns2
#   ./up.sh 4      → ns1~ns4 (확률 사다리용)
# 학습 노트: runners-high write/08_cloud/book/networking-and-kubernetes/02-04
set -euo pipefail

N=${1:-2}
PEER=${PEER:-192.168.139.238}
BR=10.10.1.1/24                 # 오타 주의 — 10.0.1.1 로 치면 ns 의 default 가 ARP FAILED 로 죽는다

echo "== 네임스페이스 · veth 쌍 · 브리지 =="
sudo ip link add br0 type bridge
sudo ip link set br0 up
sudo ip addr add "$BR" dev br0

for n in $(seq 1 "$N"); do
  sudo ip netns add "ns$n"
  sudo ip link add "veth$n" type veth peer name "veth$n-br"   # 두 끝이 다 기본 netns 에 생긴다
  sudo ip link set "veth$n" netns "ns$n"                      # 안쪽 끝만 옮긴다 — 여기서 경계를 넘는다
  sudo ip link set "veth$n-br" master br0
  sudo ip link set "veth$n-br" up
  sudo ip netns exec "ns$n" ip addr add "10.10.1.1$n/24" dev "veth$n"
  sudo ip netns exec "ns$n" ip link set "veth$n" up
  sudo ip netns exec "ns$n" ip link set lo up
  sudo ip netns exec "ns$n" ip route add default via 10.10.1.1
done

echo "== 중계 허가 =="
sudo sysctl -qw net.ipv4.ip_forward=1

echo "== 연결 추적 켜기 (규칙이 없으면 conntrack 이 비어 있다) =="
sudo iptables -A FORWARD -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

echo "== 나가는 것 위장 (MASQUERADE) =="
sudo iptables -t nat -A POSTROUTING -s 10.10.1.0/24 -o eth0 -j MASQUERADE

cat <<MSG

세팅 완료 — ns1..ns$N (10.10.1.11 ~ 10.10.1.1$N), 게이트웨이 10.10.1.1

MASQUERADE 를 걸었으므로 상대 호스트에 돌아올 경로는 필요 없습니다.
NAT 없이 순수 포워딩을 보려면 아래를 끄고 상대에 경로를 넣으십시오:
    sudo iptables -t nat -D POSTROUTING -s 10.10.1.0/24 -o eth0 -j MASQUERADE
    ssh $PEER 'sudo ip route add 10.10.1.0/24 via <이 호스트 IP>'
MSG
