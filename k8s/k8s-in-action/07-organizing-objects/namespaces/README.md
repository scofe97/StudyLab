# 07-namespaces — namespace 격리와 삭제 실습

학습 노트 [`07-01.namespace로 클러스터를 가상 분할하기`](../../runners-high/write/08_cloud/book/kubernetes-in-action/07-01.namespace로%20클러스터를%20가상%20분할하기.md)(《Kubernetes in Action, 2판》 Ch7 §7.1)에 대응하는 실행 코드입니다. namespace가 무엇을 격리하고 무엇을 격리하지 않는지, 그리고 삭제가 `Terminating`에 멈추는 고전적 장애를 손으로 재현·진단합니다.

## 무엇을 확인하나

- namespace는 **이름 스코프**를 준다 — 같은 이름 Pod가 여러 namespace에 충돌 없이 공존(§1·§4).
- namespace는 **커널·노드를 격리하지 않는다** — 서로 다른 namespace의 Pod가 같은 worker 노드에 얹힌다(§4).
- namespace 삭제가 `Terminating`에 멈추는 진짜 원인은 **finalizer 미제거**이고, `get all`이 아니라 **`status.conditions`**로 진단한다(§5).

## 파일

- `kiada-ssl.yaml` — namespace 미지정 Pod. `-n`을 바꿔가며 여러 namespace에 같은 이름으로 배치.
- `kiada-test2-ns.yaml` — Namespace 매니페스트(선언형 생성).
- `stuck-configmap.yaml` — 컨트롤러 없는 가짜 finalizer가 붙은 ConfigMap. 삭제를 붙잡는 함정 재현용.
- `run-namespace-lab.sh` — STEP 1~8을 하나씩 복사해 실행하는 러너(통째 실행 금지).

## 실행

```bash
# 전제: kind 클러스터
kind create cluster --name k8s-lab --config ../cluster/kind-config.yaml

# run-namespace-lab.sh 의 STEP을 하나씩 복사해 실행하며 출력을 노트 §과 대조
```

## 하이라이트 — finalizer로 Terminating 재현 (STEP 5~7)

책은 이 상황을 "가상 예시(hypothetical)"로만 보여줍니다. 여기서는 컨트롤러가 없는 finalizer(`example.com/pin`)를 붙여 실제로 namespace를 `Terminating`에 붙잡고, `status.conditions`의 `NamespaceFinalizersRemaining`으로 원인을 짚은 뒤, finalizer를 떼어 해소합니다. 이 편에서 가장 값진 실습입니다.
