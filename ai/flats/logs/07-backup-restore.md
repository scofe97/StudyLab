# 07 백업과 복구

실행일: 2026-10-15 10:59. 절차 근거: `docs/release.md` "Upgrading a host"(서비스 정지 → 데이터 디렉터리 통복사 → 재시작)와 `docs/configuration.md` "What to back up".

## 1. 백업 (원문)

핫 복사 시험(`VACUUM INTO`)과 문서 절차(서비스 정지 후 통복사)를 둘 다 했다. 정지 구간은 약 5초(`serve.err.log` 의 `10:59:05 shutting down` → `10:59:10 serving`)였고, 그 사이 다른 세션이 만든 flat(`try-*`)이 같은 호스트에 있었으므로 영향이 있었을 수 있다.


```text
$ cat .gitignore; git check-ignore -v backups/x
.DS_Store
*.bak
backups/
.gitignore:3:backups/	backups/x
[exit=0]
```

```text
$ sqlite3 -readonly '/Users/user/Library/Application Support/Flats/flats.db' "VACUUM INTO '/Users/user/study/flats-lab/backups/hot-flats.db'"; ls -la backups/hot-flats.db; sqlite3 -readonly backups/hot-flats.db 'select slug,live_version from flats; pragma integrity_check;'
-rw-r--r--@ 1 user  staff  233472 Oct 15 10:59 backups/hot-flats.db
k8s-network|0
k8s-network-doc|2
learning-hub|0
try-prom-talk|0
try-pod-network|0
try-pod-doc|1
ok
[exit=0]
```

```text
$ launchctl bootout gui/$(id -u)/dev.flats.serve; sleep 2; pgrep -x flats || echo 'no flats process'
no flats process
[exit=0]
```

```text
$ flats status 2>&1 | head -3
server:  not reachable at http://127.0.0.1:7878
launchd: plist present but not loaded (/Users/user/Library/LaunchAgents/dev.flats.serve.plist); run `flats install` again
flats: cannot reach Flats at http://127.0.0.1:7878: dial tcp 127.0.0.1:7878: connect: connection refused
[exit=0]
```

```text
$ cp -Rp '/Users/user/Library/Application Support/Flats' 'backups/flats-data-20261015-1100' && ls -la 'backups/flats-data-20261015-1100' && du -sh 'backups/flats-data-20261015-1100'
total 472
drwx------@ 11 user  staff     352 Oct 15 10:59 .
drwx------@  4 user  staff     128 Oct 15 10:59 ..
drwx------@  3 user  staff      96 Oct 15 10:56 cache
-rw-------@  1 user  staff     172 Oct 15 10:52 config.json
drwx------@  9 user  staff     288 Oct 15 10:58 flats
-rw-r--r--@  1 user  staff  233472 Oct 15 10:59 flats.db
-rw-------@  1 user  staff       0 Oct 15 10:52 flats.lock
drwx------@  4 user  staff     128 Oct 15 10:52 logs
drwx------@  2 user  staff      64 Oct 15 10:59 run
drwx------@  3 user  staff      96 Oct 15 10:56 runtime
-rw-------@  1 user  staff      32 Oct 15 10:52 secret.key
6.6M	backups/flats-data-20261015-1100
[exit=0]
```

```text
$ launchctl enable gui/$(id -u)/dev.flats.serve && launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.flats.serve.plist
[exit=0]
```

```text
$ flats status | grep -E 'server:|launchd:'
server:  running at http://127.0.0.1:7878
launchd: loaded, running, pid 19878, last exit (never exited)
[exit=0]
```

```text
$ flats list; flats approvals
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
try-pod-doc      docs  private     running  -         http://try-pod-doc.localhost:7879
try-pod-network  page  private     -        0         
try-prom-talk    flat  private     -        0         
learning-hub     flat  private     -        0         
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:7879
k8s-network      page  private     -        0         
ID                FLAT             ACTION           STATUS   VIA  REQUESTED         REASON
apr-yfntfdj2husr  try-pod-network  publish private  pending  cli  2026-10-15 10:58  
apr-3qgsmkm5upn7  try-prom-talk    publish private  pending  cli  2026-10-15 10:58  
apr-zn27nzgynbpk  learning-hub     publish private  pending  cli  2026-10-15 10:57  
apr-96p1503rvmiz  k8s-network      publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network      publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ diff <(cd '/Users/user/Library/Application Support/Flats' && find . -type f -not -name '*-wal' -not -name '*-shm' -not -path './logs/*' -not -name flats.lock | sort) <(cd 'backups/flats-data-20261015-1100' && find . -type f -not -name '*-wal' -not -name '*-shm' -not -path './logs/*' -not -name flats.lock | sort) && echo 'file lists identical (excluding wal/shm/logs/lock)'
30,39d29
< ./runtime/docs/cc414c3a6386-c102675bbb244f55/BUILD-INPUTS.sha256
< ./runtime/docs/cc414c3a6386-c102675bbb244f55/content.js
< ./runtime/docs/cc414c3a6386-c102675bbb244f55/mermaid-asset.js
< ./runtime/docs/cc414c3a6386-c102675bbb244f55/server.js
< ./runtime/docs/cc414c3a6386-c102675bbb244f55/THIRD_PARTY_LICENSES.txt
< ./runtime/docs/cc414c3a6386-d4fc8a110d7a252a/BUILD-INPUTS.sha256
< ./runtime/docs/cc414c3a6386-d4fc8a110d7a252a/content.js
< ./runtime/docs/cc414c3a6386-d4fc8a110d7a252a/mermaid-asset.js
< ./runtime/docs/cc414c3a6386-d4fc8a110d7a252a/server.js
< ./runtime/docs/cc414c3a6386-d4fc8a110d7a252a/THIRD_PARTY_LICENSES.txt
[exit=1]
```

```text
$ cat '/Users/user/Library/Application Support/Flats/logs/serve.err.log' | tail -5
2026/10/15 10:54:08 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
2026/10/15 10:57:57 shutting down
2026/10/15 10:57:58 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
2026/10/15 10:59:05 shutting down
2026/10/15 10:59:10 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
[exit=0]
```

- 정지 중 `flats status` 는 `not reachable`·`plist present but not loaded` 를 출력했다.
- 재시작은 `docs/release.md` 가 아니라 설치 스크립트가 출력하는 명령(`launchctl enable … && launchctl bootstrap …`, `install.sh` 의 `start_command`)을 썼다. `launchctl bootout` 은 `release.md` 에 적힌 정지 명령이다.
- 복사본과 원본의 파일 목록은 `runtime/docs/*`(서버가 시작할 때 다시 만드는 생성물) 외에 동일했다.
- 백업 위치: `~/study/flats-lab/backups/flats-data-20261015-1100`(6.6M, `.gitignore` 의 `backups/` 로 무시됨, 디렉터리 0700, `secret.key` 0600 유지).

## 2. 복구 시험 — 임시 디렉터리 + 다른 포트의 foreground 호스트

운영 호스트(`~/Library/Application Support/Flats`, 7878/7879)는 건드리지 않았다.

### 시도 1: `flats config set` 으로 임시 config 수정 — 거부됨

복사한 `config.json` 의 `host.data_dir` 이 여전히 운영 디렉터리를 가리켜서 공식 `flats config set` 이 운영 디렉터리의 락을 잡으려다 실패했다. 이는 안전장치로서는 올바르게 동작한 것이다.

### 시도 2: 복사본 `config.json` 을 직접 편집 후 foreground 기동 (원문)
$ flats config set host.data_dir '/Users/user/study/flats-lab/backups/restore-test' --config '/Users/user/study/flats-lab/backups/restore-test/config.json'
flats: data directory "/Users/user/Library/Application Support/Flats" is already in use: another Flats process holds the data directory lock: stop the running Flats host first
[exit=78]
```

```text
$ flats config set host.management_addr 127.0.0.1:17878 --config '/Users/user/study/flats-lab/backups/restore-test/config.json'
flats: data directory "/Users/user/Library/Application Support/Flats" is already in use: another Flats process holds the data directory lock: stop the running Flats host first
[exit=78]
```

```text
$ flats config set host.local_addr 127.0.0.1:17879 --config '/Users/user/study/flats-lab/backups/restore-test/config.json'
flats: data directory "/Users/user/Library/Application Support/Flats" is already in use: another Flats process holds the data directory lock: stop the running Flats host first
[exit=78]
```

```text
$ flats config validate --config '/Users/user/study/flats-lab/backups/restore-test/config.json'
/Users/user/study/flats-lab/backups/restore-test/config.json is valid
[exit=0]
```

```text
$ cat '/Users/user/study/flats-lab/backups/restore-test/config.json'
{
  "schema_version": 1,
  "host": {
    "instance_id": "495bff55-bfca-445f-8c61-c101f9a27f1c",
    "data_dir": "/Users/user/Library/Application Support/Flats"
  }
}
[exit=0]
```

```text
$ cat '/Users/user/study/flats-lab/backups/restore-test/config.json'
{
  "schema_version": 1,
  "host": {
    "instance_id": "495bff55-bfca-445f-8c61-c101f9a27f1c",
    "data_dir": "/Users/user/study/flats-lab/backups/restore-test",
    "management_addr": "127.0.0.1:17878",
    "local_addr": "127.0.0.1:17879"
  }
}[exit=0]
```

```text
$ flats config validate --config '/Users/user/study/flats-lab/backups/restore-test/config.json'
/Users/user/study/flats-lab/backups/restore-test/config.json is valid
[exit=0]
```

```text
$ flats config show --config '/Users/user/study/flats-lab/backups/restore-test/config.json' | head -12
# /Users/user/study/flats-lab/backups/restore-test/config.json (sha256 b86f9a16a4adfd5ae2d1244d467e4eac61921b6ceaaa78aa4557c72e535cb361)
{
  "schema_version": 1,
  "host": {
    "instance_id": "495bff55-bfca-445f-8c61-c101f9a27f1c",
    "data_dir": "/Users/user/study/flats-lab/backups/restore-test",
    "management_addr": "127.0.0.1:17878",
    "local_addr": "127.0.0.1:17879"
  }
}
# effective values (source)
host.instance_id                   "495bff55-bfca-445f-8c61-c101f9a27f1c" (file)
[exit=0]
```

```text
$ cat backups/restore-serve.log
2026/10/15 10:59:26 flats v0.9.1 serving; console http://127.0.0.1:17878 (loopback http://127.0.0.1:17878)
[exit=0]
```

```text
$ lsof -nP -iTCP:17878 -iTCP:17879 -sTCP:LISTEN
COMMAND   PID       USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
flats   20251 user   10u  IPv4 0x94a51db9affe3e09      0t0  TCP 127.0.0.1:17879 (LISTEN)
flats   20251 user   11u  IPv4 0x2f8283e9c2914f2b      0t0  TCP 127.0.0.1:17878 (LISTEN)
[exit=0]
```

```text
$ flats --url http://127.0.0.1:17878 status | head -12
server:  running at http://127.0.0.1:17878
  system: {
    "build": {
      "arch": "arm64",
      "commit": "e681bb5ca50c770229abe431810b3aa72564a243",
      "dirty": false,
      "go": "go1.27.1",
      "os": "darwin",
      "release": true,
      "version": "v0.9.1"
    },
    "console_url": "http://127.0.0.1:17878",
[exit=0]
```

```text
$ flats --url http://127.0.0.1:17878 list
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
try-pod-doc      docs  private     running  -         http://try-pod-doc.localhost:17879
try-pod-network  page  private     -        0         
try-prom-talk    flat  private     -        0         
learning-hub     flat  private     -        0         
k8s-network-doc  docs  private     running  -         http://k8s-network-doc.localhost:17879
k8s-network      page  private     -        0         
[exit=0]
```

```text
$ flats --url http://127.0.0.1:17878 approvals
ID                FLAT             ACTION           STATUS   VIA  REQUESTED         REASON
apr-yfntfdj2husr  try-pod-network  publish private  pending  cli  2026-10-15 10:58  
apr-3qgsmkm5upn7  try-prom-talk    publish private  pending  cli  2026-10-15 10:58  
apr-zn27nzgynbpk  learning-hub     publish private  pending  cli  2026-10-15 10:57  
apr-96p1503rvmiz  k8s-network      publish private  pending  cli  2026-10-15 10:56  
apr-47ecg86fq2fh  k8s-network      publish private  pending  cli  2026-10-15 10:55  
[exit=0]
```

```text
$ curl -s -o /dev/null -w 'doc healthz %{http_code}\n' http://k8s-network-doc.localhost:17879/_docs/healthz
doc healthz 200
[exit=0]
```

```text
$ python3 -I /private/tmp/claude-501/-Users-user-study-flats-lab/982efcab-7f67-4652-8ade-b4b9525def1a/scratchpad/mcp17878.py get_document '{"slug":"k8s-network-doc"}' | grep -o 'hash.....[0-9a-f]\{64\}' | head -1
hash\":\"b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad
[exit=0]
```

```text
$ shasum -a 256 examples/doc-k8s-network/index.md
b97c953de0eae7c7269ddeecd889794ff0860a64195d4e7face858ad5d6bbfad  examples/doc-k8s-network/index.md
[exit=0]
```

```text
$ flats --url http://127.0.0.1:17878 draft k8s-network; flats --url http://127.0.0.1:17878 draft learning-hub
k8s-network Private Draft revision 2 (base v0), 2 files, 11.2 kB; hash 6167b7d7ea643288b100471ed7118aa6c6de1bf174542f291ed327e4207dd485; type page
learning-hub Private Draft revision 1 (base v0), 4 files, 3.5 kB; hash f8fc83ac01e931d9cbd6d333d12caa45f6e49fc69b27670e3530e4ab0563aa40; type flat
[exit=0]
```

```text
$ flats --url http://127.0.0.1:17878 preview k8s-network
Preview of k8s-network Private Draft revision 2: http://k8s-network-ia9whoqr.localhost:17879
  Private via loopback or existing tailnet ACL; closes on the next deploy or after 24h without visits (now: 2026-10-16 10:59)
[exit=0]
```

```text
$ curl -s -o /dev/null -w 'restored preview %{http_code}\n' $(flats --url http://127.0.0.1:17878 preview k8s-network | head -1 | grep -o 'http://[^ ]*')
restored preview 200
[exit=0]
```

```text
$ diff <(cd backups/flats-data-20261015-1100/flats/k8s-network/draft-revs/2 && shasum -a 256 index.html) <(cd backups/restore-test/flats/k8s-network/draft-revs/2 && shasum -a 256 index.html) && echo same-content
same-content
[exit=0]
```

```text
$ pgrep -x flats | xargs ps -o pid,command -p
  PID COMMAND
19878 /Users/user/.local/bin/flats serve --config /Users/user/Library/Application Support/Flats/config.json
19890 /Users/user/.local/bin/flats worker
19894 /Users/user/.local/bin/flats worker
20251 flats serve --config /Users/user/study/flats-lab/backups/restore-test/config.json
20253 /Users/user/.local/bin/flats worker
20256 /Users/user/.local/bin/flats worker
[exit=0]
```


```text
$ pgrep -x flats | xargs ps -o pid,command -p
  PID COMMAND
19878 /Users/user/.local/bin/flats serve --config /Users/user/Library/Application Support/Flats/config.json
19890 /Users/user/.local/bin/flats worker
19894 /Users/user/.local/bin/flats worker
[exit=0]
```

```text
$ lsof -nP -iTCP:17878 -iTCP:17879 -sTCP:LISTEN || echo 'restore host ports closed'
restore host ports closed
[exit=0]
```

```text
$ cat backups/restore-serve.log
2026/10/15 10:59:26 flats v0.9.1 serving; console http://127.0.0.1:17878 (loopback http://127.0.0.1:17878)
2026/10/15 10:59:44 shutting down
[exit=0]
```

```text
$ flats status | grep -E 'server:|launchd:'; flats list | head -3
server:  running at http://127.0.0.1:7878
launchd: loaded, running, pid 19878, last exit (never exited)
SLUG             TYPE  VISIBILITY  LIVE     VERSIONS  URL
try-pod-doc      docs  private     running  -         http://try-pod-doc.localhost:7879
try-pod-network  page  private     -        0         
[exit=0]
```

```text
$ ls -la backups; du -sh backups/*; ls -la backups/flats-data-20261015-1100/secret.key
total 464
drwx------@  6 user  staff     192 Oct 15 10:59 .
drwxr-xr-x@ 12 user  staff     384 Oct 15 10:59 ..
drwx------@ 11 user  staff     352 Oct 15 10:59 flats-data-20261015-1100
-rw-r--r--@  1 user  staff  233472 Oct 15 10:59 hot-flats.db
-rw-r--r--@  1 user  staff     141 Oct 15 10:59 restore-serve.log
drwx------@ 10 user  staff     320 Oct 15 10:59 restore-test
6.6M	backups/flats-data-20261015-1100
228K	backups/hot-flats.db
4.0K	backups/restore-serve.log
6.6M	backups/restore-test
-rw-------@ 1 user  staff  32 Oct 15 10:52 backups/flats-data-20261015-1100/secret.key
[exit=0]
```

## 결론

| 질문 | 답 |
|---|---|
| 서비스 중단이 필요한가 | 문서 절차(`release.md`)는 **필요**하다고 쓴다(`launchctl bootout` 후 복사). 이번에 정지 시간은 약 5초. `configuration.md` 는 "`VACUUM INTO` 또는 호스트 정지 상태에서만, 살아 있는 DB 를 평범하게 복사하지 말라"고 쓴다 |
| DB 파일 하나로 충분한가 | **아니다.** `flats.db` 에는 메타데이터·승인·해시만 있고 콘텐츠는 `flats/<slug>/{draft-revs,versions}/` 파일에 있다. 문서의 편집기 상태는 `flats/<slug>/data/db.sqlite`. 그리고 `secret.key` 와 `config.json` 이 함께 있어야 한다. `VACUUM INTO` 결과(`hot-flats.db`)는 이 중 DB 한 개뿐이다 |
| 키·비밀 파일 | `secret.key`(32바이트, 0600): 서버 flat 비밀값 암호화 키로 `flats.db` 와 같은 시점의 것이 필요(문서). `config.json`(0600). Tailscale 등 provider 신원은 `tsnet/` `portal/` 등(이번엔 없음). **이 백업 디렉터리에는 `secret.key` 가 평문 파일로 들어가므로 백업 자체가 비밀 취급 대상**이다 |
| 복구가 되는가 | 되었다. 복사본의 flat 6개, 대기 승인 5건, 문서 해시(`b97c953d…`, 원본 파일과 일치), Page Draft 해시, 미리보기 200 이 모두 복원됐다 |
| 추가 작업이 필요한가 | `host.data_dir` 을 새 위치로 고쳐야 했다(오프라인 `flats config set` 은 옛 경로 락 때문에 실패 → 파일 직접 편집). `flats config rebind` 는 필요하지 않았다 |
| 복구본의 사이드 이펙트 | 복구 호스트에서 `flats preview` 를 실행해 복구본 DB 에만 새 preview 행이 생겼다(운영 DB 아님). 복구 호스트 종료 후 7878/7879 서비스는 변함없이 실행 중 |

## Needs Verification

- `docs/release.md` 는 "서비스를 정지하고 복사"를 요구하는데 서비스 정지 없이 `cp -R` 로 복사한 결과가 일관적인지는 시험하지 않았다(위험하다고 문서가 명시하므로 일부러 안 함).
- 승인된 버전(`versions/<n>`)과 데이터 스냅샷(`snapshots/`)이 있는 상태에서의 복구(승인 불가로 버전 없음).
- 서버 flat 의 비밀(`secrets`)이 있는 상태에서 `secret.key` 가 다르면 어떻게 되는지.
- 복구본을 운영 위치에 덮어쓰는 절차(금지 사항이라 하지 않음).
- `config rebind` 가 필요한 조건(다른 `instance_id` 의 config 와 DB 를 짝지을 때로 추정).
