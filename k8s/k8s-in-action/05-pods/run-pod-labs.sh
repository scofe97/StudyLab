#!/usr/bin/env bash
# =============================================================================
# Kubernetes in Action Ch5 §5.3~5.6 — 멀티 컨테이너·init·네이티브 사이드카 실습
#
# 네 실습으로 Pod 안 컨테이너들의 생명주기를 관찰한다:
#   A. Envoy 사이드카가 HTTPS 를 대신 처리(TLS 종료) — 앱 코드 0줄 수정
#   B. init·네이티브 사이드카·주 컨테이너의 시작 순서와 상태
#   C. init 실패 시 주 컨테이너가 시작조차 못 함(PodInitializing 갇힘)
#   D. Pod 종료 시 주 컨테이너 먼저, 네이티브 사이드카가 마지막
#
# 전제: kind 클러스터 'k8s-lab'(v1.35.0) 기동 중.
# 학습 노트(개념): runners-high/write/08_cloud/book/kubernetes-in-action/05-03
#
# ⚠️ 블록 단위로 따라 하며 출력을 관찰하세요. terminated/Completed 는 실패가 아니라
#    '정상 종료'입니다(exitCode 0). 상태는 단어가 아니라 exitCode·reason 으로 판정.
# =============================================================================
set -euo pipefail

# -----------------------------------------------------------------------------
# A. Envoy 사이드카 — HTTPS 를 대신 받아 평문으로 kiada 에 넘긴다(TLS 종료)
#    init 이 자체서명 인증서 + Envoy config 를 생성(ConfigMap 없이 initContainer 만).
#    ⚠️ 공식 멀티아키(ARM) 이미지 사용 + command 로 entrypoint 우회(아래 매니페스트 주석 참고).
# -----------------------------------------------------------------------------
kubectl apply -f pod-kiada-ssl.yaml
kubectl wait --for=condition=Ready pod/kiada-ssl --timeout=90s     # 2/2 대기(이미지 pull 포함)
# 다른 터미널에서: kubectl port-forward kiada-ssl 8080 8443 9901
#   curl localhost:8080            # HTTP  — Node.js 직접
#   curl -k https://localhost:8443 # HTTPS — Envoy 가 TLS 종료 후 8080 중계(같은 응답!)
#   curl -s localhost:9901/server_info   # Envoy admin: "state": "LIVE"
kubectl delete pod kiada-ssl --now

# -----------------------------------------------------------------------------
# B. 생명주기 — 시작 순서(사이드카 먼저) + init 은 끝나면 죽음 + 주 컨테이너 병렬
# -----------------------------------------------------------------------------
kubectl apply -f pod-lifecycle-demo.yaml
kubectl wait --for=condition=Ready pod/lifecycle-demo --timeout=40s
kubectl get pod lifecycle-demo                                     # READY 3/3(init-work 는 죽어서 제외)
kubectl get pod lifecycle-demo \
  -o jsonpath='{range .status.initContainerStatuses[*]}{.name}: {.state}{"\n"}{end}'
#   sidecar: running                          ← 네이티브 사이드카: 계속 삶
#   init-work: terminated/Completed(exit 0)   ← 일반 init: 끝나면 죽음(정상!)
kubectl get pod lifecycle-demo \
  -o jsonpath='{range .status.containerStatuses[*]}{.name}: {.state}{"\n"}{end}'
#   main·helper: running                      ← 주 컨테이너 병렬
kubectl delete pod lifecycle-demo --now

# -----------------------------------------------------------------------------
# C. init 실패 — 주 컨테이너는 시작조차 못 한다(PodInitializing 에 갇힘)
# -----------------------------------------------------------------------------
kubectl apply -f pod-init-fail.yaml
sleep 15
kubectl get pod init-fail-demo                                     # STATUS: Init:Error
kubectl get pod init-fail-demo -o jsonpath='{.status.containerStatuses[0].state}{"\n"}'
#   {"waiting":{"reason":"PodInitializing"}}  ← main 은 '시작조차 못 함'
kubectl logs init-fail-demo -c failing-init                        # 죽은 init 로그도 조회됨(원인 추적)
kubectl delete pod init-fail-demo --now

# -----------------------------------------------------------------------------
# D. 종료 순서 — 주 컨테이너 먼저, 네이티브 사이드카가 마지막
#    각 컨테이너 preStop 훅으로 종료 시각을 로그에 남긴 뒤 1초 간격으로 수집.
# -----------------------------------------------------------------------------
kubectl apply -f pod-shutdown-order.yaml
kubectl wait --for=condition=Ready pod/shutdown-order-demo --timeout=40s
kubectl delete pod shutdown-order-demo --wait=false
for i in $(seq 1 6); do
  M=$(kubectl logs shutdown-order-demo -c main    2>/dev/null | grep SIGTERM || true)
  S=$(kubectl logs shutdown-order-demo -c sidecar 2>/dev/null | grep SIGTERM || true)
  echo "t=${i}s | main:[$M] sidecar:[$S]"
  sleep 1
done
#   t=3s: main SIGTERM(먼저) → t=6s: sidecar SIGTERM(마지막)
#   3초 차 = 주 컨테이너가 다 죽은 뒤에야 사이드카에 종료 신호(로그·네트워크 사이드카 안전).
