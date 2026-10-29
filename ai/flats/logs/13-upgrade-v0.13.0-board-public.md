# 13 v0.12.0 → v0.13.0 업그레이드와 board Public 시험 (2026-10-21)

목적: v0.13.0 의 새 콘텐츠 유형 board 를 Mac 호스트에 띄우고, Portal 로 공개해 Windows 브라우저에서 Space+drag 이동을 재현한다.

## 업그레이드

릴리스 노트에 업그레이드 절이 없지만 소스에는 마이그레이션 18 이 있다(`internal/store/migrate.go`: `{18, "board revision imports", addBoardRevisions}`). 다운그레이드는 안 된다.

```text
$ shasum -a 256 logs/raw/install-v0.12.0.sh logs/raw/install-v0.13.0.sh   → 둘 다 0bb87c02…a6a9 (같음)
$ launchctl bootout gui/501/dev.flats.serve                               → exit 0, no flats listener
$ cp -Rp <data> backups/flats-data-20261021-pre-v0.13.0 ; diff -r          → backup identical (8.7M)
$ FLATS_VERSION=v0.13.0 sh logs/raw/install-v0.13.0.sh
flats-install: installed /Users/user/.local/bin/flats: flats v0.13.0 (2fc51df) go1.27.1 darwin/arm64
$ launchctl enable … && launchctl bootstrap …                               → exit 0
$ flats version                → flats v0.13.0 (2fc51df) go1.27.1 darwin/arm64
$ lsof … LISTEN                → 127.0.0.1:7879, 127.0.0.1:7878
$ flats client check           → Server v0.13.0, CLI v0.13.0, contract 2 — compatible
$ flats list | diff <전> -     → 차이 없음
$ pragma user_version          → 18
$ ls <data>/backups            → flats.v15…, flats.v16…, flats.v17.20261021T140326….db (v11 사본은 사라짐, 최근 3개만 유지)
```

## 테스트 board

```text
$ flats board apply space-pan-test examples/board-space-test/ops.json --message "Space pan test board"
Applied new board space-pan-test: revision 2, seq 2
$ flats info space-pan-test
  type: board · board: running (no Draft, versions or publish step) · visibility: private
  private: http://space-pan-test.localhost:7879
```

## Public 전환 (사용자 지시로 메인이 콘솔에서 승인)

1차 요청 `apr-9sgfa8dtrne5` 는 운영자 결정 시점에 실패했다.

```text
Result: conflict: provider not permitted: permit tailscale-funnel, portal, zrok, cloudflare-quick or netbird-expose before making this flat public
```

새 flat 은 호스트 기본 제공자(local)만 허용된 채 생긴다. 콘솔 `flats/space-pan-test/settings` → Networks → Public → Portal 체크("Portal allowed for space-pan-test.")로 이 flat 에만 Portal 을 허용했다. Portal relay listing 은 "Follow the host setting" 그대로 두었다.

2차 요청 `apr-s0ft6abh8ic0` 를 콘솔에서 승인했다(Status approved, "visibility changed to public").

```text
$ flats info space-pan-test
  visibility: public
  provider: portal configured=true permitted=true ready=true state=ready … https://space-pan-test.portal.dps0340.win
public  /           200  (<title>Board</title>, 화면 머리에 "Public · read only")
public  /metrics    404
public  /mcp        404
public  /api/flats  404
public  /approvals  404
lsof    7878·7879 둘 다 127.0.0.1
```

결론:
- board 는 Draft·버전·배포 단계 없이 바로 running 이고, Public 전환만 승인을 거친다.
- 공개 승인 전에 flat 별 Portal 허용이 따로 필요하다. 허용이 없으면 승인해도 conflict 로 실패한다.
- 같은 Portal 이라도 flat 마다 릴레이 도메인이 다를 수 있다(k8s-network 는 `portal.rabbitson87.dev`, space-pan-test 는 `portal.dps0340.win`). 주소는 `flats info` 의 `public:` 줄에서 읽는다.
- 공개 주소에서 관리 경로는 모두 404 다.

할 일: Windows 시험이 끝나면 Private 복귀 요청을 만들어 승인받는다.
