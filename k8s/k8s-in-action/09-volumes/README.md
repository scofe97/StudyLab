# 09-volumes — 볼륨과 emptyDir로 데이터 보존하기

《Kubernetes in Action, 2판》 Ch9 §9.1~9.2 실습입니다. 학습 노트는
`runners-high/write/08_cloud/book/kubernetes-in-action/09-01.볼륨 이해와 emptyDir로 데이터 보존하기.md`
에 있고, 이 폴더는 그 노트를 **kind 클러스터에서 실제로 돌려 확인**하는 코드입니다.

## 무엇을 확인하는가

Quiz 서비스(`quiz-api` + `mongo`)를 한 파드에 올려, MongoDB 컨테이너가 재시작될 때
데이터가 어떻게 되는지를 세 단계로 관측합니다. 핵심 질문은 하나입니다 —
**"컨테이너 재시작을 넘어 데이터가 살아남는가?"**

| 단계 | 폴더 | 무엇을 보나 |
|------|------|-------------|
| ① 볼륨 없음 | `01-novolume/` | mongo 재시작 → 넣은 질문이 **사라짐**(RESTARTS↑, count 0) |
| ② emptyDir | `02-emptydir/` | mongo 재시작 → 질문이 **보존됨**(RESTARTS↑, count 1) |
| ③ init 초기화 | `03-initdb/` | init 컨테이너가 초기 데이터를 볼륨에 미리 넣어 파드가 뜰 때 자동 채움 |

①과 ②의 차이는 매니페스트 딱 두 줄(`spec.volumes` + `volumeMounts`)입니다. 그 두 줄이
데이터 생존을 가르는 것을 눈으로 보는 게 이 실습의 전부입니다.

## 전제

- kind 클러스터 `k8s-lab`이 떠 있어야 합니다(`cluster/setup-cluster.sh create` 또는 재사용).
- 이미지 `luksa/quiz-api:0.1`·`mongo`는 공개 이미지라 kind가 자동으로 pull 합니다.
  (기존 kiada 실습과 달리 로컬 빌드·`kind load`가 필요 없습니다.)

## 정리

```bash
kubectl delete pod quiz --ignore-not-found   # 단계마다 파드 이름이 quiz 로 같으니 다음 단계 전에 지움
```

> ⚠️ 세 단계 모두 파드 이름이 `quiz` 로 같습니다. 다음 단계로 넘어가기 전에 반드시 이전 파드를
> 지워야 이름 충돌이 안 납니다. `emptyDir`는 파드 삭제 시 데이터도 함께 사라지므로,
> ②→③ 사이에 지우면 데이터가 초기화되는 것도 같이 관찰할 수 있습니다.
