# 02 런타임 구조 — 데이터 디렉터리·포트·로그·launchd

실행일: 2026-10-15. 서비스는 01 단계에서 설치된 launchd 에이전트 `dev.flats.serve`.

## 1. 데이터 디렉터리와 파일 (find 원문)

```text
$ find "$HOME/Library/Application Support/Flats" -not -path '*/runtime/*'
/Users/user/Library/Application Support/Flats
/Users/user/Library/Application Support/Flats/flats.db-wal
/Users/user/Library/Application Support/Flats/flats.db-shm
/Users/user/Library/Application Support/Flats/config.json
/Users/user/Library/Application Support/Flats/logs
/Users/user/Library/Application Support/Flats/secret.key
/Users/user/Library/Application Support/Flats/flats.db
/Users/user/Library/Application Support/Flats/flats.lock
/Users/user/Library/Application Support/Flats/flats
/Users/user/Library/Application Support/Flats/logs/serve.out.log
/Users/user/Library/Application Support/Flats/logs/serve.err.log
/Users/user/Library/Application Support/Flats/flats/.lifecycle
/Users/user/Library/Application Support/Flats/flats/.lifecycle/orphans
/Users/user/Library/Application Support/Flats/flats/.lifecycle/deleted

$ du -sh; ls -la
264K	/Users/user/Library/Application Support/Flats
total 520
drwx------@  10 user  staff     320 Oct 15 10:54 .
drwx------+ 132 user  staff    4224 Oct 15 10:52 ..
-rw-------@   1 user  staff     172 Oct 15 10:52 config.json
drwx------@   3 user  staff      96 Oct 15 10:52 flats
-rw-r--r--@   1 user  staff  225280 Oct 15 10:52 flats.db
-rw-r--r--@   1 user  staff   32768 Oct 15 10:54 flats.db-shm
-rw-r--r--@   1 user  staff       0 Oct 15 10:54 flats.db-wal
-rw-------@   1 user  staff       0 Oct 15 10:52 flats.lock
drwx------@   4 user  staff     128 Oct 15 10:52 logs
-rw-------@   1 user  staff      32 Oct 15 10:52 secret.key
```

## 2. config.json 과 유효값

```text
$ cat config.json
{
  "schema_version": 1,
  "host": {
    "instance_id": "495bff55-bfca-445f-8c61-c101f9a27f1c",
    "data_dir": "/Users/user/Library/Application Support/Flats"
  }
}

$ flats config show
# /Users/user/Library/Application Support/Flats/config.json (sha256 510c7fdc8f5a464ec1ba9c997896af923cfda974e0eb0d5a787adb35229c6ef4)
{
  "schema_version": 1,
  "host": {
    "instance_id": "495bff55-bfca-445f-8c61-c101f9a27f1c",
    "data_dir": "/Users/user/Library/Application Support/Flats"
  }
}

# effective values (source)
host.instance_id                   "495bff55-bfca-445f-8c61-c101f9a27f1c" (file)
host.data_dir                      "/Users/user/Library/Application Support/Flats" (file)
host.management_addr               "127.0.0.1:7878" (default)
host.local_addr                    "127.0.0.1:7879" (default)
host.console_host                  "flats" (default)
host.server_runtime                true (default)
network.permitted                  [] (default)
network.private_backend            "local" (default)
network.default_private            "" (default)
network.default_public             "" (default)
system.upload_max_bytes            20971520 (default)
system.keep_versions               10 (default)
system.disk_quota_bytes            32212254720 (default)
system.preview_ttl_seconds         86400 (default)
system.rate_limit_rps              50 (default)
system.redirect_days               7 (default)
system.events_keep                 5000 (default)
system.revision_quota              100 (default)
system.pin_quota                   20 (default)
portal.relays                      [] (default)
portal.discovery                   true (default)
portal.max_active_relays           3 (default)
portal.hide                        false (default)
zrok.environment                   null (default)
zrok.namespace                     "public" (default)
netbird.management_url             "https://api.netbird.io:443" (default)
netbird.device_name                "flats" (default)
credentials.operator_file          null (default)
credentials.tailscale_authkey_file null (default)
$ flats config validate
/Users/user/Library/Application Support/Flats/config.json is valid
```

## 3. serve 플래그

```text
$ flats serve -h
Usage of serve:
  -authkey-file string
    	legacy mode: file holding a reusable, untagged Tailscale auth key for new nodes (else TS_AUTHKEY or interactive login)
  -config string
    	absolute path of config.json (env FLATS_CONFIG); without it, legacy mode uses <data>/config.json
  -console-host string
    	tailnet host name of the console (default "flats")
  -data string
    	legacy mode: data directory holding config.json (default "/Users/user/Library/Application Support/Flats")
  -listen string
    	loopback address for the CLI, local agents and the console (default "127.0.0.1:7878")
  -local-addr string
    	address of the local network (default "127.0.0.1:7879")
  -network string
    	legacy mode: private path, tailscale or local. tailscale records a host grant and preserves Private Tailscale for pre-lifecycle flats; does not enable Funnel (default "local")
  -operator-credential-file string
    	ignored: the operator credential is no longer used
  -operator-credential-stdin
    	ignored: the operator credential is no longer used
  -permit string
    	legacy mode: comma-separated host grants: tailscale, tailscale-funnel, portal, zrok, cloudflare-quick. Local needs no grant. Recorded in config.json and does not publish a flat
  -portal
    	legacy mode: grant Portal and attach it as the legacy public network
  -relays string
    	legacy mode: comma-separated Portal relays (default: Portal CLI default discovery)
  -runtime
    	enable server flats (wazero runtime) (default true)
flats serve: flag: help requested
```

## 4. 포트·바인딩, 프로세스, launchd, 로그, SQLite 테이블, 재시작 (원문)

```text
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats
flats      9009 user   10u  IPv4 0xdaac9dff6826ffe0      0t0  TCP 127.0.0.1:7879 (LISTEN)
flats      9009 user   11u  IPv4 0xef5cd695133ab800      0t0  TCP 127.0.0.1:7878 (LISTEN)
[exit=0]
```

```text
$ lsof -nP -iTCP:7878 -sTCP:LISTEN
COMMAND  PID       USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
flats   9009 user   11u  IPv4 0xef5cd695133ab800      0t0  TCP 127.0.0.1:7878 (LISTEN)
[exit=0]
```

```text
$ lsof -nP -iTCP:7879 -sTCP:LISTEN
COMMAND  PID       USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
flats   9009 user   10u  IPv4 0xdaac9dff6826ffe0      0t0  TCP 127.0.0.1:7879 (LISTEN)
[exit=0]
```

```text
$ ps -o pid,ppid,etime,command -p $(pgrep -x flats | tr '\n' ',' | sed 's/,$//')
  PID  PPID ELAPSED COMMAND
 9009     1   01:44 /Users/user/.local/bin/flats serve --config /Users/user/Library/Application Support/Flats/config.json
[exit=0]
```

```text
$ launchctl print gui/$(id -u)/dev.flats.serve | head -60
gui/501/dev.flats.serve = {
	active count = 1
	path = /Users/user/Library/LaunchAgents/dev.flats.serve.plist
	type = LaunchAgent
	state = running

	program = /Users/user/.local/bin/flats
	arguments = {
		/Users/user/.local/bin/flats
		serve
		--config
		/Users/user/Library/Application Support/Flats/config.json
	}

	stdout path = /Users/user/Library/Application Support/Flats/logs/serve.out.log
	stderr path = /Users/user/Library/Application Support/Flats/logs/serve.err.log
	inherited environment = {
		SSH_AUTH_SOCK => /var/run/com.apple.launchd.Q2zt62uRcq/Listeners
	}

	default environment = {
		PATH => /usr/bin:/bin:/usr/sbin:/sbin
	}

	environment = {
		OSLogRateLimit => 64
		PATH => /Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/Users/user/.bun/bin:/Users/user/.antigravity/antigravity/bin:/Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/usr/local/opt/openjdk@17/bin:/opt/homebrew/opt/mariadb@10.6/bin:/Users/user/bin:/usr/local/bin:/Users/user/.local/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/pkg/env/global/bin:/Applications/Wireshark.app/Contents/MacOS:/Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/Users/user/.bun/bin:/Users/user/.antigravity/antigravity/bin:/usr/local/opt/openjdk@17/bin:/opt/homebrew/opt/mariadb@10.6/bin:/Users/user/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/Applications/Ghostty.app/Contents/MacOS:/Users/user/Library/Application Support/JetBrains/Toolbox/scripts:/Users/user/.orbstack/bin:/Users/user/Library/Application Support/JetBrains/Toolbox/scripts:/Users/user/.local/bin:/Users/user/.orbstack/bin:/Users/user/.local/bin:/Users/user/claude/.claude-config/plugins/cache/omc/oh-my-claudecode/5.5.0/bin
		XPC_SERVICE_NAME => dev.flats.serve
	}

	domain = gui/501 [100023]
	asid = 100023
	minimum runtime = 10
	exit timeout = 5
	runs = 1
	pid = 9009
	immediate reason = speculative
	forks = 0
	execs = 1
	initialized = 1
	trampolined = 1
	started suspended = 0
	proxy started suspended = 0
	checked allocations = 0 (queried = 1)
	checked allocations reason = no host
	checked allocations flags = 0x0
	last exit code = (never exited)

	resource coalition = {
		ID = 16897
		type = resource
		state = active
		active count = 1
		name = dev.flats.serve
	}

	jetsam coalition = {
		ID = 16898
		type = jetsam
		state = active
[exit=0]
```

```text
$ ls -la '/Users/user/Library/Application Support/Flats/logs'; wc -c '/Users/user/Library/Application Support/Flats/logs/'*
total 8
drwx------@  4 user  staff  128 Oct 15 10:52 .
drwx------@ 10 user  staff  320 Oct 15 10:52 ..
-rw-r--r--   1 user  staff  105 Oct 15 10:52 serve.err.log
-rw-r--r--   1 user  staff    0 Oct 15 10:52 serve.out.log
     105 /Users/user/Library/Application Support/Flats/logs/serve.err.log
       0 /Users/user/Library/Application Support/Flats/logs/serve.out.log
     105 total
[exit=0]
```

```text
$ cat '/Users/user/Library/Application Support/Flats/logs/serve.out.log' | tail -30
[exit=0]
```

```text
$ cat '/Users/user/Library/Application Support/Flats/logs/serve.err.log' | tail -30
2026/10/15 10:52:19 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
[exit=0]
```

```text
$ sqlite3 -readonly '/Users/user/Library/Application Support/Flats/flats.db' '.tables'
approvals               flat_providers          redirects             
content_groups          flats                   revision_files        
content_head            host_binding            revisions             
content_state           legacy_private_upgrade  runtime_generation    
deployments             network_settings        secrets               
draft_revisions         pagepaths               settings              
drafts                  pageviews               versions              
env_vars                portal_listing        
events                  previews              
[exit=0]
```

```text
$ curl -s -i http://127.0.0.1:7878/api/status | head -20
HTTP/1.1 200 OK
Cache-Control: no-store
Content-Type: application/json
Date: Thu, 15 Oct 2026 01:54:03 GMT
Transfer-Encoding: chunked

{
  "ok": true,
  "system": {
    "version": "v0.9.1",
    "build": {
      "version": "v0.9.1",
      "release": true,
      "commit": "e681bb5ca50c770229abe431810b3aa72564a243",
      "dirty": false,
      "go": "go1.27.1",
      "os": "darwin",
      "arch": "arm64"
    },
    "data_dir": "/Users/user/Library/Application Support/Flats",
[exit=0]
```

```text
$ pgrep -x flats
9009
[exit=0]
```

```text
$ launchctl kickstart -k gui/$(id -u)/dev.flats.serve
[exit=0]
```

```text
$ pgrep -x flats
12818
[exit=0]
```

```text
$ launchctl print gui/$(id -u)/dev.flats.serve | grep -E 'state =|runs =|pid =|last exit'
	state = running
	runs = 2
	pid = 12818
	last exit code = 0
		state = active
		state = active
[exit=0]
```

```text
$ flats status | grep -E 'server:|launchd:'
server:  running at http://127.0.0.1:7878
launchd: loaded, running, pid 12818, last exit 0
[exit=0]
```

```text
$ tail -5 '/Users/user/Library/Application Support/Flats/logs/serve.err.log'
2026/10/15 10:52:19 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
2026/10/15 10:54:08 shutting down
2026/10/15 10:54:08 flats v0.9.1 serving; console http://127.0.0.1:7878 (loopback http://127.0.0.1:7878)
[exit=0]
```

```text
$ lsof -nP -iTCP:7878 -sTCP:LISTEN
COMMAND   PID       USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
flats   12818 user   11u  IPv4 0x7a6b9a15402d7bc0      0t0  TCP 127.0.0.1:7878 (LISTEN)
[exit=0]
```

## 결론

- **데이터 디렉터리**: `~/Library/Application Support/Flats`(264K). 구성은 `config.json`(0600), `flats.db`(+`-wal`/`-shm`, SQLite 메타데이터), `secret.key`(32바이트, 0600), `flats.lock`, `logs/`, `flats/<slug>/`(콘텐츠·각 flat 의 DB·FILES, 지금은 `.lifecycle/` 만), 필요 시 생성되는 `runtime/`·`backups/`·`config-history/`. 레이아웃은 `docs/configuration.md` "Where data lives" 와 일치한다.
- **포트**: `127.0.0.1:7878`(콘솔·API·MCP·CLI 관리 리스너), `127.0.0.1:7879`(콘텐츠 Local 네트워크). 둘 다 **IPv4 루프백만** 바인딩, 0.0.0.0 아님. 같은 LAN 의 다른 기기는 접근할 수 없다.
- **로그**: stdout `logs/serve.out.log`(비어 있음), stderr `logs/serve.err.log` 에 시작·종료 한 줄씩. 로그는 launchd 가 열어 둔 파일에 쌓이며 로테이션 정책은 문서에서 못 찾았다. flat 단위 이벤트는 `flats logs <slug>`.
- **launchd**: `RunAtLoad=true`, `KeepAlive=true`. 로그인 시 시작하고, 프로세스가 종료되면 launchd 가 다시 띄운다. 에이전트(gui 도메인)라 로그인 세션이 있어야 돈다.
- **재시작**: `launchctl kickstart -k` 로 pid 9009 → 12818, `runs = 1 → 2`, 7878 재바인딩, `flats status` 정상. stderr 로그에 `shutting down` 후 `serving` 이 찍힌다. `last exit code = 0`(정상 종료 신호 처리).
- **설정 기본값**: `system.keep_versions` 유효값 **10**(flat 당, `0` = 가지치기 없음). `docs/configuration.md:131` 표의 기본값 10 과 일치한다(같은 문서 110줄의 예시 JSON 20 은 예시 값).
- plist 에 사용자 `PATH` 가 41개 항목으로 복사돼 launchd 의 `environment` 에도 그대로 나온다(위 `launchctl print`).

## Needs Verification

- 재부팅 뒤 자동 시작: 하지 않았다. plist 의 `RunAtLoad=true`·`KeepAlive=true` 가 근거이며 실제 재부팅 검증은 미실시.
- 로그 로테이션·최대 크기: 문서 미확인.
- 서버 flat 워커(`flats worker`) 프로세스가 plist 의 PATH 를 상속하는지: 서버 flat 을 만들지 않아 미확인.
