#!/usr/bin/env bash
# =============================================================================
# Kubernetes in Action Ch3 §3.2 — 첫 애플리케이션 배포와 스케일링 (실습 명령 모음)
#
# 앞 편(02-02)에서 빌드한 로컬 kiada 이미지를 kind 클러스터에 배포하고,
# Service로 노출한 뒤 3개로 스케일해 로드밸런싱까지 확인하는 전 과정입니다.
# 실제로 돌리며 부딪힌 두 함정(imagePullPolicy, port-forward)을 주석으로 남깁니다.
#
# 전제: kind 클러스터 'k8s-lab'(control-plane 1 + worker 2)이 떠 있고,
#       02-02에서 만든 kiada:0.1 / kiada:latest 이미지가 로컬 Docker에 있음.
# 학습 노트(개념): runners-high/write/08_cloud/book/kubernetes-in-action/03-02
#
# ⚠️ 이 스크립트는 한 번에 실행하기보다 블록 단위로 따라 하며 출력을 관찰하세요.
# =============================================================================
set -euo pipefail

CLUSTER="k8s-lab"

# -----------------------------------------------------------------------------
# 0. 클러스터 확인 — 어느 컨텍스트를 보고 있고 노드가 살아있나
# -----------------------------------------------------------------------------
kubectl config current-context          # kind-k8s-lab 이어야 함
kubectl get nodes                       # control-plane 1 + worker 2 = Ready

# -----------------------------------------------------------------------------
# 1. 로컬 이미지를 kind 노드로 로드
#    kind 노드는 로컬 Docker 이미지를 자동으로 못 본다 → 명시적으로 넣어야 함.
#    ⚠️ 함정 ①: latest 태그만 로드하면 :0.1 배포 시 "노드에 0.1 없음"으로 실패한다.
#              배포에 쓸 태그를 정확히 로드할 것.
# -----------------------------------------------------------------------------
kind load docker-image kiada:0.1 --name "$CLUSTER"

# 노드 안에 실제로 들어갔는지 확인 (containerd 이미지 목록)
docker exec ${CLUSTER}-worker crictl images | grep kiada

# -----------------------------------------------------------------------------
# 2. Deployment 생성 — 한 줄이 Deployment→ReplicaSet→Pod 3층을 만든다
#    Pod 이름 kiada-<ReplicaSet해시>-<Pod해시> 에 그 계층이 그대로 박힌다.
# -----------------------------------------------------------------------------
kubectl create deployment kiada --image=kiada:0.1

# ⚠️ 함정 ②: imagePullPolicy
#   - 태그가 :latest 면 기본 정책이 Always → 로컬 무시하고 Docker Hub로 감
#     → kiada 는 Hub 에 없으니 ErrImagePull / pull access denied
#   - :0.1 처럼 latest 가 아니면 기본 IfNotPresent(로컬 우선)지만,
#     그래도 노드에 그 태그가 있어야 한다(함정 ① 참고).
#   확실히 하려면 정책을 명시적으로 IfNotPresent 로 패치:
kubectl patch deployment kiada --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}]'

# Running 확인 — 이번엔 이벤트에 "already present on machine" 이 떠야 정상
kubectl get pods -l app=kiada -o wide
kubectl describe pod -l app=kiada | grep -E "Image|Pulled|present" | head -5

# -----------------------------------------------------------------------------
# 3. Service 로 노출 — 라벨 셀렉터(app=kiada)로 Pod 를 자동 등록
#    kind 엔 클라우드 로드밸런서가 없어 LoadBalancer External-IP 는 <pending>.
#    그래서 여기선 ClusterIP + port-forward 로 접근한다.
# -----------------------------------------------------------------------------
kubectl expose deployment kiada --port=8080 --target-port=8080

kubectl get svc kiada                   # ClusterIP 가 할당됨 (이 IP 는 안 바뀜)
kubectl get endpoints kiada             # Service 가 물고 있는 실제 Pod IP:port

# -----------------------------------------------------------------------------
# 4. 접속 — 02-02 의 "Request processed by ..." 가 이번엔 Pod 이름으로 찍힌다
#    K8s 에선 Pod 이름 = 호스트명(UTS 네임스페이스) 이라 그렇다.
# -----------------------------------------------------------------------------
kubectl port-forward svc/kiada 18080:8080 >/tmp/pf.log 2>&1 &   # 백그라운드
sleep 3
curl -s http://localhost:18080/          # 평문: Kiada version 0.1. Request processed by "kiada-...".
curl -s http://localhost:18080/html      # HTML 모드

# -----------------------------------------------------------------------------
# 5. 수평 스케일 — replicas=3. ReplicaSet 의 복제본 수를 바꾸는 것.
#    스케줄러가 새 Pod 를 worker / worker2 에 분산 배치한다.
#    Service Endpoints 도 자동으로 3개로 늘어난다(내가 Service 를 안 건드려도).
# -----------------------------------------------------------------------------
kubectl scale deployment kiada --replicas=3
sleep 6
kubectl get pods -l app=kiada -o wide    # 3개 Pod, NODE 열에 worker/worker2 분산
kubectl get endpoints kiada              # Endpoints 3개로 확장됨

# -----------------------------------------------------------------------------
# 6. 로드밸런싱 확인
#    ⚠️ 함정 ③: port-forward 는 Pod 하나에 직접 터널을 뚫어 로드밸런싱을 우회한다.
#              → 30번 요청이 전부 같은 Pod 로만 간다(분산 X).
# -----------------------------------------------------------------------------
# (우회 사례) port-forward 로 30번 → 한 Pod 고정
for i in $(seq 1 30); do curl -s http://localhost:18080/ | grep -oE 'kiada-[a-z0-9-]+'; done | sort | uniq -c
#   → 30 kiada-...-bhgc5      (한 Pod 만!)

# (정답) 클러스터 '안'에서 Service 이름(ClusterIP)으로 요청 → kube-proxy 가 분산
kubectl run curl-test --image=curlimages/curl --rm -it --restart=Never -- \
  sh -c 'for i in $(seq 1 30); do curl -s http://kiada:8080/ | grep -oE "kiada-[a-z0-9-]+"; done | sort | uniq -c'
#   → 13 kiada-...-2vm7s   (worker2)
#     10 kiada-...-8phwt   (worker)
#      7 kiada-...-bhgc5   (worker)
#   세 Pod 에 분산 + 노드 경계를 넘어감. 완벽한 3등분이 아닌 건 iptables 확률 분산이라서.

# -----------------------------------------------------------------------------
# 7. 정리 (실습 종료 시)
# -----------------------------------------------------------------------------
# kill %1                                 # 백그라운드 port-forward 종료
# kubectl delete service kiada
# kubectl delete deployment kiada
