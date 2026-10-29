# 11 v0.10.0 → v0.11.0 업그레이드 (2026-10-17)

10 과 같은 절차다. v0.11.0 릴리스 노트: "the first start after the upgrade adds a random flat ID (`flats.uid`) to every flat … A downgrade to v0.10.x afterwards is refused, because the schema is newer."

## 설치 스크립트 재검토

```text
$ shasum -a 256 logs/raw/install-v0.10.0.sh logs/raw/install-v0.11.0.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install-v0.10.0.sh
0bb87c02029374154f201d2339db0982814ebe2b4141f15462a82a69d63ab6a9  logs/raw/install-v0.11.0.sh
```

## 중지·백업·설치·시작

```text
$ launchctl bootout gui/$(id -u)/dev.flats.serve        → exit 0, no flats listener
$ cp -Rp "$HOME/Library/Application Support/Flats" backups/flats-data-20261017-pre-v0.11.0
$ diff -r <원본> <백업>                                  → backup identical (7.7M)
$ sh logs/raw/install-v0.11.0.sh
flats-install: downloading flats_darwin_arm64.tar.gz (v0.11.0)
flats-install: installed /Users/user/.local/bin/flats: flats v0.11.0 (3abe537) go1.27.1 darwin/arm64
flats-install: warning: the Flats service is installed but stopped; it uses the new binary once started
$ launchctl enable gui/501/dev.flats.serve && launchctl bootstrap gui/501 …/dev.flats.serve.plist   → exit 0
```

## 업그레이드 뒤 확인

```text
$ flats version
flats v0.11.0 (3abe537) go1.27.1 darwin/arm64
$ lsof -nP -iTCP -sTCP:LISTEN | grep -i flats
flats  41794 …  TCP 127.0.0.1:7879 (LISTEN)
flats  41794 …  TCP 127.0.0.1:7878 (LISTEN)
$ ls "<data>/backups/"
flats.v11.20261016T052227.476768000Z.db
flats.v15.20261017T130002.538674000Z.db
$ flats list   → 업그레이드 전과 같은 7개, k8s-network public v1 (Portal 주소 동일)
public /           200
public /mcp        404
public /api/flats  404
try-pod-network    200
try-pod-doc        200
k8s-network-doc    200
mcp tools 36
$ flats client check
Server: http://127.0.0.1:7878 (v0.11.0)
CLI: v0.11.0, contract 2 — compatible
```

결론:
- 콘텐츠·버전·Public 상태·Portal 주소·127.0.0.1 바인딩이 유지됐다. MCP 도구 개수(36)와 client contract(2)는 v0.10.0 과 같다.
- Flats 가 마이그레이션 전 DB 를 `flats.v15.….db` 로 따로 남겼다.
- 이번 버전부터 Document 의 실시간 편집도 flat 별 디스크 quota 에 포함된다. 새 설정 `console.preferred_host` 는 쓰지 않았다(Local 만 쓰므로 해당 없음).

Needs Verification:
- `flats.uid` 가 추가됐는지 `flats.db` 에서 직접 보지는 않았다.
