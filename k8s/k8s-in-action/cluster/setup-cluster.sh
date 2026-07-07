#!/usr/bin/env bash
# =============================================================================
# kind 실습 클러스터 생성·이미지 로드·삭제 — Kubernetes in Action
#
# control-plane 1 + worker 2 (3노드) 클러스터를 만들고, 로컬에서 빌드한
# kiada 이미지를 노드로 로드하는 것까지 한 번에 준비합니다.
#
# 전제: docker, kind, kubectl 설치됨. 02-02(kiada-0.1)에서 kiada:0.1 이미지 빌드됨.
# 사용법:  ./setup-cluster.sh create    # 클러스터 생성 + 이미지 로드
#          ./setup-cluster.sh load      # 이미지만 (재)로드
#          ./setup-cluster.sh delete    # 클러스터 삭제
# =============================================================================
set -euo pipefail

CLUSTER="k8s-lab"
# 이 스크립트 위치 기준으로 config 를 찾음(어디서 실행하든 동작)
CONFIG="$(cd "$(dirname "$0")" && pwd)/kind-config.yaml"
IMAGE="kiada:0.1"

create_cluster() {
  # 이미 있으면 재사용(중복 생성 방지)
  if kind get clusters | grep -qx "$CLUSTER"; then
    echo "클러스터 '$CLUSTER' 이미 존재 — 생성 건너뜀"
  else
    kind create cluster --name "$CLUSTER" --config "$CONFIG"
  fi
  kubectl cluster-info --context "kind-$CLUSTER"
  kubectl get nodes           # control-plane 1 + worker 2 = Ready 확인
  load_image
}

load_image() {
  # kind 노드는 로컬 Docker 이미지를 자동으로 못 본다 → 명시적으로 넣어야 함.
  # ⚠️ 배포에 쓸 태그(:0.1)를 정확히 로드할 것. latest 만 넣으면 :0.1 배포가 실패한다.
  if docker image inspect "$IMAGE" >/dev/null 2>&1; then
    kind load docker-image "$IMAGE" --name "$CLUSTER"
    docker exec "${CLUSTER}-worker" crictl images | grep kiada || true
  else
    echo "로컬에 $IMAGE 이미지 없음 — 먼저 02-02에서 빌드하세요:"
    echo "  cd kiada-0.1 && docker build -t $IMAGE ."
    exit 1
  fi
}

delete_cluster() {
  kind delete cluster --name "$CLUSTER"
}

case "${1:-create}" in
  create) create_cluster ;;
  load)   load_image ;;
  delete) delete_cluster ;;
  *) echo "사용법: $0 {create|load|delete}"; exit 1 ;;
esac
