
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
