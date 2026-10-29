#!/usr/bin/env bash
# 08-secret-downward 실습 러너 — 통째 실행 금지. STEP을 하나씩 복사해 실행하고,
# 출력을 학습노트 08-03 §과 대조한다.
# 전제: kind 클러스터(kind-k8s-lab)가 떠 있어야 한다. TODO 매니페스트를 먼저 채운 뒤 진행.
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# STEP 0 — 실습용 namespace 생성 + 기본 ns 전환(07-01 §3)
kubectl create namespace secret-lab
kubectl config set-context --current --namespace secret-lab

# ─────────────────────────────────────────────────────────────
# STEP 1 — generic Secret 생성 (08-03 §2)
#   --from-literal 로 user/pass 두 엔트리. kubectl create secret 은 타입을 바로 뒤에 지정한다.
kubectl create secret generic demo-secret \
  --from-literal user=admin \
  --from-literal pass=s3cr3t-p@ss

# STEP 2 — ★핵심 관찰: Base64는 암호화가 아니다 (08-03 §2·§4-1)
#   -o yaml 로 보면 data 에 Base64 문자열이 그대로 보인다. 키 없이 디코딩된다.
kubectl get secret demo-secret -o yaml | grep -A3 '^data:'
echo "--- 위 pass 값을 디코딩하면 원문이 그대로 나온다 ---"
kubectl get secret demo-secret -o jsonpath='{.data.pass}' | base64 -d; echo

# ─────────────────────────────────────────────────────────────
# STEP 3 — Secret을 환경변수로 주입 (08-03 §3)  [pod-secret-env.yaml TODO 먼저 채우기]
kubectl apply -f pod-secret-env.yaml
kubectl wait --for=condition=Ready pod/secret-env-demo --timeout=60s
kubectl logs secret-env-demo          # DB_PASSWORD=s3cr3t-p@ss 로 '평문' 주입됨을 관찰(§1 47줄: 주입 시 디코딩)

# ─────────────────────────────────────────────────────────────
# STEP 4 — Downward API: Pod 자기정보 + 리소스 한도 env 주입 (08-03 §5~6) [pod-downward-env.yaml TODO 먼저]
kubectl apply -f pod-downward-env.yaml
kubectl wait --for=condition=Ready pod/downward-env-demo --timeout=60s
kubectl logs downward-env-demo | grep -E 'POD_NAME|POD_IP|NODE_NAME|MAX_MEMORY_MIB'

# STEP 5 — 주입값이 실제 오브젝트와 일치하는지 교차 검증 (08-03 §6 244줄)
kubectl get pod downward-env-demo -o wide     # NAME·IP·NODE 열과 위 env 값 대조
echo "--- MAX_MEMORY_MIB=128 이 맞는가? limits.memory=128Mi, divisor=1Mi 이므로 128 ---"

# ─────────────────────────────────────────────────────────────
# CLEANUP
kubectl config set-context --current --namespace default
kubectl delete namespace secret-lab --ignore-not-found    # 안의 Pod·Secret 전부 자동 삭제(07-01 §5)
