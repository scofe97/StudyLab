# npg-lab

『Network Programming with Go』(Adam Woodbeck, 2021) 3~12장을 장비 프로비저닝 서비스 한 벌로 키우는 실습 저장소입니다. 장마다 기능을 하나씩 더하고, 그 기능이 만드는 판단 지점을 ADR 로 남깁니다.

runners-lab 모노레포에서는 `net/npg` 가 제품 코드이고, 장별 실습 코드(옛 npg-practice)는 `net/npg/chapters/` 에 있습니다. 학습 노트와 진행 상태는 runners-high 의 `write/02_os/book/npg_network-programming-with-go/` 에 있습니다.

## 제품 모양

```
agent ─TCP 제어 채널·하트비트(3·4장)─▶ provd
agent ─UDP HELLO 공지(5장)──────────▶ provd
agent ─TFTP 펌웨어(6장)─────────────▶ tftpd
agent ─상호 TLS(11장) → gRPC(12장)─▶ provd
운영자 ─HTTP─▶ npgproxy [받는 쪽(9장) │ 업스트림 층(8장)] ─▶ inventory 등 백엔드
provctl ─Unix 소켓(7장)─▶ provd 관리 (7장은 보류, 이슈 #5)
```

8장부터는 envoy·nginx 같은 리버스 프록시 `npgproxy` 를 키웁니다(ADR 0011). 10장의 Caddy 는 비교 대상이나 앞단으로 둡니다.

## 장 주기

요구사항(이슈) → 설계 판단(ADR) → 구현 → 시험 → 장애 리허설(예측 → 사용자가 직접 실행 → 해석) → 릴리스 태그 순서로 돕니다. 판단 지점에서는 설명을 먼저 듣고, 사용자가 결정하고, 그 결정을 ADR 로 적은 뒤에 코드를 씁니다.

## 브랜치와 태그

장마다 `feat/chNN-<주제>` 브랜치를 만들고 PR 로 main 에 병합한 뒤 `v0.N.0` 태그를 붙입니다. main 에 직접 push 하지 않습니다(첫 뼈대 커밋만 예외입니다).

## 장별 판단과 리허설

| 장 | 태그 | 직접 정할 판단 | 장애 리허설 | 노트 |
|---|---|---|---|---|
| 3 | v0.3.0 | 하트비트 간격과 읽기 데드라인 비율, 재접속 백오프 | 죽은 연결을 늦게 알아챔 | 03-03 |
| 4 | v0.4.0 | 프레임 길이 상한, keepalive 사용 여부 | 길이 필드 위조, CLOSE_WAIT 누적 | 04-01, 04-03 |
| 5 | v0.5.0 | UDP 공지 재전송 횟수·간격, MTU 아래 크기 | 공지 유실, 단편화 | 05-01, 05-02 |
| 6 | v0.6.0 | 블록 크기·타임아웃·재시도, 루트 밖 접근 차단 | T1~T7 | 06-01, 06-02 |
| 7 | v0.7.0 | 관리 소켓 인가 기준(uid·gid, `SO_PEERCRED`), 소켓 파일 권한 | S9~S11 | 07-01, 07-02 |
| 8 | v0.8.0 | 업스트림 기한 네 가지, 재시도할 요청(멱등성) | S1~S3·S8 | 08-01, 08-02 |
| 9 | v0.9.0 | 서버 타임아웃 네 필드, 드레인 순서 | S4~S7 | 09-01, 09-02 |
| 10 | v0.10.0 | 라우트 순서, `trusted_proxies` 대역 | E1~E7 | 10-01, 10-02 |
| 11 | v0.11.0 | `ClientAuth` 값, 피닝 대상(서버 인증서 / CA), 장비 인증서 이름 규칙 | K1~K5(새로) | 11-01, 11-02 |
| 12 | v0.12.0 | proto 필드 번호·버전 경로, 오류 → gRPC 상태 코드, 기한 전파 | G 장면(새로) | 12-01, 12-02 |

노트 열은 파일 이름 앞머리만 적었습니다. 노트는 다른 저장소에 있어서 상대 링크를 걸지 않습니다.

## 참조 구현

`chapters/`(옛 npg-practice)는 참조 전용입니다. 그 장 차례에 필요한 패키지와 장면만 가져와 제품 구조에 맞게 고칩니다.

## 관측

`ss`·`strace`·`tc netem` 같은 Linux 전용 관측은 OrbStack `ubuntu` 에서 합니다.

## 실행

모노레포 루트에서 돌립니다. `chapters/ch07-10-service/edge/caddymod/` 는 Caddy 의존(go 1.26.0) 때문에 자기 `go.mod` 를 가진 별도 모듈이라 Bazel 과 `./...` 에 들어가지 않습니다. 그 폴더는 `chapters/ch07-10-service/edge/build.sh` 로 따로 빌드합니다.

```bash
bazel test //net/npg/...                    # 전체 시험
go test ./net/npg/... -count=1              # Go 툴체인으로 같은 시험
bazel build //net/npg/cmd/npgproxy          # 실행 파일 하나
```

`cmd/fragsend` 는 `//go:build linux` 라서 Linux 에서만 빌드됩니다(macOS 에서는 건너뜁니다).

## 기록

기록은 이 저장소에만 남깁니다. 판단은 `docs/adr/`, 리허설은 `docs/rehearsals/` 입니다. 노트에서 빠진 곳이나 오개념은 라벨 `note-gap` 이슈로 쌓습니다.
