# 12 v0.11.0 → v0.12.0 업그레이드 (2026-10-19)

11 과 같은 절차다. v0.11.1(문서 worker 를 2분 유휴 뒤 재움, 마이그레이션 없음)은 건너뛰고 v0.12.0 으로 바로 올렸다. 설치 스크립트가 받는 바이너리는 릴리스 하나뿐이고 패치의 변경은 v0.12.0 에 포함된다.

v0.12.0 릴리스 노트에서 이 호스트에 걸리는 것:
- 마이그레이션 17 이 `usagedays` 표(flat 별 일 사용량)를 더한다. 마이그레이션을 건너는 다운그레이드는 지원하지 않는다.
- 관리 서버가 `/metrics`(Prometheus)를 항상 연다. 콘솔과 같은 Host·Origin 검사만 하고 따로 인증이 없어, 콘솔에 닿는 사람은 Private·미배포 flat 의 slug 까지 읽는다.
- 새 설정 `console.external_origins`(기본 `[]`). 선언한 origin 에는 콘솔·승인·API·MCP 전체가 인증 없이 열린다.
- CLI 플래그·client contract 변경 없음.

## 설치 스크립트 재검토

```text
$ curl -fsSL https://raw.githubusercontent.com/gosuda/flats/v0.12.0/install.sh -o logs/raw/install-v0.12.0.sh
$ shasum -a 256 logs/raw/install-v0.11.0.sh logs/raw/install-v0.12.0.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install-v0.11.0.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install-v0.12.0.sh
```

## 중지·백업·설치·시작

```text
$ launchctl bootout gui/$(id -u)/dev.flats.serve        → exit 0, no flats listener
$ cp -Rp "$HOME/Library/Application Support/Flats" backups/flats-data-20261019-pre-v0.12.0
$ diff -r <원본> <백업>                                  → backup identical (8.2M)
$ FLATS_VERSION=v0.12.0 sh logs/raw/install-v0.12.0.sh
flats-install: downloading flats_darwin_arm64.tar.gz (v0.12.0)
flats-install: installed /Users/user/.local/bin/flats: flats v0.12.0 (ec73829) go1.27.1 darwin/arm64
flats-install: warning: the Flats service is installed but stopped; it uses the new binary once started
$ launchctl enable gui/501/dev.flats.serve && launchctl bootstrap gui/501 …/dev.flats.serve.plist   → exit 0
```

## 업그레이드 뒤 확인

```text
$ flats version
flats v0.12.0 (ec73829) go1.27.1 darwin/arm64
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats
flats  87158 …  TCP 127.0.0.1:7879 (LISTEN)
flats  87158 …  TCP 127.0.0.1:7878 (LISTEN)
$ ls "<data>/backups/"
flats.v11.20261016T052227.476768000Z.db
flats.v15.20261017T130002.538674000Z.db
flats.v16.20261018T155159.038840000Z.db
$ flats client check
Server: http://127.0.0.1:7878 (v0.12.0)
CLI: v0.12.0, contract 2 — compatible
$ flats list | diff <업그레이드 전 목록> -    → 차이 없음(7개, k8s-network public v1, Portal 주소 동일)

$ sqlite3 "file:<data>/flats.db?mode=ro" "pragma user_version;"   → 17
$ sqlite3 … ".schema usagedays"   → CREATE TABLE usagedays (flat_uid TEXT NOT NULL, day TEXT NOT NULL, requests INTEGER …
$ sqlite3 … "select slug, length(uid) from flats;"   → 7개 flat 모두 32

local  try-pod-network   200
local  try-pod-doc       200
local  k8s-network-doc   200
console  127.0.0.1:7878/metrics   200 (flats_ 지표 259줄)
  flats_open_streams{flat="learning-hub",type="flat"} 0
  flats_open_streams{flat="mcp-hello",type="page"} 0
public (Portal k8s-network)  /          200
public                        /metrics   404
public                        /mcp       404
public                        /api/flats 404
MCP tools/list (127.0.0.1:7878/mcp)   36개, v0.11.0 과 같은 이름
config.json   external_origins·preferred_host 없음(기본값)
```

결론:
- 콘텐츠·버전·Public 상태·Portal 주소·127.0.0.1 바인딩이 유지됐다. 스키마는 17 이 됐고, Flats 가 마이그레이션 전 DB 를 `flats.v16.….db` 로 따로 남겼다.
- 새 `/metrics` 는 관리 서버(7878)에만 있고 Portal 공개 주소에서는 404 다. 지표에 Private·미배포 flat 이름이 실제로 들어 있으므로, 7878 을 127.0.0.1 밖으로 열거나 `console.external_origins` 를 채우면 이 목록도 함께 노출된다. 지금 설정은 둘 다 하지 않았다.
- MCP 도구 개수(36)와 client contract(2)는 그대로다.
- 11 의 Needs Verification(`flats.uid` 추가 여부)은 이번에 DB 에서 직접 확인해 닫는다: `flats.uid` 열이 있고 7개 모두 32자 값이다.

Needs Verification:
- 문서 worker 가 2분 유휴 뒤 잠들고 50ms 안에 깨어나는지(v0.11.1·v0.12.0 노트)는 재지 않았다.


크기 메모: 업그레이드 전 `du -sh` 의 20M 은 서비스가 돌던 중에 잰 값이다. 서비스를 다시 띄운 뒤 항목별로 재 보니 `runtime/` 이 19M(백업에는 0B), `flats.db-wal` 132K·`flats.db-shm` 32K(백업에는 없음)였다. 실행 중에만 생기는 파일이고, 서비스를 멈춘 뒤 뜬 백업은 `diff -r` 로 원본과 같았다.
