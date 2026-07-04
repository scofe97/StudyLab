# 새 세션에 넣을 프롬프트 대본 (책 2장, GitAIOps 방식)

> 책의 방식대로 "프롬프트 하나 입력 → 클로드가 가드레일 참조해 실행 → 확인 → 다음 프롬프트" 로 진행한다.
> 프롬프트 예시는 `_Book_GitAIOps/CLAUDE.md`의 "2장 입력 예시" 원문 기반. 클로드가 자동으로 `prompt-guardrails/ch2/2.N-*.md`를 참조한다.

## 0단계 — 새 세션 시작 위치

가드레일이 자동 로드되려면 **가이드 저장소 폴더에서** 클로드 코드를 띄운다:

```bash
cd ~/infra_ai/_Book_GitAIOps
claude
```

## 0-1단계 — 첫 컨텍스트 세팅 프롬프트 (한 번만)

새 세션은 우리 상황을 모르니, 맨 처음 이 프롬프트로 배경을 준다:

```
나는 《AI 인프라 — Claude로》 2장을 실습 중이야. 이 저장소(_Book_GitAIOps)의 CLAUDE.md 가드레일을 따라 진행해줘.
- GCP 계정: user@example.invalid / 프로젝트: project-REMOVED
- 리전·존: asia-northeast3 / asia-northeast3-a
- 실습 결과 저장소는 ~/infra_ai/notiflex-platform 에 만들 거야
- gcloud는 이미 인증돼 있고, GKE/CloudBuild/ArtifactRegistry API도 활성화돼 있어
- 참고: 이 프로젝트는 Cloud Build 첫 실행 시 기본 compute 서비스계정의 GCS 버킷 접근 403이 날 수 있어. 나면 권한 부여나 재시도로 풀어줘.
각 단계는 내가 프롬프트로 하나씩 지시할게. 한 단계 끝나면 결과를 요약하고 멈춰서 내 다음 지시를 기다려줘.
```

> 마지막 문장("멈춰서 기다려줘")이 핵심 — 이게 있어야 새 세션이 한 번에 다 하지 않고 책처럼 단계별로 멈춘다.

## 1단계씩 — 실행 프롬프트 (책 CLAUDE.md 원문)

아래를 **하나씩** 넣는다. 각 프롬프트 뒤 클로드가 해당 가드레일 파일을 참조해 실행하고 멈추면, 결과 확인 후 다음으로.

| 순서 | 넣을 프롬프트 | 클로드가 참조할 파일 |
|:---:|---|---|
| 1 | `Claude Code 설치 확인해줘` | 2.2-install-check.md |
| 2 | `gcloud CLI 설치·인증 확인해줘` | 2.3-gcloud.md |
| 3 | `프로젝트는 project-REMOVED로, 리전은 서울(asia-northeast3)로 gcloud 기본값을 설정해줘` | 2.3-gcloud.md |
| 4 | `Artifact Registry 인증 설정해줘 (서울 리전)` | 2.3-gcloud.md |
| 5 | `GitHub 저장소 만들어줘` (notiflex-platform) | 2.4-github-repo.md |
| 6 | `GKE 클러스터 생성해줘` | 2.5-gke-cluster.md |
| 7 | `Notiflex 앱 만들고 배포해줘` | 2.6-build-deploy.md |
| 8 | `커밋하고 푸시해줘` | 2.7-first-commit.md |
| 9 | `/update-docs 커스텀 스킬 만들어줘` | update-docs-skill.md |

## 진행 팁

- 한 프롬프트 넣고 → 클로드가 실행·요약하고 멈추면 → 결과 눈으로 확인 → 다음 프롬프트.
- 중간에 "왜 이렇게 해?"가 궁금하면, 실행 대신 **탐색형 질문**을 먼저 던져도 된다. 예: `GKE Standard랑 Autopilot 뭐가 달라? 왜 Standard야?` → 클로드가 decision-guide나 설명으로 답함(각 가드레일 끝의 "💬 질문해보기"가 이 용도).
- 실습 끝나면 반드시: `GKE 클러스터 삭제하고 Gateway 리소스 누수까지 정리해줘` (2.5 가이드의 cleanup 섹션대로).

## 이 대본을 만든 근거

- `_Book_GitAIOps/CLAUDE.md`의 "2장: 환경 구성 (바로 실행)" 입력 예시 테이블 원문
- 각 단계 실제 명령은 `prompt-guardrails/ch2/2.N-*.md`에 있음(클로드가 자동 참조)
- 이번(현재) 세션에서 미리 실습해 본 결과: [practice-log.md](practice-log.md)에 실전 함정(gke-auth-plugin·PATH·Cloud Build 403) 기록됨 → 새 세션에서 만나면 그걸 참고
