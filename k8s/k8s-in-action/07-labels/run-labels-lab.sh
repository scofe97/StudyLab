#!/usr/bin/env bash
# 07-labels 실습 러너 — 통째 실행 금지. STEP을 하나씩 복사해 실행하고, 출력을 학습노트 07-02 §과 대조한다.
# 전제: kind 클러스터(k8s-lab, control-plane 1 + worker 2)가 떠 있어야 한다.
set -euo pipefail

# ─────────────────────────────────────────────────────────────
# STEP 1 — 라벨 붙은 Pod 4개 생성 (07-02 §1~2)
#   app(kiada·quote) × rel(stable·canary) 2차원 조직
kubectl apply -f pods-labeled.yaml

# STEP 2 — 라벨 조회 3가지 방법 (§2)
kubectl get pods --show-labels          # LABELS 열
kubectl get pods -L app,rel             # 라벨을 각자 열로

# STEP 3 — 라벨 추가·변경·제거 (§2)
kubectl label pod kiada-stable tier=frontend      # 추가
kubectl label pod kiada-stable rel=canary         # 변경 시도 → 거부됨(--overwrite 없음)을 관찰
kubectl label pod kiada-stable rel=canary --overwrite
kubectl label pod kiada-stable tier-              # 키 뒤 마이너스 = 제거

# ─────────────────────────────────────────────────────────────
# STEP 4 — equality-based selector (§4)
kubectl get pods -l app=quote               # quote 앱 전부(stable+canary)
kubectl get pods -l 'app=quote,rel=canary'  # 콤마=AND → quote-canary만

# STEP 5 — set-based selector (§4)
kubectl get pods -l 'app in (kiada, quote)' -L app   # 집합
kubectl get pods -l 'rel'                             # rel 키가 있는 것
kubectl get pods -l '!rel'                            # rel 키가 없는 것(작은따옴표 필수)

# STEP 6 — selector로 일괄 삭제 (§4) ⚠️ 확인 없이 지운다
kubectl delete pods -l rel=canary           # canary 두 개(kiada·quote) 한 번에

# ─────────────────────────────────────────────────────────────
# STEP 7 — nodeSelector: 아직 disk=ssd 라벨 노드가 없으니 Pending 관찰 (§5)
kubectl apply -f pod-nodeselector.yaml
kubectl get pod pod-on-ssd -o wide          # STATUS=Pending (매칭 노드 없음)
# 스케줄 실패 사유 확인 — get events 가 한 줄로 깔끔하다(describe grep 은 Message 줄이 길어 -A5 필요).
kubectl get events --field-selector involvedObject.name=pod-on-ssd
# 실제 사유(k8s v1.35 실측): "2 node(s) didn't match Pod's node affinity/selector"
#   + control-plane 1개는 "untolerated taint(s)" 로 별도 제외 → 0/3 available.

# STEP 8 — worker 노드에 라벨 붙이면 스케줄된다 (§5)
kubectl label node k8s-lab-worker disk=ssd
kubectl get pod pod-on-ssd -o wide          # 이제 Running, NODE=k8s-lab-worker

# ─────────────────────────────────────────────────────────────
# STEP 9 — nodeAffinity: set-based (§5) — nodeSelector가 못 하는 것
kubectl apply -f pod-nodeaffinity.yaml
kubectl get pod pod-affinity -o wide        # disk in (ssd,nvme) 매칭 → worker에 뜸

# STEP 10 — skip-me 라벨을 붙이면 DoesNotExist 조건에 걸려 스케줄 실패 (§5)
kubectl label node k8s-lab-worker skip-me=true
kubectl delete pod pod-affinity
kubectl apply -f pod-nodeaffinity.yaml
kubectl get pod pod-affinity -o wide        # Pending — skip-me 있는 노드는 제외됨

# ─────────────────────────────────────────────────────────────
# STEP 11 — canary 배포: Service selector 하나로 stable·canary에 트래픽 분배 (§Spring 관점)
#   selector 를 app=kiada '하나로만' 잡으면 stable 2 + canary 1 이 같은 endpoint 풀에 들어가,
#   트래픽이 endpoint 수 비율(약 1/3 canary)로 나뉜다. rel 을 selector 에 넣지 않는 게 핵심.
kubectl apply -f canary-service.yaml
kubectl wait --for=condition=Ready pod -l app=kiada -n labels-lab --timeout=60s
kubectl get endpoints kiada -n labels-lab          # endpoint 3개(stable 2 + canary 1)가 다 걸림

# 트래픽 분배 관찰 — 임시 Pod에서 curl 90회, STABLE:CANARY 비율을 센다
kubectl run curl-probe --image=curlimages/curl --restart=Never -n labels-lab --rm -i --quiet -- \
  sh -c 'for i in $(seq 90); do curl -s -H "Connection: close" http://kiada.labels-lab.svc.cluster.local; done' \
  | sort | uniq -c
# 기대값 STABLE ~60 : CANARY ~30 (endpoint 3개 균등이라 stable 2개가 2배). 표본 작으면 튄다.

# 문제 있으면 canary 만 걷어내기 — app 은 그대로, rel=canary 만 지운다
kubectl delete pods -n labels-lab -l app=kiada,rel=canary

# ─────────────────────────────────────────────────────────────
# CLEANUP — 실습 끝나면 정리
kubectl delete -f pods-labeled.yaml --ignore-not-found
kubectl delete -f canary-service.yaml --ignore-not-found
kubectl delete pod pod-on-ssd pod-affinity --ignore-not-found
kubectl label node k8s-lab-worker disk- skip-me- 2>/dev/null || true
