STATUS: partial
NEXT: user-decision (콘솔에서 승인 대기 2건(k8s-network·learning-hub 의 publish)을 직접 승인할지, Chrome 확장을 연결해 이 창이 대신 누르게 할지)

# flats-lab 실측 보고서 (gosuda/flats v0.9.1, macOS 26.6.2 arm64)

실행일: 2026-10-15. 작업 저장소: `/Users/user/study/flats-lab/`. 원문 출력은 단위별 로그와 `logs/raw/` 에 있다.

## 한눈에

설치부터 런타임 구조, 문서(Document)·재시작 유지·백업/복구·Claude Code MCP 연동까지 실측했다. Page 와 Website 의 **버전 저장·Publish·롤백(승인 필요 단계)** 은 승인 결정 수단이 브라우저 콘솔뿐인데 Claude in Chrome 확장이 연결되지 않아 실행하지 못했다. 그래서 STATUS 는 `partial` 이다.

## 단위별 결과

| # | 로그 | 결과 |
|---|---|---|
| 1 | `01-install-review.md` | 완료. `install.sh` 를 내려받아 읽고 실행. v0.9.1 설치, launchd `dev.flats.serve` 등록, sudo 없음. 아카이브 SHA-256 을 따로 재검증해 일치. `flats --version` 은 없고 `flats version` |
| 2 | `02-runtime.md` | 완료. 데이터 `~/Library/Application Support/Flats`, 7878(콘솔·API·MCP)·7879(콘텐츠) 모두 `127.0.0.1` 루프백만 바인딩, plist `RunAtLoad`·`KeepAlive` true, `kickstart -k` 재시작 후 정상. 재부팅은 안 함 |
| 3 | `03-test-a-page.md` | **부분.** Draft 저장·미리보기(200)·publish 승인 요청(exit 3)까지. 정식 URL 은 404. 버전 저장 이후 미실행 |
| 4 | `04-test-b-document.md` | 완료. `flats.json` `type: docs` 로 즉시 Private 실행, 승인 없음, 수정 반영 약 3초, 라이브 해시가 원본 SHA-256 과 일치. `flats versions` 는 문서에 대해 거부 |
| 5 | `05-test-c-website.md` | **부분.** Draft·미리보기(3쪽 200)·publish 요청까지. v1/v2/롤백 미실행 |
| 6 | `06-test-d-persistence.md` | 완료(승인 전 범위). 재시작 후 콘텐츠·Draft·대기 승인 모두 유지. 원본을 고쳐도 배포본은 안 바뀌고 재배포 때만 반영. 파일·SQLite 관계 정리 |
| 7 | `07-backup-restore.md` | 완료. 문서 절차(서비스 정지 후 통복사)로 백업, 임시 디렉터리+17878/17879 foreground 호스트에서 복구 확인 후 종료. 운영 호스트는 건드리지 않음 |
| 8 | `08-mcp-claude-code.md` | 완료(승인 전 범위). `claude mcp add --scope project` → `.mcp.json`, 새 `claude -p` 세션이 도구 36개를 보고 `save_page` 로 Draft 만 저장. 승인 후 단계 미실행 |

## 완료 조건 점검

- `test -x ~/.local/bin/flats && ~/.local/bin/flats status` → 성공(07 종료 시점에도 `running at http://127.0.0.1:7878`).
- `logs/` 에 00~08 로그 모두 존재.
- `lsof -nP -iTCP:7878 -sTCP:LISTEN` 원문은 `02-runtime.md` 와 `logs/raw/02-runtime.md`.
- 테스트 A: `curl` 결과가 `03-test-a-page.md` 에 있으나 정식 URL 은 **404**(Publish 승인 미완). Publish 후 200 확인은 **미충족**.

## 결정 요청

1. **결정 문장**: 승인 대기 중인 publish 를 어떻게 결정할 것인가.
   - 근거: 승인 결정은 콘솔 브라우저에서만 가능(`flats help approvals` 는 목록뿐, MCP 에 승인 도구 없음 — `research/host-docs/docs_agent-guide.md` "Approvals" 절). 이 세션은 Chrome 확장 미연결(`tabs_context_mcp` 2회 실패). 호스트 문서는 에이전트가 콘솔 라우트·CSRF 헤더를 직접 보내는 것을 금지(`docs_agent-guide.md` 약 1920줄)해 HTTP 우회는 하지 않았다.
   - 이 랩의 대기 건: `apr-96p1503rvmiz`(k8s-network, Draft rev 2) · `apr-zn27nzgynbpk`(learning-hub, rev 1). `apr-47ecg86fq2fh` 는 rev 1 용으로 stale 이 될 것이라 무시해도 된다. `mcp-hello` 는 요청하지 않았다.
   - **추천 기본값**: 사용자가 `http://127.0.0.1:7878` 콘솔에서 위 두 건을 직접 승인 → 이어서 `flats deploy --flat k8s-network --version 1`·`flats deploy --flat learning-hub --version 1` 로 활성화 요청 후 다시 승인 → 이후 단계는 `05` 로그 말미 명령 목록대로 이 창이 진행.
   - 선택별 결과: (a) 직접 승인 — 위 흐름이 이어지고 03·05 가 완결됨. (b) Chrome 확장 연결 후 재지시 — 이 창이 127.0.0.1 콘솔에서만 클릭해 03·05·08 의 "승인 후" 를 끝냄. (c) 승인하지 않음 — 현재 상태로 종결, Draft 는 유지됨.
2. **같은 호스트의 타 세션 flat**: `try-prom-talk`·`try-pod-network`·`try-pod-doc` 은 이 세션이 만들지 않았다(`~/study/flats-lab/try/` 폴더가 있고 `apr-3qgsmkm5upn7`·`apr-yfntfdj2husr` 도 대기). 이 랩의 승인으로 취급하지 않았다. 추천: 그 승인은 해당 작업 창 소유자가 판단. 백업·복구 때 약 5초 서비스 정지가 이들에게 영향을 줬을 수 있다.

## 원격 접속은 켜지 않았다

Tailscale·Tailscale Funnel·Portal·zrok·Cloudflare quick·NetBird 어느 것도 켜지 않았다. `flats config show` 의 `network.permitted` 는 `[]`, `network.private_backend` 는 `local`(기본값)이고 `flats status` 의 제공자 중 `local` 만 configured=true, 나머지는 `configured: false · permitted: false`(01 로그 원문). 어떤 콘텐츠도 Public 으로 바꾸지 않았다.

문서상 차이(`docs/networking.md` 요약, 실행 안 함):

| 제공자 | 구분 | 도달 범위 |
|---|---|---|
| Local | Private | 이 머신의 `*.localhost` 만, 항상 켜짐 |
| Tailscale / NetBird | Private | tailnet ACL / NetBird 정책이 허용한 기기 |
| Tailscale Funnel / Portal / zrok / Cloudflare quick / NetBird Expose | Public | 인터넷의 누구나(릴레이·터널 경유) |

켜려면 호스트 허용(`network.permitted`, 콘솔 Settings)과 flat 별 허용 둘 다 필요하고, Public 전환은 승인 대상이다. Public 을 켜기 전에는 "Public 은 접근 제어가 아니다"(`agent-guide`)는 점과 콘솔에 인증이 없다는 점을 먼저 확인해야 한다.

## 이번 실측에서 문서와 달랐거나 알아둘 점

- `flats --version` 불가, `flats version`. `flats help serve` 는 serve 플래그가 아니라 전체 도움말, `flats serve -h` 를 써야 함.
- `flats deploy ./dir` 은 폴더 안 숨김 파일까지 번들에 담는다(`.omc/…` 가 Draft rev 1 에 들어감).
- 문서 타입은 내부적으로 `versions` 행을 만들지만 CLI 는 `documents have no versions` 로 거부한다.
- `flats preview` 출력의 `now:` 값이 실제 현재 시각이 아니라 24시간 뒤(만료 시각)로 찍힌다.
- 워커 로그에 `flats: ignoring the saved connection: $HOME is not defined` 가 반복된다(동작 영향 없음).
- 백업은 `secret.key` 를 평문으로 포함한다(`backups/` 는 `.gitignore` 로 무시됨, 디렉터리 0700). 백업 자체를 비밀로 다뤄야 한다.
- 콘솔·API 는 인증이 없고 루프백 도달 프로세스는 누구나 접근 가능(문서가 명시).

## 전체 Needs Verification

- 03/05: v1 저장·활성화 이후의 `curl` 200, 버전 목록, v2, v1 롤백, 승인 전후 헤더 차이.
- 03: rev 1 승인(`apr-47ecg86fq2fh`)이 실제 `stale_approval` 로 바뀌는지.
- 01: `gh attestation verify` 출력 없이 exit 0 — 성공 근거는 종료 코드뿐. 업그레이드 경로(restart/replace) 미실행.
- 02: 재부팅 후 자동 시작(plist 근거만), 로그 로테이션, 서버 flat 워커의 PATH 상속.
- 04: 브라우저 렌더·편집기·Mermaid 모습, 사람 편집과 재저장의 병합, Public 전환 시 읽기 전용(문서 근거만), `$HOME is not defined` 의미.
- 06: 승인된 버전의 재시작 후 유지, `keep_versions` 가지치기, `revisions`/`revision_files` 용도, `try-*` flat 의 생성 주체.
- 07: 서비스 정지 없이 복사했을 때의 일관성, 승인된 버전·데이터 스냅샷·서버 비밀이 있는 상태의 복구, `config rebind` 가 필요한 조건.
- 08: 승인 후 에이전트 폴링 흐름, 대화형 `claude` 에서 프로젝트 MCP 승인 후 연결 상태, 플러그인 동작(설치 안 함).

## 남긴 파일·상태

- 새로 만든 것: `logs/`, `logs/raw/`, `examples/{page-k8s-network,doc-k8s-network,site-learning-hub}`, `research/{upstream-docs,host-docs}`, `backups/{flats-data-20261015-1100,restore-test,hot-flats.db,restore-serve.log}`, `.mcp.json`.
- Flats 호스트에 남은 flat: `k8s-network`(page, Draft rev 2), `k8s-network-doc`(docs, 실행 중), `learning-hub`(flat, Draft rev 1), `mcp-hello`(page, Draft rev 1). 삭제·uninstall 하지 않았다.
- 서비스: launchd `dev.flats.serve` 실행 중.
- git 커밋·push, 전역 설정 수정, 외부 전송은 하지 않았다.
