# Ch14 §1 실측 기록 — ReplicaSet 기초

> 2026-08-12, kind `k8s-lab` (3노드, v1.35.0), 네임스페이스 `ch14-lab`.
> 원서 Chapter14 매니페스트를 이미지만 교체해 재현했습니다(`luksa/kiada:0.5` → `hashicorp/http-echo`).
> 학습 노트: `write/08_cloud/book/kubernetes-in-action/14-01.ReplicaSet 기초 — 생성·소유·스케일링.md`

## 1. 파드 이름은 랜덤이고 소유자는 ReplicaSet입니다

```
POD         OWNER_KIND   OWNER
web-cmhhw   ReplicaSet   web
web-pdmz5   ReplicaSet   web
web-wr4q2   ReplicaSet   web
```

이름이 `<rs이름>-<랜덤 5자>` 형태입니다. 재생성되면 이 뒷부분이 바뀝니다. 16장에서 StatefulSet이 `quiz-0` 같은 ordinal 이름을 쓰는 것과 대비되는 지점이며, 이름이 바뀌기 때문에 PVC나 DNS 이름을 이름으로 되찾을 수 없습니다.

`ownerReferences`가 ReplicaSet을 직접 가리킵니다. Deployment를 함께 쓰면 그 위에 한 겹이 더 생겨 `Deployment → ReplicaSet → Pod` 2단이 됩니다.

## 2. 고아 파드 흡수 — 흡수한 뒤 초과분을 지웁니다

`replicas: 3`이고 파드가 3개인 상태에서, selector와 같은 label을 가진 독립 파드 `one-web-too-many`를 만들었습니다. 8초 뒤 그 파드가 사라졌습니다.

```
Normal  SuccessfulDelete  replicaset-controller  Deleted pod: one-web-too-many
```

컨트롤러가 두 단계를 연달아 수행합니다. 먼저 label이 맞으므로 그 파드를 **흡수**해 소유권을 가져가고, 그 결과 소유 파드가 4개가 되어 `replicas` 를 초과하므로 하나를 **삭제**합니다. 즉 지운 시점에는 넷 다 자기 소유였습니다.

넷 중 하필 새 파드가 지워진 것은 우연이 아닙니다. 컨트롤러의 삭제 우선순위가 Pending → not-Ready → 최근 생성 → 재시작 많은 순인데, 방금 만든 파드가 앞의 세 조건에 모두 걸립니다. 오래 돌며 트래픽을 받던 파드를 남기는 쪽이 안전하기 때문입니다.

## 3. label을 바꾸면 소속에서 빠져나옵니다

소속 파드 하나의 `rel` 을 `stable` 에서 `escaped` 로 바꿨습니다.

```
POD         OWNER    APP   REL
web-cmhhw   <none>   web   escaped   ← 살아남고 소유자만 사라짐
web-pdmz5   web      web   stable
web-wr4q2   web      web   stable
web-ztbxn   web      web   stable    ← 부족분 1개 보충
```

빠져나온 파드는 삭제되지 않고 `ownerReferences` 만 떨어집니다. 이제 아무도 관리하지 않으므로 죽어도 되살아나지 않습니다. ReplicaSet 쪽은 selector에 맞는 파드가 2개로 줄었으니 1개만 새로 만듭니다.

이 성질이 디버깅에 쓰입니다. 문제 있는 파드의 label을 바꿔 selector 밖으로 빼면, 대체 파드가 즉시 떠서 서비스는 정상화되고 문제 파드는 살아남아 조사할 수 있습니다.

## 4. 정리 — 개수만 세는 컨트롤러

두 실험 모두 label 한 줄로 소속이 바뀌었습니다. ReplicaSet은 "내가 만든 파드"를 기억하지 않고, 매 순간 selector에 맞는 것을 세어 개수를 맞춥니다.

| 방향 | 조작 | 결과 |
|---|---|---|
| 흡수 | 고아 파드에 맞는 label 부여 | 소유권 획득 후 초과분 삭제 |
| 탈출 | 소속 파드의 label 변경 | 소유권 해제 후 부족분 보충 |

그래서 넷 중 아무거나 하나를 지워도 목적이 달성됩니다. 어느 개체인지는 상관없습니다. StatefulSet은 이것이 불가능합니다. `quiz-1` 이 없으면 반드시 `quiz-1` 을 만들어야 하고, label을 바꿔 빼내는 방법도 통하지 않습니다. 같은 이름으로 다시 만들려다 이름이 겹쳐 실패하기 때문입니다.

## 실행 중 만난 것

`kubectl rollout status rs/web` 은 동작하지 않습니다.

```
error: no status viewer has been implemented for ReplicaSet.apps
```

`rollout` 하위 명령은 Deployment·StatefulSet·DaemonSet 전용입니다. ReplicaSet은 롤아웃이라는 개념 자체가 없으므로, 준비 상태를 기다리려면 `kubectl wait --for=condition=Ready pod -l app=web` 을 씁니다.
