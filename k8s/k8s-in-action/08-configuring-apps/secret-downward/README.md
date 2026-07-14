# 08-secret-downward — Secret과 Downward API 실습

학습 노트 [`08-03.Secret과 Downward API`](../../runners-high/write/08_cloud/book/kubernetes-in-action/08-03.Secret과 Downward API.md)(《Kubernetes in Action, 2판》 Ch8 §8.3~8.4)에 대응하는 실행 코드입니다. 08-02(ConfigMap)에서 배운 key-value 주입의 두 변형 — 값이 민감할 때 쓰는 Secret, 값이 Pod 오브젝트 자신 안에 있을 때 쓰는 Downward API — 를 손으로 재현합니다.

## 무엇을 확인하나

- Secret의 `data`는 **Base64 인코딩일 뿐 암호화가 아니다** — `-o yaml`로 보면 값이 그대로 보이고, 키 없이 `base64 -d`로 디코딩된다(§2·§4-1).
- Secret을 컨테이너에 주입하면 쿠버네티스가 **디코딩해서** 넣으므로, 앱은 원문(`s3cr3t-p@ss`)을 그대로 읽는다(§1 47줄).
- 환경변수 주입 문법은 ConfigMap과 평행하다 — `configMapKeyRef` 자리에 `secretKeyRef`(§3).
- Downward API는 REST 호출이 아니라, Pod의 metadata·spec·status 값을 **env/파일로 주입**하는 방식이다. 단순 필드는 `fieldRef.fieldPath`, 리소스 값은 `resourceFieldRef`+`divisor`(§5~6).

## 파일

- `pod-secret-env.yaml` — **[TODO]** Secret을 `secretKeyRef`로 env 주입. 채울 곳: 참조 키워드 + name/key.
- `pod-downward-env.yaml` — **[TODO]** `fieldRef`로 POD_NAME·POD_IP·NODE_NAME, `resourceFieldRef`+divisor로 메모리 한도 주입. 채울 곳 4군데.
- `run-secret-downward-lab.sh` — STEP 0~5를 하나씩 복사해 실행하는 러너(통째 실행 금지).

## 실행

```bash
kubectl config use-context kind-k8s-lab
# 1) 두 TODO 매니페스트의 빈칸을 먼저 채운다
# 2) run-secret-downward-lab.sh 의 STEP을 하나씩 복사해 실행하며 출력을 노트 §과 대조
```

## 관찰 포인트 (노트 § 대조)

| STEP | 무엇을 보나 | 대응 § |
|------|------------|--------|
| 1 | `kubectl create secret generic`이 타입을 명령 뒤에 지정하는 것 | §2 |
| 2 | `data`의 값이 Base64로 보이고, 키 없이 디코딩되는 것 (**암호화 아님**) | §2·§4-1 |
| 3 | 주입된 env `DB_PASSWORD`가 **평문**으로 나오는 것(주입 시 디코딩) | §1·§3 |
| 4 | `fieldRef`로 채워진 POD_NAME·POD_IP·NODE_NAME, `resourceFieldRef`로 채워진 메모리 한도 | §5~6 |
| 5 | env 값이 `get pod -o wide`의 실제 NAME·IP·NODE와 일치하는 것 | §6 244줄 |
