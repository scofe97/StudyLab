#!/usr/bin/env bash
# OrbStack ubuntu 안에서 서버용·클라이언트용 네트워크 네임스페이스 둘을 veth 쌍으로 잇고,
# 그 veth 에만 netem 으로 지연·손실을 건다. 머신의 eth0 은 건드리지 않는다.
#
#   tftp-srv (veth-srv 10.69.0.1/24) <── veth 쌍 ──> tftp-cli (veth-cli 10.69.0.2/24)
#
# 사용 (맥에서, 저장소 루트):
#   orb -m ubuntu sudo ./scripts/tftp-netns.sh up
#   orb -m ubuntu sudo ./scripts/tftp-netns.sh set 10ms 0%   # 방향마다 10ms 지연, 손실 0%
#   orb -m ubuntu sudo ./scripts/tftp-netns.sh status
#   orb -m ubuntu sudo ./scripts/tftp-netns.sh clear         # netem 만 걷기
#   orb -m ubuntu sudo ./scripts/tftp-netns.sh down          # 네임스페이스·veth 모두 걷기
#
# set 의 지연·손실은 veth 양 끝에 각각 걸리므로 한 방향마다 한 번씩 적용된다.
# 따라서 set 10ms 이면 왕복(RTT)은 20ms 이다.
set -euo pipefail

SRV=tftp-srv CLI=tftp-cli

case "${1:-}" in
up)
  ip netns add $SRV
  ip netns add $CLI
  ip link add veth-srv type veth peer name veth-cli
  ip link set veth-srv netns $SRV
  ip link set veth-cli netns $CLI
  ip -n $SRV addr add 10.69.0.1/24 dev veth-srv
  ip -n $CLI addr add 10.69.0.2/24 dev veth-cli
  ip -n $SRV link set lo up
  ip -n $CLI link set lo up
  ip -n $SRV link set veth-srv up
  ip -n $CLI link set veth-cli up
  echo "up: $SRV(10.69.0.1) <-> $CLI(10.69.0.2)"
  ;;
set)
  delay="${2:?한 방향 지연, 예: 10ms}" loss="${3:-0%}"
  ip netns exec $SRV tc qdisc replace dev veth-srv root netem delay "$delay" loss "$loss"
  ip netns exec $CLI tc qdisc replace dev veth-cli root netem delay "$delay" loss "$loss"
  echo "set: delay $delay loss $loss (방향마다)"
  ;;
clear)
  ip netns exec $SRV tc qdisc del dev veth-srv root 2>/dev/null || true
  ip netns exec $CLI tc qdisc del dev veth-cli root 2>/dev/null || true
  echo "cleared netem"
  ;;
down)
  # 네임스페이스를 지우면 그 안의 veth 와 qdisc 도 함께 사라진다
  ip netns del $SRV 2>/dev/null || true
  ip netns del $CLI 2>/dev/null || true
  echo "down"
  ;;
status)
  echo "--- ip netns list"
  ip netns list
  for ns in $SRV $CLI; do
    if ip netns list | grep -qw $ns; then
      echo "--- $ns"
      ip -n $ns -br addr
      ip netns exec $ns tc qdisc show
    fi
  done
  ;;
*)
  echo "usage: $0 up | set <한 방향 지연> [손실%] | clear | down | status" >&2
  exit 2
  ;;
esac
