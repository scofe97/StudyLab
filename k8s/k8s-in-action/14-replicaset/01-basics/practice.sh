#!/usr/bin/env bash
# Ch14 §14.1~14.2 — ReplicaSet 기초: 생성·소유·스케일
#
# 실행: bash practice.sh <STEP>
# 정리: bash practice.sh clean
#
# 대응 학습 노트: write/08_cloud/book/kubernetes-in-action/14-01.ReplicaSet 기초 — 생성·소유·스케일링.md
set -euo pipefail
NS=ch14-lab
K="kubectl -n $NS"
cd "$(dirname "$0")"

step_00() {
  echo "== 네임스페이스 준비 =="
  kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -
}

step_01() {
  echo "== ReplicaSet 생성 (replicas 3) =="
  $K apply -f rs.web.yaml
  # rollout status 는 ReplicaSet 을 지원하지 않는다(Deployment·StatefulSet·DaemonSet 전용).
  # ReplicaSet 은 condition 대신 파드가 Ready 가 될 때까지 기다린다.
  $K wait --for=condition=Ready pod -l app=web --timeout=90s || true
  $K get rs web
  $K get pods -l app=web -o wide
}

step_02() {
  echo "== 파드 이름과 소유자 확인 =="
  echo "-- 파드 이름: <rs이름>-<랜덤5자> 형태 --"
  $K get pods -l app=web -o name
  echo
  echo "-- ownerReferences: 누가 이 파드를 소유하는가 --"
  $K get pods -l app=web \
    -o custom-columns='POD:.metadata.name,OWNER_KIND:.metadata.ownerReferences[0].kind,OWNER:.metadata.ownerReferences[0].name'
}

step_03() {
  echo "== 파드 하나 삭제 → 컨트롤러가 다시 만드는가 =="
  local victim
  victim=$($K get pods -l app=web -o jsonpath='{.items[0].metadata.name}')
  echo "삭제 대상: $victim"
  $K delete pod "$victim" --wait=false
  sleep 6
  echo "-- 삭제 직후 (이름이 바뀌어 새로 뜬다) --"
  $K get pods -l app=web
}

step_04() {
  echo "== 고아 파드 흡수 — label 이 selector 와 일치하면 =="
  $K apply -f pod.one-web-too-many.yaml
  sleep 6
  echo "-- replicas 는 3인데 파드는 4개였다가 하나가 지워진다 --"
  $K get pods -l app=web
  echo
  echo "-- 이벤트에서 삭제 판단을 확인 --"
  $K get events --sort-by=.lastTimestamp 2>/dev/null | grep -i "web" | tail -8 || true
}

step_05() {
  echo "== label 을 바꿔 selector 밖으로 빼내면 =="
  local target
  target=$($K get pods -l app=web -o jsonpath='{.items[0].metadata.name}')
  echo "대상: $target → rel=stable 을 rel=escaped 로 변경"
  $K label pod "$target" rel=escaped --overwrite
  sleep 6
  echo "-- selector 밖으로 나갔으니 ReplicaSet 이 하나를 새로 만든다 --"
  $K get pods -L app,rel
  echo
  echo "-- 빠져나온 파드는 소유자가 사라졌는가 --"
  $K get pod "$target" \
    -o custom-columns='POD:.metadata.name,OWNER:.metadata.ownerReferences[0].name'
}

step_06() {
  echo "== 스케일 =="
  $K scale rs web --replicas 5
  sleep 8
  $K get rs web
  $K get pods -l app=web
}

clean() {
  kubectl delete namespace "$NS" --ignore-not-found
}

case "${1:-}" in
  00|setup) step_00 ;;
  01) step_01 ;;
  02) step_02 ;;
  03) step_03 ;;
  04) step_04 ;;
  05) step_05 ;;
  06) step_06 ;;
  clean) clean ;;
  *) echo "usage: bash practice.sh {00|01|02|03|04|05|06|clean}"; exit 1 ;;
esac
