# k8s-in-action — Kubernetes in Action 실습

《Kubernetes in Action, 2판》(Marko Lukša, Manning)을 따라가며 손으로 만든 실습 코드 저장소입니다. 책의 예제 애플리케이션 **Kiada**(Kubernetes in Action Demo Application)를 Docker로 빌드·실행·배포하고, 이후 쿠버네티스로 확장하는 과정을 기록합니다.

학습 노트(개념 정리)는 별도 트리(`runners-high/write/08_cloud/book/kubernetes-in-action/`)에 있고, 이 저장소는 그 노트에 대응하는 **실제로 돌아가는 코드**를 담습니다.

## 구성

| 경로 | 내용 |
|------|------|
| `kiada-0.1/` | Ch2 §2.2 — Kiada 첫 버전. Node.js 웹 앱 + Dockerfile |
| `03-deploy-scale/` | Ch3 §3.2 — kiada 이미지를 kind 클러스터에 배포·노출·스케일. 실습 명령 스크립트 |

## kiada-0.1 — 첫 컨테이너

애플리케이션 버전·서버 호스트명·클라이언트 IP를 보여 주는 최소 웹 앱입니다. HTML 모드(`/html`)와 평문 모드(그 외 경로) 두 가지로 응답합니다.

```
kiada-0.1/
├── app.js            # Node.js HTTP 서버 (포트 8080)
├── html/index.html   # HTML 모드 정적 페이지
└── Dockerfile        # FROM node:23-alpine → COPY → ENTRYPOINT
```

### 빌드와 실행

```bash
cd kiada-0.1

# 1. 이미지 빌드 (-t 이름:태그, . 은 빌드 컨텍스트)
docker build -t kiada:latest .

# 2. 레이어 확인 — 지시문마다 레이어 하나
docker history kiada:latest

# 3. 컨테이너 실행 (호스트 1234 → 컨테이너 8080, 백그라운드)
docker run --name kiada-container -p 1234:8080 -d kiada

# 4. 응답 확인
curl localhost:1234          # 평문 모드
curl localhost:1234/html     # HTML 모드
# → Kiada version 0.1. Request processed by "<컨테이너ID>". Client IP: <IP>

# 5. 상태·로그
docker ps
docker logs kiada-container
```

### 레지스트리 배포

```bash
# Docker Hub 네이밍으로 재태깅 (yourid = 본인 Docker Hub ID)
docker tag kiada yourid/kiada:0.1
docker login -u yourid docker.io
docker push yourid/kiada:0.1

# 다른 호스트에서 그대로 실행
docker run --name kiada-container -p 1234:8080 -d yourid/kiada:0.1
```

### 생명주기

```bash
docker stop kiada-container    # 정지 (종료 신호 → 응답 없으면 kill)
docker start kiada-container   # 재개
docker rm kiada-container      # 컨테이너 삭제 (이미지는 남음)
docker rmi kiada:latest        # 이미지 삭제
```

## 03-deploy-scale — kind 클러스터에 배포·스케일

`kiada-0.1`에서 만든 이미지를 kind 클러스터(`k8s-lab`, control-plane 1 + worker 2)에 배포하고, Service로 노출한 뒤 3개로 스케일해 로드밸런싱까지 확인합니다. 전 과정은 `03-deploy-scale/deploy-scale.sh`에 주석과 함께 정리했습니다.

```
03-deploy-scale/
└── deploy-scale.sh   # 0.클러스터 확인 → 1.kind load → 2.배포 → 3.expose
                      #  → 4.접속 → 5.scale=3 → 6.로드밸런싱 → 7.정리
```

### 흐름 요약

```bash
# 1. 로컬 이미지를 kind 노드로 (자동으로 안 보이므로 필수)
kind load docker-image kiada:0.1 --name k8s-lab

# 2. 배포 → 3. Service 노출 → 5. 스케일
kubectl create deployment kiada --image=kiada:0.1
kubectl expose deployment kiada --port=8080 --target-port=8080
kubectl scale deployment kiada --replicas=3

# 6. 클러스터 안에서 로드밸런싱 확인 (port-forward 는 우회하므로 X)
kubectl run curl-test --image=curlimages/curl --rm -it --restart=Never -- \
  sh -c 'for i in $(seq 1 30); do curl -s http://kiada:8080/ | grep -oE "kiada-[a-z0-9-]+"; done | sort | uniq -c'
```

### 실습에서 부딪힌 함정 세 가지

| # | 증상 | 원인·해결 |
|---|------|-----------|
| ① | `kind load` 했는데도 `ErrImagePull` | 배포 태그(`:0.1`)와 로드한 태그(`:latest`) 불일치. 배포에 쓸 태그를 정확히 로드 |
| ② | `pull access denied` | `:latest`는 기본 `imagePullPolicy=Always` → 로컬 무시하고 Hub로 감. `:0.1` + `IfNotPresent` 사용 |
| ③ | `curl` 30번이 전부 같은 Pod로 | `port-forward`는 Pod 하나에 직접 터널(로드밸런싱 우회). 클러스터 안에서 ClusterIP로 요청해야 분산됨 |

## 출처

- 《Kubernetes in Action, Second Edition》(Marko Lukša, Manning, 2025)
- 예제 원본: [luksa/kubernetes-in-action-2nd-edition](https://github.com/luksa/kubernetes-in-action-2nd-edition)
