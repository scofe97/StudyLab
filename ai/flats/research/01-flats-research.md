STATUS: done
NEXT: handoff (cc-flats-lab에 실측 확인 항목 전달)

# Flats(v0.9.1) 기술 조사 보고서

## 사실 표

| 항목 | 내용 | 근거 등급 | 근거 |
| --- | --- | --- | --- |
| 최신 안정 버전과 주요 변경 | 최신 안정 버전은 v0.9.1(2026-10-14)입니다. v0.9.1에서는 릴리스 아카이브에 NetBird 포함 바이너리를 기본 번들링하도록 빌드 스크립트가 수정되었습니다(#100). 직전 v0.9.0(2026-10-14)에서는 HTML 아트보드 캔버스 기반의 새 콘텐츠 유형 Design(type: "design", 전역 리비전 관리 및 핀/롤백) 추가(#96), 버전 생성(Save as vN)과 배포(Publish vN) 승인 단계 분리(#95), 요청 바디·웹소켓·SQLite 디스크 및 FILES 오브젝트 쿼터 제한 등 보안 강화, 메타데이터 스키마 11 마이그레이션이 적용되었습니다. | 소스 확인 | https://github.com/gosuda/flats/releases/tag/v0.9.1 , https://github.com/gosuda/flats/releases/tag/v0.9.0 |
| 콘텐츠 유형(Website·Page·Document·Design)별 저장·수정·배포 방식 | 1. Website(type: "flat"): 정적 파일 또는 QuickJS(Wasm)/WASI 기반 서버 앱. 드래프트 저장(save_draft) -> 미리보기(open_preview) -> 버전 생성 승인(publish, vN 생성) -> 배포 승인(deploy {version: N}) 2단계 승인 라이프사이클을 가집니다.<br>2. Page(type: "page"): 단일 HTML 문서(index.html)와 에셋 번들. 서버 코드 및 데이터 스토리지는 없습니다. Website와 동일한 드래프트->버전 승인->배포 승인 흐름을 따르며 save_page로 수정합니다.<br>3. Document(type: "docs"): 내장 협업 에디터 기반 실시간 Markdown 문서. 드래프트·버전·미리보기·발행 승인이 없으며 save_document 또는 update_document 호출 즉시 실행(기본 Private)됩니다. Public 상태인 경우 모든 수정이 인터넷에 즉시 노출됩니다.<br>4. Design(type: "design"): HTML 아트보드 캔버스(canvas.json 및 .html 파일들). 드래프트·버전·발행 승인 없이 save_design 호출 즉시 새 리비전으로 저장 및 반영됩니다. 모든 변경은 콘텐츠 주소 지정 전역 캔버스 리비전으로 기록됩니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/core/flattype.go#L10-L40 , https://github.com/gosuda/flats/blob/main/docs/content-types.md#L8-L20 , https://github.com/gosuda/flats/blob/main/docs/content-types.md#L430-L443 |
| 로컬 데이터 저장 경로 기본값과 바꾸는 법 | 기본 데이터 디렉터리는 운영체제 사용자 설정 디렉터리 하위 Flats 폴더입니다. macOS는 `~/Library/Application Support/Flats`, Linux는 `$XDG_CONFIG_HOME/Flats` 또는 `~/.config/Flats`입니다. 경로 변경 방법은 (1) 환경변수 FLATS_DATA 설정, (2) flats serve 실행 시 `--config <경로>` 플래그로 지정(설정 파일이 위치한 디렉터리를 데이터 디렉터리로 부트스트랩), (3) 레거시 플래그 `--data <경로>`, (4) 오프라인 상태에서 `flats config set host.data_dir <경로>` 실행입니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/app/serve.go#L80-L93 , https://github.com/gosuda/flats/blob/main/docs/configuration.md#L22-L25 |
| 원본 작업 폴더와 Flats 내부 저장소의 관계(배포 시 복사인지, 원본 수정이 반영되는지) | 배포 및 업로드 시 원본 작업 폴더의 파일들을 순회하여 인메모리 tar.gz 아카이브로 패키징한 후 Flats 호스트 내부 저장소(`flats/<slug>/`)로 복사(업로드)합니다. 심볼릭 링크와 특수 파일은 업로드 시 명시적으로 거부됩니다("%s is a symbolic link; uploads cannot contain links"). 따라서 배포 후 로컬 원본 작업 폴더의 파일을 수정하더라도 Flats 내부 저장소에 자동으로 반영되지 않으며, 다시 `flats deploy` 또는 MCP `save_*` 명령을 실행해야 새 Draft/Revision으로 복사 및 반영됩니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/cli/pack.go#L36-L75 , https://github.com/gosuda/flats/blob/main/docs/configuration.md#L39-L41 |
| 버전 관리·롤백 | 1. Website/Page: 번호가 부여된 불변 버전(v1, v2, ...)으로 관리됩니다. v0.9.0부터 드래프트에서 버전 생성(publish)과 활성화/배포(deploy)가 분리되었습니다. 롤백(flats rollback, MCP rollback)은 운영자 승인을 거쳐 이전 배포 버전 코드로 복구하며, `--restore-data` 옵션 지정 시 배포 직전 생성된 DB/FILES 스냅샷 데이터까지 승인 후 복구합니다.<br>2. Document: 버전 및 롤백 개념이 없습니다. 대신 저장 시 줄 단위 병합을 수행하며, 덮어쓰여 폐기된 실시간 텍스트는 복구용 충돌 세대(최대 8개, 2MiB 한도)로 보존되어 `get_document {conflict: generation}`으로 복구할 수 있습니다.<br>3. Design: 전체 캔버스 리비전 단위로 관리됩니다. list_revisions, get_revision, pin_revision, rollback_revision 도구를 지원합니다. rollback_revision은 별도 승인 없이 대상 리비전과 동일한 새 리비전을 즉시 생성하여 복구(실행 취소 가능)합니다. 기본 쿼터는 리비전 100개, 핀 20개입니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/store/store.go#L154-L175 , https://github.com/gosuda/flats/blob/main/internal/mcpx/design.go#L20-L33 , https://github.com/gosuda/flats/blob/main/docs/content-types.md#L538-L553 |
| SQLite 메타데이터와 콘텐츠 파일 저장 방식(어느 파일에 무엇이) | 순수 Go SQLite 드라이버(modernc.org/sqlite)가 두 계층에서 사용됩니다.<br>1. 메타데이터 DB: `<data-dir>/flats.db` (+ -wal, -shm). 호스트 바인딩, flats, versions, deployments, events, approvals, previews, drafts, draft_revisions, flat_providers, settings, runtime_generation, secrets(암호화), redirects 등을 저장합니다.<br>2. 개별 플랫 저장소: `<data-dir>/flats/<slug>/`.<br>- `draft-revs/`, `versions/`: 업로드된 코드 파일 보관<br>- `data/db.sqlite`: 서버 플랫 전용 자체 SQLite 데이터베이스(`env.DB`)<br>- `data/files/`: 서버 플랫 전용 영구 문자열 파일 스토리지(`env.FILES`)<br>- `snapshots/`: 배포 직전 보존된 DB+FILES 스냅샷<br>- `previews/`: 독립 격리된 프리뷰 데이터 | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/store/store.go#L14-L285 , https://github.com/gosuda/flats/blob/main/docs/configuration.md#L30-L55 |
| 로컬 접속 주소·포트, 바인딩 주소 | 기본 바인딩 주소는 완전히 분리된 2개의 루프백 주소입니다.<br>1. 관리 리스너 및 콘솔/API/MCP: `127.0.0.1:7878` (`host.management_addr`)<br>2. 로컬 콘텐츠 서빙 리스너: `127.0.0.1:7879` (`host.local_addr`)<br>콘텐츠 접속 URL은 `http://<slug>.localhost:7879`이며, 콘솔 접속 URL은 `http://127.0.0.1:7878`입니다. 두 주소 모두 `config.json` 또는 `flats serve --listen`, `--local-addr` 플래그로 변경할 수 있으며, 루프백이 아닌 주소로 바인딩 시 보안 경고가 출력됩니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/config/config.go#L76-L81 , https://github.com/gosuda/flats/blob/main/internal/app/serve.go#L101-L103 |
| 원격 접속 옵션 전부(Tailscale·Portal·Cloudflare Quick Tunnel·NetBird 등): 필요한 계정·인증, URL 수명, 접근 제어 | 1. Local (Private): 항상 켜짐, 무계정, `*.localhost:7879`, 로컬 머신 한정.<br>2. Tailscale (Private): Tailscale 계정 및 auth key(또는 콘솔 대화형 로그인) 필요, tsnet 내장, tailnet MagicDNS HTTPS URL(안정적 수명), tailnet ACL 기반 접근 제어.<br>3. Tailscale Funnel (Public): Tailscale 계정 필요, 공용 인터넷 공개, 안정적 Funnel URL, 접근 제어 없음.<br>4. Portal (Public): 계정·인증키·회원가입 일체 불필요(로컬 SIWE/키리스 서명), 릴레이 경유 공용 HTTPS URL(임대 갱신 지속), 접근 제어 없음(`portal.hide`는 릴레이 목록 노출만 숨김).<br>5. zrok (Public): zrok 계정 및 토큰 필요, 슬러그 예약으로 재시작 후에도 동일 URL 유지, 무료 플랜은 경고 인터스티셜 노출, 접근 제어 없음.<br>6. Cloudflare Quick Tunnel (Public): 계정·도메인·토큰 일체 불필요, `api.trycloudflare.com` 경유 랜덤 `https://<words>.trycloudflare.com` URL, 유휴 시 터널 회수로 주소 변경 가능(개발/테스트용), 200 동시 요청 제한, SSE 미지원, 접근 제어 없음.<br>7. NetBird (Private): NetBird 계정 및 셋업 키 필요, 피어 오버레이 IP의 21000–21999 포트(HTTP, 재시작 시 유지), NetBird 접근 정책 기반 제어, 동일 피어 내 플랫 간 브라우저 쿠키 공유 주의.<br>8. NetBird Expose (Public): NetBird 계정 및 리버스 프록시 필요, 세션 재연결 시 접미사 변경으로 URL 바뀜, 접근 제어 없음. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/config/config.go#L35 , https://github.com/gosuda/flats/blob/main/docs/networking.md#L10-L20 , https://github.com/gosuda/flats/blob/main/docs/networking.md#L43-L206 |
| 관리 콘솔·관리 API 와 공개 콘텐츠 포트가 분리되는지, 원격 제공자를 켜면 콘솔도 노출되는지 | 관리 콘솔/관리 API/MCP(`127.0.0.1:7878`)와 콘텐츠 포트(`127.0.0.1:7879`)는 명확히 분리된 포트와 리스너로 동작합니다. 공개 원격 제공자(Portal, Cloudflare Quick Tunnel, zrok, NetBird Expose, Tailscale Funnel)를 활성화하더라도 이는 승인된 개별 플랫의 콘텐츠로만 라우팅되며, 관리 콘솔이나 관리 API는 절대 공용 인터넷에 노출되지 않습니다. 관리 콘솔은 오직 `network.private_backend: tailscale`을 명시적으로 설정했을 때만 Tailscale 비공개 tailnet 상에 별도 노드(`flats`)로 노출되며, 이 경우에도 tailnet ACL로 접근이 통제됩니다. 문서에서도 "Never publish the console port to your network: anything that reaches it can approve deploys"라고 명시하고 있습니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/internal/config/config.go#L76-L81 , https://github.com/gosuda/flats/blob/main/docs/networking.md#L43-L76 , https://github.com/gosuda/flats/blob/main/docs/container.md#L35-L42 , https://github.com/gosuda/flats/blob/main/docs/README.md#L127-L128 |
| Claude Code·Codex CLI 연동(플러그인·MCP 엔드포인트·등록 명령·도구 목록) | 1. 플러그인 등록:<br>- Claude Code: `claude plugin marketplace add gosuda/flats && claude plugin install flats@flats` (또는 `/plugin install flats --marketplace gosuda/flats`)<br>- Codex CLI: `codex plugin marketplace add gosuda/flats && codex plugin add flats@flats`<br>- Cursor: `cursor-agent plugin marketplace add https://github.com/gosuda/flats`<br>2. 직접 HTTP MCP 등록:<br>- 엔드포인트: `http://127.0.0.1:7878/mcp` (Streamable HTTP)<br>- Claude Code: `claude mcp add --transport http flats http://127.0.0.1:7878/mcp`<br>- Codex CLI: `codex mcp add flats --url http://127.0.0.1:7878/mcp`<br>3. 원격 호스트 연결: `flats connect https://flats.example.ts.net` (설정은 `flats-client/connection.json`에 저장)<br>4. MCP 도구 목록(총 34개): check_client, create_flat, delete_env, delete_flat, deploy, get_approval, get_client_skill, get_content_types, get_design, get_document, get_draft, get_flat, get_logs, get_network, get_revision, get_runtime_reference, guide, list_env, list_flats, list_revisions, list_secrets, list_versions, open_preview, pin_revision, publish, rollback, rollback_revision, save_design, save_document, save_draft, save_page, save_version, save_version_from_dir, set_env, set_visibility, update_document. | 소스 확인 | https://github.com/gosuda/flats/blob/main/docs/agents.md#L23-L69 , https://github.com/gosuda/flats/blob/main/internal/mcpx/tools.go#L29-L90 , https://github.com/gosuda/flats/blob/main/internal/mcpx/design.go#L20-L33 |
| 백업·복구 절차, 서비스 중단 필요 여부, DB 파일 하나로 충분한지 | DB 파일(`flats.db`) 하나로는 절대 충분하지 않습니다. flats.db는 메타데이터 전용이며 실제 업로드된 소스 코드(`versions/`, `draft-revs/`), 플랫별 자체 SQLite DB(`data/db.sqlite`), 파일 스토리지(`data/files/`), 스냅샷(`snapshots/`)은 `flats/<slug>/` 디렉터리에 별도 저장됩니다. 또한 시크릿 복호화 키인 `secret.key`와 호스트 설정 `config.json`, 네트워크 노드 식별자(`tsnet/`, `portal/`, `zrok/`, `netbird/`)가 누락되면 복구가 불가능하거나 서비스가 시작되지 않습니다. 서비스 중단 여부와 관련하여, 호스트가 실행 중일 때 flats.db만 백업하려면 반드시 SQLite `VACUUM INTO '<경로>'`를 사용해야 하며, WAL 모드 상태의 단순 파일 복사는 데이터 깨짐 위험이 있습니다. 전체 데이터 디렉터리를 일관성 있게 백업하려면 서비스 중단(`launchctl bootout` 또는 `systemctl --user stop`) 후 디렉터리 전체를 복사하는 것이 필요하며 공식 업그레이드 문서에서도 이를 권장합니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/docs/configuration.md#L76-L89 , https://github.com/gosuda/flats/blob/main/docs/release.md#L305-L316 |
| 비밀 정보·키 파일 위치와 보호 | 1. 플랫 시크릿 암호화 키: `<data-dir>/secret.key` (32바이트, POSIX 권한 0600). flats.db 내 시크릿을 암호화하며 파일 누락 시 호스트 시작이 거부됩니다. MCP/콘솔 API는 시크릿 이름만 반환하고 값은 노출하지 않지만, 서버 런타임(`env.NAME`)은 평문으로 접근 가능하며 로컬 데이터 디렉터리 읽기 권한을 가진 사용자는 복호화할 수 있습니다.<br>2. Tailscale 인증키: 데이터 디렉터리 외부에 별도 보관하며 `config.json`의 `credentials.tailscale_authkey_file`로 경로만 지정합니다.<br>3. zrok 환경 토큰: `<data-dir>/zrok-environment/` (권한 0600)에 보관되며 콘솔이나 설정 파일에 재노출되지 않습니다.<br>4. NetBird 피어 식별자: `<data-dir>/netbird/` (권한 0600)에 보관되며 셋업 키는 등록 시 1회 사용 후 저장되지 않습니다.<br>5. Cloudflare 터널 자격증명: `<data-dir>/cloudflare-quick/<slug>.json` (권한 0600).<br>6. 관리 콘솔 보호: 포트 7878에서 Same-Origin 브라우저 헤더 검증(CSRF 방지)만 수행하며 별도 사용자 인증은 없으므로, 로컬 머신에서는 신뢰할 수 있는 에이전트만 실행해야 합니다. | 소스 확인 | https://github.com/gosuda/flats/blob/main/docs/configuration.md#L35 , https://github.com/gosuda/flats/blob/main/docs/README.md#L307-L319 , https://github.com/gosuda/flats/blob/main/internal/config/config.go#L146 |

## 용도별 매핑

사용자가 검토 중인 네 가지 용도에 적합한 Flats 콘텐츠 유형, 채택 이유, 그리고 기술적 걸림돌 및 제약사항은 다음과 같습니다.

### 1. Networking·Linux·K8s·Observability·분산 시스템·Go 학습 문서(Markdown)
- **적합 콘텐츠 유형**: `docs` (`type: "docs"`)
- **이유**:
  - 브라우저 기반 실시간 협업 Markdown 에디터를 내장하고 있어 웹에서 즉시 읽고 편집할 수 있습니다.
  - Mermaid 다이어그램 렌더링을 네이티브 지원합니다. 다이어그램 확대/축소, 화면 맞춤, 라이트/다크 테마 전환, 소스 편집 및 SVG/PNG 다운로드가 가능하여 네트워크 토폴로지, 분산 시스템 구조, K8s 컴포넌트 관계도를 시각화하기에 최적입니다.
  - 단일 파일뿐만 아니라 최대 128개의 `.md` 파일(개별 1MiB, 총 4MiB 한도)과 정적 에셋(이미지, SVG 등)을 하나의 번들로 구성할 수 있어 체계적인 기술 문서 위키 구축이 가능합니다.
- **걸림돌 및 제약**:
  - 버전 관리 및 롤백이 지원되지 않습니다(`list_versions`, `rollback` 호출 시 거부).
  - 드래프트나 사전 미리보기 단계가 없습니다. `save_document`나 `update_document` 호출 즉시 실행 중인 문서에 반영됩니다. 특히 문서를 Public으로 전환한 경우, 사소한 오타 수정이나 실시간 편집 내용이 인터넷에 즉시 공개 노출됩니다.
  - 보안상 Markdown 내 임의의 원시 HTML이나 스크립트 실행이 불가능하며, PDF/DOCX 내보내기나 댓글 기능이 없습니다.

### 2. 패킷 흐름·K8s 아키텍처 같은 단일 HTML 시각화
- **적합 콘텐츠 유형**: `page` (`type: "page"`)
- **이유**:
  - 단일 HTML 문서(`index.html`)와 관련 에셋(SVG, Canvas, D3/Mermaid 등 인터랙티브 차트 라이브러리, CSS, 데이터 JSON)을 번들링하여 서비스하는 전용 유형입니다.
  - 복잡한 패킷 흐름 애니메이션이나 K8s 아키텍처 인터랙션을 자유로운 웹 기술로 구현할 수 있습니다.
  - Website와 동일하게 비공개 드래프트 저장, 브라우저 미리보기(`open_preview`), 운영자 승인 후 버전 생성 및 배포 단계를 거치므로 미완성 시각화가 노출되지 않습니다.
- **걸림돌 및 제약**:
  - 번들 규칙상 정확히 단 하나의 HTML 파일만 진입점(`entry`)으로 허용되며, 두 번째 HTML 파일이 포함되면 업로드가 거부됩니다.
  - 서버 사이드 코드 실행이나 데이터 저장이 불가능합니다(`kind: "static"` 전용, `env.DB` 및 `env.FILES` 미지원).
  - 수정 시 전체 드래프트를 매번 완전한 번들로 재전송해야 합니다(`save_page`).

### 3. 학습 자료 대시보드 같은 작은 사이트와 버전·롤백
- **적합 콘텐츠 유형**: `flat` (Website, 정적 사이트 또는 `kind: "server"`)
- **이유**:
  - 복수 페이지로 구성된 웹사이트 또는 QuickJS(Wasm) 기반의 소형 서버 애플리케이션을 배포할 수 있습니다.
  - 내장 SQLite(`env.DB`) 및 영구 파일 스토리지(`env.FILES`)를 지원하므로 학습 진행도 저장, 퀴즈 기록, 북마크 관리 등의 대시보드 기능을 손쉽게 구현할 수 있습니다.
  - 번호 기반 불변 버전 관리(`v1`, `v2`, ...), 코드 롤백(`flats rollback`), 그리고 배포 직전 시점의 데이터 스냅샷 복구(`--restore-data`)를 지원하므로 안전한 업데이트와 롤백이 가능합니다.
- **걸림돌 및 제약**:
  - v0.9.0부터 버전 저장(Save as vN)과 실제 배포(Publish vN)가 2단계 별도 승인으로 분리되어, 배포 과정에서 운영자 승인을 두 번 거쳐야 합니다.
  - 서버 런타임이 Node.js가 아닌 WebAssembly 기반 QuickJS이므로, Node.js 내장 모듈이나 외부 npm 라이브러리를 직접 require할 수 없고 번들러를 통해 사전에 번들링해야 합니다.
  - 플랫당 업로드 용량(기본 20MiB), 디스크 쿼터(기본 30GiB), 초당 요청 수(기본 50 RPS) 제한이 존재합니다.

### 4. Claude Code·Codex CLI 에서 만든 콘텐츠를 MCP 로 넘기되 공개 전 승인 유지
- **적합 콘텐츠 유형**: `flat` (Website) 또는 `page` (Page)
- **이유**:
  - `flat`과 `page` 유형은 엄격한 사전 승인 게이트웨이 라이프사이클을 준수합니다.
  - 에이전트가 MCP 도구(`save_draft`, `save_page`, `save_version`)를 통해 업로드하면 오직 로컬의 비공개 드래프트(Private Draft)만 변경됩니다.
  - 버전을 동결하는 단계(`publish`)와 해당 버전을 실제 서비스로 배포하는 단계(`deploy`)는 모두 콘솔의 운영자 승인 URL(`approval_url`)을 반환하며, 에이전트가 스스로 승인할 수 있는 MCP 도구는 존재하지 않습니다.
  - 따라서 인간 운영자가 브라우저 콘솔에서 직접 버튼을 누르기 전까지는 미승인 콘텐츠가 외부에 공개되거나 배포되지 않는 안전장치가 완벽히 보장됩니다.
- **걸림돌 및 제약**:
  - 만약 에이전트가 `docs`나 `design` 유형으로 플랫을 생성할 경우, 승인 절차 없이 `save_document`나 `save_design` 호출 즉시 실행 상태에 반영되므로 공개 전 승인 원칙이 훼손될 수 있습니다. 따라서 반드시 `flat` 또는 `page` 유형을 강제해야 합니다.
  - v0.9.0의 버전 분리로 인해 기존의 단일 배포 자동화 스크립트 대신 `publish`(버전 승인 대기) -> `deploy`(배포 승인 대기)로 이어지는 2회 승인 대기 흐름을 에이전트 작업 프롬프트에 명시적으로 반영해야 합니다.

## Claude Artifact 와의 차이

Flats는 프로젝트 설명에서 스스로를 "Open-source alternative to sites/artifacts"(sites 및 artifacts의 오픈소스 대안)라고 정의하고 있습니다. Claude Artifact와 Flats의 핵심 기술적 차이는 다음과 같습니다.

| 비교 측면 | Claude Artifact | Flats (v0.9.1) |
| --- | --- | --- |
| **저장 위치** | Anthropic 클라우드 인프라 및 CDN에 중앙 집중식으로 저장·호스팅됩니다. | 100% 사용자 로컬 머신(`~/.local/bin`, `<data-dir>`)에 자체 호스팅(Self-hosted)됩니다. 코드, SQLite 데이터베이스, 파일, 암호화된 시크릿이 사용자 로컬 디스크에만 보관됩니다. |
| **공유 방식** | Claude UI 내 인라인 렌더링 또는 Anthropic이 생성한 클라우드 공유 링크를 통해서만 공유할 수 있으며, 네트워크 경계나 터널링에 대한 제어권이 없습니다. | 기본적으로 로컬 루프백(`127.0.0.1:7879`)에 비공개 서빙됩니다. 필요 시 Tailscale/NetBird(비공개 VPN/ACL) 또는 Portal/Cloudflare Quick Tunnel/zrok/Tailscale Funnel(공개 릴레이·터널)을 통해 네트워크 경계를 정밀하게 선택하여 공유할 수 있습니다. |
| **승인 체계** | LLM이 응답을 생성하는 즉시 UI에 렌더링되며, 배포나 버저닝에 대한 서버 레벨의 정식 승인 게이트웨이나 접근 통제 정책이 없습니다. | 호스트 관리 콘솔(`127.0.0.1:7878`)에 정식 승인 게이트웨이가 내장되어 있습니다. 드래프트->버전 생성, 버전 배포, 공개/비공개 가시성 전환, 롤백, 데이터 복구, 삭제 등 모든 상태 변경에 명시적 운영자 승인을 요구하며 에이전트에게 승인 권한을 주지 않습니다. |
| **버전 관리** | 단일 대화 맥락 내에서 단순 선형 버전 목록(v1, v2 UI 탭)으로만 제공되며, 데이터 스냅샷 분리나 격리된 프리뷰 환경이 없습니다. | 콘텐츠 유형별로 고도화된 버전 모델을 제공합니다.<br>- Website/Page: 독립된 비공개 드래프트, 불변 번호 버전(`v1`, `v2`, ...), 독립 격리 프리뷰, 코드 롤백, 배포 직전 시점 DB+FILES 스냅샷 복구(`--restore-data`)<br>- Design: 콘텐츠 주소 지정 기반 전역 캔버스 리비전, 핀 고정, 즉시 롤백(`rollback_revision`)<br>- Document: 줄 단위 병합 및 덮어쓰기 발생 시 최대 8세대(2MiB) 충돌 복구 스냅샷 보존 |

## 옆 창이 실측으로 확인해야 할 것

공식 문서와 Go 소스 코드 정적 분석만으로는 확정할 수 없어, 실제 설치 및 실행 환경(cc-flats-lab)에서 직접 실측하여 검증해야 할 항목 목록입니다.

1. **QuickJS Wasm 런타임의 실측 성능 및 메모리 소비량**
   - QuickJS(Wasm) 기반 서버 플랫에서 대용량 SQLite 쿼리나 복잡한 JS 연산 수행 시의 실제 처리 지연 시간(Latency)과 메모리 점유율.
   - 기본 설정된 50 RPS 속도 제한 하에서 동시 요청 부하가 발생할 때의 CPU 점유율 및 안정성.
2. **Cloudflare Quick Tunnel 및 Portal Relay의 실제 연결 속도와 주소 수명**
   - Cloudflare Quick Tunnel(`api.trycloudflare.com`) 활성화 시 터널 개설까지 걸리는 실제 체감 시간, 유휴(Idle) 상태가 얼마 동안 유지될 때 주소가 만료되어 바뀌는지 여부.
   - Portal Relay 연결 시 E2E 핸드셰이크 지연 시간 및 `portal.hide` 플래그 변경 후 공용 릴레이 디렉터리에서 실제로 숨겨지기까지 걸리는 시간(문서상 최대 90초).
3. **Claude Code CLI 환경에서 flats@flats 플러그인의 체감 연동 UX**
   - Claude Code CLI 환경에서 `claude plugin marketplace add gosuda/flats` 및 `claude plugin install flats@flats` 명령이 별도 에러 없이 매끄럽게 설치되는지 여부.
   - `flats deploy` 또는 MCP `publish` -> `deploy` 호출 시 종료 코드 3(승인 대기)이 반환될 때 Claude Code가 어떻게 사용자에게 승인 URL을 안내하고 대기하는지 실제 상호작용 검증.
4. **NetBird 동일 피어 내 플랫 간 브라우저 쿠키 공유 실측**
   - 소스 및 문서에서 경고한 대로, NetBird 비공개 포트(21000–21999)로 서빙되는 여러 서버 플랫을 동일 브라우저에서 열었을 때 포트 구분 없이 세션 쿠키가 실제로 공유 및 덮어쓰기되는지 브라우저 개발자 도구에서 실측 확인.
5. **Docs 플랫의 실시간 협업 에디터와 MCP 수정 간의 충돌 병합 동작**
   - 브라우저 에디터에서 사용자가 타이핑 중인 상태에서 에이전트가 MCP `update_document` 또는 `save_document`를 실행했을 때 실제 줄 단위 병합이 오류 없이 동작하는지.
   - 덮어쓰기가 발생했을 때 `get_document {conflict: generation}`으로 유실된 텍스트가 정상 복구되는지 실측.
6. **무중단 VACUUM INTO 백업 실행 시의 락 경합 및 무결성**
   - 플랫에 실시간 HTTP 트래픽이 인입되는 도중 `sqlite3 flats.db "VACUUM INTO 'backup.db'"`를 실행했을 때 서비스 중단이나 락 타임아웃 없이 안전하게 메타데이터 DB가 복제되는지 실측.
