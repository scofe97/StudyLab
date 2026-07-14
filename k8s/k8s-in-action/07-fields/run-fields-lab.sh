#!/usr/bin/env bash
# 07-fields 실습 러너 — 통째 실행 금지. STEP을 하나씩 복사해 실행하고, 출력을 학습노트 07-03 §과 대조한다.
# 전제: kind 클러스터(k8s-lab, control-plane 1 + worker 2)가 떠 있어야 한다.
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# STEP 1 — field selector 실습용 Pod 생성 (running 2 + pending 1)
kubectl apply -f pods-for-fieldselector.yaml
kubectl config set-context --current --namespace fields-lab   # 기본 ns 전환(07-01 §3)
kubectl get pods -o wide          # running-a·running-b는 Running(NODE 채워짐), pending-one은 Pending

# ─────────────────────────────────────────────────────────────
# STEP 2 — field selector: spec.nodeName 으로 특정 노드의 Pod (07-03 §1)
#   먼저 running-a가 어느 노드에 떴는지 확인하고, 그 노드 이름으로 필터
kubectl get pod running-a -o jsonpath='{.spec.nodeName}{"\n"}'   # 예: k8s-lab-worker
kubectl get pods --field-selector spec.nodeName=k8s-lab-worker    # 그 노드의 Pod만
kubectl get pods --field-selector spec.nodeName=k8s-lab-worker2   # 다른 노드

# STEP 3 — field selector: status.phase!=Running 으로 비실행 Pod (07-03 §1)
kubectl get pods --field-selector status.phase!=Running          # pending-one만 잡힘
kubectl get pods --field-selector status.phase!=Running -A        # 클러스터 전체 비실행 Pod

# STEP 4 — 지원 안 하는 필드로 필터하면 에러로 알려준다 (07-03 §1: "써 보면 알려준다")
kubectl get pods --field-selector spec.containers[0].image=x 2>&1 || true   # field label not supported

# ─────────────────────────────────────────────────────────────
# STEP 5 — YAML 따옴표 함정: 따옴표 없는 annotation은 apply 거부 (07-03 §3)
kubectl apply -f pod-annotations-bad.yaml 2>&1 || true    # 에러: expects " or n, but found t ...

# STEP 6 — 따옴표 씌운 성공 버전 (07-03 §2~3)
kubectl apply -f pod-annotations.yaml

# STEP 7 — annotation 조회: get 에는 열이 없다. describe / jq 로 읽는다 (07-03 §3)
kubectl describe pod pod-anno | grep -A4 Annotations
kubectl get pod pod-anno -o json | jq .metadata.annotations   # jq 없으면: -o jsonpath='{.metadata.annotations}'

# STEP 8 — 핵심 확인: annotation으로는 필터링이 안 된다 (07-03 §2 결정적 차이)
kubectl get pods -l app=demo                                   # label로는 걸림 → pod-anno
kubectl get pods --field-selector metadata.annotations.managed=yes 2>&1 || true   # annotation은 field selector로도 못 거름

# STEP 9 — annotation 수정·제거 (07-03 §3, label과 같은 문법)
kubectl annotate pod pod-anno created-by='Humpty Dumpty' --overwrite
kubectl annotate pod pod-anno created-by-                     # 키 뒤 마이너스 = 제거

# ─────────────────────────────────────────────────────────────
# CLEANUP
kubectl config set-context --current --namespace default
kubectl delete namespace fields-lab --ignore-not-found        # 안의 Pod 전부 자동 삭제(07-01 §5)
