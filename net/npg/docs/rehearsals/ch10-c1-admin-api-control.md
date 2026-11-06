# 떠 있는 Caddy 의 설정을 한 칸만 고쳐라 (C1)

- 장: ch10
- 날짜: 2026-11-04
- 앞 장면과의 연결: E5 포스트모템에서 "`load` 로 안 바뀌는 설정은 C1 에서 이어서 본다"고 적었다. E5 의 `e5-internal-tls.json` 이 떠 있는 상태에서 시작한다.

## 신고

> "떠 있는 Caddy 의 응답 헤더 하나만 관리 API 로 바꾸고 싶다. 잘못 올리면 서비스가 멈추는가, 재시작하면 고친 값은 남는가."

이 장면은 장애를 고치는 장면이 아니라 운영 제어 장면이다. 관리 API 의 읽기·쓰기 규칙, 잘못된 설정을 올렸을 때의 동작, `validate`·`reload` 명령, 재시작 뒤 복구, 관리 API 의 문단속을 차례로 본다.

### 이 장면의 설정

- 떠 있는 설정: `deploy/caddy/scenarios/e5-internal-tls.json` (관리 API `127.0.0.1:2019`, `origins`·`enforce_origin` 없음)
- 잘못된 설정 세 벌(`e5-internal-tls.json` 에서 파생): `c1-bad-syntax.json`(마지막 `}` 없음), `c1-bad-module.json`(핸들러 `reverse_proxyy`), `c1-bad-port.json`(edge 가 npgproxy 의 `127.0.0.1:28080` 을 들으려 함)
- 학습자 셸 함수: `cget 경로`(GET), `csend 메서드 경로 본문`(JSON 본문을 보냄), `H=/id/npg_route/handle/0/response/set`

## 진단 과정

| 순서 | 본 신호(명령·지표) | 세운 가설 | 남김 / 버림과 이유 |
|---|---|---|---|
| 1 | 없는 서버 `servers/nope` → `null` [200], `routes/5` → 400 `array index out of bounds` | 없는 이름은 400, 배열 번호는 통과 (학습자) | 버림. 정반대. 맵의 없는 키는 Go 맵 영값 nil 이 `null` 로 나가고, 배열은 범위를 검사한다 |
| 2 | 있는 칸 PUT → 409 `key already exists`, 없는 칸 PATCH → 404 `key does not exist` | 둘 다 최신 값으로 바뀐다 (학습자) | 버림. Caddy 의 PUT 은 "새로 만들기만", PATCH·DELETE 는 "있는 것만" 이다. 읽기는 너그럽고 쓰기는 의도가 어긋나면 거절한다 |
| 3 | 문법·모듈·포트 오류 셋 다 500, 뒤이어 `:8000` 은 `X-Edge-Route: npg` [200] | 모두 실패하고 옛 값으로 돈다 (학습자) | 남김. 오류 본문으로 실패 단계가 갈린다(본문 해석 / Provision / Start). 새 설정이 다 켜져야만 바꿔 끼운다 |
| 4 | `caddy validate` 가 포트 파일만 `Valid configuration` | API 처럼 셋 다 실패 (학습자) | 버림. validate 는 Provision 까지만 하고 포트를 열지 않는다. 학습자 정리: "포트 충돌은 열어야 알 수 있고, 검사하려 열면 떠 있는 서버와 부딪힌다" |
| 5 | 같은 파일 reload → `config is unchanged`, `--force` → 서버 셋 새로 켜고 옛 것 종료, 인증서 발급 로그 없음 | TLS 수정이라 진행, `--force` 는 캐시를 지우고 새로 캐싱 (학습자) | 버림. 그 파일은 떠 있는 설정과 같았다. `--force` 는 다시 올리되 인증서 캐시는 남긴다 |
| 6 | API 로 `npg-drift` 로 바꾼 뒤 `--resume` 재시작 → `npg-drift`, `--config` 만 → `npg` | A 는 drift, B 는 그대로 (학습자) | 남김. B 는 저장본(`autosave.json`)까지 파일 내용으로 덮었다 |
| 7 | Host `evil.example:2019` → 403, Origin `http://evil.example` → 403 | `listen` 만 있으니 둘 다 통과 (학습자) | 버림. Host 검사는 늘 켜져 있고, Origin 검사는 요청에 Origin 이 있으면 돈다 |

## 실행 전 예측

2026-11-04 에 적었고 실행 뒤 고치지 않았다.

1. C1-1 없는 칸 읽기: "nopes는 안보여서 400관련 에러 아닐까요? // 통과하지 않을까요? 저 Routes뒤에 숫자는 pathvariable같은데"
2. C1-2 있는 칸 PUT, 없는 칸 PATCH: "Edge-Route를 PUT으로 수정하는것과 PATCH로 새로운 헤더에 x를 넣는거 같다. -> 둘다 최신값으로 바뀌지 않을까요?"
3. C1-3 잘못된 설정 셋: "문법오류는 모두 실패로 반환해줄거고 500, 8080은 이전값 그대로 들고 반환해주지 않을까요?" (8080 은 8000 을 뜻함)
4. C1-4 `caddy validate` 로 세 파일: "api도 실패시켰는데 똑같이 다 실패하는거 아냐?"
   - 4b `caddy reload` (잘못된 포트 / 같은 파일 / 같은 파일 `--force`): "reload로 잘못된 포트 올리면 실패할거 같습니다. -> tls수정은 문제 없으면 진행될겁니다 (다만 서버 자체 캐시라서 제대로 안될수도 있다고 배움) -> 변경 사항이 없는데 강제로 하면 이전 캐싱 지우고 새롭게 캐싱한다? (사실 다시 올린다의 흐름을 잘 모르겠음)"
5. C1-5 재시작 두 방식: "resume은 뭔가 재개한다는 옵션 아닌가? (…상황상 아마 수정된걸로 보임) -> A는 npg-drift, B는 그대로"
6. C1-6 문단속: "지금 admin은 Listen으로 듣고만 있다 이 설정이 지금 호스트나 오리진에 악성 도메인을 넣는다고해서 무언가 문제가 생기는가?, API로 이제 오리진에 대한 보안을 활성화하면 Origin에 대해 허용된 출처만 읽을거다, 파일 다시 올리면 api로 했으니 없어지겠지?"

## 실행 (사용자가 친 명령)

```bash
cd net/npg/deploy/caddy
export XDG_DATA_HOME=$PWD/.run/data XDG_CONFIG_HOME=$PWD/.run/config
cget() { curl -s -w ' [%{http_code}]\n' "http://127.0.0.1:2019$1"; }
csend() { curl -s -X "$1" -H 'Content-Type: application/json' -d "$3" -w ' [%{http_code}]\n' "http://127.0.0.1:2019$2"; }
H=/id/npg_route/handle/0/response/set

# C1-1 읽기
cget /config/apps/http/servers/nope
cget /config/apps/http/servers/edge/routes/5

# C1-2 쓰기
csend PATCH $H/X-Edge-Route '["npg-v2"]'
csend PUT $H/X-Edge-Route '["npg-v3"]'
csend PATCH $H/X-Nope '["x"]'
csend PATCH $H/X-Edge-Route '["npg-v3"]'
csend PUT $H/X-Nope '["x"]'
csend DELETE $H/X-Nope
csend PATCH $H/X-Edge-Route '["npg"]'

# C1-3 잘못된 설정
csend PATCH $H/X-Edge-Route '["npg"'
csend PATCH /id/npg_proxy/handler '"reverse_proxyy"'
csend PATCH /config/apps/http/servers/edge/listen '["127.0.0.1:28080"]'
curl -s -o /dev/null -D - http://127.0.0.1:8000/v1/devices | grep -i x-edge-route

# C1-4 validate·reload
for f in c1-bad-syntax c1-bad-module c1-bad-port; do caddy validate --config scenarios/$f.json; done
caddy reload --config scenarios/c1-bad-port.json
caddy reload --config scenarios/e5-internal-tls.json
caddy reload --config scenarios/e5-internal-tls.json --force
tail -n 12 .run/logs/caddy.log | grep -o '"msg":"[^"]*"'

# C1-5 재시작 뒤 복구
csend PATCH "$H/X-Edge-Route" '["npg-drift"]'
./run.sh down
nohup caddy run --resume --config scenarios/e5-internal-tls.json >> .run/logs/caddy.log 2>&1 &
echo $! > .run/caddy.pid
./run.sh down
./run.sh up scenarios/e5-internal-tls.json

# C1-6 문단속
curl -s -H 'Host: evil.example:2019' -w ' [%{http_code}]\n' http://127.0.0.1:2019/config/admin
curl -s -H 'Origin: http://evil.example' -w ' [%{http_code}]\n' http://127.0.0.1:2019/config/admin
csend POST /config/admin/enforce_origin 'true'
cget /config/admin
caddy reload --config scenarios/e5-internal-tls.json
cget /config/admin
```

## 관찰

- C1-1: `null` [200], `{"error":"… array index out of bounds: 5"}` [400].
- C1-2: PATCH 로 `npg-v2` 성공, 손님 응답 `X-Edge-Route: npg-v2`. 성공한 PATCH 한 번에 로그 `server running` 여러 줄과 `autosaved config` 가 찍혔다. 한 칸만 고쳐도 설정 전체를 다시 올린다는 뜻이다. 실패한 PUT·PATCH 뒤에는 이런 줄이 없다.
- C1-3 오류 본문
  - 문법: `decoding request body: unexpected end of JSON input`
  - 모듈: `loading new config: … provision http: server edge: … unknown module`
  - 포트: `loading new config: http app module: start: listening on 127.0.0.1:28080: … bind: address already in use`
  - 셋 다 500, 그 뒤 `:8000` 은 `X-Edge-Route: npg` [200].
- C1-4: validate 는 syntax `config is not valid JSON`, module `unknown module`, port `Valid configuration`. `caddy reload` 로 bad-port 를 올리면 `HTTP 400 … address already in use` (`/load` 는 400, `/config/` 는 500).
- C1-4b 로그(logger 와 함께 읽음)
  - 실패한 bad-port: `admin endpoint started` → `servers shutting down with eternal grace period`(새로 만든 것 정리) → `request error` → admin `stopped previous server` `127.0.0.1:2019`. 앱이 켜지기 전에 관리 API 는 이미 새것으로 바뀌었다.
  - 같은 파일: `config is unchanged` → `load complete`.
  - `--force`: `server running` 셋(edge, edge_tls, remaining_auto_https_redirects) → http `servers shutting down with eternal grace period` → `autosaved config` → `load complete` → admin `stopped previous server`. 인증서 발급·갱신 로그는 없다.
- C1-5: PATCH 뒤 `npg-drift`. `--resume` 재시작 뒤 `npg-drift`. `--config` 만으로 재시작한 뒤 `npg`, 이때 `autosave.json` 의 값도 `npg` 로 덮였다(창 확인).
- C1-6: 가·나 둘 다 403. `enforce_origin` 을 켠 뒤 `cget` 은 403 `client is not allowed to access from origin ''`. 같은 상태에서 `caddy reload` 는 통과했고 `admin` 은 `{"listen":"127.0.0.1:2019"}` 로 돌아왔다.
- 손입력 사고: 처음 가·나는 포트 2010, 헤더 앞 빈칸, `::2019` 로 쳐서 000 이 났다. 포트를 고친 뒤에도 `' Origin: …'` 처럼 앞에 빈칸이 남아 200 이 났다. curl 은 빈칸째 보내고, HTTP/1.1 은 빈칸으로 시작하는 줄을 앞 헤더(`Accept`)에 이어진 줄로 읽는다. 그래서 서버가 받은 요청에는 Origin 이 없었다. 복사해서 붙인 세 번째 실행만 기록으로 쓴다.

## 전후 수치

| 지표 | 관리 API 로 고침 | 파일로 재시작 |
|---|---|---|
| 손님 응답 `X-Edge-Route` | `npg-drift` | `npg` |
| `autosave.json` 의 값 | `npg-drift` | `npg` |
| `--resume` 재시작 뒤 값 | `npg-drift` | `npg` |

| 요청 | 기본(`listen` 만) | `enforce_origin: true` |
|---|---|---|
| Origin 없는 curl | 200 | 403 |
| 나쁜 Host | 403 | 403 |
| 나쁜 Origin | 403 | 403 |
| `caddy` CLI (Origin 을 스스로 붙임) | 통과 | 통과 |

## 포스트모템

- 타임라인: E5 설정이 떠 있는 상태 → 읽기(C1-1) → 한 칸 쓰기(C1-2) → 잘못된 설정 세 벌 거절(C1-3) → validate·reload(C1-4) → API 로 고친 뒤 두 방식 재시작(C1-5) → 문단속(C1-6) → 파일 설정으로 돌아옴. Caddy 는 C1-5 에서 pid 6101 → 28582 로 바뀌었다.
- 근본 원인(이 장면에서 확인한 동작)
  - 한 칸 쓰기도 설정 전체를 다시 만들어 올린다. 새 설정이 조립·Provision·Start 를 모두 통과해야 바꿔 끼우고, 실패하면 옛 설정을 그대로 둔다.
  - 맥에서는 새 서버가 `SO_REUSEPORT` 로 같은 포트에 소켓을 하나 더 열어 겹쳐 둔 뒤, 옛 서버를 우아하게 끈다. 손님 연결이 끊기지 않는다. 이 표시 없이 포트를 쥔 다른 프로그램(npgproxy)과는 겹칠 수 없다.
  - API 로 고친 값은 메모리와 `autosave.json` 에만 있고 정본 파일에는 없다. 파일로 다시 올리면 저장본까지 덮인다.
  - 관리 API 는 기본으로 Host 를 검사하고, 요청에 Origin 이 있으면 Origin 도 검사한다. 같은 기기 안의 프로그램이 Origin 없이 보내는 요청은 기본으로 통과한다.
- 조치
  - 설정 파일을 정본으로 둔다. API 로 급히 고쳤으면 같은 날 정본 파일에 다시 적는다(학습자 정리: "API 를 쓰면 메모리와 파일이 어긋나니 지양하고 파일 수정이 가장 좋다").
  - 올리기 전에 `caddy validate` 를 쓰되, 포트 충돌은 잡지 못한다는 것을 안다. 포트는 reload 가 실패해도 옛 설정이 남으니 reload 결과로 확인한다.
  - 관리 API 를 같은 기기의 다른 프로그램에게서도 막아야 하면 `enforce_origin`, 유닉스 소켓과 파일 권한, `admin.disabled` 중에서 고른다.
- 재발 방지: 이 장면은 코드가 아니라 운영 규칙이라 시험 대신 위 조치를 기록으로 남긴다.

### 창이 설명을 고친 곳

C1-4b 를 실행하기 전에 창이 "다시 올린다"의 흐름을 설명했고, 그중 두 곳이 틀려 실행 뒤 고쳤다.

- `stopped previous server` 를 HTTP 서버를 끄는 표지로 설명했다. 실제 logger 는 `admin` 이고 관리 API(2019)를 바꿨다는 표지다. HTTP 서버를 끄는 표지는 `servers shutting down with eternal grace period` 다.
- "같은 포트는 `listenerPool` 보관함에서 넘겨받는다"고 설명했다. 그것은 unix 가 아닌 OS 의 경로(`listen.go`)다. 맥은 `listen_unix.go` 를 쓰고, `listenerPool` 은 몇 설정이 그 포트를 쓰는지 세기만 한다.

## 원문 근거

- caddy v2.11.7
  - `admin.go` L1180 `unsyncedConfigAccess`, L1279-1285 (맵 GET 은 `v[part]` 를 그대로 인코딩), L1302-1325 (PUT 은 있으면 409, PATCH·DELETE 는 없으면 404), L1345-1347 (배열 범위 검사)
  - `admin.go` L1043 (`/config/` 는 GET 오류만 400), L1088 (`Cache-Control: must-revalidate` 면 강제), L1091 (`errSameConfig` 는 오류로 보지 않음)
  - `admin.go` L230 (`enforceHost`), L337-341 (기본 허용 주소 셋), L872-896 (Host 검사, Origin 검사 조건), L942-990 (`checkHost`·`checkOrigin`), L756-769 (`stopped previous server` 는 관리 서버)
  - `caddy.go` L158-264 `changeConfig` (L211 전체 다시 인코딩, L220-222 같은 설정이면 `config is unchanged`, L245-264 실패하면 옛 JSON 으로), L337-375 `unsyncedDecodeAndRun` (L363 새 설정 시작, L369-375 성공해야 교체·옛 것 정지), L564-570 (앱보다 관리 API 를 먼저 교체), L744-750 `Validate` = `run(cfg, false)`
  - `caddyconfig/load.go` L112-115 (`/load` 오류는 400)
  - `modules/caddyhttp/app.go` L754-768 (`grace_period` 가 없으면 기한 없이 기다림)
  - `listen_unix.go` (`unix && !solaris`) L106-111 (`listenerPool` 은 쓰는 설정 수를 셈), L147-156·L216-217 (`SO_REUSEPORT`). `listen.go` (`!unix || solaris`) 는 같은 소켓을 넘겨줌
  - `cmd/commandfuncs.go` L207-251 (`--resume` 은 `autosave.json` 을 `--config` 보다 앞세우고, 없으면 오류 없이 넘어감), L399 (`--force` 헤더), L821 (CLI 가 Origin 을 붙임)
- curl 8.7.1, HTTP/1.1 obs-fold(RFC 9112 §5.2, 빈칸으로 시작하는 헤더 줄은 앞 줄에 이어짐)

## 이어 읽을 노트

- 10-01.Caddy 는 JSON 설정을 관리 API 로 실시간 바꾸고 모듈과 어댑터로 넓힌다.md (관리 엔드포인트와 설정 로드, 설정 조회와 경로 탐색, 원자적 리로드와 Graceful 전환)
- 09장 리허설 [ch09-deploy-drain-cut.md](ch09-deploy-drain-cut.md) (우아한 종료와 재시작 중 끊김)
