
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
