#!/usr/bin/env bash
# =============================================================================
# 07-namespaces 실습 러너 — namespace의 격리·삭제를 손으로 관찰
#
# 학습 노트: runners-high/write/08_cloud/book/kubernetes-in-action/
#            07-01.namespace로 클러스터를 가상 분할하기.md
#
# ⚠️ 이 스크립트는 "한 번에 자동 실행"용이 아니다. 한 STEP씩 복사해서 직접 치고,
#    출력을 눈으로 확인하며 노트의 §과 대조하는 용도다(학습용 코딩).
#    그래서 맨 위에서 곧바로 exit 한다 — 통째로 돌리지 말 것.
# =============================================================================
set -euo pipefail
echo "이 스크립트는 통째로 실행하지 말고, 아래 STEP을 하나씩 복사해 실행하세요."
exit 0

# ── 전제: kind 클러스터가 떠 있어야 한다 ──────────────────────────────────────
# 없으면: kind create cluster --name k8s-lab --config ../cluster/kind-config.yaml
kubectl config current-context          # kind-k8s-lab 인지 확인
kubectl get nodes                        # control-plane 1 + worker 2 = 3노드


# ── STEP 1. 클러스터에 이미 있는 namespace 조회 (노트 §2) ─────────────────────
# default + kube-* 시스템 namespace가 기본으로 존재한다.
kubectl get namespaces                   # 축약: kubectl get ns
# 관찰: kube-system·kube-public·kube-node-lease는 쿠버네티스 예약 namespace.
#       kube-system의 Pod를 보면 시스템 컴포넌트가 여기 격리돼 있다:
kubectl get pods -n kube-system


# ── STEP 2. namespace 두 개 생성 (노트 §3) ───────────────────────────────────
kubectl create namespace kiada-test1     # 명령형 — 가장 빠른 방법
kubectl apply -f kiada-test2-ns.yaml     # 선언형 — Namespace 매니페스트로
kubectl get ns | grep kiada              # 둘 다 Active 로 뜨는지 확인


# ── STEP 3. 같은 이름 Pod를 세 namespace에 배치 (노트 §1·§4 핵심) ─────────────
# 같은 kiada-ssl.yaml 을 -n 만 바꿔 세 번 적용한다. 이름이 같아도 충돌 없음.
kubectl apply -f kiada-ssl.yaml                       # default 에
kubectl apply -f kiada-ssl.yaml -n kiada-test1        # kiada-test1 에
kubectl apply -f kiada-ssl.yaml -n kiada-test2        # kiada-test2 에

# 전체 namespace를 가로질러 보기(-A). "kiada-ssl" 이름이 셋 다 뜬다:
kubectl get pods -A -o wide | grep kiada-ssl
# 관찰 포인트 1 (§1): 같은 이름 Pod 3개가 문제없이 공존 → namespace가 "이름 스코프"를 준다.
# 관찰 포인트 2 (§4): NODE 열을 보라. 세 Pod가 어느 노드에 스케줄됐는지 확인.
#   서로 다른 namespace의 Pod가 같은 worker 노드에 얹힐 수 있다 →
#   "가상 클러스터"는 이름까지만 가상이고, 커널·노드는 공유한다는 증거.


# ── STEP 4. 현재 namespace 전환 (노트 §3) ────────────────────────────────────
# 지금은 명령마다 -n 을 붙여야 한다. 현재 컨텍스트의 기본 namespace를 바꾸면 편하다.
kubectl config set-context --current --namespace kiada-test1
kubectl get pods                          # 이제 -n 없이도 kiada-test1 것이 나온다
# 되돌리기(실습 끝나면):
# kubectl config set-context --current --namespace default


# ── STEP 5. ⭐ finalizer로 Terminating 재현 (노트 §5 — 이 편의 하이라이트) ─────
# 컨트롤러 없는 가짜 finalizer가 붙은 ConfigMap을 kiada-test2 에 만든다:
kubectl apply -f stuck-configmap.yaml

# 이제 kiada-test2 namespace를 지운다. --wait=false 로 안 기다리고 즉시 반환:
kubectl delete ns kiada-test2 --wait=false

# 잠시 뒤 상태를 보면 Active가 아니라 Terminating 에 "멈춰" 있다:
kubectl get ns kiada-test2
# 관찰: 몇 분을 기다려도 안 사라진다. 왜? → STEP 6에서 진단.


# ── STEP 6. ⭐ 원인 진단 — get all이 아니라 status.conditions (노트 §5) ───────
# 흔한 실수: "뭐가 남았지?" 하고 get all 을 친다. 그런데 아무것도 안 나온다:
kubectl get all -n kiada-test2
# ⚠️ 함정(§5): get all 은 일부 타입만 보여준다(ConfigMap·Secret 누락).
#    아무것도 안 나와도 namespace가 비었다는 뜻이 절대 아니다.

# 진짜 원인은 namespace 오브젝트 자신의 status.conditions 에 있다.
# describe는 (책 집필 시점 버그로) namespace status를 안 보여주므로 -o yaml 로 직접 본다:
kubectl get ns kiada-test2 -o yaml
# 관찰: status.conditions 에서 아래 3종을 찾아라(노트 §5 예시와 대조):
#   - type: NamespaceContentRemaining      → 남은 리소스가 있음
#   - type: NamespaceFinalizersRemaining   → status: "True", message에 example.com/pin
#                                            ← 이게 원인. 어느 finalizer가 안 떨어졌는지 지목.
#   - phase: Terminating

# 남은 오브젝트를 직접 짚어보면(ConfigMap은 살아있다):
kubectl get configmap -n kiada-test2
kubectl get configmap stuck-configmap -n kiada-test2 -o jsonpath='{.metadata.finalizers}'; echo


# ── STEP 7. ⭐ 해소 — finalizer 제거 (노트 §5) ───────────────────────────────
# 담당 컨트롤러가 없으니 사람이 손으로 finalizer 목록을 비워준다.
# JSON merge patch로 finalizers를 빈 배열로:
kubectl patch configmap stuck-configmap -n kiada-test2 \
  -p '{"metadata":{"finalizers":[]}}' --type=merge

# finalizer가 사라지는 순간 ConfigMap이 삭제되고 → 연쇄로 namespace도 사라진다:
kubectl get ns kiada-test2
# 관찰: 이제 kiada-test2 가 목록에서 없어졌다(또는 잠깐 뒤 사라진다).
#       "finalizer 하나 떼니 namespace 삭제가 완료됐다" = §5 인과를 손으로 확인.


# ── STEP 8. 정리 ─────────────────────────────────────────────────────────────
kubectl config set-context --current --namespace default   # 기본 namespace 복구
kubectl delete pod kiada-ssl -n default --ignore-not-found
kubectl delete pod kiada-ssl -n kiada-test1 --ignore-not-found
kubectl delete ns kiada-test1 --ignore-not-found
# kiada-test2 는 STEP 7에서 이미 사라졌다.
