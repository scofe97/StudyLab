# Ch16 실측 기록 — StatefulSet 전 과정

> 2026-08-12, kind `k8s-lab` (3노드, **v1.35.0**), 네임스페이스 `kiada`.
> 원서 `Chapter16/` 매니페스트를 그대로 사용했습니다(`book/` 하위).
> 학습 노트: `write/08_cloud/book/kubernetes-in-action/16-0{1,2,3}.*.md`

## 확정한 미확인 각주 두 건

학습 노트가 "클러스터가 없어 확인하지 못했다"고 남긴 항목을 이번에 실측으로 닫았습니다.

### 1. PVC 이름 규칙 — `<템플릿명>-<파드명>`

16-01 `[^pvc-name]`은 이 규칙을 nginx 예제에서 **유추한 것**이라고 밝히며, 확정하려면 실제 `kubectl get pvc` 출력이 필요하다고 적었습니다.

```
db-data-quiz-0   Bound   pvc-1e05a58b-...   1Gi   RWO   standard
db-data-quiz-1   Bound   pvc-eed449a0-...   1Gi   RWO   standard
db-data-quiz-2   Bound   pvc-aec4fa52-...   1Gi   RWO   standard
```

`volumeClaimTemplates` 의 name 이 `db-data`, 파드가 `quiz-0` 일 때 PVC 는 `db-data-quiz-0` 입니다. 유추가 맞았고 이제 실측 근거가 있습니다.

### 2. `maxSurge` 는 StatefulSet 에 없습니다

16-03 `[^max-unavailable]`은 개념 문서에 `maxSurge` 가 등장하지 않는다는 것까지만 확인하고, 부재의 확정은 API 레퍼런스나 `kubectl explain` 이 필요하다고 남겼습니다. v1.35 클러스터에서 확인했습니다.

```
$ kubectl explain sts.spec.updateStrategy.rollingUpdate
FIELDS:
  maxUnavailable  <IntOrString>
  partition       <integer>
```

필드가 **둘뿐이고 `maxSurge` 는 없습니다.** 같은 출력이 `maxUnavailable` 에 대해 "This field is beta-level and is enabled by default" 라고 적어, 노트가 인용한 v1.35 beta 표기와도 일치합니다. 덧붙여 "This setting might not be effective for the OrderedReady podManagementPolicy" 라는 단서가 있는데, 노트에는 없는 내용입니다.

## 신원 세 겹의 실증

### 파드를 지워도 이름·PVC 가 유지됩니다

```
$ kubectl delete pod quiz-1
$ kubectl get pods -l app=quiz
quiz-1   2/2   Running   0   20s        ← 같은 이름, AGE 만 새것
$ kubectl get pod quiz-1 -o jsonpath='{...claimName}'
db-data-quiz-1                          ← 같은 PVC
```

Ch14 의 ReplicaSet 과 대비됩니다. 그쪽은 `web-wr4q2` 가 죽으면 `web-2xdfc` 라는 다른 이름으로 떴습니다. 이름이 달라도 되니 대체가 성립했고, StatefulSet 은 같은 이름이어야 하므로 PVC 를 되찾을 수 있습니다.

### 스케일다운은 데이터 삭제가 아니라 일시 정지입니다

3 에서 1 로 줄였다가 다시 3 으로 올렸습니다.

```
스케일다운 후:  quiz-0 만 남음, PVC 는 3개 그대로 Bound
스케일업  후:  quiz-2 가 25초 전 생성, 데이터 6건
```

판별 근거는 **PV UID** 입니다. `db-data-quiz-2` 가 스케일 전후로 같은 `pvc-aec4fa52-...` 를 가리켰습니다. UID 가 같으므로 새 볼륨을 만들어 복제로 채운 것이 아니라 **옛 볼륨을 재부착**한 것입니다.

### 별개 볼륨인데 데이터가 같은 이유는 복제입니다

```
quiz-0: 6    (primary)
quiz-2: 6    (secondary, setReadPref 로 자기 사본을 읽음)
PV UID: 세 개 모두 다름
```

공유가 아닙니다. 볼륨 세 개에 데이터가 **세 벌** 들어 있고, MongoDB 가 oplog 로 전파해 같아진 것입니다. 쿠버네티스는 볼륨을 붙여 준 것까지가 역할이며, 내용을 같게 만든 주체는 MongoDB 입니다.

## 부팅 순환 — publishNotReadyAddresses 가 한 일

StatefulSet 생성 직후 세 파드가 전부 `1/2` 였습니다. mongo 는 떴지만 quiz-api 의 readiness probe 가 MongoDB ping 에 실패하기 때문이며, 원인은 리플리카 셋 미초기화입니다.

이 상태에서 `rs.initiate` 가 성공했습니다.

```
$ kubectl exec quiz-0 -c mongo -- mongosh --eval 'rs.initiate({...})'
{ ok: 1 }
$ kubectl get pods -l app=quiz
quiz-0   2/2   Running                  ← 1/2 에서 올라옴
```

세 파드가 **not-ready 인데도** DNS 이름이 resolve 됐다는 뜻입니다. `svc.quiz-pods.yaml` 의 `publishNotReadyAddresses: true` 가 없었다면 레코드가 없어 `Host not found` 로 실패했을 것이고, 16-01 §3 이 다룬 순환에 빠졌을 것입니다.

`rs.initiate` 자체는 쿠버네티스 명령이 아닙니다. StatefulSet 은 파드를 띄우고 신원을 줄 뿐 "MongoDB 는 이 명령을 한 번 쳐야 한다" 는 지식이 없습니다. 16-03 의 Operator 가 자동화하는 대상이 정확히 이 지점입니다.

## partition 카나리

`partition: 2` 를 걸고 템플릿을 0.2 로 바꿨습니다.

```
quiz-0   quiz-c5cd4d99f   0.1
quiz-1   quiz-c5cd4d99f   0.1
quiz-2   quiz-7fccc5676   0.2   ← 이것만 갈림
```

`CONTROLLER-REVISION-HASH` 가 파드마다 갈립니다. 이 label 값이 곧 ControllerRevision 오브젝트 이름이며, 16-01 에서 본 label 이 실제로 가리키는 대상이 무엇인지 여기서 드러납니다.

### 경계 아래 파드는 삭제해도 옛 버전으로 돌아옵니다

템플릿이 이미 0.2 인 상태에서 `quiz-1` 을 지웠습니다.

```
quiz-1   20s   0.1      ← 방금 만들어졌는데 옛 버전
```

공식 문서의 "even if they are deleted, they will be recreated at the previous version" 이 실증됐습니다. 컨트롤러가 파드별 옛 버전을 기억하는 것이 아니라, 재생성 시점에 ordinal 과 partition 을 비교해(`1 < 2`) 해당 ControllerRevision 에서 템플릿을 꺼내 씁니다.

### partition 0 으로 완료

```
quiz-0   6s    0.2      ← 마지막
quiz-1   24s   0.2
quiz-2   39m   0.2      ← 카나리 때 이미 갈림
```

AGE 를 보면 순서가 나옵니다. `quiz-1`(24초 전) 다음 `quiz-0`(6초 전) 으로 **역순** 입니다. 부트스트랩 패턴에서 ordinal 이 낮을수록 다른 파드가 의존하는 쪽이므로, 가장 덜 위험한 것부터 갈고 quiz-0 을 마지막에 둡니다.

ControllerRevision 은 세 개가 쌓였습니다.

```
quiz-5c47945c46   REVISION 3    ← 0.2 + mongo 복구본
quiz-7fccc5676    REVISION 2    ← 0.2 (mongo 누락본)
quiz-c5cd4d99f    REVISION 1    ← 0.1
```

전면 교체 뒤에도 데이터는 6건 그대로였고 PV UID 도 바뀌지 않았습니다. 컨테이너는 갈아도 데이터는 남는다는 StatefulSet 의 요점이 확인됩니다.

## 실행 중 만난 것

### StatefulSet 은 수정 가능한 필드가 7개뿐입니다

`sts.quiz.0.2.partition.yaml` 을 `apply` 하자 거부됐습니다.

```
Forbidden: updates to statefulset spec for fields other than
'replicas', 'ordinals', 'template', 'updateStrategy',
'revisionHistoryLimit', 'persistentVolumeClaimRetentionPolicy',
'minReadySeconds' are forbidden
```

원인은 그 파일에 `podManagementPolicy` 가 **없다는 것** 이었습니다. 현재 오브젝트는 `Parallel` 인데 파일에 없으니 `apply` 가 기본값 `OrderedReady` 로 되돌리려 했고, 그 필드가 허용 목록 밖이라 거부됐습니다.

`patch` 로 `updateStrategy` 만 건드릴 때는 통과했습니다. 두 명령의 성격이 다릅니다.

| | 대상 | 결과 |
|---|---|---|
| `patch` | 지정한 필드만 | 허용 필드면 통과 |
| `apply` | 파일 전체를 현재와 맞춤 | 금지 필드가 하나라도 다르면 거부 |

`volumeClaimTemplates` 와 `serviceName` 이 불변인 이유는 PVC 이름과 DNS 이름이 거기서 파생되기 때문입니다. 바꾸면 기존 파드가 자기 볼륨을 잃습니다.

### strategic merge patch 로 컨테이너 배열을 다룰 때

`containers` 에 `quiz-api` 만 적어 patch 했더니 `mongo` 컨테이너가 사라져 파드가 `1/1` 이 됐습니다.

```
'quiz-2... (not reachable/healthy)'    ← 리플리카 셋에서 이탈
```

의도치 않은 결과였지만 quorum 을 실물로 보여줬습니다. 3개 중 2개가 살아 있어 quiz-0 이 PRIMARY 를 유지했고 서비스는 계속 돌았습니다. 하나 더 죽었다면 quorum 을 잃어 읽기·쓰기가 모두 멈췄을 것입니다.

배열을 patch 할 때는 유지할 컨테이너도 함께 명시해야 합니다.
