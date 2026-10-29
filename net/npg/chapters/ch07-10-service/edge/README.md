# edge — 사내 API 서비스의 앞단(Caddy)

---

『Network Programming with Go』 10장(Caddy)을 실무 장면으로 옮긴 실습입니다. 같은 서비스의 Go 쪽(`cmd/api`·`cmd/upstream`)은 이 폴더 바깥 `ch07-10-service/` 에 있습니다. 여기서는 그 앞에 서는 Caddy 를 다룹니다.

시나리오마다 설정과 코드를 먼저 읽고, 예측 칸에 답을 적은 뒤 돌립니다. 이 README 에는 결과가 없습니다.

## 이 프로젝트로 배우는 것

1. Caddy 관리 API 로 라우트와 업스트림을 바꾸는 일과 프로세스를 재시작하는 일이 클라이언트에게 어떻게 다르게 보이는지 실패 수와 오류 갈래로 설명할 수 있습니다.
2. 앞 라우트가 뒤 라우트를 가리는 장애를 설정 조회, 응답 헤더, 디버그 로그 순서로 진단할 수 있습니다.
3. `X-Forwarded-For` 를 누가 믿는지(Caddy 의 두 층 `trusted_proxies`, api 의 `-trusted-proxies`, 백엔드 직접 접근)를 짝지어 위조가 통하는 경로를 찾을 수 있습니다.
4. 로드밸런싱, 능동·수동 헬스 체크, 재시도 기간이 배포 중 실패의 어느 구간을 막는지 실측해서 고를 수 있습니다.
5. `tls internal` 로 사내 도메인에 HTTPS 를 붙이고, 무엇을 신뢰시켜야 하는지와 인증서 수명·갱신 주기를 관찰할 수 있습니다.
6. xcaddy 로 커스텀 미들웨어 모듈을 빌드해 넣고, 관리 API 를 Unix 소켓과 파일 권한 뒤로 옮길 수 있습니다.

| 시나리오 | 장애 한 줄 | 배우는 것 | 노트 |
|---|---|---|---|
| E1 | 설정을 바꿀 때마다 재시작해서 연결이 끊긴다 | 1 | [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) 목표 3, [08-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/08-02.Go%20HTTP%20%ED%81%B4%EB%9D%BC%EC%9D%B4%EC%96%B8%ED%8A%B8%EB%8A%94%20%EC%9D%91%EB%8B%B5%20%EB%B3%B8%EB%AC%B8%EC%9D%84%20%EB%8B%AB%EC%95%84%EC%95%BC%20%EC%97%B0%EA%B2%B0%EC%9D%84%20%EC%9E%AC%EC%82%AC%EC%9A%A9%ED%95%98%EA%B3%A0%20context%20%EB%A1%9C%20%EC%9A%94%EC%B2%AD%EC%9D%84%20%EB%81%8A%EB%8A%94%EB%8B%A4.md) §2 |
| E2 | 새 라우트를 넣었더니 `/api` 가 404 | 2 | [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) 목표 3·4 |
| E3 | IP 차단이 우회됐다 (통합) | 3 | [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) 목표 1·2 |
| E4 | 배포할 때마다 502 가 몇 건 난다 (통합) | 4 | [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §1, Caddy `reverse_proxy` 문서 |
| E5 | 사내 도메인에 HTTPS 를 붙여라 | 5 | [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) 목표 5 |
| E6 | 공통 차단 규칙을 모듈로 만들어라 | 6 | [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) 목표 2·5 |
| E7 | 관리 API 를 아무나 부를 수 있었다 | 6 | [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §3, [07-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/07-02.%ED%94%BC%EC%96%B4%20%EC%9E%90%EA%B2%A9%20%EC%A6%9D%EB%AA%85%EC%9C%BC%EB%A1%9C%20%EC%A0%91%EC%86%8D%ED%95%9C%20%ED%94%84%EB%A1%9C%EC%84%B8%EC%8A%A4%EB%A5%BC%20%EC%9D%B8%EC%A6%9D%ED%95%98%EB%8A%94%20%EC%84%9C%EB%B9%84%EC%8A%A4.md) |

통합 시나리오(E3·E4)는 service 창의 진짜 `cmd/api` 로 돌려야 결론이 섭니다. `cmd/api` 가 빌드되지 않으면 `run.sh` 가 계약만 흉내 내는 임시 백엔드(`cmd/fakeapi`)를 띄우므로, 그때 얻은 값은 연습으로만 봅니다. 임시 백엔드는 `-deny-ips`·`-xff-pick`·`-drain-close-keepalive` 를 받기만 하고 흉내 내지 않으며, 받았다는 줄을 로그에 남깁니다. 어느 쪽이 떴는지는 `.run/backend-kind` 에 적힙니다.

이미 해 본 것은 다시 다루지 않습니다. TLS 핸드셰이크와 인증서 서명 해부는 paw 4장이 다뤘습니다.

- paw 4장: [04-01](~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-01.TLS%20%ED%95%B8%EB%93%9C%EC%85%B0%EC%9D%B4%ED%81%AC%20%EC%9D%BD%EA%B8%B0.md), [04-02](~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-02.%EC%9D%B8%EC%A6%9D%EC%84%9C%EC%99%80%20%EB%91%90%20%EC%84%9C%EB%AA%85.md), [04-04](~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-04.%EC%8B%A4%EC%8A%B5%20%E2%80%94%20TLS%20%ED%95%B8%EB%93%9C%EC%85%B0%EC%9D%B4%ED%81%AC%EB%A5%BC%20%EC%A7%81%EC%A0%91%20%EC%9E%A1%EA%B8%B0.md)
- 리버스 프록시 기본(앞단이 연결을 끊고 새로 맺는다): `~/study/gateway-lab/docs/02-lab-plan/03-01.gateway-lab-phase1-plan.md`

여기서는 Caddy 에만 있는 것(관리 API, 모듈, 자동 HTTPS, 라우트 평가)과 그 운영 장애에 집중합니다.



## 준비

모든 명령은 이 폴더(`~/study/runners-lab/net/npg/chapters/ch07-10-service/edge`)에서 실행합니다.

```bash
# 처음 한 번 — xcaddy 로 bin/caddy(커스텀 모듈 포함)와 bin/caddy-stock(표준만)을 만든다
go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.7
./build.sh

# 스택을 띄우고, 상태를 보고, 내린다
./run.sh up
./run.sh status
./run.sh down
```

Caddy v2.11.7 의 `go.mod` 는 `go 1.26.0` 을 요구합니다. 로컬 Go 가 그보다 낮아도 `GOTOOLCHAIN=auto`(기본값)면 go 명령이 맞는 툴체인을 받아 와 빌드하고, 루트 모듈(`runners-lab`, go 1.25.1)은 그대로 둡니다. Caddy 의존은 `caddymod/go.mod` 에만 있습니다.

| 포트·경로 | 누가 | 비고 |
|---|---|---|
| `127.0.0.1:2019` | Caddy 관리 API | E7 에서 `.run/caddy-admin.sock` 으로 옮긴다 |
| `127.0.0.1:8000` | Caddy 앞단 HTTP | 이 맥에서 8080 은 다른 프로세스가 써서 8000 을 쓴다 |
| `127.0.0.1:8081`, `:8082` | api 두 인스턴스 | `./run.sh api-restart 1` 이 배포 한 번을 흉내 낸다 |
| `127.0.0.1:9090` | upstream | service 창의 `cmd/upstream` 이 있을 때만 |
| `127.0.0.1:8443`, `:8088` | E5 의 HTTPS 와 리다이렉트 | E5 설정을 올렸을 때만 |
| `.run/` | 로그·PID·소켓·Caddy 데이터 | `XDG_DATA_HOME`·`XDG_CONFIG_HOME` 을 여기로 돌려 홈 디렉터리를 건드리지 않는다 |

`./run.sh admin <메서드> <경로> [JSON 파일]` 은 관리 API 를 부르는 얇은 래퍼입니다. 응답 본문은 표준 출력에, `[HTTP 200]` 같은 상태 줄은 표준 오류에 찍히므로 본문을 `jq` 로 바로 넘길 수 있습니다. 관리 엔드포인트가 Unix 소켓으로 옮겨 가면 그쪽으로 보냅니다.

`.run/bin/load` 는 `up` 때 함께 빌드되는 부하 도구(`cmd/load`)입니다. 초마다 상태 코드와 전송 오류 갈래(`refused`·`eof`·`reset`·`timeout`·`addrnotavail`·`other`)를 세고, 마지막에 합계, `-header` 로 고른 응답 헤더의 분포, 전송 오류가 난 구간과 갈래별 첫 오류 원문을 찍습니다.

```bash
.run/bin/load -url http://127.0.0.1:8000/api/v1/rates/usd -c 8 -d 15s -header X-Upstream
```



## 파일 구성

| 경로 | 내용 |
|---|---|
| `caddy.json` | 정본 설정. 라우트 두 갈래(`api_route`·`static_route`)와 `@id` 를 단 `reverse_proxy`(`api_proxy`) |
| `Caddyfile`, `Caddyfile.adapted.json` | 같은 앞단을 Caddyfile 로 적은 판과, 그것을 `caddy adapt` 로 바꾼 결과 |
| `caddymod/` | `restrict_prefix` 모듈(자체 `go.mod`)과 테스트 |
| `build.sh`, `run.sh` | xcaddy 빌드, 프로세스 띄우기·걷기·관리 API 호출 |
| `cmd/load`, `cmd/fakeapi` | 부하 도구, 임시 백엔드 (표준 라이브러리만) |
| `site/` | 정적 파일. `.env`·`.secrets/` 는 E6 에서 막을 실험용 가짜 파일 |
| `scenarios/` | 시나리오별로 관리 API 에 올릴 설정 조각과 Caddyfile 변형 |



## 설정 두 판 — JSON 과 Caddyfile

Caddy 코어가 읽는 형식은 JSON 하나입니다. Caddyfile 은 어댑터가 JSON 으로 바꿔 넘기는 사람용 형식입니다([10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §4). 실무에서는 Caddyfile 이 흔합니다. 그래도 관리 API 는 JSON 경로로 설정을 가리키므로 이 랩은 JSON 을 정본으로 둡니다.

```bash
bin/caddy validate --config caddy.json
bin/caddy validate --config Caddyfile
bin/caddy adapt --config Caddyfile --pretty > Caddyfile.adapted.json
```

`caddy.json` 과 `Caddyfile.adapted.json` 을 나란히 놓고 다른 곳을 세어 봅니다. 차이마다 요청 처리를 바꾸는 차이인지, 이름과 감싸는 모양만 다른 차이인지 가릅니다. `@id` 를 양쪽에서 찾아보고, 관리 API 로 `/id/api_proxy/...` 를 부르는 E1·E3·E4 를 Caddyfile 판으로 띄웠다면 무엇을 대신 써야 할지 적습니다.



## E1 설정을 바꿀 때마다 재시작해서 연결이 끊긴다

**증상** — 앞단 설정을 배포할 때마다 API 호출이 몇 초씩 실패합니다. 배포 스크립트는 Caddy 를 내렸다 올립니다.

**실행 전에 적을 예측**

1. 부하를 거는 동안 관리 API 로 라우트 하나를 더하고 업스트림을 두 번 바꾸면 `load` 합계에 실패가 몇 건 찍힐까요? 실패가 있다면 어느 갈래일까요?
2. 같은 부하 중에 Caddy 프로세스를 내렸다 올리면 실패 갈래와 실패 구간 길이는 어떻게 될까요?
3. `caddy reload --force` 를 세 번 할 때 GET 부하와 POST 부하의 실패 수는 같을까요? `-keepalive=false` 로 바꾸면 어떨까요?
4. 설정을 바꾸는 순간 이미 맺혀 있던 keep-alive 연결은 옛 서버와 새 서버 가운데 어디에 붙어 있을까요?
5. `PATCH /id/api_proxy/upstreams` 로 업스트림을 8082 하나로 줄였다가 되돌리는 동안 `X-Upstream` 분포는 어떻게 움직일까요?

**돌리는 법**

```bash
./run.sh up

# 터미널 A
.run/bin/load -d 20s -header X-Upstream

# 터미널 B — A 가 도는 동안 차례로
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e1-version-route.json
curl -s http://127.0.0.1:8000/version
./run.sh admin PATCH /id/api_proxy/upstreams scenarios/e1-upstreams-8082.json
./run.sh admin PATCH /id/api_proxy/upstreams scenarios/e1-upstreams-both.json
```

```bash
# 대조군 — 재시작
./run.sh load caddy.json
.run/bin/load -d 10s & sleep 3; ./run.sh caddy-restart; wait

# 변형 — 같은 설정을 강제로 세 번 다시 올리기. GET, POST, keep-alive 를 끈 GET 으로 한 번씩
RL() { for i in 1 2 3; do bin/caddy reload --config caddy.json --force; sleep 1; done; }
.run/bin/load -d 6s                         & sleep 1.5; RL; wait
.run/bin/load -d 6s -method POST            & sleep 1.5; RL; wait
.run/bin/load -d 6s -keepalive=false -c 2   & sleep 1.5; RL; wait
```

api 는 업무 엔드포인트에 GET 만 받으므로 POST 는 `405` 로 돌아옵니다. `405` 는 응답이 온 것이니 실패로 세지 않고, 세는 것은 그 밖의 상태 코드와 `err:` 줄입니다. keepalive 를 끈 판은 요청마다 임시 포트를 하나씩 쓰므로 `-c 2` 정도로 낮춰 돌립니다.

**볼 것**

- `load` 의 초별 줄, `전송 오류 구간`, 갈래별 첫 오류 원문. `refused` 는 듣는 소켓이 없을 때, `eof`·`reset` 은 맺혀 있던 연결이 상대 쪽에서 끊길 때 납니다. 두 갈래는 원인이 다르므로 섞어 세지 않습니다.
- `.run/logs/caddy.log` 에서 관리 API 요청(`"logger":"admin.api"`) 시각과 실패가 몰린 초를 맞춰 봅니다.
- 두 부하의 결과는 [08-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/08-02.Go%20HTTP%20%ED%81%B4%EB%9D%BC%EC%9D%B4%EC%96%B8%ED%8A%B8%EB%8A%94%20%EC%9D%91%EB%8B%B5%20%EB%B3%B8%EB%AC%B8%EC%9D%84%20%EB%8B%AB%EC%95%84%EC%95%BC%20%EC%97%B0%EA%B2%B0%EC%9D%84%20%EC%9E%AC%EC%82%AC%EC%9A%A9%ED%95%98%EA%B3%A0%20context%20%EB%A1%9C%20%EC%9A%94%EC%B2%AD%EC%9D%84%20%EB%81%8A%EB%8A%94%EB%8B%A4.md) §2 와 `net/http` `Transport` 문서를 근거로 설명합니다.

**이어 읽을 노트**

- [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §3 원자적 리로드 네 단계와 면접 2
- [08-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/08-02.Go%20HTTP%20%ED%81%B4%EB%9D%BC%EC%9D%B4%EC%96%B8%ED%8A%B8%EB%8A%94%20%EC%9D%91%EB%8B%B5%20%EB%B3%B8%EB%AC%B8%EC%9D%84%20%EB%8B%AB%EC%95%84%EC%95%BC%20%EC%97%B0%EA%B2%B0%EC%9D%84%20%EC%9E%AC%EC%82%AC%EC%9A%A9%ED%95%98%EA%B3%A0%20context%20%EB%A1%9C%20%EC%9A%94%EC%B2%AD%EC%9D%84%20%EB%81%8A%EB%8A%94%EB%8B%A4.md) §2·§4



## E2 새 라우트를 넣었더니 `/api` 가 404

**증상** — 사내 문서 사이트를 붙이는 라우트를 넣은 뒤부터 `/api` 호출이 404 입니다. api 로그에는 요청이 한 줄도 없습니다.

**실행 전에 적을 예측**

1. `scenarios/e2-docs-route.json`(매처 없음)을 `routes` 맨 앞(인덱스 0)에 넣었다고 합시다. `/api/v1/rates/usd` 응답에는 어느 `X-Edge-Route` 가 붙을까요? `/` 와 `/docs/` 는 각각 어떤 상태 코드와 `X-Edge-Route` 로 올까요?
2. 같은 라우트를 `POST .../routes` 로 배열 끝에 덧붙이면 어떨까요?
3. 매처 `/docs/*` 를 단 `e2-docs-route-matched.json` 을 맨 앞에 넣으면 `/docs/` 는 어떻게 될까요? 그때 `file_server` 는 디스크의 어느 경로를 열려고 할까요?
4. Caddyfile 에서 매처 없는 `handle` 을 먼저 적으면 `caddy adapt` 결과의 `routes` 순서는 적은 순서와 같을까요? 같은 두 갈래를 `route` 블록 안에 적으면 어떨까요? 매처를 이름 붙은 매처(`@api`)로 바꾸면 어떨까요?

**돌리는 법**

```bash
./run.sh load caddy.json
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e2-docs-route.json
for u in /api/v1/rates/usd / /docs/; do
  printf '%-20s ' "$u"
  curl -s -o /dev/null -D - "http://127.0.0.1:8000$u" | grep -iE '^HTTP|x-edge-route' | tr -d '\r' | paste -sd' ' -
done

# 변형 2 — 배열 끝에 덧붙이기
./run.sh load caddy.json
./run.sh admin POST /config/apps/http/servers/edge/routes scenarios/e2-docs-route.json

# 변형 3 — 매처를 단 판을 맨 앞에
./run.sh load caddy.json
./run.sh admin PUT /config/apps/http/servers/edge/routes/0 scenarios/e2-docs-route-matched.json

# Caddyfile 변형 — 순서를 보고, 직접 올려 본다
for f in handle-reversed route-reversed named-reversed; do
  echo "== $f"
  bin/caddy adapt --config scenarios/e2/Caddyfile.$f 2>/dev/null | jq -c '.apps.http.servers.srv0.routes'
done
bin/caddy reload --config scenarios/e2/Caddyfile.route-reversed --adapter caddyfile
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8000/api/v1/rates/usd
./run.sh load caddy.json
```

진단은 이 순서로 합니다. 앞 단계에서 원인이 보이면 거기서 멈춥니다.

1. 응답 헤더 `X-Edge-Route` 로 어느 라우트가 답했는지 봅니다. 라우트마다 이 헤더를 달아 둔 것이 진단 장치입니다.
2. `./run.sh admin GET /config/apps/http/servers/edge/routes | jq -c '.[] | {id: ."@id", match}'` 로 지금 올라가 있는 순서와 매처를 봅니다.
3. `.run/logs/api1.log`, `api2.log` 에 요청이 닿았는지 봅니다. 닿지 않았다면 앞단에서 끝난 요청입니다.
4. 디버그 로그를 켭니다. `./run.sh admin PUT /config/logging scenarios/e2-debug-logging.json` 을 올리고 요청을 하나 보냅니다. 그다음 `grep 'sanitized path join' .run/logs/caddy.log | tail -1 | jq .` 로 `file_server` 가 만든 파일 경로를 봅니다. 설정을 다시 올리면(`load`) 로그 수준도 원래대로 돌아갑니다.

**볼 것**

- `X-Edge-Route` 값
- 관리 API 가 돌려준 `routes` 배열의 순서
- 디버그 로그의 `result` 경로
- `adapt` 결과의 `routes` 순서와 `group`·`subroute`

**이어 읽을 노트**

- [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) §2 와 면접 2
- Caddyfile 지시어가 `adapt` 결과의 라우트로 바뀌는 규칙은 공식 문서 [Directives](https://caddyserver.com/docs/caddyfile/directives)



## E3 IP 차단이 우회됐다 (통합)

**증상** — api 의 차단 목록에 넣은 주소가 로그에는 다른 IP 로 찍히며 계속 들어옵니다. 클라이언트가 `X-Forwarded-For` 를 직접 채워 보내고 있었습니다.

아래 다섯 판마다 클라이언트는 `X-Forwarded-For: 203.0.113.195` 를 실어 `/whoami` 를 부릅니다. api 는 직전 홉(`remote_addr`)이 `-trusted-proxies` 대역 안에 있을 때만 XFF 를 읽어 `client_ip` 를 정합니다.

**실행 전에 적을 예측** — 판마다 `/whoami` 의 세 값(`remote_addr`, `x_forwarded_for`, `client_ip`)과 Caddy 접근 로그의 `request.remote_ip`·`request.client_ip` 를 표로 적습니다.

| 판 | Caddy | api `-trusted-proxies` | 요청 경로 |
|---|---|---|---|
| a | 기본(`caddy.json`) | `127.0.0.1/32` | Caddy 경유 |
| b | `reverse_proxy` 핸들러에 `trusted_proxies: ["127.0.0.1/32"]` | `127.0.0.1/32` | Caddy 경유 |
| c | 서버 단 `trusted_proxies`(static, `127.0.0.1/32`) | `127.0.0.1/32` | Caddy 경유 |
| d | b 와 같음 | 비움 | Caddy 경유, 그리고 api 직접 |
| e | 상관없음 | `127.0.0.1/32` | api(8081) 직접 |

그 밖에 세 가지를 더 적습니다.

- b 와 c 는 api 에게 같은 헤더를 넘길까요? 다르다면 Caddy 의 어디가 다를까요?
- 블로그에서 본 `trusted_proxies private_ranges` 를 JSON 의 `ranges` 에 문자열 그대로 적어 올리면 어떻게 될까요?
- 이 실습은 클라이언트·Caddy·api 가 모두 `127.0.0.1` 에 있습니다. 이 점은 판 e 의 결론을 실제 서버 배치로 옮길 때 무엇을 흐릴까요?

**돌리는 법**

```bash
W() { curl -s -H 'X-Forwarded-For: 203.0.113.195' "${1:-http://127.0.0.1:8000}/whoami"; echo; }
L() { grep '"http.log.access' .run/logs/caddy.log | tail -1 | jq -c '.request | {remote_ip, client_ip}'; }

./run.sh load caddy.json;                                                              W; L   # a
./run.sh admin PUT /id/api_proxy/trusted_proxies scenarios/e3-handler-trust-loopback.json;   W; L   # b
./run.sh load caddy.json
./run.sh admin PUT /config/apps/http/servers/edge/trusted_proxies scenarios/e3-server-trust-loopback.json; W; L   # c
./run.sh load caddy.json
./run.sh admin PUT /config/apps/http/servers/edge/trusted_proxies scenarios/e3-server-trust-private-json.json   # private_ranges
W http://127.0.0.1:8081                                                                      # e

# d — api 를 신뢰 대역 없이 다시 띄운다
API_TRUSTED= ./run.sh api-restart 1; API_TRUSTED= ./run.sh api-restart 2
./run.sh load caddy.json; ./run.sh admin PUT /id/api_proxy/trusted_proxies scenarios/e3-handler-trust-loopback.json
W; W http://127.0.0.1:8081

# 원래대로
./run.sh load caddy.json; ./run.sh api-restart 1; ./run.sh api-restart 2
```

증상의 "차단 우회" 는 api 의 `-deny-ips` 로 직접 만들어 봅니다. 이 실습의 클라이언트는 `127.0.0.1` 이므로 `-deny-ips 127.0.0.1/32` 는 지금 요청하는 클라이언트를 차단 목록에 넣는다는 뜻입니다. 판 a·b·e 마다 XFF 를 실은 요청과 싣지 않은 요청의 상태 코드를 먼저 적고 돌립니다.

```bash
API_EXTRA='-deny-ips 127.0.0.1/32' ./run.sh api-restart 1; API_EXTRA='-deny-ips 127.0.0.1/32' ./run.sh api-restart 2
D() {  # XFF 를 실은 요청, 싣지 않은 요청
  for h in 'X-Forwarded-For: 203.0.113.195' 'X-Nothing: 1'; do
    curl -s -o /dev/null -w '%{http_code} ' -H "$h" "${1:-http://127.0.0.1:8000}/api/v1/rates/usd"
  done; echo
}
./run.sh load caddy.json; D                                                                 # a
./run.sh admin PUT /id/api_proxy/trusted_proxies scenarios/e3-handler-trust-loopback.json; D   # b
D http://127.0.0.1:8081                                                                     # e
./run.sh load caddy.json; ./run.sh api-restart 1; ./run.sh api-restart 2
```

api 는 XFF 를 오른쪽부터 읽을지 왼쪽부터 읽을지를 `-xff-pick rightmost|leftmost` 로 고릅니다(기본 rightmost). 판 b 에서 XFF 에 주소를 두 개(`198.51.100.7, 203.0.113.195`) 실어 두 방향의 `client_ip` 를 예측한 뒤, `API_EXTRA='-xff-pick leftmost'` 로 두 인스턴스를 다시 띄워 확인합니다.

```bash
W2() { curl -s -H 'X-Forwarded-For: 198.51.100.7, 203.0.113.195' "${1:-http://127.0.0.1:8000}/whoami"; echo; }
./run.sh load caddy.json; ./run.sh admin PUT /id/api_proxy/trusted_proxies scenarios/e3-handler-trust-loopback.json
W2; W2                                                                                       # b, rightmost (api1·api2 한 번씩)
API_EXTRA='-xff-pick leftmost' ./run.sh api-restart 1; API_EXTRA='-xff-pick leftmost' ./run.sh api-restart 2
W2; W2                                                                                       # b, leftmost
./run.sh load caddy.json; ./run.sh api-restart 1; ./run.sh api-restart 2
```

**볼 것**

- `/whoami` 의 `remote_addr` 는 api 와 TCP 를 맺은 직전 홉이고, `x_forwarded_for` 는 api 가 받은 헤더 그대로이며, `client_ip` 는 api 의 판정입니다.
- 접근 로그의 `remote_ip` 와 `client_ip` 가 갈리는 판이 어느 것인지 봅니다. 그 판에서 Caddy 의 `client_ip` 매처는 무엇을 기준으로 삼게 될지도 적습니다.
- `private_ranges` 판은 관리 API 의 상태 줄과 본문을 그대로 적습니다

**이어 읽을 노트**

- [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) §1·§3 과 면접 1
- service 창의 `internal/clientip`
- Caddy 소스에서 헤더를 만드는 곳은 [`addForwardedHeaders` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddyhttp/reverseproxy/reverseproxy.go#L929)
- 서버 단 옵션은 [`trusted_proxies` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddyhttp/server.go#L279)



## E4 배포할 때마다 502 가 몇 건 난다 (통합)

**증상** — api 를 한 대씩 재시작하는 배포 때마다 502 가 찍힙니다. 남은 한 대는 멀쩡한데도 그렇습니다.

api 는 SIGTERM 을 받으면 `/healthz` 를 503 `draining` 으로 바꿉니다. 얼마 뒤에는 리스너를 닫고 끝납니다. 진짜 api 는 그 시간을 `-drain-delay`(기본 5s)로 정하고, 임시 백엔드는 `-drain 3s` 로 흉내 냅니다.

`./run.sh api-restart 1` 은 api1 에 SIGTERM 을 보내고 끝날 때까지 기다렸다가 다시 띄웁니다.

**실행 전에 적을 예측** — 네 판마다 실패 수와 실패가 몰리는 구간(드레인 시작, 리스너가 닫힌 뒤, 다시 뜨기 전)을 적습니다. 상태 코드는 기본 판을 뺀 세 판만 적습니다.

| 판 | 올리는 조각 |
|---|---|
| 기본 | 없음 (`round_robin`, 헬스 체크·재시도 없음) |
| 능동 헬스 체크 | `scenarios/e4-active-health.json` (1초마다 `/healthz`) |
| 재시도 기간 | `scenarios/e4-try-duration.json` (`try_duration` 2s, `try_interval` 100ms) |
| 둘 다 | 위 두 조각 |

그리고 세 가지를 더 적습니다.

- api2 가 멀쩡한데도 능동 헬스 체크만 켠 판에서 실패가 날 수 있을까요? 난다면 어떤 상태 코드일까요?
- 부하를 POST 로 바꾸면 재시도 기간 판의 실패 수는 GET 과 같을까요?
- 수동 헬스 체크(`e4-passive-health.json`)는 어느 구간을 줄이고 어느 구간은 못 줄일까요?
- api 가 드레인을 시작할 때 keep-alive 를 끄게 하면(`API_EXTRA='-drain-close-keepalive'`) 기본 판의 실패는 어떻게 바뀔까요?

**돌리는 법**

```bash
E4() {  # 판 하나: E4 <이름> [경로 조각]...
  ./run.sh load caddy.json
  while [[ $# -ge 3 ]]; do ./run.sh admin POST "$2" "$3"; set -- "$1" "${@:4}"; done
  sleep 1.5
  .run/bin/load -d 12s -method "${M:-GET}" -header X-Upstream > .run/e4.txt &
  sleep 2; ./run.sh api-restart 1; wait
  echo "== $1"; cat .run/e4.txt
}
E4 기본
E4 능동 /id/api_proxy/health_checks scenarios/e4-active-health.json
E4 재시도 /id/api_proxy/load_balancing scenarios/e4-try-duration.json
E4 둘다 /id/api_proxy/health_checks scenarios/e4-active-health.json /id/api_proxy/load_balancing scenarios/e4-try-duration.json
M=POST E4 재시도-POST /id/api_proxy/load_balancing scenarios/e4-try-duration.json
E4 수동 /id/api_proxy/health_checks scenarios/e4-passive-health.json

# 드레인 때 keep-alive 를 끄는 api 로 기본 판을 다시
API_EXTRA='-drain-close-keepalive' ./run.sh api-restart 1; API_EXTRA='-drain-close-keepalive' ./run.sh api-restart 2
API_EXTRA='-drain-close-keepalive' E4 기본-drain-close-keepalive
./run.sh api-restart 1; ./run.sh api-restart 2
```

**볼 것**

- `load` 의 초별 줄에서 실패가 몰린 초와 api1 이 드레인·종료 단계를 남긴 시각을 맞춥니다. 진짜 api 는 `grep -E 'drain start|draining|shutdown (start|done)' .run/logs/api1.log` 로 봅니다. 임시 백엔드라면 `grep -E 'SIGTERM|Shutdown|stopped' .run/logs/api1.log` 입니다.
- Caddy 로그에서 logger 가 `http.handlers.reverse_proxy.health_checker` 로 시작하는 줄과 `http.log.error` 줄이 찍힌 시각을 봅니다.
- `X-Upstream 분포` 에서 `(없음)` 은 응답에 `X-Upstream` 헤더가 없던 요청입니다. 전송 오류는 이 분포에 세지 않습니다.

**이어 읽을 노트**

- [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §1
- Caddy [`reverse_proxy` 문서](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)의 load balancing·health checks 절
- 결과를 설명할 때는 소스의 재시도 판단 [`tryAgain` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddyhttp/reverseproxy/reverseproxy.go#L1393)과 [`RoundRobinSelection.Select` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddyhttp/reverseproxy/selectionpolicies.go#L334)를 근거로 씁니다



## E5 사내 도메인에 HTTPS 를 붙여라

**증상** — 사내 도메인 `api.corp.internal` 에 HTTPS 를 붙이라는 요청이 왔습니다. 공인 도메인이 아니어서 Let's Encrypt 는 쓸 수 없습니다.

`scenarios/e5-internal-tls.json` 은 `caddy.json` 에 HTTPS 서버(`127.0.0.1:8443`, 호스트 매처 `api.corp.internal`)와 내부 발급자(`internal`) 자동화 정책을 더한 판입니다. `http_port`·`https_port` 를 8088·8443 으로 옮겨 80·443 을 쓰지 않습니다. 로컬 CA 는 `.run/data/caddy/pki/authorities/local/` 에 생기고, `install_trust: false` 로 macOS 키체인에는 넣지 않습니다.

**실행 전에 적을 예측**

1. 루트, 중간, 잎 인증서의 수명은 각각 얼마일까요? 누가 누구를 서명할까요?
2. curl 에 `--cacert` 로 무엇을 주면 통과할까요? 아무것도 주지 않으면, 중간 인증서만 주면 어떨까요? 클라이언트에 중간 인증서를 신뢰시켜 두면 나중에 무슨 일이 날까요?
3. `http://api.corp.internal:8088/whoami` 로 오면 Caddy 는 `Location` 을 어디로 줄까요?
4. 잎 수명을 3분으로 줄인 판(`e5-short-lifetime.json`, 호스트 `short.corp.internal`)은 언제 새 인증서로 바뀔까요? 같은 이름으로 기본 수명 인증서가 이미 저장소에 있었다면 어땠을까요?
5. `.run/data` 를 지우고 다시 띄우면 루트 인증서는 어떻게 될까요? 그때 이미 루트를 신뢰시켜 둔 클라이언트는 어떻게 될까요?

**돌리는 법**

```bash
./run.sh load scenarios/e5-internal-tls.json
CA=.run/data/caddy/pki/authorities/local
R=(--resolve api.corp.internal:8443:127.0.0.1)
curl -s -o /dev/null -w '%{http_code} %{errormsg}\n' "${R[@]}" https://api.corp.internal:8443/whoami
curl -s -o /dev/null -w '%{http_code} %{errormsg}\n' "${R[@]}" --cacert $CA/root.crt https://api.corp.internal:8443/whoami
curl -s -o /dev/null -w '%{http_code} %{errormsg}\n' "${R[@]}" --cacert $CA/intermediate.crt https://api.corp.internal:8443/whoami
curl -s -o /dev/null -w '%{http_code} -> %{redirect_url}\n' --resolve api.corp.internal:8088:127.0.0.1 http://api.corp.internal:8088/whoami

for f in $CA/root.crt $CA/intermediate.crt; do openssl x509 -in $f -noout -subject -issuer -dates; done
openssl s_client -connect 127.0.0.1:8443 -servername api.corp.internal </dev/null 2>/dev/null |
  openssl x509 -noout -subject -issuer -dates -serial

# 갱신 관찰 — 15초마다 serial 과 만료 시각, 4분쯤
./run.sh load scenarios/e5-short-lifetime.json
scenarios/e5-watch-serial.sh short.corp.internal 15 16

./run.sh load caddy.json
```

루트를 운영체제에 신뢰시키는 명령은 `XDG_DATA_HOME=$PWD/.run/data bin/caddy trust` 이고 되돌리는 명령은 `caddy untrust` 입니다. 키체인을 바꾸므로 이 랩에서는 실행하지 않고 `curl --cacert` 로 확인합니다.

ACME 는 공인 도메인이 없어 돌리지 않고, 조건만 따져 봅니다. 아래 Caddyfile 을 공인 서버에 올린다고 할 때 인증서가 나오려면 무엇이 갖춰져야 하는지, 80 을 막고 443 만 열면 어느 챌린지가 남는지, 와일드카드가 필요하면 무엇을 더해야 하는지 적습니다.

```nginx
{
	email ops@example.com
}
api.example.com {
	reverse_proxy 127.0.0.1:8081 127.0.0.1:8082
}
```

**볼 것**

- `openssl` 출력의 `notBefore`·`notAfter`·`issuer`
- curl 의 `errormsg`
- 리다이렉트의 `Location`
- Caddy 로그의 `pki.ca.local`(신뢰 저장소 설치 여부)·`tls.obtain`·`tls.renew` 메시지
- `e5-watch-serial.sh` 출력에서 serial 이 바뀐 시각
- 핸드셰이크 자체는 다시 해부하지 않고 paw [04-04](~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-04.%EC%8B%A4%EC%8A%B5%20%E2%80%94%20TLS%20%ED%95%B8%EB%93%9C%EC%85%B0%EC%9D%B4%ED%81%AC%EB%A5%BC%20%EC%A7%81%EC%A0%91%20%EC%9E%A1%EA%B8%B0.md) 의 방법을 그대로 쓰면 됩니다

**이어 읽을 노트**

- [10-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-02.Caddy%20%EB%A5%BC%20%EC%95%9E%EB%8B%A8%EC%97%90%20%EB%91%90%EA%B3%A0%20%EB%B0%B1%EC%97%94%EB%93%9C%EB%A1%9C%20%EB%A6%AC%EB%B2%84%EC%8A%A4%20%ED%94%84%EB%A1%9D%EC%8B%9C%ED%95%98%EB%A9%B0%20%EC%A0%95%EC%A0%81%20%ED%8C%8C%EC%9D%BC%EA%B3%BC%20%EC%9E%90%EB%8F%99%20HTTPS%20%EB%A5%BC%20%EB%B6%99%EC%9D%B8%EB%8B%A4.md) §4 와 면접 3·4
- paw [04-02](~/runners-high/write/02_os/book/paw_packet-analysis-wireshark/04-02.%EC%9D%B8%EC%A6%9D%EC%84%9C%EC%99%80%20%EB%91%90%20%EC%84%9C%EB%AA%85.md)
- 리다이렉트 주소를 만드는 곳은 [`makeRedirRoute` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddyhttp/autohttps.go#L544)
- 수명 기본값은 [`caddypki/ca.go` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddypki/ca.go#L475)와 [`internalissuer.go` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/modules/caddytls/internalissuer.go#L200)



## E6 공통 차단 규칙을 모듈로 만들어라

**증상** — 정적 사이트 폴더에 실수로 들어간 `.env` 가 외부에서 내려받혔습니다. 서버마다 막는 설정을 복사해 두지 말고 공통 모듈 하나로 만들라는 요청이 왔습니다.

`caddymod/restrictprefix.go` 는 노트 10-01 §5 의 `restrict_prefix` 를 바탕으로 접두사를 여러 개 받게 넓힌 모듈입니다. 코드를 먼저 읽고, 노트의 코드와 다른 곳 두 군데(기본값을 채우는 자리, 막을 때 응답을 내는 방식)를 찾아 둡니다.

**실행 전에 적을 예측**

1. `bin/caddy` 와 `bin/caddy-stock` 의 `list-modules` 는 무엇이 다를까요? 바이너리 크기는 얼마나 다를까요?
2. 기본 접두사(`.`)로 넣은 뒤 `/.well-known/security.txt`, `/%2eenv`, `/docs/v1.2/` 는 각각 어떻게 될까요? `/.env` 처럼 뻔한 경로는 테스트가 계약으로 확인하므로 예측에서 뺍니다.
3. `Validate` 가 거부하는 설정(`"prefixes": [".env", "/admin"]`)을 관리 API 로 올리면 응답 코드와 본문은 어떨까요? 그 직후 `GET /config/` 는 어느 설정을 보여 줄까요? 그 사이 들어온 요청은 어떻게 처리될까요?
4. 표준 바이너리로 띄운 Caddy 에 같은 JSON 조각을 올리면 어떻게 될까요? `scenarios/e6/Caddyfile` 을 두 바이너리로 각각 `adapt` 하면 어떨까요?
5. 노트 코드처럼 응답을 직접 쓰고 `nil` 을 돌려줄 때와, 이 모듈처럼 `caddyhttp.Error` 를 돌려줄 때 접근 로그와 응답 본문은 무엇이 다를까요?

**돌리는 법**

```bash
cd caddymod && go vet ./... && go test ./... -count=1 -v; cd ..
ls -l bin/
bin/caddy list-modules | grep restrict; bin/caddy-stock list-modules | grep restrict

./run.sh load caddy.json
./run.sh admin PUT /id/static_route/handle/1 scenarios/e6-restrict.json
for u in /.env /.secrets/token.txt /.well-known/security.txt /%2eenv /docs/v1.2/ /; do
  printf '%-28s ' "$u"; curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:8000$u"
done

# 거부될 설정을 올려 본다
./run.sh load caddy.json
./run.sh admin PUT /id/static_route/handle/1 scenarios/e6-restrict-bad.json
./run.sh admin GET /id/static_route/handle | jq -c 'map(.handler)'

# 표준 바이너리
CADDY_BIN=bin/caddy-stock ./run.sh caddy-restart
./run.sh admin PUT /id/static_route/handle/1 scenarios/e6-restrict.json
./run.sh caddy-restart
bin/caddy adapt --config scenarios/e6/Caddyfile >/dev/null; echo "exit=$?"
bin/caddy-stock adapt --config scenarios/e6/Caddyfile >/dev/null; echo "exit=$?"
```

**볼 것**

- `go test -v` 의 `t.Logf` 줄(예측 2 의 경계 경로)
- 관리 API 응답 본문에서 오류가 난 단계(모듈 로드·프로비저닝·검증)
- 거부 뒤의 `GET` 결과
- 접근 로그의 `status`
- 디버그 로그(`e2-debug-logging.json` 을 올린 뒤)의 `http.handlers.restrict_prefix` 줄

**이어 읽을 노트**

- [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §2·§5 와 면접 4
- [09-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/09-02.%EB%AF%B8%EB%93%A4%EC%9B%A8%EC%96%B4%EB%8A%94%20%ED%95%B8%EB%93%A4%EB%9F%AC%EB%A5%BC%20%EA%B0%90%EC%8B%B8%EA%B3%A0%20ServeMux%20%EB%8A%94%20%EA%B2%BD%EB%A1%9C%EB%A5%BC%20%EB%82%98%EB%88%84%EB%A9%B0%20HTTP2%20%EC%84%9C%EB%B2%84%20%ED%91%B8%EC%8B%9C%EB%8A%94%20%EB%B8%8C%EB%9D%BC%EC%9A%B0%EC%A0%80%EA%B0%80%20%EA%B1%B7%EC%96%B4%EB%83%88%EB%8B%A4.md) §3
- xcaddy 가 만드는 `main` 모듈 모양은 [xcaddy README](https://github.com/caddyserver/xcaddy/blob/v0.4.7/README.md)
- 모듈 작성 규칙은 공식 문서 [Extending Caddy](https://caddyserver.com/docs/extending-caddy)



## E7 관리 API 를 아무나 부를 수 있었다

**증상** — 보안 점검에서, 서버의 아무 로컬 프로세스나 `curl localhost:2019/load` 로 앞단 설정을 통째로 바꿀 수 있다는 지적을 받았습니다.

Caddy 는 관리 엔드포인트를 `unix/<경로>` 로 Unix 소켓에 열 수 있습니다. `unix/<경로>|0660` 처럼 `|` 뒤에 8진 권한 비트도 붙일 수 있습니다. 근거는 공식 문서 [Conventions — Network addresses](https://caddyserver.com/docs/conventions#network-addresses)입니다.

service 창의 관리 소켓 S9 는 커널이 알려 주는 피어 자격 증명으로 접속자를 확인합니다. 근거는 [07-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/07-02.%ED%94%BC%EC%96%B4%20%EC%9E%90%EA%B2%A9%20%EC%A6%9D%EB%AA%85%EC%9C%BC%EB%A1%9C%20%EC%A0%91%EC%86%8D%ED%95%9C%20%ED%94%84%EB%A1%9C%EC%84%B8%EC%8A%A4%EB%A5%BC%20%EC%9D%B8%EC%A6%9D%ED%95%98%EB%8A%94%20%EC%84%9C%EB%B9%84%EC%8A%A4.md) §1·§3 입니다. Caddy 관리 소켓이 접속자를 무엇으로 가르는지는 예측 5에서 따집니다.

**실행 전에 적을 예측**

1. `scenarios/e7-admin-unix.json` 을 TCP 관리 API 로 올리면 그 요청의 응답은 올까요? 그 다음 2019 포트는 어떻게 될까요?
2. 권한 비트를 적지 않은 소켓 파일의 권한은 무엇일까요? 그 권한으로 소유자인 내가 접속할 수 있을까요?
3. `chmod` 로 소유자 쓰기 권한을 빼면 어떻게 될까요? 읽기 권한만 빼면 어떨까요?
4. Caddy 를 내리면 소켓 파일은 남을까요? 남는다면 다음 기동에 문제가 될까요?
5. 같은 사용자로 도는 다른 프로세스(예: 탈취된 api)는 파일 권한으로 막을 수 있을까요? 운영 서버에서는 무엇으로 경계를 세워야 할까요?

**돌리는 법**

```bash
./run.sh load caddy.json
./run.sh load scenarios/e7-admin-unix.json
lsof -nP -iTCP:2019 -sTCP:LISTEN; ls -l .run/caddy-admin.sock
curl -s --unix-socket .run/caddy-admin.sock http://localhost/config/admin; echo
curl -s http://127.0.0.1:2019/config/admin; echo "exit=$?"

chmod 0400 .run/caddy-admin.sock; curl -sS --unix-socket .run/caddy-admin.sock http://localhost/config/admin; echo
chmod 0200 .run/caddy-admin.sock; curl -sS --unix-socket .run/caddy-admin.sock http://localhost/config/admin; echo

./run.sh load scenarios/e7-admin-unix-0600.json; ls -l .run/caddy-admin.sock

# 예측 4 — 소켓에 연 채로 내렸다가 같은 설정으로 다시 띄운다
./run.sh caddy-stop; ls -l .run/caddy-admin.sock
./run.sh caddy-start scenarios/e7-admin-unix-0600.json; ls -l .run/caddy-admin.sock
tail -5 .run/logs/caddy.log | jq -c '{logger, msg, error}'

# TCP 로 되돌리고 내린다
./run.sh load caddy.json; lsof -nP -iTCP:2019 -sTCP:LISTEN; ls -l .run/caddy-admin.sock
./run.sh down
```

`./run.sh admin` 은 소켓 파일이 있으면 그쪽으로 먼저 보내고, 접속이 안 되면 TCP 로 넘어가며 한 줄을 남깁니다. 소켓으로 옮긴 뒤 Caddy CLI 를 쓸 때는 `bin/caddy reload --address unix/.run/caddy-admin.sock ...` 처럼 주소를 넘깁니다.

**볼 것**

- `lsof` 의 LISTEN 줄
- `ls -l` 의 권한 문자열(맨 앞 `s` 는 소켓 파일)
- curl 의 종료 코드와 오류 문장
- Caddy 로그의 `"logger":"admin"` 줄에 찍힌 주소

**이어 읽을 노트**

- [07-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/07-01.Unix%20%EB%8F%84%EB%A9%94%EC%9D%B8%20%EC%86%8C%EC%BC%93%EC%9D%80%20%ED%8C%8C%EC%9D%BC%20%EA%B2%BD%EB%A1%9C%EB%A1%9C%20%EB%AC%B6%EC%9D%B4%EA%B3%A0%20%EC%84%B8%20%EA%B0%80%EC%A7%80%20%ED%83%80%EC%9E%85%EC%9D%B4%20%EA%B2%BD%EA%B3%84%EB%A5%BC%20%EB%8B%A4%EB%A5%B4%EA%B2%8C%20%EB%8B%A4%EB%A3%AC%EB%8B%A4.md)
- [07-02](~/runners-high/write/02_os/book/npg_network-programming-with-go/07-02.%ED%94%BC%EC%96%B4%20%EC%9E%90%EA%B2%A9%20%EC%A6%9D%EB%AA%85%EC%9C%BC%EB%A1%9C%20%EC%A0%91%EC%86%8D%ED%95%9C%20%ED%94%84%EB%A1%9C%EC%84%B8%EC%8A%A4%EB%A5%BC%20%EC%9D%B8%EC%A6%9D%ED%95%98%EB%8A%94%20%EC%84%9C%EB%B9%84%EC%8A%A4.md)
- [10-01](~/runners-high/write/02_os/book/npg_network-programming-with-go/10-01.Caddy%20%EB%8A%94%20JSON%20%EC%84%A4%EC%A0%95%EC%9D%84%20%EA%B4%80%EB%A6%AC%20API%20%EB%A1%9C%20%EC%8B%A4%EC%8B%9C%EA%B0%84%20%EB%B0%94%EA%BE%B8%EA%B3%A0%20%EB%AA%A8%EB%93%88%EA%B3%BC%20%EC%96%B4%EB%8C%91%ED%84%B0%EB%A1%9C%20%EB%84%93%ED%9E%8C%EB%8B%A4.md) §3
- 권한 비트를 읽는 소스는 [`SplitUnixSocketPermissionsBits` (v2.11.7)](https://github.com/caddyserver/caddy/blob/v2.11.7/internal/sockets.go#L34)
- 원격 관리가 필요할 때의 대안은 공식 문서 [admin — remote](https://caddyserver.com/docs/json/admin/remote/)(mTLS 와 접근 제어)



## 이번에 만들지 않은 것

노트 10-01 목표 4 의 TOML 설정 어댑터(E8)는 실무 빈도가 낮아 만들지 않았습니다. 해 보려면 `caddymod/` 에 `caddyconfig.Adapter` 를 구현하는 패키지를 하나 더 두고 `build.sh` 의 `--with` 로 함께 넣으면 됩니다.
