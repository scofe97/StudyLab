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

---

# 3장 실습 로그 (2026-07-12) — ArgoCD GitOps·롤링·롤백

> 정독 노트 `03-01`을 가이드로 §3.1~§3.3 실습. 클러스터는 2026-07-04 ch2 상태 보존분(0노드로 축소돼 있던 것)을 복구해 이어감.

## [정리] 클러스터 복구 (0→2 노드)

```bash
# JOURNEY.md 기록대로 default-pool이 0노드로 축소돼 있어 복구
gcloud container clusters resize notiflex-cluster --zone=asia-northeast3-a \
  --node-pool=default-pool --num-nodes=2 --quiet
# ch2 배포(notiflex-api)가 새 노드에 스케줄되어 2/2 Ready 확인
kubectl --context gke-sysnet4admin_book_gitaiops wait --for=condition=Ready \
  pod -l app=notiflex-api -n notiflex --timeout=90s
```
- 컨텍스트: 학습자 kubeconfig에 이미 `gke-sysnet4admin_book_gitaiops` 별칭이 등록돼 있어 하네스/책 명령의 `--context` 리터럴이 그대로 동작.

## [책 3.2 기반, 버전만 stable] ArgoCD 설치

```bash
kubectl --context gke-sysnet4admin_book_gitaiops create namespace argocd
kubectl --context gke-sysnet4admin_book_gitaiops apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml \
  --server-side=true --force-conflicts=true
```
- 설치 버전 **v3.4.5**(책은 v2.14.11). 학습자가 "stable + v3 함정 디버깅 경험"을 택함.
- Pod 7개 확인 = 노트 그대로. application-controller만 StatefulSet(`-0`).
- ⚠️ **UI 인증서 함정**: 기본 secure(HTTPS 8443)라 크롬이 self-signed 인증서로 접속 차단, 확장이 오류 페이지에 attach 불가. → `argocd-cmd-params-cm`에 `server.insecure:"true"` 패치 후 argocd-server 재시작 → `http://localhost:8080` 정상. (로컬 학습 한정, port-forward로만 접근)
- 초기 비번: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d`

## [명세생성] Application 생성 — path 함정

```yaml
# notiflex-platform/argocd/notiflex-smb.yaml (핵심)
spec:
  source:
    repoURL: https://github.com/scofe97/GitAIOps.git
    targetRevision: main
    path: notiflex-platform/k8s/smb   # ← 여기가 함정
  destination: { server: https://kubernetes.default.svc, namespace: notiflex }
  syncPolicy: { automated: { prune: true, selfHeal: true }, syncOptions: [CreateNamespace=true] }
```
- ⚠️ **path 함정 (노트 보강 포인트)**: 처음 `path: k8s/smb`로 뒀더니 `ComparisonError: k8s/smb: app path does not exist`, Sync=Unknown.
  - 오진 1차: NetworkPolicy egress 차단(v3 함정) 의심 → repo-server에서 `git ls-remote` 성공 확인, **아님**.
  - 오진 2차: repo-server 캐시 → hard refresh·repo-server 재시작해도 동일, **아님**.
  - 실제 원인: repo-server 안에서 직접 `git clone` 해보니 이 저장소(GitAIOps)는 `infra_ai` **전체**를 담고 `notiflex-platform`은 그 **하위 폴더**. 즉 repo 루트 기준 매니페스트 경로는 `notiflex-platform/k8s/smb`. (책 저자 repo는 notiflex-platform이 루트라 `k8s/smb`가 맞음 — 환경 구조 차이)
  - 교훈: "구조 사실은 추측 말고 결정론적으로 확정" — repo-server 컨테이너 내 실제 clone 트리를 봐서 확정.
- 수정 후 apply → **Synced/Healthy**, 리소스 트리 Namespace/Service/Deployment 전부 Synced.

## [책 3.3] 롤링 업데이트 — git push만으로

```bash
# /version 엔드포인트 추가(main.go), 앱 버전 상수 v0.1.1
gcloud builds submit app/ \
  --tag asia-northeast3-docker.pkg.dev/<PROJECT>/notiflex/api:v0.1.1 \
  --region=asia-northeast3            # scratch 멀티스테이지, 53초, SUCCESS
# deployment.yaml 태그 v0.1.0→v0.1.1, 커밋 후 push (kubectl 없이!)
git add main.go deployment.yaml argocd/notiflex-smb.yaml
git commit -m "feat: add /version endpoint (v0.1.1)" && git push
```
- push 직후 ArgoCD 자동 감지 → Rolling Update. 새 ReplicaSet `78f665df77`(rev:2)가 maxSurge:1/maxUnavailable:0으로 기존 `7dc545b5c8`(rev:1)을 하나씩 교체.
- 검증: 클러스터 내부 curl → `{"version":"v0.1.1","go_version":"go1.25.12","hostname":"notiflex-api-78f665df77-..."}` (Go 런타임은 빌드 이미지 golang:1.25-alpine 기준 go1.25.12).

## [책 3.3.2] 롤백 — git revert 한 줄

```bash
git revert HEAD --no-edit && git push      # 9a86942 "Revert ..." → v0.1.0 자동 롤백
# 검증: /version → HTTP 404(v0.1.0엔 없음), /id → id=1 pod=...7dc545b5c8(rev:1 복귀)
git revert HEAD --no-edit && git push      # e10100e "Reapply ..." → v0.1.1 재적용
# 검증: /version → 200 {"version":"v0.1.1",...}
```
- git log에 원본(f46e165)·revert(9a86942)·reapply(e10100e) 3커밋 모두 보존 → "누가·언제·왜 롤백" 이 깃에 남는 게 `kubectl rollout undo`와의 결정적 차이(노트 §3.3.2).
- deploy 리소스 rev:1→2→3→4로 이력 누적.

## 다음 단계 (예정)

- [ ] §3.4 GitHub Actions CI (빌드 자동화) — `gcloud builds submit` 수동 실행 없애기
- [ ] §3.5 CI + ArgoCD 연결
- [ ] Phase 4 검증(6문항 자답) — 실습 연결 축 포함
- [ ] 노트 03-01 보강: (1) path 함정, (2) ArgoCD 서버 vs argocd CLI 3개념(Repo/Cluster/Project), (3) 하위호환 구체예시, (4) v3 insecure/NetworkPolicy 함정
- [ ] ⚠️ 비용: 실습 종료 시 `gcloud container clusters resize ... --num-nodes=0` (또는 ArgoCD Pod 7개 감안해 유지 여부 결정)

## 스크린샷 (ArgoCD UI 기록)

`practice-screenshots/ch3/` 에 저장:
- `03-argocd-app-tree-synced.png` — Application 상세 트리. APP HEALTH Healthy / SYNC Synced(e10100e) / 리소스 트리(ns·svc·deploy rev:4 → ReplicaSet 78f665df77 rev:4 · 7dc545b5c8 rev:3).
- `04-history-and-rollback.png` — HISTORY AND ROLLBACK 패널. 배포 이력 3건이 커밋 단위로 추적됨을 실증:
  - e10100e (20:55) Reapply "feat: add /version endpoint (v0.1.1)" — Active 57m
  - 9a86942 (20:54) Revert "feat: add /version endpoint (v0.1.1)" — Active 57s
  - f46e165 (20:52) 원본 feat 커밋
  - 각 배포에 Authored by(심보현)·Initiated by(automated sync policy)·Source URL(github.com/scofe97/GitAIOps)이 남음 → kubectl rollout undo와의 결정적 차이(누가·언제·왜가 깃/UI에 보존).

> 참고: 크롬 확장의 save_to_disk는 확장 IndexedDB에 저장돼 파일시스템에 안 남으므로, macOS `screencapture`(osascript로 ArgoCD 탭을 앞으로 가져온 뒤)로 파일 저장함.

---

# 3장 후반부 실습 로그 (2026-07-12) — GitHub Actions CI + WIF

> 정독 노트 `03-02`(§3.4~3.7) 실습. 책은 SA 키 기반이나 조직 정책상 키 생성 불가 → WIF로 진행.

## [함정] SA 키 생성이 조직 정책으로 금지됨

```bash
gcloud iam service-accounts keys create /tmp/key.json --iam-account=github-ci@...
# → 실패: constraints/iam.disableServiceAccountKeyCreation (0 bytes 파일)
```
- 원인: company 조직 정책 `iam.disableServiceAccountKeyCreation`이 프로젝트에 적용. 책의 "키 기반 CI 인증"을 이 환경에선 못 씀.
- ⚠️ 실수: 실패했는데 뒤 단계가 진행돼 빈 파일이 GCP_SA_KEY Secret으로 등록됨 → 삭제함. 명령 실패 시 후속 중단 확인 필요.
- 대응: 노트 심화가 예고한 대로 WIF(Workload Identity Federation)로 전환 — 완성본 저장소와 같은 방향.

## [WIF] 키 없는 인증 설정

```bash
# Pool + OIDC Provider (repo 제한)
gcloud iam workload-identity-pools create github-pool --location=global
gcloud iam workload-identity-pools providers create-oidc github-provider \
  --workload-identity-pool=github-pool --location=global \
  --issuer-uri="https://token.actions.githubusercontent.com" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository" \
  --attribute-condition="assertion.repository=='scofe97/GitAIOps'"
# SA에 workloadIdentityUser 바인딩 (해당 repo만 이 SA 사용 가능)
gcloud iam service-accounts add-iam-policy-binding github-ci@... \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/884125530666/locations/global/workloadIdentityPools/github-pool/attribute.repository/scofe97/GitAIOps"
```
- ci.yaml은 `credentials_json`(키) 대신 `workload_identity_provider`+`service_account`, `permissions`에 `id-token: write` 추가.

## [함정] 워크플로가 저장소 루트 .github/에 있어야 인식됨

- 처음 `notiflex-platform/.github/workflows/ci.yaml`에 둠 → `gh workflow list` 비어있음(인식 안 됨).
- 원인: GitHub Actions는 **저장소 루트의 `.github/workflows/`만** 인식. 하위 폴더는 무시. (03-01 source.path 함정의 CI 판)
- 해결: `git mv`로 루트 `.github/workflows/ci.yaml`로 이동. 파일 내용은 경로가 이미 루트 기준이라 불변.

## [성공] 전체 파이프라인 end-to-end

```
app/main.go 버전 v0.1.2 → git push (673f4df)
  → GitHub Actions CI 트리거 (paths: notiflex-platform/app/** 필터 통과)
  → WIF 인증 성공 → docker build → AR push (api:673f4df)
  → sed로 deployment.yaml 태그 갱신 → CI가 커밋 (4f5c9ae, github-actions[bot])
  → ArgoCD 감지 → 자동 배포 (REVISION 4f5c9ae, ReplicaSet 75657f7f69)
  → /version = {"version":"v0.1.2",...} 확인
```
- CI 1m1s 성공(build-and-push 57s). 사람이 한 일은 code push 한 번뿐.
- 커밋 SHA 태그(api:673f4df) — 사람 버전 태그 아닌 SHA로 추적성·멱등성.
- 무한루프 안 남: CI 커밋(4f5c9ae)은 매니페스트만 바꿔서 paths(app/**) 필터에 안 걸림 + GITHUB_TOKEN 재귀 방지.

## 스크린샷 추가

- `05-github-actions-ci-success.png` — GitHub Actions 실행 요약. Status Success, build-and-push 57s, 673f4df main push 트리거.
- (03·04 재크롭: 브라우저 탭바·주소창·디버깅 배너 제거, ArgoCD/GitHub 콘텐츠만. `_full.png`는 원본 백업)

# ch4.2 — 메트릭 모니터링 (Prometheus + Grafana)

## [성공] kube-prometheus-stack Helm 설치

```bash
# values 파일: notiflex-platform/helm-values/kube-prometheus.yaml (책 §4.2.3 다이어트본)
kubectl create namespace monitoring
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring -f helm-values/kube-prometheus.yaml
```
- 노트가 예측한 Pod 7개 그대로 Running: prometheus-0(StatefulSet)·grafana·alertmanager-0·operator·kube-state-metrics·node-exporter×2(DaemonSet, 노드당 1개).
- 리소스 다이어트가 필수였음: ArgoCD가 이미 노드 CPU requests의 72~77%를 잡고 있어, 기본값(넉넉)으로는 Pending 위험. values로 Prometheus 100m·Grafana 50m·Alertmanager 25m 등으로 낮춰 Pending 없이 전부 스케줄됨.

## [검증] targets API + PromQL

```bash
kubectl port-forward svc/prometheus-operated -n monitoring 9090:9090
curl -s .../api/v1/targets  # activeTargets = 18 (책은 15, GKE는 kubelet 타깃이 더 많아 환경차)
curl -s '.../api/v1/query?query=up'  # 15 시리즈 중 13 up / 2 down
```
- **down 2개 = coredns** (`9153:/metrics` connection refused). GKE 관리형은 CoreDNS 메트릭 포트를 기본 비노출 → **정상 현상**. 앱·노드·오브젝트 메트릭 수집과 무관.
- `kube_pod_status_phase{namespace="notiflex"}` → 10 시리즈, Pod 2개 각 Running=1 → **kube-state-metrics(오브젝트 상태) 작동 확인**.
- `rate(container_cpu_usage_seconds_total{namespace="notiflex"}[5m])` → 6 시리즈 → **node-exporter/kubelet(하드웨어 CPU) 작동 확인**.
- 즉 "node-exporter=하드웨어 / kube-state-metrics=K8s 오브젝트" 역할 분담이 실제 데이터로 재현됨.

## [성공] Grafana 접속 + 기본 대시보드

```bash
kubectl port-forward svc/kube-prometheus-grafana -n monitoring 3000:80
# admin / <password> (values에 설정한 평문 — 학습용)
```
- 기본 대시보드 **29개** 자동 생성(책 "20개+"와 부합). Kubernetes/Compute Resources 계열, Node Exporter, CoreDNS, etcd 등.
- "Namespace (Pods)" 대시보드에서 notiflex CPU Quota 테이블: Requests 0.05·Limits 0.2 — deployment.yaml의 `cpu: 50m/200m`이 그대로 실측 반영됨.

## [함정] Chrome 확장 save_to_disk는 파일 안 남김

- `computer screenshot save_to_disk:true`가 IndexedDB에만 저장하고 파일시스템엔 안 남김(ch3 때와 동일 재현).
- 해결: macOS `screencapture -x -o`로 전체 화면 캡처 → `sips --cropOffset`로 브라우저 크롬(탭바·URL바)·Chrome 디버깅 배너를 잘라내 콘텐츠만 남김. `-l <windowId>`는 Chrome AppleScript window id가 CG window number와 달라 실패 → 전체 캡처+크롭이 안정적.

## 스크린샷 추가 (ch4)

- `ch4/01-grafana-namespace-pods-dashboard.png` — notiflex 네임스페이스 CPU/메모리 대시보드(스탯 4패널 + CPU Usage 그래프 + CPU Quota 테이블).
- `ch4/02-prometheus-targets-up.png` — Prometheus targets, serviceMonitor별 UP 상태.
- (탭바·URL·디버깅 배너 제거, 콘텐츠만)

## 남은 정리

- ⚠️ 실습 종료 시 클러스터 노드 0으로 축소(비용). WIF 리소스(pool/provider)·SA는 유지(다음 실습 재사용). monitoring 스택은 helm으로 재설치 가능(values 파일이 깃에 있음).
