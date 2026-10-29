#!/usr/bin/env bash
# =============================================================================
# Kubernetes in Action Ch4 §4.x — Node와 Event 오브젝트로 보는 필드 실습
#
# API 오브젝트의 네 섹션(apiVersion·kind·metadata·spec·status)을 살아있는 Node
# 오브젝트로 읽고, status.conditions 가 왜 단일 필드가 아니라 '리스트'로 설계됐는지,
# PIDPressure 같은 condition 이 실제로 무엇을 감시하는지 커널 값까지 내려가 확인합니다.
# 이어서 Event 가 왜 오브젝트 매니페스트에 안 들어가는 독립 오브젝트인지 봅니다.
#
# 전제: kind 클러스터 'k8s-lab'(control-plane 1 + worker 2, v1.35.0) 기동 중.
#       노드 이름은 k8s-lab-control-plane / k8s-lab-worker / k8s-lab-worker2.
# 학습 노트(개념): runners-high/write/08_cloud/book/kubernetes-in-action/04-02
#
# ⚠️ 블록 단위로 따라 하며 출력을 관찰하세요. 실측값(주석의 예시 숫자)은 클러스터
#    상태에 따라 달라집니다 — 값 자체가 아니라 '무엇을 보는가'에 집중하세요.
# =============================================================================
set -euo pipefail

# -----------------------------------------------------------------------------
# 1. Node 오브젝트 목록 — kind 클러스터의 세 노드가 세 Node 오브젝트로 보인다
# -----------------------------------------------------------------------------
kubectl get nodes                                          # NAME/STATUS/ROLES/AGE/VERSION
#   k8s-lab-control-plane   Ready   control-plane   3d   v1.35.0
#   k8s-lab-worker          Ready   <none>          3d   v1.35.0
#   k8s-lab-worker2         Ready   <none>          3d   v1.35.0

# -----------------------------------------------------------------------------
# 2. Node 매니페스트의 네 섹션 — 최상위 키가 apiVersion·kind·metadata·spec·status
#    -o yaml 전체는 길다. 최상위 섹션 헤더만 뽑아 '구조'를 먼저 본다.
# -----------------------------------------------------------------------------
kubectl get node k8s-lab-worker -o yaml \
  | grep -nE "^(apiVersion|kind|metadata|spec|status):"    # 다섯 줄이 순서대로 잡힘
#   이 순서(apiVersion→kind→metadata→spec→status)가 우연히 알파벳 순과 겹쳐 읽기 편하다.

# -----------------------------------------------------------------------------
# 3. status.conditions — '리스트'로 설계된 이유를 눈으로
#    단일 필드가 아니라 여러 condition 을 나열한다: 왜(진단 가능) + 확장성(open-closed).
#    각 줄 = type / status / reason.  Pressure 3종은 False 가 정상, Ready 는 True 가 정상.
# -----------------------------------------------------------------------------
kubectl get node k8s-lab-worker \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\n"}{end}'
#   MemoryPressure  False  KubeletHasSufficientMemory
#   DiskPressure    False  KubeletHasNoDiskPressure
#   PIDPressure     False  KubeletHasSufficientPID
#   Ready           True   KubeletReady
#   → 새 자원 압박 유형이 생겨도 '필드 추가'가 아니라 '리스트 원소 추가'로 확장된다.

# -----------------------------------------------------------------------------
# 4. PIDPressure 가 실제로 감시하는 것 — 커널 PID 공간까지 내려가 확인
#    condition 은 추상적 신호일 뿐. 그 아래 진짜 한계값을 노드 컨테이너 안에서 읽는다.
#    (kind 노드 = 도커 컨테이너라 docker exec 로 그 안 /proc 를 읽는다)
# -----------------------------------------------------------------------------
docker exec k8s-lab-worker cat /proc/sys/kernel/pid_max      # 4194304  PID 번호 공간(2^22)
docker exec k8s-lab-worker cat /proc/sys/kernel/threads-max  # 15742    동시 태스크 실제 상한
docker exec k8s-lab-worker sh -c 'ls -d /proc/[0-9]* | wc -l' # 29      현재 사용(거의 놀고 있음)
#   pid_max(419만)는 PID '번호'가 wrap around 하는 한계일 뿐, 실제 병목은 대개
#   threads-max(여기 15,742). 이 값은 노드 메모리에 비례해 커널이 자동 설정한다.
#   스레드 누수 앱이 threads-max 근처까지 끌어올리면 kubelet 이 PIDPressure:True 로
#   올리고 스케줄러가 이 노드를 회피한다. 방어선은 cgroup(--pod-max-pids → pids.max).

# -----------------------------------------------------------------------------
# 5. kubectl describe — 같은 정보를 사람이 읽기 좋게 (+ Pod 목록·오버커밋 주석)
# -----------------------------------------------------------------------------
kubectl describe node k8s-lab-worker2                       # Conditions/Capacity/Non-terminated Pods/Events
#   'Total limits may be over 100 percent, i.e., overcommitted' 주석:
#   모든 Pod 가 동시에 한도까지 쓰지 않는다는 가정 아래 노드를 오버커밋해 활용률을 높인다.

# -----------------------------------------------------------------------------
# 6. Event 는 독립 오브젝트 — 매니페스트 안에 없다
#    Node -o yaml 에는 event 가 없다. Event 는 짧은 시간에 한 오브젝트에 여러 개가
#    생기므로 오브젝트의 일부가 아니라 별도 오브젝트로 존재한다 → get events 로 나열.
# -----------------------------------------------------------------------------
kubectl get events                                         # 몇 개만 뜨는 게 정상(아래 이유)
#   ⚠️ "모든 이벤트"가 아니다:
#     ① 기본 보존 1시간 — 오래된 이벤트는 이미 사라짐
#     ② 기본은 현재(default) 네임스페이스만 — 전체는 -A
kubectl get events -A --sort-by='.lastTimestamp'           # 전체 네임스페이스, 시간순
#   즉 get events 출력 = "최근 1시간 + 지정 네임스페이스"이지 전수 로그가 아니다.

# -----------------------------------------------------------------------------
# 7. 정리 — 이 실습은 read-only. 만든 오브젝트가 없어 정리할 것도 없다.
#    (Node·Event 는 조회만 했다)
# -----------------------------------------------------------------------------
