# 07-labels — label·label selector·노드 스케줄링 실습

학습 노트 [`07-02.label과 label selector로 오브젝트 조직하기`](../../runners-high/write/08_cloud/book/kubernetes-in-action/07-02.label과%20label%20selector로%20오브젝트%20조직하기.md)(《Kubernetes in Action, 2판》 Ch7 §7.2~7.3)에 대응하는 실행 코드입니다. label을 붙이고 selector로 부분집합을 골라내는 흐름, 그리고 같은 selector 메커니즘이 노드 스케줄링(nodeSelector·nodeAffinity)에 재사용되는 것을 손으로 재현합니다.

## 무엇을 확인하나

- label은 **한 Pod에 여러 축(app·rel)을 동시에** 달아, 어느 축으로든 가로질러 뽑게 한다(§1~2).
- selector는 **equality-based**(`app=quote`)와 **set-based**(`app in (…)`·`!rel`) 두 종류이고, **콤마가 AND**다(§4).
- `nodeSelector`는 **등호만** 되지만, `nodeAffinity`는 **set-based까지** 표현해 `nodeSelector`가 못 하는 "여러 값 중 하나"·"특정 라벨 없는 노드 제외"를 한다(§5).

## 파일

- `pods-labeled.yaml` — app·rel 두 라벨을 단 Pod 4개(kiada/quote × stable/canary). 2차원 조직 재현.
- `pod-nodeselector.yaml` — `disk=ssd` 노드에만 가는 Pod. 매칭 노드가 없으면 `Pending`.
- `pod-nodeaffinity.yaml` — `disk in (ssd,nvme)` + `skip-me` 없는 노드. set-based 재현.
- `canary-service.yaml` — Service + stable 2·canary 1 Pod. selector 하나로 트래픽이 비율 분배되는 canary 뼈대(STEP 11).
- `run-labels-lab.sh` — STEP 1~11을 하나씩 복사해 실행하는 러너(통째 실행 금지).

## 실행

```bash
# 전제: kind 클러스터(k8s-lab, control-plane 1 + worker 2)가 떠 있어야 한다
kubectl config use-context kind-k8s-lab

# run-labels-lab.sh 의 STEP을 하나씩 복사해 실행하며 출력을 노트 §과 대조
```

## 관찰 포인트 (노트 § 대조)

| STEP | 무엇을 보나 | 대응 § |
|------|------------|--------|
| 3 | `--overwrite` 없이 기존 라벨 변경 시 **거부**되는 것 | §2 |
| 4 | 콤마(`app=quote,rel=canary`)가 **AND**로 걸려 1개만 남는 것 | §4 |
| 5 | `!rel`이 셸 해석을 막으려 **작은따옴표** 필수인 것 | §4 |
| 6 | `kubectl delete -l`이 **확인 없이** canary 2개를 지우는 것 | §4 |
| 7 | 매칭 노드 없으면 `Pending` + 이벤트에 스케줄 실패 사유 | §5 |
| 8 | worker에 라벨 붙이는 순간 **자동 스케줄**되는 것 | §5 |
| 10 | `DoesNotExist` 조건에 걸려 `Pending`으로 되돌아가는 것 | §5 |
| 11 | Service selector 하나로 stable·canary에 트래픽이 **비율 분배**되는 것 | §Spring |

## 실제 출력 (kind k8s-lab, v1.35.0 실측)

아래는 위 STEP을 실제로 돌린 결과입니다. 실습을 다시 하지 않고도 대조할 수 있게 핵심 전환만 남깁니다.

STEP 3 — `--overwrite` 없이 기존 라벨 값을 바꾸려 하면 거부됩니다.

```
$ kubectl label pod kiada-stable -n labels-lab rel=canary
error: 'rel' already has a value (stable), and --overwrite is false
```

STEP 4 — 콤마는 AND라, 두 조건을 다 만족하는 Pod 하나만 남습니다.

```
$ kubectl get pods -n labels-lab -l 'app=quote,rel=canary'
NAME           READY   STATUS    RESTARTS   AGE
quote-canary   1/1     Running   0          61s
```

STEP 6 — 셀렉터 삭제는 확인을 묻지 않고 canary 두 개를 한 번에 지웁니다.

```
$ kubectl delete pods -n labels-lab -l rel=canary
pod "kiada-canary" deleted from labels-lab namespace
pod "quote-canary" deleted from labels-lab namespace
```

STEP 7 — `disk=ssd` 노드가 없어 `Pending`에 머뭅니다. 사유는 이벤트에서 봅니다.

```
$ kubectl get pod pod-on-ssd -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP       NODE     NOMINATED NODE   READINESS GATES
pod-on-ssd   0/1     Pending   0          4s    <none>   <none>   <none>           <none>

$ kubectl get events --field-selector involvedObject.name=pod-on-ssd
LAST SEEN   TYPE      REASON             OBJECT           MESSAGE
4s          Warning   FailedScheduling   pod/pod-on-ssd   0/3 nodes are available: 1 node(s) had untolerated taint(s), 2 node(s) didn't match Pod's node affinity/selector. ...
```

사유 문자열은 `didn't match Pod's node affinity/selector`입니다(nodeSelector·nodeAffinity 둘 다 이 메시지를 씁니다). 노드 3개 중 worker 2개는 라벨 불일치로 제외되고, control-plane 1개는 taint 때문에 애초에 일반 Pod를 받지 않아, 합쳐서 `0/3`이 됩니다.

STEP 8 — worker에 라벨을 붙이는 순간 스케줄러가 같은 Pod를 그 노드에 배치합니다.

```
$ kubectl label node k8s-lab-worker disk=ssd
node/k8s-lab-worker labeled

$ kubectl get pod pod-on-ssd -o wide
NAME         READY   STATUS    RESTARTS   AGE   IP            NODE             NOMINATED NODE   READINESS GATES
pod-on-ssd   1/1     Running   0          22s   10.244.1.47   k8s-lab-worker   <none>           <none>
```

STEP 10 — 그 노드에 `skip-me` 라벨을 붙이면 `DoesNotExist` 조건에 걸려 다시 `Pending`이 됩니다.

```
$ kubectl label node k8s-lab-worker skip-me=true
node/k8s-lab-worker labeled

$ kubectl get pod pod-affinity -o wide
NAME           READY   STATUS    RESTARTS   AGE   IP       NODE     NOMINATED NODE   READINESS GATES
pod-affinity   0/1     Pending   0          6s    <none>   <none>   <none>           <none>
```

STEP 11 — Service selector를 `app=kiada` 하나로만 잡으면 stable 2개와 canary 1개가 같은 endpoint 풀에 들어갑니다. curl 90회를 돌리면 트래픽이 endpoint 수 비율대로 나뉘어, canary가 약 1/3을 받습니다.

```
$ kubectl get endpoints kiada -n labels-lab
NAME    ENDPOINTS                                            AGE
kiada   10.244.1.49:5678,10.244.2.46:5678,10.244.2.47:5678   8s

$ # curl 90회 → 응답 텍스트(STABLE/CANARY) 집계
  65 STABLE
  25 CANARY
```

기대값은 STABLE 60 : CANARY 30입니다(endpoint 3개 균등 분배라 stable이 2배). 표본이 작으면 튀므로 30회보다 90회 이상이 안정적입니다.

문제가 생기면 `kubectl delete pods -l app=kiada,rel=canary` 한 줄로 canary만 걷어냅니다. Service selector는 `app=kiada` 그대로 두므로 stable 트래픽은 끊기지 않습니다.
