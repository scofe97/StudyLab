#!/usr/bin/env bash
# 묶음 D — 같은 "안 된다"가 DROP 과 REJECT 로 갈리는 것을 시간과 응답으로 본다.
# 상대 호스트에서 먼저: nc -lk -p 9000
set -uo pipefail
PEER=${PEER:-192.168.139.238}

echo "== 기준선 — 막지 않았을 때 =="
sudo ip netns exec ns1 nc -vz -w3 "$PEER" 9000

echo; echo "== (가) DROP — 아무 말 없이 버린다 =="
sudo iptables -I FORWARD 1 -s 10.10.1.0/24 -p tcp --dport 9000 -j DROP
time sudo ip netns exec ns1 nc -vz -w5 "$PEER" 9000
sudo iptables -D FORWARD 1

echo; echo "== (나) REJECT — 거절을 지어 보낸다 =="
sudo iptables -I FORWARD 1 -s 10.10.1.0/24 -p tcp --dport 9000 -j REJECT
time sudo ip netns exec ns1 nc -vz -w5 "$PEER" 9000

echo; echo "== 거절을 만든 쪽은 누구인가 — 응답의 출발지 IP 를 본다 =="
sudo ip netns exec ns1 timeout 6 tcpdump -ni veth1 'tcp port 9000 or icmp' &
sleep 1; sudo ip netns exec ns1 nc -vz -w3 "$PEER" 9000; wait

sudo iptables -D FORWARD 1
echo; echo "원복 완료"
