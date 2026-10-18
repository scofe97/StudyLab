# 05 테스트 C — Website (v1 → v2 → 롤백)

실행일: 2026-10-15. 원본: `examples/site-learning-hub/{flats.json,index.html,networking.html,storage.html}`. slug `learning-hub`.

**상태: 부분 완료 — Draft 저장·미리보기·승인 요청까지. v1 저장 승인 이후(v1 publish, v2, 롤백)는 승인 불가로 미실행.**

## 실측 (원문)

### Draft 저장, 미리보기, publish 요청

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://learning-hub.localhost:7879/
404
[exit=0]
```

```text
$ flats deploy ./examples/site-learning-hub --flat learning-hub --save-only --message 'test C: v1 draft'
Saved Private Draft revision 1 of learning-hub: 4 files, 3.5 kB (type flat)
Current version is unchanged. Request approval with `flats publish learning-hub`, or preview with `flats preview learning-hub`.
Took 38ms (pack 3ms, upload 1.1 kB, server round trip 20ms)
[exit=0]
```

```text
$ flats info learning-hub
learning-hub (Learning Hub)
  type:       flat
  publication: unpublished
  visibility: private
  Draft:      revision 1 (base v0), dirty=true, type=flat
  live:       not deployed
  versions:   0 (3.5 kB on disk)
[exit=0]
```

```text
$ flats preview learning-hub
Preview of learning-hub Private Draft revision 1: http://learning-hub-pwtok4l7.localhost:7879
  Private via loopback or existing tailnet ACL; closes on the next deploy or after 24h without visits (now: 2026-10-16 10:57)
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}\n' http://learning-hub.localhost:7879/
404
[exit=0]
```

```text
$ flats publish learning-hub
This action needs the operator's approval. Share the approval link with them; nothing changes until they approve it in the web console.
Status: pending_approval
Approval needed: http://127.0.0.1:7878/approvals/apr-zn27nzgynbpk
Approval apr-zn27nzgynbpk is waiting for the operator; check with `flats approvals`.
[exit=3]
```

```text
$ flats approvals
ID                FLAT          ACTION           STATUS   VIA  REQUESTED         REASON
apr-zn27nzgynbpk  learning-hub  publish private  pending  cli  2026-10-15 10:57  
apr-96p1503rvmiz  k8s-network   publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network   publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

### 미리보기 호스트의 쪽별 응답

```text
$ curl -s -o /dev/null -w '%{http_code}  /\n' http://learning-hub-pwtok4l7.localhost:7879/
200  /
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}  /networking.html\n' http://learning-hub-pwtok4l7.localhost:7879/networking.html
200  /networking.html
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}  /storage.html\n' http://learning-hub-pwtok4l7.localhost:7879/storage.html
200  /storage.html
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}  /nope.html\n' http://learning-hub-pwtok4l7.localhost:7879/nope.html
404  /nope.html
[exit=0]
```

```text
$ curl -s -o /dev/null -w '%{http_code}  /flats.json\n' http://learning-hub-pwtok4l7.localhost:7879/flats.json
404  /flats.json
[exit=0]
```

```text
$ curl -s http://learning-hub-pwtok4l7.localhost:7879/ | grep -o 'version marker: v[0-9]*'
version marker: v1
[exit=0]
```

## 시점별 `curl` 결과

| 시점 | `http://learning-hub.localhost:7879/` | 미리보기 `learning-hub-pwtok4l7` |
|---|---|---|
| flat 생성 전 | 404 | – |
| Draft revision 1 저장 후 | 404 | `/`·`networking.html`·`storage.html` 200 (위 표) |
| v1 저장 승인 대기(pending) | 404 | 200 |
| v1 저장 승인 후 / v1 publish 후 | **미실행** | – |
| v2 수정·저장·publish 후 | **미실행** | – |
| v1 롤백 후 | **미실행** | – |

## 막힌 지점

테스트 A 와 동일하다. 버전 저장과 Publish, 롤백은 모두 콘솔에서만 승인할 수 있고 Chrome 확장이 연결되지 않았다. 승인 대기 중: `http://127.0.0.1:7878/approvals/apr-zn27nzgynbpk`(publish, Draft revision 1).

## 사용자가 승인 후 이어서 할 명령

```sh
flats deploy --flat learning-hub --version 1      # v1 활성화 요청 → 승인
curl -s -o /dev/null -w '%{http_code}\n' http://learning-hub.localhost:7879/
# examples/site-learning-hub/*.html 의 "version marker: v1" 을 v2 로 수정
flats deploy ./examples/site-learning-hub --flat learning-hub   # v2 저장 승인 요청
flats deploy --flat learning-hub --version 2                    # v2 활성화 승인
flats rollback learning-hub --to 1                               # v1 롤백 승인
```

## 결론

- Website 는 Page 와 같은 Draft → 버전 저장 → 활성화 흐름이며, Draft 는 폴더 안 파일 4개(`flats.json` 포함)가 그대로 번들에 들어간다.
- `flats.json` 은 번들에는 들어가지만 미리보기에서 `/flats.json` 은 404 라 서빙되지 않는다. 없는 경로(`/nope.html`)도 404.
- 정식 URL 은 활성화 승인 전까지 404, 미리보기 URL 은 Draft 단계에서도 200 이다.
- 롤백·v2 동작은 실측하지 못했다.

## Needs Verification

- v1/v2 저장 승인 후 `flats versions` 출력, 활성 전환 시점의 응답 변화, 롤백 승인 요청 형식(`flats rollback --to`).
- `flats.json` 의 `kind: static` 명시가 필요한지(생략 가능한지) — 이번엔 명시했다.
