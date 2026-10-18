# 01 설치 검토 — install.sh 읽기 → 실행 → CLI 확인

실행일: 2026-10-15, macOS 26.6.2 arm64. 환경: zsh, `~/.local/bin` 은 이미 PATH 에 있음.

## 1. install.sh 를 파이프 없이 받아 읽기

```text
$ curl -fsSL https://raw.githubusercontent.com/gosuda/flats/main/install.sh -o logs/raw/install.sh
$ wc -l logs/raw/install.sh
     618 logs/raw/install.sh
```

원문 전체는 `logs/raw/install.sh`(618줄)에 보존했다. 읽고 확인한 내용(스크립트 줄 단위 근거는 해당 파일):

| 항목 | 스크립트가 하는 일 |
|---|---|
| 권한 | 파일 머리 주석 "The installer never uses sudo and never edits shell profiles." 실제로 `sudo` 호출은 Linux `enable_linger` 안내 문구(`warn`)에만 문자열로 등장하고 실행하지 않는다. `.zshrc` 등 프로필을 건드리는 코드는 없다(`PATH` 에 없으면 안내만 출력) |
| 플랫폼 | `uname -s`/`uname -m` 으로 `flats_darwin_arm64.tar.gz` 결정. Rosetta 면 `sysctl.proc_translated` 로 arm64 로 보정 |
| 버전 | `--version` 미지정이면 `https://github.com/gosuda/flats/releases/latest` 의 최종 리다이렉트 URL 에서 태그를 먼저 해석(`latest_tag`) 해 아카이브와 `checksums.txt` 가 다른 릴리스에서 오지 않게 고정 |
| 다운로드 | `curl -fsSL --proto '=https' --tlsv1.2 --retry 3`. `FLATS_DOWNLOAD_BASE` 로 미러·로컬 디렉터리 지정 가능(이번엔 안 씀) |
| 체크섬 | 같은 릴리스의 `checksums.txt` 에서 해당 아카이브 줄을 `awk` 로 뽑아 `shasum -a 256` 결과와 비교, 불일치면 `die`. 서명 검증이 아니라 같은 출처 내 일관성 확인이다(`docs/release.md` 도 같은 표현) |
| 설치 위치 | `~/.local/bin/flats`(`--dir` 로 변경). 같은 디렉터리에 `.flats.install.$$` 로 복사 후 `mv -f` 로 원자적 교체. 교체 전에 `"$staged" version` 으로 실행 가능 여부 확인 |
| 서비스 | `check_service_manager`(macOS 는 `launchctl print gui/<uid>`) 통과 후 `flats --url <url> install --executable <binary> -- <serve flags>` 호출 → launchd 에이전트 `dev.flats.serve` 등록. 서비스 파일 경로는 `~/Library/LaunchAgents/dev.flats.serve.plist` |
| 기존 서비스 | plist 가 있고 serve 플래그를 안 주면 `mode=restart`(실행 중이면 `launchctl kickstart -k`) 또는 `replace`(멈춘 서비스는 그대로 둠) |
| 기본 네트워크 | serve 플래그를 안 주면 기본값(Local, Portal off). 비로컬 제공자는 `-- --network tailscale` 처럼 명시해야만 켜짐 |
| 건강 확인 | `wait_healthy` 가 최대 30초 동안 launchd `state = running` 과 `GET <url>/api/status` 를 확인 |
| 데이터 디렉터리 | `data_dir()` 기본값 `~/Library/Application Support/Flats`(`FLATS_DATA` 로 변경). 단 이 값은 서비스를 새로 설치할 때만 적용 |

Needs Verification: 스크립트가 만드는 plist 의 정확한 내용은 스크립트가 아니라 `flats install` 바이너리가 쓴다 → 아래 §3 에서 실물 확인.

## 2. 실행 (원문 그대로)

```text
$ sh logs/raw/install.sh
flats-install: downloading flats_darwin_arm64.tar.gz (v0.9.1)
flats-install: installed /Users/user/.local/bin/flats: flats v0.9.1 (e681bb5) go1.27.1 darwin/arm64
flats-install: installing the Flats service
Installed launchd agent dev.flats.serve (/Users/user/Library/LaunchAgents/dev.flats.serve.plist)
  runs:   /Users/user/.local/bin/flats serve --config /Users/user/Library/Application Support/Flats/config.json
  logs:   /Users/user/Library/Application Support/Flats/logs/serve.out.log
          /Users/user/Library/Application Support/Flats/logs/serve.err.log
Flats is running at http://127.0.0.1:7878
flats-install: Flats is running at http://127.0.0.1:7878

Console:  http://127.0.0.1:7878
Status:   flats status
Remove:   flats uninstall   (keeps your flats and data)
[exit=0]
```

설치 전 상태: `~/.local/bin/flats` 없음, `~/Library/Application Support/Flats` 없음(`ls` 로 확인, 파일 없음 오류).

## 3. 생성된 launchd plist 원문

```text
$ cat ~/Library/LaunchAgents/dev.flats.serve.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>dev.flats.serve</string>
	<key>ProgramArguments</key>
	<array>
		<string>/Users/user/.local/bin/flats</string>
		<string>serve</string>
		<string>--config</string>
		<string>/Users/user/Library/Application Support/Flats/config.json</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>/Users/user/Library/Application Support/Flats/logs/serve.out.log</string>
	<key>StandardErrorPath</key>
	<string>/Users/user/Library/Application Support/Flats/logs/serve.err.log</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>/Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/Users/user/.bun/bin:/Users/user/.antigravity/antigravity/bin:/Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/usr/local/opt/openjdk@17/bin:/opt/homebrew/opt/mariadb@10.6/bin:/Users/user/bin:/usr/local/bin:/Users/user/.local/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/System/Cryptexes/App/usr/bin:/usr/bin:/bin:/usr/sbin:/sbin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/local/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/bin:/var/run/com.apple.security.cryptexd/codex.system/bootstrap/usr/appleinternal/bin:/pkg/env/global/bin:/Applications/Wireshark.app/Contents/MacOS:/Users/user/.local/bin:/Users/user/.nvm/versions/node/v20.18.1/bin:/Users/user/.bun/bin:/Users/user/.antigravity/antigravity/bin:/usr/local/opt/openjdk@17/bin:/opt/homebrew/opt/mariadb@10.6/bin:/Users/user/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/Applications/Ghostty.app/Contents/MacOS:/Users/user/Library/Application Support/JetBrains/Toolbox/scripts:/Users/user/.orbstack/bin:/Users/user/Library/Application Support/JetBrains/Toolbox/scripts:/Users/user/.local/bin:/Users/user/.orbstack/bin:/Users/user/.local/bin:/Users/user/claude/.claude-config/plugins/cache/omc/oh-my-claudecode/5.5.0/bin</string>
	</dict>
</dict>
</plist>
```

관찰: `ProgramArguments` 는 `flats serve --config <데이터>/config.json` 뿐이다(네트워크 플래그 없음 → config.json 기본값). `RunAtLoad`·`KeepAlive` 모두 `true`. 사용자 셸의 `PATH` 가 plist 의 `EnvironmentVariables` 로 통째로 복사됐다(41개 항목, 중복 제거 시 24개). 이 값 때문에 서비스가 실행하는 워커·도구가 사용자 PATH 를 그대로 본다.

## 4. 체크섬을 별도로 재검증

```text
$ curl -fsSL https://github.com/gosuda/flats/releases/download/v0.9.1/checksums.txt
69a4d1397adb7b8f5c2eb9eda53924209bd3f7a9d575283e216ccd14a7c80546  flats_darwin_amd64.tar.gz
2cf9d0245fd22fbd71782801d1688227642d37b6291d7f5886b6eba55a440687  flats_darwin_arm64.tar.gz
a8a1b3c95f0e7b0fcdfe2c703062f5010f60beb449ef71cbc32c4ed07446e05b  flats_linux_amd64.tar.gz
97125c86369c960ed90cdcf372da172f71f025e5243ac5ce3ce822d2b4386f37  flats_linux_arm64.tar.gz

$ shasum -a 256 flats_darwin_arm64.tar.gz      # 별도로 내려받은 아카이브
2cf9d0245fd22fbd71782801d1688227642d37b6291d7f5886b6eba55a440687  flats_darwin_arm64.tar.gz

$ tar -xzf flats_darwin_arm64.tar.gz && shasum -a 256 flats ~/.local/bin/flats
683314aa09b254a7d3fc2118e4d568caebba124d5df31d8b44a217d3e90a863f  flats
683314aa09b254a7d3fc2118e4d568caebba124d5df31d8b44a217d3e90a863f  /Users/user/.local/bin/flats

$ gh attestation verify flats_darwin_arm64.tar.gz --repo gosuda/flats
[exit=0]   (표준 출력·오류 모두 비어 있음. 종료 코드 0 만 확인)

$ codesign -dv ~/.local/bin/flats
Executable=/Users/user/.local/bin/flats
Identifier=a.out
Format=Mach-O thin (arm64)
CodeDirectory v=20400 size=677086 flags=0x20002(adhoc,linker-signed) hashes=21156+0 location=embedded
Signature=adhoc
```

## 5. 버전·상태·도움말

```text
$ flats status      # 원문
server:  running at http://127.0.0.1:7878
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
    "console_url": "http://127.0.0.1:7878",
    "data_dir": "/Users/user/Library/Application Support/Flats",
    "local_url": "http://127.0.0.1:7878",
    "mcp_url": "http://127.0.0.1:7878/mcp",
    "private": {
      "detail": "loopback only, port 7879 (no Tailscale)",
      "enabled": true,
      "hosts": null,
      "kind": "local"
    },
    "providers": [
      {
        "configured": true,
        "detail": "backend configured; route readiness is reported per flat",
        "permitted": true,
        "provider": "local",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "tailscale",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "tailscale-funnel",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "portal",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "zrok",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "cloudflare-quick",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "netbird",
        "ready": false,
        "state": "unavailable"
      },
      {
        "configured": false,
        "detail": "backend is not configured for this process",
        "permitted": false,
        "provider": "netbird-expose",
        "ready": false,
        "state": "unavailable"
      }
    ],
    "server_flats": true,
    "version": "v0.9.1"
  }
  upload_max_bytes: 20971520
launchd: loaded, running, pid 9009, last exit (never exited)
```

## 6. 하위 명령 도움말 (원문)

```text
$ flats version
flats v0.9.1 (e681bb5) go1.27.1 darwin/arm64
[exit=0]
```

```text
$ flats --version
flats: flag provided but not defined: -version
[exit=2]
```

```text
$ flats help deploy
usage: flats deploy <dir|archive> --flat <slug> [--save-only] [--message m]
       flats deploy --flat <slug> --version <n>
[exit=0]
```

```text
$ flats help draft
usage: flats draft <slug> [<dir|archive>] [--expected-revision n] [--message m]
[exit=0]
```

```text
$ flats help publish
usage: flats publish <slug> [--revision n] [--hash hash]
[exit=0]
```

```text
$ flats help versions
usage: flats versions <slug>
[exit=0]
```

```text
$ flats help rollback
usage: flats rollback <slug> [--to n] [--restore-data]
[exit=0]
```

```text
$ flats help approvals
usage: flats approvals [--status pending|approved|rejected|failed]
[exit=0]
```

```text
$ flats help visibility
usage: flats visibility <slug> private|public [--reason r]
[exit=0]
```

```text
$ flats help serve
flats hosts agent-built websites on this machine.

usage: flats [--url url] [--json] <command> [args]

commands:
  draft       read or save a Private Draft
  publish     request operator approval to save Draft as a new version
  deploy      save Draft and request version creation, or activate a saved version
  list        list flats
  info        show one flat
  versions    list published versions
  rollback    request approval to activate an earlier version
  preview     open a private preview of a version
  visibility  change who can open a flat
  rename      change a flat's slug
  delete      request deletion of a flat
  logs        show a flat's events
  env         manage ordinary server environment variables
  network     manage server HTTP(S) origin permissions
  secret      manage server-flat secrets
  approvals   list approval requests
  status      show server and service status
  install     run `flats serve` at login (launchd/systemd)
  uninstall   remove the launchd agent
  connect     choose the Flats host this CLI and `flats mcp` use
  mcp         serve the host's MCP tools over stdio (for agent plugins)
  mcp-config  print MCP setup for agent tools
  config      manage the host configuration (config.json)
  client      check local client compatibility
  update      update this standalone CLI (never share with a running foreground/custom server)
  skill       sync a standalone skill (managed plugins must use their plugin manager)
  version     print the flats version
  serve       run the server
  worker      internal: server-flat worker process

Run `flats help <command>` for details. Exit codes: 0 ok, 1 error, 2 usage, 3 pending approval.
[exit=0]
```

```text
$ flats help install
usage: flats install [--config path] [--data dir] [--executable path] [-- legacy serve flags...]
[exit=0]
```

```text
$ flats help config
usage: flats config init [--data dir] [--config path] [--listen addr] [--local-addr addr] [--console-host name] [--runtime=bool]
       flats config show [--config path | --data dir]
       flats config validate [--config path | --data dir]
       flats config set KEY VALUE [--config path | --data dir]
       flats config unset KEY [--config path | --data dir]
       flats config migrate [--data dir] [--config path] [--dry-run] [--yes] [-- legacy serve flags]
       flats config rebind --yes [--config path | --data dir]
[exit=0]
```

```text
$ flats help mcp
usage: flats mcp [--url url]
[exit=0]
```

```text
$ flats help mcp-config
usage: flats mcp-config [--url url]
[exit=0]
```

```text
$ flats help connect
usage: flats connect [<url>] [--clear]
[exit=0]
```

```text
$ flats help preview
usage: flats preview <slug> [--version n]
[exit=0]
```

```text
$ flats help logs
usage: flats logs <slug> [--follow] [--kind k] [--limit n]
[exit=0]
```

## 결론

- 설치는 문서대로 동작했다. 릴리스 v0.9.1(커밋 `e681bb5`, go1.27.1) 바이너리가 `~/.local/bin/flats` 에, launchd 에이전트 `dev.flats.serve` 가 `~/Library/LaunchAgents/` 에 생겼다. sudo 는 쓰지 않았다.
- `flats --version` 은 **지원하지 않는다**(`flag provided but not defined: -version`, exit 2). 버전은 `flats version`.
- `flats help serve` 는 serve 플래그가 아니라 전체 도움말을 출력한다. serve 플래그는 `flats serve -h` 로 본다(02 로그).
- 체크섬: 설치 스크립트가 검증했고, 아카이브를 따로 받아 `checksums.txt` 와 SHA-256 이 일치함을 직접 재확인했다. 설치된 바이너리와 아카이브 안 바이너리 해시도 같다.
- 바이너리는 ad-hoc 서명(`Signature=adhoc`, linker-signed)이다. Developer ID 서명·공증은 없다. 설치 스크립트가 내려받은 파일이라 `com.apple.provenance` 속성만 있다.
- Exit code 규약: 0 ok, 1 error, 2 usage, 3 pending approval.
- 승인 결정용 CLI 명령은 없다(`flats approvals` 는 목록만). 승인은 콘솔(브라우저)에서만 한다 → 03 이후 단위에서 Claude in Chrome 사용.

## Needs Verification

- `gh attestation verify` 가 출력 없이 exit 0 이었다. 성공으로 읽지만 출력 원문 근거는 없다.
- 설치 스크립트는 이번에 한 번만 돌려 "업그레이드(restart/replace) 경로"는 실행하지 않았다.
- PATH 가 plist 에 통째로 박힌 점이 보안·재현성에 어떤 영향이 있는지는 미확인(02 로그에서 워커 실행 환경과 함께 본다).
