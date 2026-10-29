# 묶음 A 실측 메모 (2026-09-22, 학습자 실행) — 05-03 실습 편에 채울 값

## A1 정상 DORA (pcap/a1-server.pcap, 서버 쪽)
- 11줄 = DHCP 6 + ICMP 5. 선 위에서는 DHCP 5 (DISCOVER 둘·OFFER·REQUEST·ACK) — OFFER 중복은 자기 캡처 함정.
- DISCOVER: eth.dst ff:ff:ff:ff:ff:ff, 0.0.0.0 → 255.255.255.255, yiaddr 0.0.0.0, xid 0x8e237041. 1번→2번 사이 3.008초 (서버의 ICMP 확인 대기 중 재전송).
- 3번 ICMP: 서버 .20 → .137 (OFFER 전 사전 확인).
- OFFER: eth.dst 클라이언트 MAC c6:68:75:04:f6:c1, 10.55.0.20 → 10.55.0.137 (아직 없는 주소로 유니캐스트), yiaddr 10.55.0.137, 옵션 54 = 10.55.0.20.
- REQUEST: 브로드캐스트, 옵션 54 = .20, 옵션 50 = .137, xid 같음.
- ACK: 유니캐스트, yiaddr .137. 전체 3.02초.
- 이후 ping 출발지 10.55.0.137. lease 120초.
- 학습자 예측: 메시지 4개(실제 선 위 5, 재전송 1) · OFFER 목적지 "출발지로 되돌림"(출발지가 0.0.0.0이라 불가 — MAC으로 꽂음).
- 게이트: 서버가 DISCOVER의 eth.src·chaddr 에서 MAC을 얻고, NIC 는 MAC 으로 받고 udhcpc 는 raw 소켓으로 읽는다 — 힌트 두 번 뒤 도달.

## A2 옆자리 (pcap/a1-neighbor.pcap)
- 3줄: DISCOVER 둘 + REQUEST 하나. 전부 ff:ff:ff:ff:ff:ff → 255.255.255.255. OFFER·ACK·ICMP 없음.
- 학습자 예측: "브로드캐스트만 남고 2~4는 없다" — REQUEST 가 브로드캐스트인 것을 놓침. 힌트(옵션 54 + 서버 둘 가정) 뒤 "한 곳에만 보내면 안 되니 브로드캐스트"로 도달.

## 학습자 질문 (보강 후보로)
- 67/68 이 갈린 이유, lease 뜻, inet 표기, port=0 의 뜻, dnsmasq 가 주소를 고르는 방식(MAC 해시 + 사전 ICMP), 네 단계 로직이 커널이 아니라 udhcpc 에 있다는 것, 랩 구성·설정 설명을 실습 문서에 넣어 달라는 요청.

## A3 서버 꺼짐 (pcap/a3-neighbor.pcap)
- DISCOVER 4개, 2초 간격(-T 2), xid 0x309d8c09 넷 다 같음. 서버 답 없음, `no lease, failing`.
- 학습자 예측 정확(4개·타임아웃 판단·서버가 알릴 수단 없음). "Offer 단계 4번"이라 부른 것만 DISCOVER 로 정정.
- AI 설명(캡처 아님): 규격은 지수 백오프.

## A4 갱신·반납 (pcap/a4-server.pcap)
- 1~6 DORA(xid 0x7ca7dc24, OFFER 중복은 자기 캡처). 7 REQUEST 갱신: 10.55.0.137 → 10.55.0.20 유니캐스트, eth.dst 서버 MAC, ciaddr .137, 같은 xid. 8 ACK. 9 RELEASE: 유니캐스트, 새 xid 0xcf0b0c37, 서버 무응답. 반납 뒤 ip -4 addr 빈 출력.
- 학습자 예측: 갱신 2개·반납 1개(맞음), 둘 다 브로드캐스트(틀림 — 주소·서버를 아니 유니캐스트). 되묻기 한 번 뒤 도달. 처음엔 1~6만 읽고 7~9 대조 안 함.

## A5 DHCPv6 (pcap/a5-neighbor.pcap)
- 첫 시도 실패: `ip addr flush` 가 링크 로컬까지 지워 dhclient -6 가 `no link-local IPv6 address` — 관찰 자체가 학습점(IPv6 는 링크 로컬이 있어야 시작). link down/up 으로 복구. 그 전에 zsh `echo ==` 함정으로 한 번 더 헛돌음.
- SARR 로그: Solicit → Advertise(fe80::9c62:bff:fe21:9467) → Request → Reply → Bound. rapid: Solicit → Reply.
- 옆자리 4줄 전부 fe80::c468:75ff:fe04:f6c1 → ff02::1:2: SOLICIT(0xcf33cd) · REQUEST(0x3c05ce, xid 다름) · RELEASE(8) · SOLICIT+옵션14(0xdfd776). ADVERTISE·REPLY 는 유니캐스트라 없음.
- 학습자 예측: 멀티캐스트 그룹(맞음), rapid 2줄(dhclient 관점 맞음, 옆자리엔 1줄). 4줄 대조는 못 해 표로 제공(도움받은 응답).
- 학습자 질문: 멀티캐스트인데 옆자리에 왜 오나 → 브리지 스누핑 없음 + promiscuous. IGMP/MLD·스누핑 뜻, PRC/XMT/RCV, TCP 에 xid 있나, 시그널·USR1/2, 그룹 가입 방식.
