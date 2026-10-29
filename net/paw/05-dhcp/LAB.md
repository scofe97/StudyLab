# 5장 랩 — 주소를 받아 오는 프로토콜 · 이름과 요청

『Packet Analysis with Wireshark』 5장 실습 환경입니다. 노트는
`runners-high/write/02_os/book/paw_packet-analysis-wireshark/` 의 05-01·05-02 입니다.
한 세션에 한 묶음씩, 단계마다 예측 → 직접 실행 → 관찰 순서로 진행합니다.

## 구성

| 컨테이너 | 주소 | 역할 |
|---|---|---|
| `dhcp-server` | 10.55.0.20 · fd00:55::20 | dnsmasq(DHCP·DNS), HTTP 서버, TLS 서버 |
| `dhcp-server2` | 10.55.0.21 | 두 번째 DHCP 서버(B1), 프록시(C6) |
| `dhcp-client` · `dhcp-client2` | 도커가 .2~.14 에서 자동 배정 | 주소를 지우고 다시 받는 쪽 |
| `dhcp-neighbor` | 자동 배정 | 옆자리. 남의 교환이 어디까지 보이는지 캡처하는 쪽 |
| `dhcp-router` | 10.55.0.30 · 10.56.0.30 | 두 세그먼트에 걸친 라우터. 릴레이가 도는 자리(B5) |
| `dhcp-client-b` | seg-b 자동 배정 | 서버와 다른 세그먼트의 클라이언트(B5) |

세그먼트는 둘입니다. `seg-a` 는 10.55.0.0/24 와 fd00:55::/64, `seg-b` 는 10.56.0.0/24 입니다.
dnsmasq 가 나눠 주는 범위는 .100 이후라 도커의 자동 배정(.2~.14)·고정 주소(.20·.21·.30)와 겹치지 않습니다.

```bash
docker compose up -d --build     # 시작
docker compose down              # 정리
./serve.sh dhcp-server dhcp-a.conf          # dnsmasq 를 임대 기록 없이 새로 띄움
./serve.sh dhcp-server stop
./look-dhcp.sh pcap/a1-server.pcap          # DHCP 캡처를 표로
```

## 사전 검증 결과 (2026-09-21, OrbStack · 맥북)

AI 가 학습자에게 넘기기 전에 전 단계를 한 번씩 돌려 도는지만 확인했습니다. 학습자의 관찰 기록이 아닙니다.

| 단계 | 돌아가는가 | 확인된 것 |
|---|---|---|
| A1 정상 DORA | 예 | 네 메시지, 출발지·목적지, 걸음별 yiaddr, 옵션 54·50 |
| A2 옆자리 캡처 | 예 | 옆자리에는 브로드캐스트만 도착 |
| A3 서버 꺼짐 | 예 | DISCOVER 반복 |
| A4 갱신·반납 | 예 | 유니캐스트 REQUEST→ACK, RELEASE |
| A5 DHCPv6 SARR · rapid commit | 예 | 트랜잭션 ID 둘, SOLICIT→REPLY 두 메시지(옵션 14) |
| B1 서버 두 대 | 예 | OFFER 둘, 진 서버의 임대 기록은 비어 있음 |
| B2 NAK | 예 | dhclient 가 옛 주소를 다시 요청 → NAK → 처음부터 |
| B3 풀 소진 | 예 | 서버 로그 `no address available`, 선 위에는 아무 응답도 없음 |
| B4 주소 충돌 | 예 | 서버의 사전 ICMP 확인 / `--no-ping` 뒤 클라이언트 ARP → DECLINE → 20초 뒤 재시작 |
| B5 릴레이 | 예 | seg-b 쪽은 브로드캐스트, seg-a 쪽은 giaddr 가 찍힌 유니캐스트 |
| C1 이름 실패 대 연결 실패 | 예 | NXDOMAIN 뒤 SYN 없음 / SYN 뒤 RST |
| C2 질의 타입 · CNAME 체인 | 예 | A·AAAA·CNAME·MX·TXT·NS·SOA |
| C3 TCP 전환 · EDNS(0) | 예 | `+noedns` 는 TC 플래그 뒤 TCP, 기본은 UDP 한 번 |
| C4 http.time | 예 | 연결 하나(SYN 한 번)에 응답 셋, http.time 은 응답마다 |
| C5 재조립 켬·끔 | 예 | 2MB 응답이 세그먼트 수십 개, 끄면 Continuation 으로 보임 |
| C6 평문 POST · CONNECT 터널 | 예 | 폼 값 평문 / CONNECT 평문 뒤 터널 안 ClientHello 의 SNI 평문 |
| C7 DoT | 예(인터넷 필요) | `dig +tls @1.1.1.1` — dns 필터 0줄, 853 포트에 TLS |
| C8 mDNS | 예 | `avahi-resolve` — 224.0.0.251:5353 멀티캐스트, 제3자 캡처에도 찍힘 |

## 돌리다 걸린 자리

- **고정 주소를 도커의 자동 배정 범위 안에 두면 서버가 못 뜹니다.** 다른 컨테이너가 먼저 그 주소를 받아 `Address already in use` 가 납니다. 그래서 서버는 `ip_range`(.0/28) 밖인 .20·.21 입니다.
- **캡처를 멈추기 전에 2초 기다립니다.** tcpdump 는 커널 버퍼에 모인 패킷을 묶음으로 받아 오므로, 마지막 메시지 직후에 죽이면 뒤쪽 프레임이 파일에 안 남습니다(DISCOVER 하나만 남은 적이 있습니다).
- **보낸 쪽에서 뜬 캡처에는 자기가 보낸 DHCP 프레임이 두 번 찍힙니다.** udhcpc·dnsmasq 가 raw 소켓으로 내보내는 프레임입니다. 선 위에서는 한 번이고, 옆자리나 상대편 캡처로 확인됩니다. 개수를 셀 때는 받는 쪽 캡처를 봅니다.
- **dnsmasq 는 제안하기 전에 그 주소로 ICMP 를 보내 누가 쓰고 있는지 확인합니다.** 응답이 오면 다음 주소를 시도합니다(dnsmasq(8) `--no-ping`). B4 의 관찰 대상이고 `--no-ping` 으로 끕니다. 처음 여기 "3초쯤 기다린다"고 적었으나 B4a 캡처와 어긋나 고쳤습니다(2026-09-26). 응답은 0.3ms 만에 왔고 다음 후보에는 ping 뒤 0.2ms 에 OFFER 가 나갔습니다. 3초는 클라이언트의 재전송 간격입니다. 대기 규칙은 매뉴얼에 없습니다.
- **dnsmasq 가 제안하는 주소는 클라이언트 MAC 에서 나오고, 컨테이너를 다시 만들면 MAC 이 바뀝니다.** B4 는 주소를 외워 두지 말고 그 자리에서 먼저 제안된 주소를 확인해 옆자리에 넣습니다.
- **`dhcp-authoritative` 를 켠 서버는 남의 서버를 고른 REQUEST 에도 NAK(`wrong server-ID`)을 브로드캐스트합니다.** B1 에서는 빼 두었고, B2 에서만 켭니다.
- **NAK 은 udhcpc 로는 못 만듭니다.** 옛 주소를 기억했다가 다시 요청하는 동작이 없어서입니다. B2 만 `dhclient` 를 씁니다.
- **닫힌 포트 접속이 거절이 아니라 타임아웃으로 끝난 적이 있습니다.** 이름에 AAAA 를 달아 놓고 그 IPv6 주소를 서버에 주지 않았던 탓입니다. 서버에 fd00:55::20 을 고정해 해결했습니다.
- **mDNS 는 `dig @224.0.0.251 -p 5353` 로는 응답을 못 받습니다.** 질의는 나가지만 avahi 가 답하지 않습니다. 클라이언트에도 avahi 를 띄우고 `avahi-resolve` 를 씁니다.
- **zsh 에서 `echo == SARR` 은 멈춥니다.** zsh 는 `=` 로 시작하는 단어를 명령 경로 찾기로 해석해 `zsh: = not found` 로 그 줄에서 끊고, 뒤 명령이 돌지 않아 캡처가 빕니다(2026-09-22 A5 에서 발생). 구분 표시는 `echo '== SARR'` 처럼 따옴표로 감쌉니다.
- **`ip addr flush dev eth0` 은 IPv6 링크 로컬까지 지웁니다.** 그 뒤 `dhclient -6` 은 `no link-local IPv6 address for eth0` 로 시작도 못 합니다(2026-09-22 A5). IPv4 만 지울 때는 `ip -4 addr flush dev eth0` 을 쓰고, 이미 지웠으면 `ip link set eth0 down; ip link set eth0 up` 으로 링크 로컬을 다시 만듭니다.
- **`sh -c 'pgrep -f "s_server" || ...'` 는 자기 자신을 찾아냅니다.** `-f` 는 명령줄 전체를 보므로 검사하는 `sh -c` 문자열 안의 `s_server` 가 걸려 "이미 떠 있음"으로 판단하고 서버를 띄우지 않습니다(2026-09-26 C6, 프록시가 CONNECT 에 500). 서버는 `docker exec -d dhcp-server openssl s_server ...` 로 직접 띄우고 `ss -ltn | grep :443` 으로 LISTEN 을 확인합니다.
- **보조 표 스크립트 `look.sh`.** `./look.sh <pcap> <필터|-> <필드...>` 는 빈 칸을 `-` 로 채워 `column -t` 의 열 밀림을 막습니다(2026-09-26 C3 에서 열이 밀려 포트를 잘못 읽은 뒤 만듦).
- **mDNS 는 켜지는 순간 알림을 보내 서로 캐시합니다.** avahi 를 띄운 직후 `avahi-resolve` 하면 선에 질의가 안 나갑니다. 질의를 보려면 클라이언트의 avahi 만 다시 띄워 캐시를 비운 뒤 풉니다(2026-09-26 C8).
