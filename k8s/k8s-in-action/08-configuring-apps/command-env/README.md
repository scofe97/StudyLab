# 08-config — command·args와 환경변수 실습

학습 노트 [`08-01.command·args와 환경변수`](../../runners-high/write/08_cloud/book/kubernetes-in-action/08-01.command%C2%B7args%EC%99%80%20%ED%99%98%EA%B2%BD%EB%B3%80%EC%88%98.md)에 대응하는 실행 코드입니다. 이미지의 ENTRYPOINT·CMD·ENV와 Pod의 command·args·env가 어느 시점에 결합되는지 실제 프로세스와 로그로 확인합니다.

## 무엇을 확인하나

- `args`만 지정하면 이미지 ENTRYPOINT는 유지되고 CMD만 교체됩니다.
- `$(VAR_NAME)`은 Kubernetes가 컨테이너 시작 전에 확장하며, env 정의 순서가 결과를 바꿉니다.
- `$VAR_NAME`은 컨테이너 실행 중 셸이 실제 환경을 보고 확장합니다.
- `exec`가 없으면 셸이 PID 1이고, 있으면 대상 프로세스가 PID 1을 이어받습니다.

## 파일

| 파일 | 역할 |
|------|------|
| `image/` | ENTRYPOINT·CMD·ENV가 모두 있는 재현용 이미지 |
| `namespace.yaml` | 실습 리소스를 격리하는 namespace |
| `pod-args-only.yaml` | command 생략 + args 재정의 |
| `pod-env-expansion.yaml` | 앞·뒤·이미지 환경변수와 이스케이프 비교 |
| `pod-shell-no-exec.yaml` | 셸이 PID 1로 남는 사례 |
| `pod-shell-exec.yaml` | `exec`로 sleep이 PID 1을 이어받는 사례 |

## 실행

대화형 학습 세션에서 관련 명령을 묶음 단위로 실행합니다. 각 묶음의 결과를 먼저 예상하고 실제 출력과 대조한 뒤 다음 단계로 넘어갑니다. 실습이 끝나면 `learning-command-args` namespace 전체를 삭제합니다.

## 실습 명령과 해설

### 1. 클러스터와 이미지 준비

```bash
# 실습 리소스를 실제 운영·공유 클러스터에 만들지 않도록 로컬 kind 컨텍스트로 전환한다.
kubectl config use-context kind-k8s-lab

# 출력이 kind-k8s-lab인지 확인한다. 다른 이름이면 이후 apply 명령을 실행하지 않는다.
kubectl config current-context

# 클러스터 노드의 상태와 Kubernetes 버전을 확인한다.
kubectl get nodes

# 08-configuring-apps/command-env/image/Dockerfile로 실습 이미지를 빌드하고 이름과 태그를 붙인다.
docker build -t command-env-lab:0.1 ./08-configuring-apps/command-env/image

# 이미지 전체 JSON 대신 ENTRYPOINT·CMD·ENV 필드만 골라 출력한다.
docker image inspect command-env-lab:0.1 \
  --format 'ENTRYPOINT={{json .Config.Entrypoint}} CMD={{json .Config.Cmd}} ENV={{json .Config.Env}}'

# 로컬 Docker 이미지를 kind 클러스터의 모든 노드에 복사한다.
kind load docker-image command-env-lab:0.1 --name k8s-lab

# 실습 리소스를 격리할 namespace를 생성한다.
kubectl apply -f 08-configuring-apps/command-env/namespace.yaml
```

### 2. args만 덮어쓰기

```bash
# sed는 파일 내용을 가공하는 도구다. -n은 기본 출력을 끄고, 1,160p는 1~160행만 출력한다.
sed -n '1,160p' 08-configuring-apps/command-env/pod-args-only.yaml

kubectl apply -f 08-configuring-apps/command-env/pod-args-only.yaml
kubectl wait --for=condition=Ready pod/args-only \
  -n learning-command-args --timeout=60s
kubectl logs args-only -n learning-command-args

# jsonpath로 Pod 전체 YAML 중 command와 args만 골라 출력한다.
kubectl get pod args-only -n learning-command-args \
  -o jsonpath='command={.spec.containers[0].command}{"\n"}args={.spec.containers[0].args}{"\n"}'
```

### 3. Kubernetes 확장과 셸 확장

```bash
sed -n '1,200p' 08-configuring-apps/command-env/pod-env-expansion.yaml
kubectl apply -f 08-configuring-apps/command-env/pod-env-expansion.yaml
kubectl wait --for=condition=Ready pod/env-expansion \
  -n learning-command-args --timeout=60s
kubectl logs env-expansion -n learning-command-args

# printenv에 변수 이름을 넘겨 컨테이너의 실제 환경값을 확인한다.
kubectl exec env-expansion -n learning-command-args -- \
  printenv EARLY MESSAGE LATE IMAGE_ONLY

# 존재하지 않는 변수는 빈 값으로 존재하는 것이 아니다. printenv가 실패하고 종료 코드 1을 반환한다.
# $?는 바로 앞에서 실행한 명령의 종료 코드를 담는 셸 특수 변수다.
kubectl exec env-expansion -n learning-command-args -- \
  sh -c 'printenv UNKNOWN; printf "exit=%s\n" "$?"'
```

`UNKNOWN` 값은 출력되지 않고 `exit=1`만 출력됩니다. 여기서 `1`은 환경변수 값이 아니라 `printenv UNKNOWN`의 실패 종료 코드입니다. 변수가 존재해 값을 정상 출력하면 종료 코드는 `0`입니다.

### 4. exec 전후 PID 1 비교

```bash
# grep은 패턴이 있는 행을 찾는다. -n은 행 번호를 붙이고, *는 셸이 두 YAML 파일명으로 확장한다.
grep -n 'args:' 08-configuring-apps/command-env/pod-shell-*.yaml

kubectl apply -f 08-configuring-apps/command-env/pod-shell-no-exec.yaml
kubectl apply -f 08-configuring-apps/command-env/pod-shell-exec.yaml
kubectl wait --for=condition=Ready pod/shell-no-exec pod/shell-exec \
  -n learning-command-args --timeout=60s

# /proc/1/cmdline은 PID 1의 명령과 인자를 NUL 문자로 구분해 저장한다.
# tr은 NUL을 공백으로 바꿔 터미널에서 읽을 수 있게 한다.
kubectl exec shell-no-exec -n learning-command-args -- \
  sh -c 'printf "without exec: "; tr "\000" " " </proc/1/cmdline; echo'
kubectl exec shell-exec -n learning-command-args -- \
  sh -c 'printf "with exec:    "; tr "\000" " " </proc/1/cmdline; echo'

# ps로 PID·부모 PID(PPID)·명령을 함께 확인한다.
kubectl exec shell-no-exec -n learning-command-args -- \
  ps -o pid,ppid,comm,args
kubectl exec shell-exec -n learning-command-args -- \
  ps -o pid,ppid,comm,args
```

no-exec Pod에서는 PID 1인 `sh` 아래에 `sleep`이 자식으로 보입니다. exec Pod에서는 셸이 교체되어 `sleep`이 PID 1입니다. `kubectl exec`로 추가 실행한 `ps`는 컨테이너 런타임이 별도로 시작한 프로세스이므로, 컨테이너 PID namespace 안에서 부모가 보이지 않아 PPID가 0으로 표시될 수 있습니다.

### 5. 실습 리소스 정리

```bash
# namespace를 삭제하면 그 안의 네 Pod도 함께 삭제된다.
kubectl delete namespace learning-command-args

# 이름이 조회되지 않으면 정리가 완료된 것이다.
kubectl get namespace learning-command-args --ignore-not-found
```

Docker 이미지까지 지우려면 다음 명령을 추가로 실행합니다. 이후 다시 실습하려면 이미지를 빌드하고 kind 노드에 다시 로드해야 합니다.

```bash
docker image rm command-env-lab:0.1
```
