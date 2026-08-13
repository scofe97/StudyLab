#!/usr/bin/env bash
# 11-01 Service 기초 — kind 실습 (2026-07-20 학습 세션 재현용)
# 전제: 03-02에서 배포한 kiada Deployment(3 replicas) + kiada ClusterIP Service가 살아있음.
#   없으면: kubectl expose deployment kiada --port=8080 --target-port=8080
# 각 블록을 순서대로 실행하며 출력을 관찰한다.
set -euo pipefail

echo "=== 실습 A — Service→Endpoints 자동 등록 (label selector 실증) ==="
kubectl get svc kiada                       # ClusterIP는 고정 (10.96.x.x)
kubectl get endpoints kiada                 # 파드 IP 목록 (10.244.x.x) — v1.33+는 EndpointSlice 권장
kubectl get pods -l app=kiada -o wide       # 위 endpoints IP와 파드 IP가 일치함을 확인
# 관찰: ClusterIP(가상, 고정) ≠ Endpoints(파드 실제 IP, 유동). 대역도 다름(10.96 vs 10.244).

echo "=== 실습 B — 파드 삭제 → Endpoints 자동 갱신 ==="
kubectl get endpoints kiada                 # 삭제 전 스냅샷
POD=$(kubectl get pods -l app=kiada -o jsonpath='{.items[0].metadata.name}')
kubectl delete pod "$POD"                    # 파드 하나 삭제 (Service는 건드리지 않음)
sleep 5
kubectl get endpoints kiada                 # 새 파드 IP로 자동 교체됨 (내가 Service 안 건드렸는데도)
kubectl get pods -l app=kiada -o wide
# 관찰: 파드 IP는 바뀌어도 ClusterIP는 그대로. EndpointSlice 컨트롤러가 라벨 매칭으로 명단 갱신.

echo "=== 실습 C-1 — port-forward는 Service 우회 (분산 안 됨) ==="
kubectl port-forward svc/kiada 18080:8080 & PF_PID=$!
sleep 2
for i in $(seq 1 30); do curl -s localhost:18080/ | grep -oE 'kiada-[a-z0-9-]+'; done | sort | uniq -c
kill "$PF_PID"
# 관찰: 30번이 한 파드에만 몰림. port-forward는 파드 하나에 직접 터널 → kube-proxy 로드밸런싱 우회.

echo "=== 실습 C-2 — 클러스터 내부 ClusterIP는 분산 (+ DNS 이름 resolve 겸) ==="
kubectl run curl-test --image=curlimages/curl --rm -it --restart=Never -- \
  sh -c 'for i in $(seq 1 30); do curl -s http://kiada:8080/ | grep -oE "kiada-[a-z0-9-]+"; done | sort | uniq -c'
# 관찰: 세 파드에 분산(예 13/8/9, 노드 경계 넘음). http://kiada 짧은 이름이 search+ndots로 resolve됨.
#       완벽한 10/10/10이 아닌 건 kube-proxy iptables random 분산이라서.
