# 08-configmap — ConfigMap 실습

학습 노트 `08-02.ConfigMap으로 설정 분리하기`에 대응하는 실습입니다. 같은 이미지에 ConfigMap을 다르게 주입하고, 환경변수와 볼륨의 갱신 시점이 어떻게 다른지 확인합니다.

## 무엇을 확인하나

- `--from-literal`, `--from-file`, `--from-env-file`이 만드는 키 구조가 다릅니다.
- `configMapKeyRef`는 한 키를 선택하고, `envFrom`은 모든 키를 가져옵니다.
- 필수 ConfigMap이 없으면 컨테이너가 `CreateContainerConfigError`로 기다립니다.
- ConfigMap 환경변수는 기존 Pod에서 바뀌지 않아 Pod를 롤링 교체해야 합니다.
- ConfigMap 볼륨은 나중에 갱신되지만 `subPath` 마운트는 갱신되지 않습니다.
- immutable ConfigMap은 수정할 수 없으므로 새 이름으로 교체합니다.

## 파일

| 파일 | 역할 |
|------|------|
| `application.env` | `--from-env-file` 입력 파일 |
| `application.yml` | `--from-file` 입력 파일 |
| `namespace.yaml` | 실습 리소스를 격리하는 namespace |
| `configmaps.yaml` | 환경변수용·파일용 ConfigMap |
| `pod-injection.yaml` | `envFrom`과 `configMapKeyRef` 비교 |
| `pod-missing.yaml` | 필수 참조 실패와 선택 참조 비교 |
| `deployment-env.yaml` | 기존 Pod와 새 Pod의 환경변수 비교 |
| `pod-volume.yaml` | 일반 볼륨과 `subPath` 갱신 비교 |
| `immutable-configmaps.yaml` | 변경할 수 없는 설정의 버전 교체 예시 |

## 실습 명령과 해설

아래 명령은 저장소 루트 `k8s_in_action/`에서 실행합니다. 각 묶음의 예상 결과를 먼저 생각한 뒤 실제 출력과 대조합니다.

### 1. 로컬 클러스터와 namespace 준비

```bash
# 운영 클러스터에 실습 리소스를 만들지 않도록 로컬 kind 컨텍스트로 전환한다.
kubectl config use-context kind-k8s-lab

# 반드시 kind-k8s-lab이 출력되는지 확인한다.
kubectl config current-context

# 실습 리소스를 격리할 namespace를 생성한다.
kubectl apply -f 08-configuring-apps/configmap/namespace.yaml
```

### 2. 생성 방식별 data 구조 비교

```bash
# --dry-run=client는 API 서버에 저장하지 않고 로컬에서 결과만 만든다.
# -o yaml은 만들어질 오브젝트를 YAML로 출력한다.
kubectl create configmap literal-preview \
  --from-literal=LOG_LEVEL=debug \
  --dry-run=client -o yaml

# 파일명 application.yml 하나가 키가 되고 파일 전체가 값이 된다.
kubectl create configmap file-preview \
  --from-file=08-configuring-apps/configmap/application.yml \
  --dry-run=client -o yaml

# application.env의 각 KEY=value 줄이 별도 키가 된다.
kubectl create configmap env-preview \
  --from-env-file=08-configuring-apps/configmap/application.env \
  --dry-run=client -o yaml
```

첫 결과에는 `data.LOG_LEVEL`, 둘째에는 `data.application.yml`, 셋째에는 `APP_MODE`, `LOG_LEVEL`, `SERVER_PORT`가 각각 별도 키로 나타납니다.

### 3. 한 키 선택과 전체 키 주입

```bash
# sed는 파일 내용을 가공하는 명령이다. -n은 기본 출력을 끄고 1~180행만 출력한다.
sed -n '1,180p' 08-configuring-apps/configmap/configmaps.yaml
sed -n '1,180p' 08-configuring-apps/configmap/pod-injection.yaml

kubectl apply -f 08-configuring-apps/configmap/configmaps.yaml
kubectl apply -f 08-configuring-apps/configmap/pod-injection.yaml
kubectl wait --for=condition=Ready pod/configmap-injection \
  -n learning-configmap --timeout=60s

kubectl logs configmap-injection -n learning-configmap
kubectl exec configmap-injection -n learning-configmap -- \
  printenv APP_MODE LOG_LEVEL SERVER_PORT HTTP_PORT
```

`APP_MODE`, `LOG_LEVEL`, `SERVER_PORT`는 `envFrom`이 한 번에 주입합니다. `HTTP_PORT`는 `configMapKeyRef`가 `SERVER_PORT` 키 하나를 선택해 다른 환경변수 이름으로 주입합니다.

### 4. 필수 참조와 optional 참조

```bash
kubectl apply -f 08-configuring-apps/configmap/pod-missing.yaml

# Pod 생성은 비동기다. 생성 직후의 ContainerCreating 대신 안정된 상태를 보도록 잠시 기다린다.
sleep 5

# 두 Pod의 전체 상태와 각 컨테이너의 준비 상태를 확인한다.
kubectl get pod configmap-required configmap-optional \
  -n learning-configmap

# describe는 Events까지 보여 주므로 컨테이너가 시작하지 못한 원인을 찾을 때 쓴다.
kubectl describe pod configmap-required -n learning-configmap

# jsonpath는 전체 JSON에서 컨테이너 이름과 상태만 골라 출력한다.
kubectl get pod configmap-required -n learning-configmap \
  -o jsonpath='{range .status.containerStatuses[*]}{.name}{" => "}{.state}{"\n"}{end}'

kubectl wait --for=condition=Ready pod/configmap-optional \
  -n learning-configmap --timeout=60s
kubectl logs configmap-optional -n learning-configmap
```

필수 참조를 가진 `required` 컨테이너는 `CreateContainerConfigError`로 기다리지만, 같은 Pod의 `independent` 컨테이너는 실행됩니다. optional Pod는 실행되며 `MISSING_VALUE`가 존재하지 않아 `printenv_exit=1`을 출력합니다.

### 5. 환경변수 갱신과 롤링 교체

```bash
kubectl apply -f 08-configuring-apps/configmap/deployment-env.yaml
kubectl rollout status deployment/configmap-rollout \
  -n learning-configmap --timeout=60s

# -l은 label selector다. 해당 label을 가진 모든 Pod의 시작 로그를 모아 본다.
kubectl logs -l app=configmap-rollout -n learning-configmap --prefix=true

# merge patch로 ConfigMap의 LOG_LEVEL만 info에서 debug로 바꾼다.
kubectl patch configmap app-config -n learning-configmap \
  --type merge -p '{"data":{"LOG_LEVEL":"debug"}}'

# 기존 2개는 그대로 두고 Pod 하나를 새로 만들어 설정 혼재를 재현한다.
kubectl scale deployment/configmap-rollout --replicas=3 \
  -n learning-configmap
kubectl rollout status deployment/configmap-rollout \
  -n learning-configmap --timeout=60s
kubectl logs -l app=configmap-rollout -n learning-configmap --prefix=true
```

기존 Pod 둘은 `info`, 새 Pod 하나는 `debug`를 출력합니다. 다음 묶음으로 세 Pod를 모두 새 설정으로 통일합니다.

```bash
# Deployment의 Pod 템플릿에 재시작 annotation을 추가해 롤링 교체를 시작한다.
kubectl rollout restart deployment/configmap-rollout \
  -n learning-configmap
kubectl rollout status deployment/configmap-rollout \
  -n learning-configmap --timeout=60s
kubectl logs -l app=configmap-rollout -n learning-configmap --prefix=true
```

새로 만들어진 세 Pod가 모두 `LOG_LEVEL=debug`를 출력해야 합니다.

### 6. 볼륨 갱신과 subPath 예외

```bash
kubectl apply -f 08-configuring-apps/configmap/pod-volume.yaml
kubectl wait --for=condition=Ready pod/configmap-volume \
  -n learning-configmap --timeout=60s

kubectl exec configmap-volume -n learning-configmap -- \
  cat /config/live/application.conf
kubectl exec configmap-volume -n learning-configmap -- \
  cat /config/frozen/application.conf

kubectl patch configmap file-config -n learning-configmap \
  --type merge -p '{"data":{"application.conf":"message=after\nfeature.enabled=true\n"}}'
```

ConfigMap 볼륨 갱신은 kubelet 동기화 주기에 따라 시간이 걸릴 수 있습니다. 다음 명령을 잠시 뒤 다시 실행해 두 파일을 비교합니다.

```bash
kubectl exec configmap-volume -n learning-configmap -- \
  cat /config/live/application.conf
kubectl exec configmap-volume -n learning-configmap -- \
  cat /config/frozen/application.conf
```

일반 마운트는 `message=after`로 바뀌지만 `subPath`로 마운트한 파일은 `message=before`에 머뭅니다. 일반 파일이 아직 이전 값이면 kubelet 동기화 전이므로 조금 기다린 뒤 다시 확인합니다.

### 7. immutable ConfigMap 교체

```bash
kubectl apply -f 08-configuring-apps/configmap/immutable-configmaps.yaml

# immutable-config-v1의 data 변경을 시도한다. API 서버가 오류로 거부해야 정상이다.
kubectl patch configmap immutable-config-v1 -n learning-configmap \
  --type merge -p '{"data":{"LOG_LEVEL":"debug"}}'

# v1을 고치는 대신 새 이름으로 만든 v2의 값을 확인한다.
kubectl get configmap immutable-config-v1 immutable-config-v2 \
  -n learning-configmap \
  -o custom-columns='NAME:.metadata.name,IMMUTABLE:.immutable,LOG_LEVEL:.data.LOG_LEVEL'
```

실제 워크로드에서는 ConfigMap 이름을 `immutable-config-v1`에서 `immutable-config-v2`로 변경해 Pod 템플릿을 갱신합니다. 이름 변경이 Deployment의 롤링 업데이트를 일으킵니다.

### 8. 실습 리소스 정리

```bash
# namespace를 삭제하면 안의 ConfigMap, Pod, Deployment도 함께 삭제된다.
kubectl delete namespace learning-configmap

# 아무것도 출력되지 않으면 정리가 끝난 것이다.
kubectl get namespace learning-configmap --ignore-not-found
```
