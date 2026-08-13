#!/usr/bin/env bash
# 11-02 외부 노출 — NodePort·externalTrafficPolicy kind 실습 (2026-07-20 학습 세션 재현용)
# 전제: 11-01에서 배포한 kiada Deployment(3 replicas)가 살아있음. 없으면 11-01 practice.sh 먼저.
# 노드 IP는 환경마다 다름 — 아래 NODE_* 를 `kubectl get nodes -o wide`로 확인해 교체.
# 실행 위치: k8s_in_action 폴더 기준. (cd 11-service/02-nodeport 후 파일 참조)
set -euo pipefail

# 노드 IP 채우기 (kind k8s-lab 예시값 — 본인 환경으로 교체)
NODE1=172.21.0.2   # 파드 없을 수도 있는 노드
NODE2=172.21.0.3
NODE3=172.21.0.4

echo "=== 실습 A — NodePort 생성 + 6포트 매핑 확인 ==="
kubectl apply -f 11-service/02-nodeport/svc-nodeport.yaml
kubectl get svc kiada-np                                  # PORT(S)에 80:30080만 요약 표시 (targetPort는 숨김)
kubectl get svc kiada-np -o jsonpath='{.spec.ports[0].port} {.spec.ports[0].nodePort} {.spec.ports[0].targetPort}{"\n"}'
# 관찰: port=80(ClusterIP) / nodePort=30080(외부 진입) / targetPort=8080(파드). get은 port:nodePort만 요약.

echo "=== 실습 B — 모든 노드의 30080이 응답 (파드 없는 노드 포함) ==="
for N in "$NODE1" "$NODE2" "$NODE3"; do
  echo "--- $N:30080 ---"
  kubectl run np-test --image=curlimages/curl --rm -i --restart=Never -- \
    curl -s --max-time 3 "http://$N:30080/" | grep -oE 'kiada-[a-z0-9-]+' || echo "(무응답)"
done
# 관찰: kube-proxy가 각 노드 iptables에 NodePort 규칙을 심어, 파드 없는 노드도 다른 노드 파드로 DNAT.

echo "=== 실습 C-1 — Cluster 정책(기본): Client IP가 노드 IP로 SNAT ==="
kubectl get svc kiada-np -o jsonpath='{.spec.externalTrafficPolicy}{"\n"}'  # Cluster
kubectl run np-test --image=curlimages/curl --rm -i --restart=Never -- \
  sh -c "curl -s --max-time 3 http://$NODE3:30080/ | grep -i 'client ip' || echo '(client ip 필드 없으면 앱 응답 확인)'"
# 관찰: 파드가 보는 Client IP = 받은 노드 IP (원본 클라이언트 손실). SNAT/masquerade가 개입.

echo "=== 실습 C-2 — Local 정책: 원본 IP 보존 + 파드 없는 노드 타임아웃 ==="
kubectl patch svc kiada-np -p '{"spec":{"externalTrafficPolicy":"Local"}}'
for N in "$NODE1" "$NODE2" "$NODE3"; do
  echo "--- $N:30080 (Local) ---"
  kubectl run np-test --image=curlimages/curl --rm -i --restart=Never -- \
    curl -s --max-time 3 "http://$N:30080/" >/dev/null && echo "응답" || echo "(타임아웃 — 로컬 파드 없음)"
done
# 관찰: 파드 있는 노드는 원본 IP 보존(SNAT 안 함), 파드 없는 노드는 로컬 전용 라우팅이라 타임아웃.

echo "=== 정리 — Cluster로 복원 ==="
kubectl patch svc kiada-np -p '{"spec":{"externalTrafficPolicy":"Cluster"}}'
kubectl get svc kiada-np -o jsonpath='{.spec.externalTrafficPolicy}{"\n"}'
# 정리: 실습 후 기본값 Cluster로 되돌림. (kubectl delete -f 11-service/02-nodeport/svc-nodeport.yaml 로 서비스 삭제 가능)
