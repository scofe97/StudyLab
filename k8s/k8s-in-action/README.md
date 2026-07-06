# k8s-in-action — Kubernetes in Action 실습

《Kubernetes in Action, 2판》(Marko Lukša, Manning)을 따라가며 손으로 만든 실습 코드 저장소입니다. 책의 예제 애플리케이션 **Kiada**(Kubernetes in Action Demo Application)를 Docker로 빌드·실행·배포하고, 이후 쿠버네티스로 확장하는 과정을 기록합니다.

학습 노트(개념 정리)는 별도 트리(`runners-high/write/08_cloud/book/kubernetes-in-action/`)에 있고, 이 저장소는 그 노트에 대응하는 **실제로 돌아가는 코드**를 담습니다.

## 구성

| 경로 | 내용 |
|------|------|
| `kiada-0.1/` | Ch2 §2.2 — Kiada 첫 버전. Node.js 웹 앱 + Dockerfile |

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

## 출처

- 《Kubernetes in Action, Second Edition》(Marko Lukša, Manning, 2025)
- 예제 원본: [luksa/kubernetes-in-action-2nd-edition](https://github.com/luksa/kubernetes-in-action-2nd-edition)
