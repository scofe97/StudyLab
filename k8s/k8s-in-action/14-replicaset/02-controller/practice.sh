#!/usr/bin/env bash
# Ch14 §14.3~14.4 — ReplicaSet 컨트롤러: reconciliation·노드 장애·cascade 삭제
#
# 실행: bash practice.sh <STEP>
# 정리: bash practice.sh clean
#
# 주의: STEP 03 은 워커 노드의 네트워크를 끊습니다. 반드시 04 로 복구하세요.
# 대응 학습 노트: 14-02.ReplicaSet 컨트롤러 — reconciliation·장애복구·삭제.md
set -euo pipefail
NS=ch14-lab
K="kubectl -n $NS"
VICTIM_NODE=k8s-lab-worker2
cd "$(dirname "$0")"

step_01() {
  echo "== 현재 상태 =="
  $K get rs web
  $K get pods -l app=web -o wide --no-headers | awk '{print $1, $3, $7}'
}

step_02() {
  echo "== reconciliation: 파드를 한꺼번에 지우면 =="
  $K delete pods -l app=web --wait=false
  echo "-- 즉시 --"; sleep 2; $K get pods -l app=web --no-headers | awk '{print $1, $3}'
  echo "-- 15초 후: 컨트롤러가 전부 다시 만든다 --"; sleep 15
  $K get pods -l app=web --no-headers | awk '{print $1, $3}'
}

step_03() {
  echo "== 노드 장애 시뮬레이션: $VICTIM_NODE 네트워크 차단 =="
  echo "-- 차단 전 노드별 분포 --"
  $K get pods -l app=web -o wide --no-headers | awk '{print $1, $7}'
  docker exec "$VICTIM_NODE" ip link set eth0 down
  echo "차단 완료. control plane 이 NotReady 로 표시하기까지 40초 이상 걸립니다."
  echo "다음으로 관찰하세요:"
  echo "  kubectl get nodes -w"
  echo "  kubectl -n $NS get pods -l app=web -o wide -w"
}

step_04() {
  echo "== 노드 복구 =="
  docker exec "$VICTIM_NODE" ip link set eth0 up
  docker exec "$VICTIM_NODE" ip route add default via 172.18.0.1 2>/dev/null || true
  sleep 10
  kubectl get nodes
}

step_05() {
  echo "== cascade 삭제 (기본) — 파드도 함께 지워진다 =="
  $K delete rs web
  sleep 5
  $K get pods -l app=web 2>&1 | head -3
}

step_06() {
  echo "== --cascade=orphan — 파드는 남는다 =="
  $K apply -f ../01-basics/rs.web.yaml
  $K wait --for=condition=Ready pod -l app=web --timeout=90s >/dev/null 2>&1 || true
  $K get rs web
  echo "-- orphan 삭제 --"
  $K delete rs web --cascade=orphan
  sleep 3
  echo "-- ReplicaSet 은 사라졌지만 파드는 남고 소유자만 없어진다 --"
  $K get pods -l app=web \
    -o custom-columns='POD:.metadata.name,STATUS:.status.phase,OWNER:.metadata.ownerReferences[0].name'
  echo
  echo "-- 같은 ReplicaSet 을 다시 만들면 남은 파드를 흡수한다 --"
  $K apply -f ../01-basics/rs.web.yaml
  sleep 5
  $K get pods -l app=web \
    -o custom-columns='POD:.metadata.name,OWNER:.metadata.ownerReferences[0].name'
  $K get rs web
}

clean() {
  kubectl delete namespace "$NS" --ignore-not-found
}

case "${1:-}" in
  01) step_01 ;;
  02) step_02 ;;
  03) step_03 ;;
  04) step_04 ;;
  05) step_05 ;;
  06) step_06 ;;
  clean) clean ;;
  *) echo "usage: bash practice.sh {01|02|03|04|05|06|clean}"; exit 1 ;;
esac
