# Notiflex 여정 기록

이 파일은 독자가 실제로 진행한 내용을 기록한다. AI가 각 챕터 완료 시 자동으로 업데이트한다.

## 진행 현황

| 챕터 | 서브챕터 | 상태 | 완료일 | 비고 |
|------|---------|------|--------|------|
| ch2 | 2.2 설치 확인 | ✅ | 2026-07-04 | gcloud/kubectl/gh 확인 |
| ch2 | 2.3 gcloud 설정 | ✅ | 2026-07-04 | project-REMOVED / asia-northeast3-a |
| ch2 | 2.4 GitHub 저장소 | ✅ | 2026-07-04 | scofe97/GitAIOps (public) |
| ch2 | 2.5 GKE 클러스터 | ✅ | 2026-07-04 | notiflex-cluster, Spot VM 2노드, Gateway API standard |
| ch2 | 2.6 빌드/배포 | ✅ | 2026-07-04 | api:v0.1.0, notiflex 네임스페이스 2/2 Running |
| ch2 | 2.7 첫 커밋 | ✅ | 2026-07-04 | JOURNEY.md 생성, GitAIOps에 푸시 |
| ch3 | 3.2 GitOps 도구 | ⬜ | | |
| ch3 | 3.3 기능 추가 | ⬜ | | |
| ch3 | 3.4 CI | ⬜ | | |
| ch3 | 3.5 CI-CD 연결 | ⬜ | | |
| ch4 | 4.2 메트릭 모니터링 | ⬜ | | |
| ch4 | 4.3 로그 수집 | ⬜ | | |
| ch4 | 4.4 알림 | ⬜ | | |
| ch5 | 5.2 트래픽 관리 | ⬜ | | |
| ch5 | 5.3 무중단 배포 | ⬜ | | |
| ch6 | 6.1 캐시 | ⬜ | | |
| ch6 | 6.2 시크릿 관리 | ⬜ | | |
| ch6 | 6.3 Canary 전환 | ⬜ | | |
| ch7 | 7.2 멀티 노드풀 | ⬜ | | |
| ch7 | 7.3 App of Apps | ⬜ | | |
| ch7 | 7.4 멀티테넌시 | ⬜ | | |
| ch8 | 8.1 메시징 | ⬜ | | |
| ch8 | 8.2 트레이싱 | ⬜ | | |
| ch8 | 8.3 CronJob | ⬜ | | |
| ch9 | 9.1 저장소 분석 | ⬜ | | |
| ch9 | 9.2 회고 | ⬜ | | |
| ch9 | 9.3 온보딩 문서 | ⬜ | | |
| ch9 | 9.4 GitAIOps 분석 | ⬜ | | |
| ch9 | 9.5 마무리 | ⬜ | | |

## 도구 선택 기록

독자가 3-프롬프트 패턴(탐색→비교→실행)에서 실제로 선택한 도구와 이유를 기록한다.

| 영역 | 선택 | 검토한 대안 | 선택 이유 |
|------|------|-----------|----------|
| 클러스터 모드 | GKE Standard (Zonal) | Autopilot, Regional | 실습 비용 최소화(Spot VM 직접 제어), 노드 설정 학습 |
| 컨테이너 이미지 | scratch 멀티스테이지 | Alpine, Distroless | 최소 이미지 크기·공격 표면, 정적 Go 바이너리 |

## 현재 버전

| 컴포넌트 | 버전 | 변경 이력 |
|---------|------|----------|
| Go | 1.25 | ch2.6 초기 설정 |
| Notiflex 이미지 | api:v0.1.0 | ch2.6 최초 빌드·배포 |
| ArgoCD | | |
| Kafka | | |
| OTel SDK | | |

## 현재 리소스

| 노드풀 | 머신 타입 | 노드 수 | 주요 워크로드 |
|--------|----------|---------|-------------|
| default-pool | e2-medium (Spot, 30GB) | 2 → 0 (실습 종료 후 비용 절감 축소) | notiflex-api |

> ⚠️ **실습 중단 상태**: 비용 절감을 위해 default-pool을 0노드로 축소함(2026-07-04). 클러스터·매니페스트·이미지는 보존됨. 다음 실습 재개 시 `gcloud container clusters resize notiflex-cluster --zone=asia-northeast3-a --num-nodes=2 --node-pool=default-pool`로 복구한다.

## 트러블슈팅 이력

독자가 겪은 문제와 해결 방법을 기록한다. 같은 문제를 다시 겪지 않도록 한다.

| 챕터 | 문제 | 해결 |
|------|------|------|
| ch2.6 | Cloud Build 403: Compute 기본 SA에 소스 버킷 접근 권한 없음 | 새 프로젝트라 IAM 역할 부재 → `roles/cloudbuild.builds.builder` 부여 |
| ch2.5 | GatewayClass가 생성 직후 조회 안 됨 | 설정·CRD는 정상, 컨트롤러 전파 지연(수 분). 대기 후 4종 ACCEPTED 확인 |
