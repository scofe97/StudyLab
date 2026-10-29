# 03 테스트 A — Page (Kubernetes 네트워크 흐름)

실행일: 2026-10-15. 원본: `examples/page-k8s-network/{index.html,flats.json}`. slug `k8s-network`.

**상태: 부분 완료 — Draft 저장·미리보기까지 실측, 버전 저장(승인)·Publish(승인)는 미실행.** 이유는 아래 "막힌 지점".

## 등록 방법 (공식 경로)

- Page 전용 CLI 명령은 없다. `flats deploy <dir> --flat <slug>` 가 업로드 폴더의 `flats.json` 에 `"type": "page"` 가 있으면 Page 로 취급한다(`docs/content-types.md` "Bundle rules": `flats deploy` 는 업로드에 매니페스트가 있어야 함). `flats.json` 내용: `{"type": "page", "name": "Kubernetes 네트워크 흐름"}`.
- 번들은 HTML 파일이 정확히 하나(entry), 나머지는 asset. 서버 코드·DB 없음.
- 승인: `flats publish`/`flats deploy` 는 승인 요청만 만들고 exit 3 을 반환한다. 승인 결정용 CLI 명령은 없다(`flats approvals` 는 목록뿐).

## 단계별 실측 (원문)

### 0. 등록 전

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network.localhost:7879/
404
[exit=0]
```

```text
$ flats list
No flats yet. Deploy one with `flats deploy <dir> --flat <slug>`.
[exit=0]
```

```text
$ flats deploy ./examples/page-k8s-network --flat k8s-network --save-only --message 'test A: draft only'
Saved Private Draft revision 1 of k8s-network: 3 files, 11.6 kB (type page)
Current version is unchanged. Request approval with `flats publish k8s-network`, or preview with `flats preview k8s-network`.
Took 44ms (pack 5ms, upload 4.9 kB, server round trip 29ms)
[exit=0]
```

```text
$ flats list
SLUG         TYPE  VISIBILITY  LIVE  VERSIONS  URL
k8s-network  page  private     -     0         
[exit=0]
```

```text
$ flats info k8s-network
k8s-network (Kubernetes 네트워크 흐름)
  type:       page
  publication: unpublished
  visibility: private
  Draft:      revision 1 (base v0), dirty=true, type=page
  live:       not deployed
  versions:   0 (11.6 kB on disk)
[exit=0]
```

### 1. Draft 저장 후 · 미리보기 · 승인 요청

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network.localhost:7879/
404
[exit=0]
```

```text
$ flats preview k8s-network
Preview of k8s-network Private Draft revision 1: http://k8s-network-6fgw0tb3.localhost:7879
  Private via loopback or existing tailnet ACL; closes on the next deploy or after 24h without visits (now: 2026-10-16 10:55)
[exit=0]
```

```text
$ flats --json info k8s-network
{
  "type": "page",
  "slug": "k8s-network",
  "name": "Kubernetes 네트워크 흐름",
  "visibility": "private",
  "live_version": 0,
  "created_at": "2026-10-15T01:55:13.87Z",
  "updated_at": "2026-10-15T01:55:13.887Z",
  "private_url": "",
  "versions": 0,
  "disk_bytes": 11558,
  "running": false,
  "publication": "unpublished",
  "providers": [
    "local"
  ],
  "endpoints": [
    {
      "provider": "local",
      "url": "http://k8s-network-6fgw0tb3.localhost:7879",
      "state": "ready",
      "configured": true,
      "permitted": true,
      "ready": true,
      "audience": "draft",
      "host": "k8s-network-6fgw0tb3"
    }
  ],
  "portal_listing": "default",
  "portal_hidden": false,
  "draft": {
    "flat": "k8s-network",
    "revision": 1,
    "hash": "b7b4e442240c5d4cb9830396e3ad7eb8dcfad53d4f387982fa34228ce93c7336",
    "base_version": 0,
    "dirty": true,
    "size": 11558,
    "files": 3,
    "kind": "static",
    "manifest": {
      "type": "page",
      "name": "Kubernetes 네트워크 흐름",
      "kind": "static",
      "entry": "index.html",
      "health": "/"
    },
    "git_dirty": false,
    "message": "test A: draft only",
    "updated_at": "2026-10-15T01:55:13.887Z",
    "created_at": "2026-10-15T01:55:13.887Z",
    "type": "page"
  }
}
[exit=0]
```

```text
$ flats publish k8s-network
This action needs the operator's approval. Share the approval link with them; nothing changes until they approve it in the web console.
Status: pending_approval
Approval needed: http://127.0.0.1:7878/approvals/apr-47ecg86fq2fh
Approval apr-47ecg86fq2fh is waiting for the operator; check with `flats approvals`.
[exit=3]
```

```text
$ flats approvals
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ flats --json approvals
{
  "approvals": [
    {
      "id": "apr-47ecg86fq2fh",
      "flat": "k8s-network",
      "action": "publish",
      "params": {
        "revision": 1,
        "hash": "b7b4e442240c5d4cb9830396e3ad7eb8dcfad53d4f387982fa34228ce93c7336",
        "base_version": 0,
        "visibility": "private",
        "providers": "local\naf8b9ae92444c00381d4272a52722a0ca5c56ec66f6e8f8d57f762da190c0be5"
      },
      "status": "pending",
      "via": "cli",
      "requested_at": "2026-10-15T01:55:17.462Z",
      "idempotency_key": "4e7b5b9e03844138706ea6fa8acc038aed91f544ef7a9fa5bb70420bdc2cdb62"
    }
  ]
}
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network-6fgw0tb3.localhost:7879/
200
[exit=0]
```

```text
$ curl -s -i http://k8s-network-6fgw0tb3.localhost:7879/ | head -12
HTTP/1.1 200 OK
Accept-Ranges: bytes
Cache-Control: no-cache
Content-Length: 11157
Content-Type: text/html; charset=utf-8
Etag: "b014ebabd738bae1ca86e328788a25c2"
X-Content-Type-Options: nosniff
X-Robots-Tag: noindex
Date: Thu, 15 Oct 2026 01:55:23 GMT

<!doctype html>
<html lang="ko">
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network.localhost:7879/
404
[exit=0]
```

### 2. 이벤트 로그 · 대기 승인

```text
$ flats logs k8s-network
2026-10-15 10:55:13  info   flat        flat created via cli
2026-10-15 10:55:13  info   draft       saved draft revision 1 (3 files, 11558 bytes) via cli
2026-10-15 10:55:17  info   preview     preview of draft revision 1 at http://k8s-network-6fgw0tb3.localhost:7879
2026-10-15 10:55:17  warn   approval    publish requested via cli; waiting for the operator: http://127.0.0.1:7878/approvals/apr-47ecg86fq2fh
[exit=0]
```

```text
$ flats approvals --status pending
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network.localhost:7879/
404
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://k8s-network-6fgw0tb3.localhost:7879/
200
[exit=0]
```

### 3. 예제 폴더에 섞인 훅 상태 파일 발견 → 제거 후 Draft revision 2 재저장

revision 1 은 `3 files` 였다. 원인은 작업 디렉터리를 예제 폴더로 두었을 때 OMC 훅이 만든 `examples/page-k8s-network/.omc/state/sessions/<id>/pre-tool-advisory-throttle.json` 이 번들에 포함된 것이었다(`find .omc -type f` 로 확인). 파일을 지우고 revision 2(2 files)로 다시 저장했다. revision 1 의 승인 요청 `apr-47ecg86fq2fh` 는 Draft 가 바뀌어 `stale_approval` 로 실패할 것이므로(`docs` 승인 규칙) 새 요청 `apr-96p1503rvmiz` 를 만들었다. 둘 다 아직 pending 이다.

```text
$ flats deploy ./examples/page-k8s-network --flat k8s-network --save-only --message 'test A: draft rev2 without stray .omc'
Saved Private Draft revision 2 of k8s-network: 2 files, 11.2 kB (type page)
Current version is unchanged. Request approval with `flats publish k8s-network`, or preview with `flats preview k8s-network`.
Took 63ms (pack 6ms, upload 4.6 kB, server round trip 46ms)
[exit=0]
```

```text
$ flats draft k8s-network
k8s-network Private Draft revision 2 (base v0), 2 files, 11.2 kB; hash 6167b7d7ea643288b100471ed7118aa6c6de1bf174542f291ed327e4207dd485; type page
[exit=0]
```

```text
$ flats approvals --status pending
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ flats approvals
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ flats publish k8s-network
This action needs the operator's approval. Share the approval link with them; nothing changes until they approve it in the web console.
Status: pending_approval
Approval needed: http://127.0.0.1:7878/approvals/apr-96p1503rvmiz
Approval apr-96p1503rvmiz is waiting for the operator; check with `flats approvals`.
[exit=3]
```

```text
$ flats approvals
ID                FLAT         ACTION           STATUS   VIA  REQUESTED         REASON
apr-96p1503rvmiz  k8s-network  publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network  publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ flats preview k8s-network
Preview of k8s-network Private Draft revision 2: http://k8s-network-guubxv0d.localhost:7879
  Private via loopback or existing tailnet ACL; closes on the next deploy or after 24h without visits (now: 2026-10-16 10:56)
[exit=0]
```

```text
$ flats info k8s-network
k8s-network (Kubernetes 네트워크 흐름)
  type:       page
  publication: unpublished
  visibility: private
  Draft:      revision 2 (base v0), dirty=true, type=page
  provider:   local configured=true permitted=true ready=true state=ready audience=draft http://k8s-network-6fgw0tb3.localhost:7879
  provider:   local configured=true permitted=true ready=true state=ready audience=draft http://k8s-network-guubxv0d.localhost:7879
  live:       not deployed
  versions:   0 (22.8 kB on disk)
[exit=0]
```

## 각 단계에서 URL 접근 결과

| 시점 | `http://k8s-network.localhost:7879/` (정식 URL) | 미리보기 URL |
|---|---|---|
| flat 생성 전 | 404 | 없음 |
| Draft revision 1 저장 직후 | 404 | `http://k8s-network-6fgw0tb3.localhost:7879/` → 200, `Cache-Control: no-cache`, `X-Robots-Tag: noindex` |
| publish 승인 요청 후(pending) | 404 | 200 |
| Draft revision 2 저장·재요청 후 | (미확인: 같은 404 로 추정) | `http://k8s-network-guubxv0d.localhost:7879` (rev 2, 별도 호스트명) |
| 버전 v1 저장(승인) 후 | **미실행** | – |
| v1 Publish(승인) 후 | **미실행** → 완료 조건의 "Page URL curl 결과"는 아직 200 으로 확인하지 못함 | – |

미리보기는 Draft 단계에서 자기 호스트명을 가지며 24시간 방문이 없거나 다음 deploy 때 닫힌다(출력 원문). 정식 URL 은 publish 승인과 deploy 승인이 끝나야 응답한다.

## 막힌 지점

- 승인은 콘솔(브라우저)에서만 결정된다. Claude in Chrome 확장이 이 세션에 연결되지 않았다(`tabs_context_mcp` 두 번 모두 "Browser extension is not connected").
- 호스트가 서빙하는 에이전트 가이드(`http://127.0.0.1:7878/docs/agent-guide.md` 의 "Approvals" 절)는 "Never approve your own request, call console routes or send console headers" 라고 적고 있다. 콘솔 CSRF 헤더를 직접 보내 승인하는 방법은 존재하지만 공식 에이전트 경로가 아니라고 판단해 쓰지 않았다.
- 승인 대기 URL: `http://127.0.0.1:7878/approvals/apr-96p1503rvmiz` (publish, Draft revision 2).

## 결론

- Page 는 CLI 만으로 Draft 생성·미리보기·승인 요청까지 가능하고, 정식 URL 은 승인 전 404 다.
- Draft 저장은 `.omc` 같은 숨김 파일까지 폴더째 묶는다. 업로드 폴더에 의도하지 않은 파일이 섞이면 번들에 그대로 들어가며 경고는 없었다(출력에 warnings 줄 없음).
- 승인 이후 단계(v1 저장, v1 활성화)는 사용자가 위 URL 에서 승인한 뒤 `flats deploy --flat k8s-network --version 1` 부터 이어서 하면 된다.

## Needs Verification

- v1 저장 승인 후 `versions` 목록·`live` 값, publish 후 `curl` 200 여부, 승인 전후 응답 헤더 차이.
- Draft revision 2 저장 후 revision 1 미리보기 URL 이 계속 200 인지(문서는 "다음 deploy 때 닫힘").
- 미리보기 출력의 "now: 2026-10-16 10:55" 는 실제 현재 시각(2026-10-15 10:55)보다 24시간 뒤다. 만료 시각을 "now" 로 잘못 표기하는 것으로 보이나 소스 미확인.
- 승인 요청 두 건 중 첫 건이 실제로 stale 로 바뀌는지(승인 전엔 `pending` 으로만 표시).
