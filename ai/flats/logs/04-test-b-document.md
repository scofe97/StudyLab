# 04 테스트 B — Markdown Document

실행일: 2026-10-15. 원본: `examples/doc-k8s-network/{index.md,flats.json}`(`{"type": "docs", ...}`). slug `k8s-network-doc`. 테스트 A 와 같은 내용을 Markdown(+Mermaid 도식, 표)으로 썼다. 공개 상태로는 바꾸지 않았다.

## 등록·접근·수정 (원문)

### 등록과 Private 접근

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network-doc.localhost:7879/
404
[exit=0]
```

```text
$ flats deploy ./examples/doc-k8s-network --flat k8s-network-doc
Saved document k8s-network-doc; it is running now (private) at http://k8s-network-doc.localhost:7879. Documents have no publish step.
[exit=0]
```

```text
$ flats list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:7879
k8s-network      page  private     -        0         
[exit=0]
```

```text
$ flats info k8s-network-doc
k8s-network-doc (Kubernetes 네트워크 흐름 (문서))
  type:       docs
  document:   running (no Draft, versions or publish step)
  visibility: private
  connection: ready
  provider:   local configured=true permitted=true ready=true state=ready audience=current http://k8s-network-doc.localhost:7879
  disk:       4.2 kB
  private: http://k8s-network-doc.localhost:7879
[exit=0]
```

```text
$ flats --json info k8s-network-doc
{
  "type": "docs",
  "slug": "k8s-network-doc",
  "name": "Kubernetes 네트워크 흐름 (문서)",
  "visibility": "private",
  "live_version": 1,
  "created_at": "2026-10-15T01:56:33.963Z",
  "updated_at": "2026-10-15T01:56:38.045Z",
  "private_url": "http://k8s-network-doc.localhost:7879",
  "private_state": "ready",
  "live": {
    "flat": "k8s-network-doc",
    "number": 1,
    "hash": "baa5fc91e717019a44db12f1da06b7683f75047d0ddace2c7b3b7fdb9a0506a2",
    "size": 2098,
    "files": 2,
    "kind": "server",
    "manifest": {
      "type": "docs",
      "name": "Kubernetes 네트워크 흐름 (문서)",
      "kind": "server",
      "entry": "index.md",
      "health": "/_docs/healthz"
    },
    "git_dirty": false,
    "created_at": "2026-10-15T01:56:36.962Z",
    "pruned": false,
    "published": true,
    "role": "published",
    "deployed": true,
    "type": "docs"
  },
  "versions": 1,
  "latest_version": 1,
  "latest_deployed": true,
  "disk_bytes": 4196,
  "running": true,
  "publication": "published",
  "providers": [
    "local"
  ],
  "connection_state": "ready",
  "endpoints": [
    {
      "provider": "local",
      "url": "http://k8s-network-doc.localhost:7879",
      "state": "ready",
      "configured": true,
      "permitted": true,
      "ready": true,
      "audience": "current",
      "host": "k8s-network-doc"
    }
  ],
  "portal_listing": "default",
  "portal_hidden": false,
  "draft": {
    "flat": "k8s-network-doc",
    "revision": 1,
    "hash": "baa5fc91e717019a44db12f1da06b7683f75047d0ddace2c7b3b7fdb9a0506a2",
    "base_version": 1,
    "dirty": false,
    "size": 2098,
    "files": 2,
    "kind": "server",
    "manifest": {
      "type": "docs",
      "name": "Kubernetes 네트워크 흐름 (문서)",
      "kind": "server",
      "entry": "index.md",
      "health": "/_docs/healthz"
    },
    "git_dirty": false,
    "updated_at": "2026-10-15T01:56:38.054Z",
    "created_at": "2026-10-15T01:56:33.966Z",
    "type": "docs"
  }
}
[exit=0]
```

### 접근 결과 — 껍데기만 오고 본문은 웹소켓

```text
$ curl -s -i http://k8s-network-doc.localhost:7879/ | head -25
HTTP/1.1 200 OK
Cache-Control: no-store
Content-Length: 1268
Content-Security-Policy: default-src 'self'; script-src 'self'; connect-src 'self' ws://k8s-network-doc.localhost:7879 wss://k8s-network-doc.localhost:7879; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'
Content-Type: text/html; charset=utf-8
Date: Thu, 15 Oct 2026 01:56:43 GMT
Referrer-Policy: same-origin
X-Content-Type-Options: nosniff

<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Kubernetes 네트워크 흐름 (문서) · Flats</title><link rel="stylesheet" href="/_docs/assets/client.css"><script type="module" src="/_docs/assets/client.js"></script></head><body data-doc="index.md" data-readonly="false"><header><h1>Kubernetes 네트워크 흐름 (문서)</h1><div id="modes" role="group" aria-label="Document mode"><button type="button" data-mode="edit" aria-pressed="true">Edit</button><button type="button" data-mode="view" aria-pressed="false">View</button></div><span id="status" role="status" aria-live="polite">Connecting…</span><div id="presence" aria-label="People in this document"></div></header><div id="notice" role="alert" hidden></div><div id="conflict-notice" role="status" hidden></div><div class="workspace"><nav id="documents" aria-label="Documents" hidden></nav><main id="page"><div class="surface"><section id="editor" aria-label="Document editor" hidden></section><article id="content" aria-label="Document"></article><footer class="powered">Powered by <a href="https://github.com/gosuda/flats" target="_blank" rel="noopener noreferrer">Flats</a></footer></div></main></div></body></html>[exit=0]
```

```text
$ curl -s http://k8s-network-doc.localhost:7879/ | wc -c
    1268
[exit=0]
```

```text
$ curl -s http://k8s-network-doc.localhost:7879/ | grep -c 'CoreDNS'
0
[exit=1]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network-doc.localhost:7879/_docs/healthz
200
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network-doc.localhost:7879/index.md
200
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: k8s-network-doc.localhost:7879' http://127.0.0.1:7879/
200
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/
404
[exit=0]
```

### 수정 후 재배포 · 반영 확인 · 이벤트 로그

```text
$ curl -s -i http://k8s-network-doc.localhost:7879/index.md | head -12
HTTP/1.1 200 OK
Cache-Control: no-store
Content-Length: 1268
Content-Security-Policy: default-src 'self'; script-src 'self'; connect-src 'self' ws://k8s-network-doc.localhost:7879 wss://k8s-network-doc.localhost:7879; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'
Content-Type: text/html; charset=utf-8
Date: Thu, 15 Oct 2026 01:56:49 GMT
Referrer-Policy: same-origin
X-Content-Type-Options: nosniff

<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Kubernetes 네트워크 흐름 (문서) · Flats</title><link rel="stylesheet" href="/_docs/assets/client.css"><script type="module" src="/_docs/assets/client.js"></script></head><body data-doc="index.md" data-readonly="false"><header><h1>Kubernetes 네트워크 흐름 (문서)</h1><div id="modes" role="group" aria-label="Document mode"><button type="button" data-mode="edit" aria-pressed="true">Edit</button><button type="button" data-mode="view" aria-pressed="false">View</button></div><span id="status" role="status" aria-live="polite">Connecting…</span><div id="presence" aria-label="People in this document"></div></header><div id="notice" role="alert" hidden></div><div id="conflict-notice" role="status" hidden></div><div class="workspace"><nav id="documents" aria-label="Documents" hidden></nav><main id="page"><div class="surface"><section id="editor" aria-label="Document editor" hidden></section><article id="content" aria-label="Document"></article><footer class="powered">Powered by <a href="https://github.com/gosuda/flats" target="_blank" rel="noopener noreferrer">Flats</a></footer></div></main></div></body></html>[exit=0]
```

```text
$ curl -s http://k8s-network-doc.localhost:7879/index.md | shasum -a 256; shasum -a 256 examples/doc-k8s-network/index.md
73459f08b7ca6cd9077f46c42806ecdbaf6986d09fe28af1b1f8460c045b8d6f  -
71f76c536823c2db99f66ac490d826b08d0e6e95b5ba91be93bdf51dae65eb73  examples/doc-k8s-network/index.md
[exit=0]
```

```text
$ flats deploy ./examples/doc-k8s-network --flat k8s-network-doc
Saved document k8s-network-doc; it is running now (private) at http://k8s-network-doc.localhost:7879. Documents have no publish step.
[exit=0]
```

```text
$ curl -s http://k8s-network-doc.localhost:7879/index.md | grep -c '2차 저장'
0
[exit=1]
```

```text
$ curl -s http://k8s-network-doc.localhost:7879/index.md | shasum -a 256; shasum -a 256 examples/doc-k8s-network/index.md
73459f08b7ca6cd9077f46c42806ecdbaf6986d09fe28af1b1f8460c045b8d6f  -
b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad  examples/doc-k8s-network/index.md
[exit=0]
```

```text
$ flats versions k8s-network-doc
error: invalid request: documents have no versions, drafts, previews or publishing; save_document applies at once and update_document edits the live text
[exit=1]
```

```text
$ flats info k8s-network-doc
k8s-network-doc (Kubernetes 네트워크 흐름 (문서))
  type:       docs
  document:   running (no Draft, versions or publish step)
  visibility: private
  connection: ready
  provider:   local configured=true permitted=true ready=true state=ready audience=current http://k8s-network-doc.localhost:7879
  disk:       6.5 kB
  private: http://k8s-network-doc.localhost:7879
[exit=0]
```

```text
$ flats logs k8s-network-doc
2026-10-15 10:56:33  info   flat        flat created via cli
2026-10-15 10:56:33  info   draft       saved draft revision 1 (2 files, 2098 bytes) via cli
2026-10-15 10:56:34  error  runtime     flats: ignoring the saved connection: $HOME is not defined
2026-10-15 10:56:36  info   runtime     worker started in 2618ms
2026-10-15 10:56:36  info   health      health check of the saved document: GET /_docs/healthz -> 200 in 97ms 
2026-10-15 10:56:37  error  runtime     flats: ignoring the saved connection: $HOME is not defined
2026-10-15 10:56:38  info   runtime     worker started in 997ms
2026-10-15 10:56:38  info   deploy      document saved and running via cli
2026-10-15 10:56:49  info   draft       saved draft revision 2 (2 files, 2195 bytes) via cli
2026-10-15 10:56:49  error  runtime     flats: ignoring the saved connection: $HOME is not defined
2026-10-15 10:56:50  info   runtime     worker started in 1071ms
2026-10-15 10:56:50  info   health      health check of the saved document: GET /_docs/healthz -> 200 in 72ms 
2026-10-15 10:56:51  error  runtime     flats: ignoring the saved connection: $HOME is not defined
2026-10-15 10:56:52  info   runtime     worker started in 1347ms
2026-10-15 10:56:52  info   deploy      document saved and running via cli
[exit=0]
```

```text
$ flats approvals --status pending
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-96p1503rvmiz  k8s-network  publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

### 라이브 Markdown 을 MCP `get_document` 로 직접 읽기

`curl` 은 편집기 HTML 껍데기(1268바이트)만 받는다. 문서 본문은 웹소켓으로 전달되므로 서버가 가진 라이브 Markdown 은 로컬 MCP 엔드포인트(`http://127.0.0.1:7878/mcp`, 읽기 전용 도구 `get_document`)로 읽었다. 결과 원문: `logs/raw/04-mcp-get_document.json`.

- 반환된 `hash` = `b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad` — 수정한 `examples/doc-k8s-network/index.md` 의 SHA-256(위 `shasum` 출력)과 **같다**.
- 본문에 `> 2차 저장: 이 문장은 테스트 B 에서 …` 줄이 들어 있다 → 재배포가 라이브 문서에 반영됨.

## Page(테스트 A)와의 차이

| 항목 | Page (A) | Document (B) |
|---|---|---|
| 등록 직후 | Private Draft, 정식 URL 404, 미리보기만 200 | 즉시 실행, Private URL 200 (`private_state: ready`) |
| 승인 | 버전 저장·Publish 각각 승인 필요(CLI 는 exit 3) | **저장·수정에 승인 없음**. visibility 변경·삭제만 승인 |
| 버전 | `flats versions` 로 v1, v2… | `flats versions` → `error: invalid request: documents have no versions, drafts, previews or publishing` (exit 1) |
| 미리보기 | `flats preview` 별도 호스트명 | 해당 없음 |
| 서빙 방식 | 정적 파일 그대로 | 내장 협업 편집기 앱(kind `server`)이 렌더, 본문은 웹소켓 |
| 렌더 | 내가 쓴 HTML·CSS 그대로 | Markdown + Mermaid 만. 원시 HTML 은 렌더되지 않음(`docs/content-types.md:146`) |
| 사람이 수정 | 불가(저장 코드가 정본) | Private 에서 브라우저 편집 가능, 에이전트 저장은 줄 단위로 병합 |
| 롤백 | 버전 롤백 | 없음. 데이터 스냅샷도 없음 |

## 수정 반영 시점

- 첫 저장: 10:56:33(생성) → `deploy document saved and running` 10:56:38. 약 5초(워커 기동 2.6초 포함).
- 수정 저장: 10:56:49(draft revision 2) → `document saved and running` 10:56:52. 약 3초.
- 반영은 "재배포 명령 → 라이브 문서"이며, 별도 승인·publish 단계는 없다. 내부적으로 `draft revision N` 이 찍히지만 `flats versions`·`flats draft` 로는 접근할 수 없다(문서 내부 용어).

## Private 상태 접근 결과

- `http://k8s-network-doc.localhost:7879/` → 200, 편집기 쉘 `data-readonly="false"`, 상단에 Edit/View 토글. `127.0.0.1:7879` 에 Host 헤더만 바꿔 보내도 같은 응답(200), Host 없이 `127.0.0.1:7879` 는 404 — 호스트명으로 flat 을 구분한다.
- `Cache-Control: no-store`, CSP 가 `ws://k8s-network-doc.localhost:7879` 로의 연결만 허용.
- Private 이라 해도 **인증은 없다**: 루프백에 도달하는 프로세스는 편집기에 들어갈 수 있다(`README` Security 절: "no separate authentication").

## Public 으로 바꿨을 때(문서 근거만, 실행 안 함)

`docs/content-types.md:29-31`: Private 링크는 편집기를 열고 Edit/View 를 전환할 수 있으며, **Public 링크는 읽기 전용 렌더 문서**다. `:62-64`: Public 문서는 저장과 라이브 편집이 즉시 모두에게 보인다. 가시성 변경은 양방향 모두 승인이 필요하다. Public 은 Local 네트워크만으로는 만들 수 없고 공개 가능한 제공자(Portal/Funnel 등) 허가가 있어야 한다(`docs/networking.md`) — 이번 환경은 Local 뿐이라 시도하지 않았다.

## 결론

- Document 는 `flats deploy <dir>` 로 올리면 `flats.json` 의 `type: docs` 를 보고 문서로 처리해 **승인 없이 즉시** Private 실행된다. 같은 CLI 명령이 Page 에서는 Draft 만, 문서에서는 즉시 실행으로 갈린다.
- 수정 반영은 수 초. 서버 쪽 라이브 Markdown 해시가 로컬 파일 SHA-256 과 일치해 내용이 그대로 보존됨을 확인했다.
- `flats logs` 에 매 시작마다 `error runtime flats: ignoring the saved connection: $HOME is not defined` 가 찍힌다. 문서 워커 프로세스가 HOME 없이 뜬다는 뜻으로 보이며 동작에는 영향이 없었다(헬스체크 200). 원인·영향은 소스 미확인.

## Needs Verification

- 브라우저에서 편집기·Mermaid 가 실제로 어떻게 보이는지(렌더 스크린샷은 못 찍음: Chrome 확장 미연결).
- 사람이 브라우저에서 편집한 내용과 `flats deploy` 재저장이 병합되는 동작(줄 단위 병합, 충돌 시 saved text 우선)은 문서 근거만 있고 실측 안 함.
- `$HOME is not defined` 오류의 의미.
- Public 전환 시 읽기 전용 렌더(문서 근거만).
