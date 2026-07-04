# 2장 실습 로그 — 실제 사용 명령·출력·막힌 지점

> 이 파일은 실습하며 실제로 친 명령과 겪은 것을 시간순으로 쌓는다. 실습 종료 후 02-01·02-02 정독 노트 보강의 1차 자료. (사용자 요청: "사용한 명령어들 문서에 모두 보강")
> **출처 구분**: [책] = _Book_GitAIOps 가이드 원문 / [명세생성] = 가이드 명세 따라 생성한 코드 / [정리] = 계정·환경 정리용(책 무관)

## 환경 확정 (2026-07-04)

- 계정: `user@example.invalid` (user) — user97은 billing 개설 중단(₩16000 소액인증에서 멈춤)
- 프로젝트: `project-REMOVED` (dev-server 3대 삭제 후 비움)
- billing: `010758-C81075-FA9979` (OPEN, KRW, billingEnabled:true) — 유료 진행
- 리전/존: asia-northeast3 / asia-northeast3-a
- 로컬: gcloud 560.0.0, kubectl v1.34.1, docker 28.4.0, go 1.25.1

## [정리] dev-server 3대 삭제 (책 무관, 사용자 요청)

```bash
# VM 3대 + 부트디스크 삭제
gcloud compute instances delete dev-server dev-server-2 dev-server-3 \
  --zone=asia-northeast3-a --delete-disks=all --quiet
# 고정 IP 3개 삭제
gcloud compute addresses delete dev-server-ip dev-server-2-ip dev-server-3-ip \
  --region=asia-northeast3 --quiet
```
→ 삭제 후 디스크·스냅샷·이미지·LB·GKE·SQL·버킷·BigQuery 전부 0 확인.

## [책 2.3 기반] API 활성화·인증 (실제 실행)

```bash
# 계정·프로젝트·리전 설정
gcloud config set account user@example.invalid
gcloud config set project project-REMOVED
gcloud config set compute/zone asia-northeast3-a
gcloud config set compute/region asia-northeast3

# 실습 API 활성화 (compute는 이미 켜져 있었음)
gcloud services enable container.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com
```
- 겪은 것: 처음 프로젝트엔 compute만 활성. 위 3개 켜니 containerfilesystem·containerregistry도 함께 켜짐.
- ⚠️ billing 관문: billing 안 열린 프로젝트/계정에선 GKE 생성 불가. user97은 billing 소유가 아니라(companybh 소유 billing에 뷰어 권한만) `billing.resourceAssociations.create` 거부됨.

## [책 2.5] GKE 클러스터 생성 (실제 실행 — 진행 중)

```bash
gcloud container clusters create notiflex-cluster \
  --zone=asia-northeast3-a \
  --machine-type=e2-medium \
  --num-nodes=2 \
  --spot \
  --gateway-api=standard \
  --disk-size=30
```
- 책 가이드 2.5 원문 그대로. `--spot`(비용절감, 구글이 선점 가능), `--gateway-api=standard`(5장 대비)는 노트 02-01엔 없던 실전 디테일.
- **결과**: 약 5~7분 소요. STATUS RUNNING, 노드 2개(e2-medium), GKE 1.35.5-gke.1163012, 마스터IP 8.230.20.232.

### ⚠️ 실전 함정 (노트 02-01엔 없음 — 보강 필수)

**함정 1 — get-credentials 후 kubectl이 `gke-gcloud-auth-plugin not found`**
- 최신 kubectl(v1.26+)은 GKE 인증에 별도 플러그인 필요. get-credentials는 되는데 kubectl 명령이 전부 실패.
```bash
gcloud components install gke-gcloud-auth-plugin --quiet
```
**함정 2 — 설치 후에도 PATH에서 못 찾음 (homebrew gcloud)**
- homebrew로 깐 gcloud는 실 바이너리가 `/opt/homebrew/share/google-cloud-sdk/bin/`에 있는데 PATH엔 `/opt/homebrew/bin`(심링크)만 걸려 플러그인 심링크가 없음.
```bash
export PATH="/opt/homebrew/share/google-cloud-sdk/bin:$PATH"
# → 영구 적용하려면 ~/.zshrc에 추가
```
- 해결 후: `kubectl get nodes` → 노드 2개 Ready 확인. context = gke_project-REMOVED-..._notiflex-cluster (02-01 get-credentials 학습 검증됨).

## [명세생성] Notiflex 앱 (책 2.6 명세 따라 생성)

- `app/main.go`: net/http, GET /health(ok), GET /id(atomic 카운터 + HOSTNAME=Pod이름), 포트 8080
- `app/Dockerfile`: 멀티스테이지 golang:1.25-alpine→scratch, CGO_ENABLED=0
- `app/go.mod`: module notiflex, go 1.25
- 출처: 가이드 2.6은 명세만 줌(코드 없음). 사용자 결정 = "명세 따라 직접 생성(책 방식)".

### 로컬 빌드·실행 검증 (Cloud Build 전 사전 검증)

```bash
# 정적 바이너리 빌드
CGO_ENABLED=0 GOOS=linux go build -o /tmp/notiflex-api .
file /tmp/notiflex-api
# → "statically linked" 확인 = scratch에서 동작 가능 (02-02 학습 검증됨)
```
- **겪은 것 (학습 포인트)**: 로컬 8080은 사용자의 Java(Spring) 앱이 이미 점유(`bind: address already in use`) → curl이 엉뚱하게 Spring 404 JSON에 붙음. 우리 코드 문제 아님. 포트 19090으로 바꿔 재검증하니 정상:
  - `/health` → `ok`
  - `/id` x3 → `id=1 pod=local` / `id=2` / `id=3` (인메모리 카운터 순차 증가, HOSTNAME 없으면 pod=local)
- **k8s에선**: 각 Pod가 독립 컨테이너·독립 8080이라 로컬 포트 충돌 무관. HOSTNAME이 Pod 이름으로 주입돼 pod=notiflex-api-xxxxx로 나온다.
- ⚠️ 노트 보강 포인트: "인메모리 카운터"는 Pod마다 독립이라 replicas 2면 카운터가 두 개 → 5장 무중단·6장 Valkey에서 "상태를 밖으로" 빼는 동기가 여기서 체감됨.

## [책 2.6] 이미지 빌드 — Cloud Build 권한 함정 (노트 02-02 보강 필수)

```bash
# Artifact Registry repo 생성 (성공)
gcloud artifacts repositories create notiflex --repository-format=docker --location=asia-northeast3
# Cloud Build 빌드·푸시 시도
gcloud builds submit app/ --tag=asia-northeast3-docker.pkg.dev/<PROJECT>/notiflex/api:v0.1.0
```
- ⚠️ **함정**: 처음 빌드가 403 실패 —
  `<PROJECT_NUMBER>-compute@developer.gserviceaccount.com does not have storage.objects.get access ... _cloudbuild bucket`
- **원인**: 프로젝트에서 Cloud Build를 처음 쓸 때, 빌드용 서비스 계정(기본 compute SA)이 소스 업로드용 GCS 버킷(`<PROJECT>_cloudbuild`)에 접근 권한이 아직 없거나 전파 안 됨. 신규 프로젝트 첫 빌드에서 흔함.
- **해결 방향** (새 세션에서 만나면): (a) 잠시 후 재시도(권한 전파 대기), 또는 (b) Cloud Build 기본 SA에 `roles/storage.objectViewer`(또는 cloudbuild.builds.builder) 부여, 또는 (c) 최근 GCP는 Cloud Build에 전용 SA 사용 권장 → `--service-account` 지정. 책 가이드엔 이 함정 언급 없음.
- **참고**: 이번 세션은 "처음부터 다시(새 세션에서 책 방식으로)"로 결정해 이 빌드는 폐기하고 repo 삭제함.

## 정리 (이번 세션 리소스 폐기 — 새 세션 대비)

```bash
gcloud container clusters delete notiflex-cluster --zone=asia-northeast3-a --quiet
gcloud artifacts repositories delete notiflex --location=asia-northeast3 --quiet
```
- 클러스터 삭제 시 노드 VM·디스크는 GKE가 함께 정리(STOPPING→완전삭제). 고정IP·forwarding-rules 누수 없음 확인(매니페스트 배포 전이라 Gateway 리소스 미생성).

## 다음 단계 (예정)

- [ ] 클러스터 생성 완료 → get-credentials → kubectl get nodes
- [ ] Dockerfile·go.mod 생성 → 로컬 go build 검증
- [ ] Artifact Registry repo 생성 → gcloud builds submit
- [ ] k8s/smb 매니페스트(namespace·deployment·service) → apply → port-forward 확인
- [ ] 첫 커밋 → GitHub 새 저장소
- [ ] /update-docs 스킬
- [ ] ⚠️ 실습 종료 시 클러스터 삭제 + Gateway 리소스 누수 정리(2.5 cleanup)
