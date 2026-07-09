#!/usr/bin/env bash
# =============================================================================
# Kubernetes in Action Ch6 §6.1 — Pod 상태: phase·conditions·컨테이너 상태
#
# Pod status 를 세 축으로 읽는다: phase(한 단어 요약) / conditions(여러 축 세부) /
# containerStatuses(컨테이너별). 특히 "phase 는 Running 인데 Ready 는 False" 상태를
# 직접 만들어, phase 만으론 부족하고 conditions 가 왜 필요한지 눈으로 확인한다.
#
# 전제: kind 클러스터 'k8s-lab'(v1.35.0) 기동 중. 기존 kiada Deployment Pod 존재.
# 학습 노트(개념): runners-high/write/08_cloud/book/kubernetes-in-action/06-01
#
# ⚠️ 블록 단위로 따라 하며 출력을 관찰하세요. 상태 이름(terminated/Running)은 단어가
#    아니라 exitCode·reason·ready 필드로 성공/준비 여부를 판정합니다.
# =============================================================================
set -euo pipefail

# -----------------------------------------------------------------------------
# 1. phase 전이 — 배포 '즉시' 는 Pending, 곧 Running
#    phase 는 Pod 생애의 어느 단계인지를 한 단어로 요약한다(성공/실패 판정이 아님).
# -----------------------------------------------------------------------------
kubectl apply -f pod-readiness-demo.yaml && kubectl get pod readiness-demo
#   0s : Pending 0/2   ← 아직 준비 중(스케줄·이미지 pull)
sleep 10
kubectl get pod readiness-demo
#   10s: Running 1/2   ← phase 는 Running 인데 컨테이너 하나가 not-ready → 1/2

# -----------------------------------------------------------------------------
# 2. conditions — phase 가 못 담는 세부를 여러 축으로
#    Ready=False 인 '이유'(reason)까지 나온다. phase=Running 과 대비해서 본다.
# -----------------------------------------------------------------------------
kubectl get pod readiness-demo \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\n"}{end}'
#   PodReadyToStartContainers  True                       ← v1.29+ 추가된 5번째(책엔 4개)
#   Initialized                True
#   Ready                      False  ContainersNotReady  ← Pod 전체는 아직 not-ready
#   ContainersReady            False  ContainersNotReady
#   PodScheduled               True

# -----------------------------------------------------------------------------
# 3. 컨테이너별 ready — 1/2 의 정체
#    컨테이너 하나라도 ready=false 면 Pod 전체 Ready condition 이 False → READY 1/2.
#    즉 컨테이너별 ready 를 '종합'해 Pod 전체 ready 를 판정한다.
# -----------------------------------------------------------------------------
kubectl get pod readiness-demo \
  -o jsonpath='{range .status.containerStatuses[*]}{.name}: ready={.ready}{"\n"}{end}'
#   ready-ok:   ready=true    ← readiness probe 통과
#   ready-fail: ready=false   ← readiness probe 실패(없는 파일 체크) → Pod 전체 not-ready

# -----------------------------------------------------------------------------
# 4. phase 단방향성 — 컨테이너가 재시작해도 phase 는 Running 유지
#    재시작은 '컨테이너 수준' 사건. Pod phase 는 Running 에서 Pending 으로 돌아가지 않는다.
# -----------------------------------------------------------------------------
kubectl get pod -l app=kiada \
  -o jsonpath='{range .items[*]}{.metadata.name}  phase={.status.phase}  restarts={.status.containerStatuses[0].restartCount}{"\n"}{end}'
#   kiada-...  phase=Running  restarts=4   ← 4번 죽었다 살아나도 phase=Running
#   복구는 phase 되돌림이 아니라 Deployment 가 '새 Pod'(Pending 부터)를 만드는 것.

# -----------------------------------------------------------------------------
# 5. 정리
# -----------------------------------------------------------------------------
kubectl delete pod readiness-demo --now
