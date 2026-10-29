
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
