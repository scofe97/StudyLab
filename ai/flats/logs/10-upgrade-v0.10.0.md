# 10 v0.9.1 → v0.10.0 업그레이드 (2026-10-16)

절차는 v0.10.0 릴리스 노트의 Upgrade notes 와 `docs/release.md`(ref v0.10.0) 「Upgrading a host」를 따랐다. 릴리스 노트: "This release migrates both `config.json` (schema → 4) and `flats.db` (`user_version` → 15) … downgrading to v0.9.x afterwards is not supported."

## 설치 스크립트 재검토

```text
$ curl -fsSL https://raw.githubusercontent.com/gosuda/flats/main/install.sh -o logs/raw/install-v0.10.0.sh
$ shasum -a 256 logs/raw/install.sh logs/raw/install-v0.10.0.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install-v0.10.0.sh
```

01 에서 검토한 스크립트와 같다.

## 중지와 전체 백업

```text
$ launchctl bootout gui/$(id -u)/dev.flats.serve        → exit 0
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats           → no flats listener
$ cp -Rp "$HOME/Library/Application Support/Flats" backups/flats-data-20261016-pre-v0.10.0
$ diff -r <원본> <백업>                                  → backup identical
$ du -sh <백업>                                          → 6.9M
```

백업에 든 것: cache, config-history, config.json, flats/, flats.db, flats.lock, logs, portal/, run, runtime, secret.key. 정상 종료 뒤라 `-wal`·`-shm` 파일은 없었다.

## 설치와 시작

```text
$ sh logs/raw/install-v0.10.0.sh
flats-install: downloading flats_darwin_arm64.tar.gz (v0.10.0)
flats-install: installed /Users/user/.local/bin/flats: flats v0.10.0 (7133b7c) go1.27.1 darwin/arm64
flats-install: warning: the Flats service is installed but stopped; it uses the new binary once started
Start the service:  launchctl enable gui/501/dev.flats.serve && launchctl bootstrap gui/501 /Users/user/Library/LaunchAgents/dev.flats.serve.plist
$ launchctl enable gui/501/dev.flats.serve && launchctl bootstrap gui/501 …/dev.flats.serve.plist   → exit 0
```

## 업그레이드 뒤 확인

```text
$ flats version
flats v0.10.0 (7133b7c) go1.27.1 darwin/arm64
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats
flats     65159 …  TCP 127.0.0.1:7879 (LISTEN)
flats     65159 …  TCP 127.0.0.1:7878 (LISTEN)
$ ls "~/Library/Application Support/Flats/backups/"
flats.v11.20261016T052227.476768000Z.db
$ grep schema_version config.json
"schema_version": 4
$ flats list
k8s-network      page  public      v1  1  https://k8s-network.portal.rabbitson87.dev (*)
try-pod-network  page  private     v1  1  http://try-pod-network.localhost:7879
learning-hub     flat  private     -   1
try-prom-talk    flat  private     -   1
mcp-hello        page  private     -   0
try-pod-doc      docs  private     running  -  http://try-pod-doc.localhost:7879
k8s-network-doc  docs  private     running  -  http://k8s-network-doc.localhost:7879
```

```text
public /           200
public /mcp        404
public /api/flats  404
try-pod-network    200
try-pod-doc        200
k8s-network-doc    200
$ flats client check   (flats-lab 에서)
Server: http://127.0.0.1:7878 (v0.10.0)
CLI: v0.10.0, contract 2 — compatible
```

MCP `tools/list` 36개. v0.9.1 대비 빠진 도구 `publish`·`save_version`·`save_version_from_dir`, 새 도구 `create_version`·`delete_version`·`save_draft_from_dir`.

```text
check_client create_flat create_version delete_env delete_flat delete_version deploy get_approval get_client_skill get_content_types get_design get_document get_draft get_flat get_logs get_network get_revision get_runtime_reference guide list_env list_flats list_revisions list_secrets list_versions open_preview pin_revision rollback rollback_revision save_design save_document save_draft save_draft_from_dir save_page set_env set_visibility update_document
```

결론:
- 데이터·버전 번호·Public 상태(Portal 주소 포함)·바인딩이 그대로 유지됐다. Flats 는 마이그레이션 전 DB 를 `<data>/backups/` 에 스스로 남겼지만, `config.json`·`secret.key` 까지 담은 것은 우리가 만든 전체 백업뿐이다.
- 업그레이드 중 약 1분 동안 서비스가 멈췄고 그동안 Portal 공개 페이지도 내려갔다.
- 릴리스 노트에 따라 업그레이드 전 걸려 있던 `publish` 승인 대기(learning-hub·try-prom-talk)는 거부된다. 두 flat 은 지금 unpublished 이고 v1 까지만 있다. 새 흐름(create_version 승인 → deploy 승인)으로 다시 요청해야 한다.
- `system.keep_versions` 가 `system.version_quota`(기본 20)로 바뀌었고, 이제 버전이 자동으로 정리되지 않는다.

Needs Verification:
- 되돌리기는 v0.9.1 재설치(`--version v0.9.1`) + `backups/flats-data-20261016-pre-v0.10.0` 복원으로 할 수 있다고 문서에 적혀 있다. 실제로 해 보지는 않았다.
- 콘솔에서 새 Document History 패널(revisions)이 보이는지는 확인하지 않았다.
- `flats client check` 의 `Guide: unknown`·`Bootstrap: unknown` 이 무엇을 뜻하는지 확인하지 않았다. 플러그인을 설치하지 않았기 때문으로 보인다.
