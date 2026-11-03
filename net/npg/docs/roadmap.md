# npgproxy 로드맵

이 문서는 리버스 프록시 `npgproxy` 를 키우는 순서표입니다. 책을 다 읽은 뒤에도 envoy·nginx·haproxy 에 가까운 모양이 될 때까지 같은 프록시를 계속 키웁니다. 2026-11-02 에 정했고, 단계마다 세부는 GitHub 이슈에, 판단은 ADR 에 남깁니다.



## 두 트랙

> 책 13장까지는 장 단위로 배우고 바로 실습하고, 그 뒤 책에 없는 프록시 기능을 확장 트랙으로 이어 붙입니다.

책 트랙은 『Network Programming with Go』 9~13장입니다. 장을 배운 직후 그 장의 판단을 npgproxy 에 실습합니다. 배운 내용을 몇 주 묵혀 두면 ADR 판단 때 다시 읽어야 하므로, 책을 다 읽고 실습을 몰아서 하지 않습니다.

확장 트랙은 책에 없지만 실제 프록시가 갖춘 기능입니다. 단계 이름을 X1~X13 으로 붙이고, 앞 단계가 뒤 단계의 재료가 되도록 순서를 정했습니다. 예를 들어 X2 의 이상 감지는 뺄 업스트림이 여러 대 있어야 의미가 있어서 X1(업스트림 여러 대) 뒤에 둡니다. 근거 자료는 책 대신 envoy·nginx·haproxy 의 원문(공식 문서·소스)을 씁니다.

```pseudocode
책 트랙   ch09 ─ ch10 ─ ch11 ─ ch12 ─ ch13
                                       │
확장 트랙  X1 LB ─ X2 CB ─ X3 Rate ─ X4 Backpressure ─ X5 동적 설정 ─ X6 JWT·ACL
           ─ X7 L4·SNI ─ X8 HTTP/2 ─ X9 리버스 터널 ─ X10 다중화
           ─ X11 Egress ─ X12 제어 평면 ─ X13 공개 relay(선택)
```



## 진행 규칙

> 장 주기는 README 그대로이고, 시작 관문과 선행 학습 안내 두 가지를 더합니다.

모든 단계는 README 「장 주기」의 순서(이슈 → ADR → 구현 → 시험 → 장애 리허설 → 태그)를 그대로 돕니다.

실습 시작 관문은 그 장 노트의 Phase 1~2 통과입니다. Phase 4 자답 검증은 복습 회차에서 따로 하므로 실습을 막지 않습니다. Phase 4 까지 기다리면 3~9장 검증을 먼저 해야 해서 진행 중인 장이 멈추기 때문입니다.

이슈마다 `## 선행 학습 (write/)` 절이 있습니다. 실습을 시작하기 전에 그 절의 문서를 배우고 옵니다. 경로는 `runners-high` 저장소의 `write/` 아래 경로 문자열로 적습니다. 다른 저장소라 상대 링크가 깨지기 때문입니다. write/ 에 다룬 문서가 없는 개념은 "write 없음 — 진입 시 학습 문서부터" 라고 적어 두었습니다.

책 트랙 이슈에는 `npg` 라벨을, 확장 트랙 이슈에는 `npg` 와 `npg-ext` 를 붙입니다. 마일스톤은 책 트랙 장마다 하나(`npg v0.10`~`npg v0.13`, 기한 있음)와 확장 트랙 네 묶음(X1~X4·X5~X8·X9~X12·X13, 기한은 그 묶음에 들어갈 때 정함)으로 둡니다. 태그는 장 번호를 이어 받아 X1 이 `v0.14.0` 입니다. 14장은 범위 밖이라 번호만 빌립니다.



## 책 트랙

> 9장은 PR #8 로 코드가 병합됐고, 10장은 다른 worktree 에서 시작했습니다.

| 장 | 이슈 | 태그 | 실습 주제 | 이 장에 흡수한 키워드 |
|---|---|---|---|---|
| 9 | #2 | v0.9.0 | 서버 타임아웃, 드레인 | Graceful Shutdown·Connection Draining |
| 10 | #3 | v0.10.0 | Caddy 앞단, 라우트 순서, `trusted_proxies` | L7 라우팅, `X-Forwarded-For` 신뢰 체인, TLS 종료 |
| 11 | #4 | v0.11.0 | 상호 TLS, 피닝 | mTLS |
| 12 | #5 | v0.12.0 | gRPC, 기한 전파 | (X12 제어 평면의 전송 방식이 됨) |
| 13 | #9 | v0.13.0 | 접근 로그와 지표 다시 판단 | Observability (Metrics·Logs) |

7장(Unix 소켓 관리 채널, #1)은 2026-10-23 결정대로 구현을 보류합니다. 14장(클라우드 서버리스)은 프록시와 관계가 없어 범위 밖입니다.



## 확장 트랙

> 책에 없는 기능을 X1~X13 순서로 붙이고, 마지막 X13 만 맥북 밖 서버를 씁니다.

| 단계 | 이슈 | 태그 | 기능 | 앞 단계에 기대는 이유 |
|---|---|---|---|---|
| X1 | #10 | v0.14.0 | 업스트림 여러 대, 부하 분산, 헬스 체크 | 지금은 업스트림 한 대만 봅니다(ADR 0011) |
| X2 | #11 | v0.15.0 | Outlier Detection, Circuit Breaker | 뺄 대상이 여러 대 있어야 합니다 |
| X3 | #12 | v0.16.0 | Rate Limit (로컬 토큰 버킷 → 공유 카운터) | 제한 키가 10장 `trusted_proxies` 결정에 기댑니다 |
| X4 | #13 | v0.17.0 | Backpressure, 버퍼 상한 | ADR 0012 격벽을 넓힙니다 |
| X5 | #14 | v0.18.0 | 동적 설정, DNS 서비스 발견 | X1 의 업스트림 목록을 실행 중에 바꿉니다 |
| X6 | #15 | v0.19.0 | JWT 검증, ACL | 11장 연결 신원 위에 사용자 신원을 올립니다 |
| X7 | #16 | v0.20.0 | L4 TLS passthrough, SNI 라우팅 | 11장 TLS 와 4장 `io.Copy` 프록시를 씁니다 |
| X8 | #17 | v0.21.0 | HTTP/2 (ALPN, h2c) | 연결 하나에 요청 여러 개가 실릴 때 풀·기한이 바뀝니다 |
| X9 | #18 | v0.22.0 | 리버스 터널 (NAT 뒤 장비 agent, 로컬 모의) | X1 업스트림 목록, X5 실행 중 등록을 씁니다 |
| X10 | #19 | v0.23.0 | 터널 다중화 (yamux 식) | X8 과 같은 문제를 터널 위에서 풉니다 |
| X11 | #20 | v0.24.0 | Egress Forward Proxy (`CONNECT`, 허용 목록) | 같은 코어를 나가는 방향으로 씁니다 |
| X12 | #21 | v0.25.0 | 제어 평면 흉내 (xDS 식 설정 배포) | X5 동적 설정을 원격 배포로 넓힙니다 |
| X13 | #22 | v0.26.0 | 공개 relay (선택) | X3·X6·X9 가 끝나야 엽니다 |

X3 과 X6 은 X1 에 기대지 않습니다. 실무에서 먼저 필요해지면 앞으로 당겨도 됩니다.

X9 리버스 터널에서는 NAT 뒤 장비 agent 가 npgproxy 로 먼저 연결을 걸어 둡니다. 운영자 요청은 npgproxy 가 받아 그 연결로 넘깁니다. 이 시나리오를 고른 이유는 npg 제품의 장비 이야기와 이어지기 때문입니다. 3장에서 정한 하트비트와 재접속 백오프(ADR 0001·0002)도 터널 유지에 다시 쓸 수 있습니다.

완료 기준은 로컬 모의입니다. 맥이 relay 가 되고, OrbStack `ubuntu` 가 NAT 뒤 agent 가 됩니다.

X13 은 같은 relay 를 공인 IP 서버에 올려 로컬 서비스를 인터넷에 노출하는 선택 단계입니다. 실습은 맥북만 쓴다는 원칙의 유일한 예외라서 선택으로 두었습니다. 인증과 속도 제한 없이 터널을 인터넷에 열면 아무나 로컬 서비스에 닿을 수 있으므로, X3·X6 이 끝나기 전에는 열지 않습니다.



## 실무 키워드와 단계 대조

> 프록시 실무 키워드 15개가 어느 장이나 단계에서 다뤄지는지 한 표로 봅니다.

| 키워드 | 다루는 곳 |
|---|---|
| Reverse Proxy / Forward Proxy | 4장 `io.Copy` 프록시, ch08 이후 npgproxy, X11 |
| L4 / L7 Proxy, Routing | 10장(L7 라우트), X7(L4) |
| TLS Termination / Passthrough | 10·11장(종료), X7(passthrough·SNI) |
| Connection Pooling / Keep-Alive | 8장(ADR 0012) 완료 |
| Load Balancing / Health Check | X1 |
| Timeout / Retry / Backoff | 8장(ADR 0012·0013) 완료, 9장(ADR 0014) |
| Circuit Breaker / Outlier Detection | X2 |
| Rate Limiting / Token Bucket | X3 |
| `X-Forwarded-For`, `Forwarded`, PROXY Protocol | 9장 S7, 10장 `trusted_proxies` |
| mTLS, OIDC, JWT | 11장(mTLS), X6(JWT) |
| ACL, Header Sanitization | X6 |
| Observability | 이미 있는 `internal/proxy` 의 지표·trace 코드, 13장 |
| Graceful Shutdown / Draining | 9장(ADR 0014) |
| DNS Resolution / Service Discovery | X5 |
| Backpressure / Buffering | X4 |



## portal-tunnel 에서 가져오는 것

> 일반 프록시·터널도 하는 기능만 가져오고, portal 고유의 설계는 학습 문서로만 둡니다.

gosuda/portal-tunnel 은 MIT 라이선스이고, 기준 커밋은 `129b6de5` 입니다. 정독 노트는 `runners-high` 의 `write/02_os/project/portal-tunnel/` 에 있습니다. 코드는 이 저장소에 넣지 않고 링크로만 참조합니다(AGENTS.md 「공개 저장소 규칙」).

| portal 기능 | 노트 | 이 로드맵에서 |
|---|---|---|
| SNI 라우팅 (ClientHello 만 읽음) | 02-01·02-02 | X7 에 흡수 |
| 역방향 연결 풀, 첫 바이트 프레이밍, lease | 01-01·01-03·01-04·01-06 | X9 |
| 스트림 다중화 (yamux) | 01-02·01-05 | X10 |
| relay 진입 정책, 신원 헤더 제거 | 03-02·03-03 | X3·X6 선행 학습 |
| ACME·DNS-01 | 02-05 | X13 에서만 |
| keyless TLS, MITM 자가 탐침, secp256k1·SIWE 신원, ENS, discovery·overlay, QUIC 백홀 | 02-03·02-04·03-01·03-02·03-04·01-07 | 가져오지 않음 |

가져오지 않는 기능은 portal 이 탈중앙 relay 망을 전제로 한 설계라서, 일반 리버스 프록시에는 대응하는 자리가 없습니다.



## 범위 밖과 다른 lab 과의 경계

> 프록시 학습과 결이 다른 것은 빼고, gonet-lab 과 겹치는 곳은 역할을 나눕니다.

범위 밖으로 둔 것은 14장, WAF, QUIC·HTTP/3 입니다. WAF 는 공격 패턴 규칙 엔진이라 프록시의 연결·트래픽 판단과 결이 다릅니다. QUIC 은 외부 라이브러리가 대부분을 맡아 직접 정할 판단이 적습니다.

`net/gonet` 로드맵의 TCP Proxy 단계(Phase 5)와 Reverse Tunnel 단계(Phase 8)는 X7·X9 와 겹칩니다. gonet-lab 은 바이트를 잇고 닫는 원리를 맡고, npgproxy 는 기한·지표·라우팅·인증 같은 제품 기능을 맡습니다.

X7·X9 이슈의 선행 학습에 gonet 단계를 적어 두었습니다. gonet-lab 이 늦어지면 npg 4장 노트 04-02 로 대신합니다.



## 다시 볼 조건

이 순서는 ADR 0011 「다시 볼 조건」(9장 다운스트림 층, 10장 Caddy 역할 분담)을 이어받습니다. 실무에서 특정 기능(Rate Limit, JWT 같은)이 먼저 필요해지거나, 한 단계가 예상보다 커서 둘로 나눠야 할 때 이 표와 이슈 번호를 함께 고칩니다.
