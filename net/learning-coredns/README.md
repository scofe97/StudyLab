# learning-coredns — 사내 DNS 장애를 재현하는 랩

『Learning CoreDNS』 1~4장(DNS 기초·Corefile·플러그인·존 데이터)을 실제 서비스에서 날 법한 장애 일곱 개로 확인합니다. 가상의 쇼핑몰 서비스 `shop.example` 이 사내 DNS 로 CoreDNS 한 대를 쓴다는 설정입니다. 사내 이름(`db`, `api`, `coupon` 등)은 존 파일로 답하고, 외부 이름은 상위 DNS 로 넘깁니다.

시나리오마다 신고 문장만 주어지고 원인은 적혀 있지 않습니다. 원인을 예측한 뒤 `dig` 출력과 CoreDNS 로그로 확인하는 것이 이 랩의 진행 방식이기 때문입니다. Corefile·존 파일·명령은 학습자가 직접 입력합니다.

## 관련 문서

| 문서 | 경로 |
|---|---|
| 실습 기록 (시나리오별 예측·관측·원인) | `~/runners-high/write/08_cloud/project/learning-coredns-lab/` |
| 책 정독 노트 1~4장 | `~/runners-high/write/08_cloud/book/learning-coredns/` (`01-01`~`04-01`) |

## 버전

| 도구 | 버전 | 출처 | 확인일 |
|---|---|---|---|
| CoreDNS | 1.15.0 | [v1.15.0 릴리즈](https://github.com/coredns/coredns/releases/tag/v1.15.0) · Homebrew `coredns` stable 1.15.0 | 2026-11-02 |
| dig | 9.10.6 (macOS 기본 `/usr/bin/dig`) | macOS 내장 | 2026-11-02 |
| 기기 | 맥북 Apple Silicon | — | 2026-11-02 |

## 실행

```bash
brew install coredns                       # CoreDNS 1.15.0 설치
cd ~/study/runners-lab/net/learning-coredns/<시나리오 폴더>
coredns -conf Corefile -dns.port 1053       # -conf: 설정 파일 경로, -dns.port: 듣는 포트(1053 은 관리자 권한 불필요)
dig @127.0.0.1 -p 1053 db.shop.example      # @: 물을 서버, -p: 포트
```

CoreDNS 는 앞쪽 터미널에 띄워 두고 로그를 보며, `dig` 는 다른 터미널에서 돌립니다.

## 시나리오

| 폴더 | 신고 문장 | 관련 장 |
|---|---|---|
| `s1-db-failover/` | DB 장애조치 후 `db.shop.example` 을 새 IP 로 고쳤는데, 앱이 계속 옛 DB 에 붙는다 | 2·4장 |
| `s2-stale-answer/` | S1 을 고쳤는데 몇 분 동안 일부 요청은 여전히 옛 IP 로 간다 | 3장 |
| `s3-new-record-missing/` | 신규 서비스 `coupon` 레코드를 넣었는데 한동안 "그런 이름 없음"이 나온다 | 2·3장 |
| `s4-internal-name-leak/` | 사내 존 블록을 추가한 뒤, 사내 이름 질의가 외부 DNS 로 새어 나간다 | 3장 |
| `s5-upstream-down/` | 상위 DNS 두 대 중 하나가 죽은 뒤, 결제사 API 이름 해석이 가끔 느리다 | 3장 |
| `s6-auto-new-zone/` | 팀마다 존이 늘어 `auto` 로 바꿨는데, 새 팀 존 하나만 응답하지 않는다 | 4장 |
| `s7-hosts-override/` | 긴급히 이름 하나를 `hosts` 로 덮어썼더니 그 존의 다른 이름이 전부 실패한다 | 3·4장 |

시나리오 폴더는 실습을 시작할 때 만듭니다. 사내 이름은 예약 도메인 `example`, 주소는 문서용 대역(RFC 5737 의 `192.0.2.0/24`·`198.51.100.0/24`)만 씁니다. 공개 저장소의 민감정보 검사가 사설 대역 주소를 사내 주소로 보고 걸러내기 때문입니다.
