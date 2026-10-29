# 07-fields — field selector와 annotation 실습

학습 노트 [`07-03.field selector와 annotation`](../../runners-high/write/08_cloud/book/kubernetes-in-action/07-03.field selector와 annotation.md)(《Kubernetes in Action, 2판》 Ch7 §7.4~7.5)에 대응하는 실행 코드입니다. label selector의 빈틈을 메우는 두 장치 — 라벨 아닌 필드로 거르는 field selector, 라벨에 못 담는 정보를 붙이는 annotation — 을 손으로 재현합니다.

## 무엇을 확인하나

- field selector는 **라벨이 아닌 오브젝트 필드**(`spec.nodeName`·`status.phase`)로 거른다. 노드 배치는 스케줄러가 `spec.nodeName`에 써넣으므로 라벨이 없어도 필터된다(§1).
- field selector는 **아무 필드나 되지 않는다** — 지원 안 하는 필드는 kubectl이 에러로 알려준다(§1).
- annotation은 **selector로 못 거른다** — 붙여두고 `describe`/`jq`로 읽기만 한다. 이것이 label과의 결정적 차이(§2).
- YAML에서 `yes`·`3`처럼 불리언/숫자로 보이는 annotation 값은 **따옴표로 문자열 강제**해야 apply된다(§3).

## 파일

- `pods-for-fieldselector.yaml` — fields-lab namespace에 Running 2개 + Pending 1개(존재하지 않는 노드 라벨 요구). `status.phase!=Running` 대상 재현.
- `pod-annotations-bad.yaml` — 따옴표 없는 `managed: yes`·`revision: 3`. **일부러 apply가 거부되는** 함정 재현용.
- `pod-annotations.yaml` — 따옴표로 문자열 강제한 성공 버전. 공백·특수문자 든 값 담기.
- `run-fields-lab.sh` — STEP 1~9를 하나씩 복사해 실행하는 러너(통째 실행 금지).

## 실행

```bash
kubectl config use-context kind-k8s-lab
# run-fields-lab.sh 의 STEP을 하나씩 복사해 실행하며 출력을 노트 §과 대조
```

## 관찰 포인트 (노트 § 대조)

| STEP | 무엇을 보나 | 대응 § |
|------|------------|--------|
| 1 | Pod 생성 시 `spec.nodeName`이 스케줄러에 의해 채워지는 것(NODE 열) | §1 |
| 2 | `spec.nodeName=` 으로 특정 노드의 Pod만 걸러지는 것 | §1 |
| 3 | `status.phase!=Running` 으로 Pending Pod만 잡히는 것 | §1 |
| 4 | 지원 안 하는 필드로 필터 시 **에러로 알려주는** 것 | §1 |
| 5 | 따옴표 없는 `yes`·`3`이 **apply 거부**되는 것 | §3 |
| 7 | annotation은 `get` 열에 없고 `describe`/`jq`로만 보이는 것 | §3 |
| 8 | annotation은 label과 달리 **selector로 못 걸리는** 것 | §2 |
