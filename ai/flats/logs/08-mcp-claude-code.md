# 08 Claude Code MCP 연동 (프로젝트 범위)

실행일: 2026-10-15. Claude Code 2.1.300, Codex CLI 0.160.0. 등록 범위는 `/Users/user/study/flats-lab` 프로젝트(`.mcp.json`)뿐이다. `~/.claude`·`~/claude/.claude-config`·`~/.codex` 설정은 수정하지 않았다.

## 1. 공식 방법 (문서 근거)

`docs/agents.md` 「MCP without the plugin」과 `flats mcp-config` 출력이 같은 명령을 안내한다. 플러그인 없이 HTTP 엔드포인트 직접 등록이다.

## 2. 등록과 상태 (원문)
$ flats mcp-config
Flats MCP endpoint: http://127.0.0.1:7878/mcp  (Streamable HTTP)

Claude Code:
  claude mcp add --transport http flats http://127.0.0.1:7878/mcp
  (add --scope user to make it available in every project)

Codex (~/.codex/config.toml):
  [mcp_servers.flats]
  url = "http://127.0.0.1:7878/mcp"
  # needed for `codex exec`; interactive sessions can prompt instead
  default_tools_approval_mode = "approve"
  or: codex mcp add flats --url http://127.0.0.1:7878/mcp
  Flats requires operator approval for every publish, activation, rollback,
  deletion and visibility change in both directions. Client auto-approval
  permits Draft edits and pending requests; it does not make a version live
  or grant operator authority. Use only with trusted agents.
  Discover runtime APIs: read flats://docs/runtime-api/v1 or call
  get_runtime_reference with {} (no installed skill required).

Cursor (.cursor/mcp.json in a project, or ~/.cursor/mcp.json for all projects):
  {
    "mcpServers": {
      "flats": {
        "url": "http://127.0.0.1:7878/mcp"
      }
    }
  }

The Flats plugin for these clients runs `flats mcp` instead; choose its host with `flats connect <url>`.
[exit=0]
```
$ claude --version
2.1.300 (Claude Code)
[exit=0]
```

```text
$ claude mcp add --transport http --scope project flats http://127.0.0.1:7878/mcp
Added HTTP MCP server flats with URL: http://127.0.0.1:7878/mcp to project config
File modified: /Users/user/study/flats-lab/.mcp.json
[exit=0]
```

```text
$ cat .mcp.json
{
  "mcpServers": {
    "flats": {
      "type": "http",
      "url": "http://127.0.0.1:7878/mcp"
    }
  }
}[exit=0]
```

```text
$ claude mcp list 2>&1 | grep -i -E 'flats|checking|^$' | head
Checking MCP server health…

flats: http://127.0.0.1:7878/mcp (HTTP) - ⏸ Pending approval (run `claude` to approve)
[exit=0]
```

```text
$ claude mcp get flats
flats:
  Scope: Project config (shared via .mcp.json)
  Status: ⏸ Pending approval (run `claude` to approve)
  Type: http
  URL: http://127.0.0.1:7878/mcp

To remove this server, run: claude mcp remove flats -s project
[exit=0]
```

- 프로젝트 범위 서버는 **첫 사용 전 승인(trust)** 이 필요해 `claude mcp list/get` 에서 `⏸ Pending approval (run claude to approve)` 로 표시된다. 이 승인은 Claude Code 의 보안 프롬프트이며 Flats 의 운영자 승인과 별개다. 비대화 모드에서 이 대기 상태를 풀려고 사용자 설정을 건드리지 않고, 새 세션에서 `--mcp-config .mcp.json`(같은 프로젝트 파일)로 명시 지정해 사용했다.

## 3. 새 Claude 세션(`claude -p`, 이 폴더)으로 작은 Page 저장

명령(원문):

```text
$ claude -p "<프롬프트: flats 도구 이름 나열 → save_page 로 slug mcp-hello Private Draft 만 저장, deploy·publish 금지, 승인 직접 처리 금지>" \
    --mcp-config .mcp.json --allowedTools "mcp__flats" --output-format text
[exit=0]
**1) flats 도구 목록**
check_client, create_flat, delete_env, delete_flat, deploy, get_approval, get_client_skill, get_content_types, get_design, get_document, get_draft, get_flat, get_logs, get_network, get_revision, get_runtime_reference, guide, list_env, list_flats, list_revisions, list_secrets, list_versions, open_preview, pin_revision, publish, rollback, rollback_revision, save_design, save_document, save_draft, save_page, save_version, save_version_from_dir, set_env, set_visibility, update_document

**2) 저장 결과**
`save_page`를 `deploy: false`로 호출해 slug `mcp-hello`, 제목 "MCP Hello"로 한 문단짜리 페이지를 저장했습니다. publish, deploy, 승인 요청은 하지 않았습니다.

**3) 상태**
- **status:** Draft revision 1로 저장됐습니다. `get_flat` 기준 `publication: unpublished`, `visibility: private`, `live_version: 0`, 저장된 버전은 0개입니다.
- **정식 URL 접근:** 불가능합니다. `private_url`이 비어 있고 `endpoints`도 없어서 지금 열 수 있는 주소가 없습니다.
- **미리보기 URL:** 없습니다. 열린 미리보기가 없고, 범위 밖이라 `open_preview`도 호출하지 않았습니다.
- **경고 2건(차단 아님):** 콘솔 썸네일용 screenshot이 없고, favicon이 없어 브라우저가 `/favicon.ico`를 요청하면 404가 납니다.

**다음 단계(승인 필요):** 먼저 `publish`를 요청하면 `approval_url`이 나오고, 운영자가 Flats 콘솔에서 승인해야 이 Draft가 번호 붙은 버전(v1)으로 저장됩니다. 그 뒤 `deploy {version: 1}`을 요청하면 다시 운영자 승인이 필요하고, 승인돼야 실제로 live가 되어 Private URL이 생깁니다.
```

### 도구 목록

위 세션이 열거한 36개: `check_client, create_flat, delete_env, delete_flat, deploy, get_approval, get_client_skill, get_content_types, get_design, get_document, get_draft, get_flat, get_logs, get_network, get_revision, get_runtime_reference, guide, list_env, list_flats, list_revisions, list_secrets, list_versions, open_preview, pin_revision, publish, rollback, rollback_revision, save_design, save_document, save_draft, save_page, save_version, save_version_from_dir, set_env, set_visibility, update_document`. 호스트의 `agent-guide.md` 는 "MCP has no approval tool" 이라고 하며 목록에도 `approve*` 도구가 없다. (`claude mcp list` 자체는 도구 이름을 출력하지 않는다.)

## 4. 호스트 쪽 교차 확인 (원문)

```text
$ flats list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
mcp-hello        page  private     -        0         
try-pod-doc      docs  private     running  -         http://try-pod-doc.localhost:7879
try-pod-network  page  private     -        0         
try-prom-talk    flat  private     -        0         
learning-hub     flat  private     -        0         
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:7879
k8s-network      page  private     -        0         
[exit=0]
```

```text
$ flats info mcp-hello
mcp-hello (MCP Hello)
  type:       page
  publication: unpublished
  visibility: private
  Draft:      revision 1 (base v0), dirty=true, type=page
  live:       not deployed
  versions:   0 (405 B on disk)
[exit=0]
```

```text
$ flats draft mcp-hello
mcp-hello Private Draft revision 1 (base v0), 2 files, 405 B; hash c522a589c1d7be560ee99149e32c18a4f78173d5bd35549892842b3d4d84bf56; type page
[exit=0]
```

```text
$ flats logs mcp-hello
2026-10-15 11:00:49  info   flat        flat created via mcp
2026-10-15 11:00:49  info   draft       saved draft revision 1 (2 files, 405 bytes) via mcp
[exit=0]
```

```text
$ curl -s -o /dev/null -w 'canonical %{http_code}\n' http://mcp-hello.localhost:7879/
canonical 404
[exit=0]
```

```text
$ flats approvals --status pending | grep -c mcp-hello
0
[exit=1]
```

```text
$ claude mcp get flats
flats:
  Scope: Project config (shared via .mcp.json)
  Status: ⏸ Pending approval (run `claude` to approve)
  Type: http
  URL: http://127.0.0.1:7878/mcp

To remove this server, run: claude mcp remove flats -s project
[exit=0]
```

```text
$ git -C /Users/user/study/flats-lab status --short -- .mcp.json
?? .mcp.json
[exit=0]
```

## 승인 전과 승인 후 차이

| 시점 | 관측 |
|---|---|
| 승인 전 (실측) | `flats list` 에 `mcp-hello page private LIVE - VERSIONS 0`, `flats info`: `publication: unpublished`, `live: not deployed`, Draft revision 1(2 files, 405 B). 이벤트 로그에 `flat created via mcp`, `saved draft revision 1 … via mcp`. 정식 URL `mcp-hello.localhost:7879` 404, 승인 대기 건 없음. `via` 값이 CLI 는 `cli`, 이 경로는 `mcp` |
| 승인 후 | **미실행.** 승인 결정 수단이 콘솔(브라우저)뿐이고 Chrome 확장이 연결되지 않아 `publish`·`deploy` 조차 요청하지 않았다 |

## 그 밖의 에이전트·플러그인 (설치하지 않음, 문서 기록만)

- **Claude Code 플러그인**: `claude plugin marketplace add gosuda/flats && claude plugin install flats@flats` 또는 세션 안에서 `/plugin install flats --marketplace gosuda/flats`. 플러그인은 `flats mcp`(stdio 릴레이)를 서버로 쓰고 호스트 선택은 `flats connect <url>` 로 한다(`flats-client/connection.json` 에 저장). 마켓플레이스·플러그인 설치는 전역 설정 변경이라 실행하지 않았다. 현재 이 환경의 `claude plugin marketplace list` 에는 `anthropic-plugin-directory` 만 있고 flats 플러그인은 없다.
- **Codex**: `codex mcp add flats --url http://127.0.0.1:7878/mcp` 또는 `~/.codex/config.toml` 에 `[mcp_servers.flats] url = "…"`; `codex exec` 용으로 `default_tools_approval_mode = "approve"` 가 필요하다고 `flats mcp-config` 가 안내. `codex plugin marketplace add gosuda/flats && codex plugin add flats@flats` 도 있다. 이 머신 `codex mcp list` 에는 `flats` 항목이 없다(`awk '{print $1}' | grep -cx flats` → 0). 설정은 바꾸지 않았다.
- **Cursor**: `.cursor/mcp.json` 에 `{"mcpServers":{"flats":{"url":"…"}}}`.

## 결론

- 프로젝트 범위 등록은 한 줄이고 `.mcp.json` 에 `{"type":"http","url":"http://127.0.0.1:7878/mcp"}` 만 기록된다. 파일은 git 추적 대상(`??`)이라 팀과 공유될 수 있다. 인증 헤더는 없다.
- 새 세션에서 에이전트는 도구 36개를 보고, `save_page` 로 Draft 만 만들 수 있었고, 스스로 정식 URL 이 없다는 사실과 다음 승인 단계를 정확히 설명했다. 에이전트가 만든 Draft 에는 favicon·screenshot 경고(차단 아님)가 달렸다.
- 에이전트 경로(MCP)와 CLI 모두 `publish`/`deploy` 는 승인 요청만 만들 수 있고 결정할 수 없다. 로컬 프로세스가 콘솔 CSRF 헤더만 맞추면 승인할 수 있다는 점은 호스트 문서가 직접 인정한다(`README` Security).

## Needs Verification

- 승인 후 에이전트 세션의 `get_flat`·`get_approval` 폴링 흐름과 `live_version`·URL 변화.
- 대화형 `claude` 에서 프로젝트 MCP 를 승인한 뒤 `claude mcp list` 가 `✓ Connected` 로 바뀌는지.
- `claude -p` 가 `.mcp.json` 을 `--mcp-config` 없이도 읽는지(첫 승인 필요 때문에 시도하지 않음).
- 플러그인 설치 후 skill·MCP 번들 동작(설치 안 함).
