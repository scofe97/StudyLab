# 07-labels 오답 노트

> 실습·복습에서 미끄러진 지점을 모읍니다. 재방문 트리거 날짜가 되면 해당 항목을 가리고 스스로 답한 뒤 펼쳐 대조합니다.

## 2026-07-14 — equality-based vs set-based selector

**재방문 트리거: 2026-07-15**

미끄러진 지점: `nodeSelector`와 `nodeAffinity`의 관계를 "nodeAffinity는 nodeSelector를 닮은 것"으로 설명했는데, 이건 방향이 틀렸습니다. nodeAffinity는 nodeSelector를 **넘어섭니다**.

- `nodeSelector`는 **equality-based**만 됩니다 — `disk: ssd`처럼 "키=값" 등호 하나뿐입니다. not-equal도, 여러 값 중 하나도 표현하지 못합니다.
- `nodeAffinity`의 `matchExpressions`는 **set-based**입니다 — `In`(여러 값 중 하나), `NotIn`, `Exists`, `DoesNotExist`, `Gt`, `Lt` 여섯 연산자로 "ssd 또는 nvme"나 "특정 라벨이 **없는** 노드"까지 고릅니다.

자문해서 답하기:
1. `nodeSelector`로 "disk가 ssd 또는 nvme인 노드"를 표현할 수 있는가? → 못 합니다. equality 하나뿐이라 값 하나만 지정됩니다. set-based가 필요하면 nodeAffinity로 갑니다.
2. "skip-me 라벨이 없는 노드에만"은 어느 연산자인가? → `DoesNotExist`. STEP 10에서 이 조건 때문에 Pod가 `Pending`으로 돌아갑니다.
3. 셀렉터 여러 개를 콤마로 이으면 AND인가 OR인가? → AND. `app=quote,rel=canary`는 둘 다 만족하는 Pod만 남깁니다. 반면 nodeAffinity의 `nodeSelectorTerms`는 term 사이가 OR, 한 term 안 `matchExpressions`는 AND입니다.

실습 근거: `pod-nodeaffinity.yaml`이 `disk In (ssd,nvme)` + `skip-me DoesNotExist`로 이 둘을 한 번에 재현합니다. STEP 9(매칭 → Running)와 STEP 10(skip-me 붙이면 Pending)의 대비로 눈에 익힙니다.

## 2026-07-14 — FailedScheduling 사유 문자열

k8s v1.35 실측에서 스케줄 실패 사유는 `didn't match nodeSelector`가 아니라 **`didn't match Pod's node affinity/selector`**입니다(nodeSelector·nodeAffinity가 같은 메시지를 씁니다). control-plane 노드는 taint로 따로 빠져 `0/3 nodes are available`이 됩니다. 사유는 `kubectl describe`보다 `kubectl get events --field-selector involvedObject.name=<pod>`가 한 줄로 깔끔합니다.
