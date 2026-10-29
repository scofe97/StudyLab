# 묶음 C 실측 메모 (학습자 실행) — 05-03 실습 편에 채울 값

## C1 이름 실패 대 연결 실패 (2026-09-24, pcap/c1-client.pcap)
- 클라이언트 .137(DHCP), resolv.conf nameserver 10.55.0.20, 서버는 dns.conf + server.py(80).
- 1·2 DNS nope.lab.test → rcode 3(NXDOMAIN), SYN 없음, curl exit 6.
- 3·4 DNS web.lab.test → rcode 0, A 10.55.0.20. 5 SYN → :81, 6 RST·ACK(.20 → 33910). curl exit 7.
- 학습자 예측: 1 NXDOMAIN 뒤 SYN 없음(맞음, Phase 1 질문 1 과 일치). 2 "3-way 다 돈 뒤 실패"(틀림 — SYN 에 곧바로 RST).
- 몰랐던 것: NXDOMAIN 이 rcode 3.
- 질문: .conf 가 범용 개념인가(예), dnsmasq 를 공부해야 하나(참고만 — 옮겨 갈 개념은 설정 주도 서버·LISTEN).
- 게이트: RST 를 보낸 주체 = 커널 TCP 스택, 목적지 포트로 LISTEN 소켓을 찾고 없으면 RST — 힌트 한 번 뒤 도달("어디서 하는지 정확히 설명은 어렵다"며 커널 TCP 계층이라고 답함).
- 음성 받아쓰기: nope→"노 블랙", _ws.col→"웹소켓 콜"(ws=Wireshark, col=column), SYN→"시인", ACK→"HK".

## C2 질의 타입·CNAME (2026-09-26, pcap/c2-client.pcap)
- dig 출력: A web 10.55.0.20 / AAAA fd00:55::20 / multi A .41·.42 / www CNAME web + A .20 / MX 10 mail.lab.test / TXT "v=paw-lab" / NS server.lab.test / SOA server.lab.test hostmaster... 1790258654 1200 180 1209600 600. TTL 600.
- tshark 17줄: 질의·응답 짝 8쌍. 1·2 번 web A 질의가 두 번(원인 미확정 — 문서화 때 시간 열 확인). 8 www 질의(answers 0) → 9 응답(answers 2). web.lab.test 재질의 없음.
- 학습자 예측: 1 www 두 줄 = A + AAAA(틀림 — CNAME + A, 관찰로 스스로 정정). 2 클라이언트가 web 을 다시 묻는다(틀림).
- 오독: dns.flags.response False/True 를 실패/성공으로 읽음 → 질의/응답 정정.
- 몰랐던 것: MX·TXT·NS·SOA 뜻(설명 제공), dig + 옵션 문법, SOA 약자·읽는 법, 존의 뜻(05-02 §1 에 있음을 안내).
- 게이트: 8번 뒤 web 질의 없음 → 서버가 체인을 따라가 한 응답에 담았다 — 열 정정 뒤 도달. count.answers 0 을 "실패"로 한 번 더 말함 → 질의라서 0 이라고 정정.

## C3 TCP 전환·EDNS(0) (2026-09-26, pcap/c3-client.pcap)
- 1·2 UDP 질의 42115→53, 72B(같은 질의 두 번 — C2 와 같은 현상, 원인 미확정) · 3 UDP 응답 TC=1, answers 0, rcode 0, 72B · 4~6 3-way(32827↔53) · 7 TCP 질의 98B · 9 TCP 응답 answers 1, 959B · 11~13 FIN·FIN·ACK 종료 · 14 EDNS 질의 udp_payload_size 1232, 95B · 15 UDP 응답 944B, TC 없음. dig: +noedns → `Truncated, retrying in TCP mode`, rcvd 891(TCP) / 기본 → `udp: 1232`, rcvd 902(UDP).
- AI 실수: 드린 tshark 명령에 빈칸 채우기(awk) 누락 → 열 밀림으로 학습자가 42115 등을 못 읽음. AI 가 정렬해 다시 읽어 줌.
- 학습자 예측: 1 "서버가 TCP 필요하다고 응답 → 3-way → DNS"(흐름 맞음, 그 응답이 TC 플래그라는 것은 모름) · 2 EDNS 는 UDP 한 번(맞음).
- 오독: 3번을 실패로 읽음(rcode 0, 잘림 표시), FIN 줄(A···F)을 "주소 두 개"로 읽음(3장 연결 종료), MSG SIZE rcvd 를 rwnd 로 읽음, big.lab.test 를 IP 로 봄(TXT).
- 좋은 질문: 질의가 512 넘을 수 있나 → 넘는 건 응답(주소 다수·긴 TXT·DNSSEC).
- 게이트: EDNS 켬·끔 차이 — 도달. 가리킨 열은 truncated(응답 쪽)로 틀려 udp_payload_size 로 정정(도움받은 응답).

## C4 http.time (2026-09-26, pcap/c4-client.pcap)
- curl num_connects 1,0,0 — 연결 재사용. SYN 1(0.000000) · SYN-ACK 2(0.000059, 59µs). tcp.stream 전부 0.
- 요청→응답: /fast 4→8 http.time 0.034924 · /slow 10→14 2.048778 · /fast 16→20 0.043358.
- 학습자 예측: SYN 한 번(맞음), 첫 /fast 는 3-way 포함 0.5초(틀림 — http.time 은 request_in 프레임부터라 3-way 제외), SYN 간격으로는 요청의 느림 못 가림(맞음).
- 질문: HTTP 필터 표에서 번호가 비는 이유 → 사이 프레임은 ACK·응답 앞 조각.
- 게이트: 3-way 는 프레임 1·2(59µs)에 끝났고 http.time 은 4↔8 사이 → 3-way 불포함 — 힌트(두 칸 짚기) 뒤 도달. SYN-ACK 시각을 0.00059 로 한 자리 잘못 읽음. Phase 1 잘못 알던 인과("느린 요청은 SYN·ACK 간격") 닫힘.

## C5 재조립 켬·끔 (2026-09-26, pcap/c5-client.pcap)
- /big 2,097,152B. 서버 데이터 조각 45(켬·끔 공통, 필터 없이 셈).
- 켬 + http 필터: 4 요청, 74 응답 http.time 0.046956 tcp.segment.count 45.
- 끔 + http 필터: 46줄 = 요청 1 + 6 `200 OK`(http.time 0.044190) + Continuation 44.
- 학습자 예측: 켬 한 줄(맞음), 켜면 조각 프레임이 목록에서 사라진다(틀림 — 남고 마지막 조각에 모은 결과가 붙음), http.time 의 끝점은 혼란("내가 잘못 이해하나").
- 게이트: 차이 2.8ms 를 "네트워크 시간"이라 함 → AI 가 "나머지 44조각 본문 전송"으로 좁히고 첫 바이트/마지막 바이트 표 제공(도움받은 응답) → 다운로드 느림은 켬, 서버 처리는 끔, 차이로 네트워크 몫 분리 — 도달. 차이는 왕복 전부가 아니라 본문 전송이라고 보충.

## C6 평문 POST · CONNECT 터널 (2026-09-26, pcap/c6-client.pcap · c6b-client.pcap)
- POST: 4번 프레임 urlencoded user,password = paw,hunter2 평문.
- 첫 시도 CONNECT 500: AI 명령의 `sh -c 'pgrep -f s_server || ...'` 가 자기 자신을 매칭해 443 서버를 안 띄움(tinyproxy 로그 `Could not establish a connection to web.lab.test:443`). AI 가 s_server 직접 기동 후 재실행. 학습자가 이전 명령을 한 번 더 돌려 같은 500 — 새 명령 안내.
- c6b: 4 CONNECT web.lab.test:443 평문 · 6 200 · 8 ClientHello(SNI web.lab.test) 평문 · 9 ServerHello+CCS+Application Data×4(TLS 1.3, 인증서부터 암호화) · 10~23 Application Data(opaque_type 23). IP 는 전부 .137↔.21.
- AI 실수: 표 명령에 tls.record.content_type 만 넣어 TLS 1.3 암호화 레코드가 `-` 로 보임 → opaque_type 으로 다시 읽어 줌.
- 학습자 예측: CONNECT·200 평문(맞음), "ClientHello 는 암호화, SNI 만 평문"(틀림 — ClientHello 통째 평문), TLS 상대 클라이언트↔서버(맞음).
- 게이트: IP 가 프록시뿐인데 왜 서버가 상대인가 → "패킷만으로는 판단 못 하고 CONNECT 문맥을 봐야" — 통과(근거 CONNECT 대상·SNI, 확정은 반대편 캡처).
- 질문: 암호화는 어디서 보나(Application Data·opaque 23), 호스트별 라우팅 프록시는(TLS 종단 vs SNI 패스스루 — gateway-lab 연결).

## C7 DoT (2026-09-26, pcap/c7-client.pcap, 인터넷 필요)
- 평문 dig @1.1.1.1 / DoT dig +tls @1.1.1.1 둘 다 example.com → 104.20.23.154, 172.66.147.243.
- dns 필터 2줄: 1 질의 → 1.1.1.1:53, 2 응답 → .137:55213. DoT 0줄.
- 853: 6 ClientHello(SNI 없음) · 8 ServerHello+CCS+AppData(opaque 23) · 10 CCS+AppData · 12 AppData · 14 AppData×2.
- 학습자 예측: dns 필터 2줄(맞음), DoT 12개 안팎(대체로 맞음), 질의 이름 보임(틀림 — 암호화), SNI 보임(틀림 — IP 로 접속해 이름이 없음).
- C6 재확인: 학습자가 opaque_type 23 을 직접 확인.
- 질문: TLS 1.3 흐름을 문서에서 다루나(04-04 C10 안내, key_share 로 1-RTT), 암호화 뒤에도 주소가 들어 있나(예, Host 등이 AppData 안).
- 게이트: C6 은 이름, C7 은 IP 로 접속 → SNI 에 적을 이름이 없음 — 명령 나란히 놓기 + SNI 풀이 힌트 두 번 뒤 도달. 보충: 보이는 건 "누구에게 물었나", 가려지는 건 "무엇을 물었나".

## C8 mDNS (2026-09-26, pcap/c8-bystander.pcap · c8b-bystander.pcap)
- 첫 캡처: avahi 기동 직후 알림만 — client.local(.137), neighbor.local(.2), server.local(.20) 각자 224.0.0.251 로 자기 이름 광고(묻지 않은 응답). client 는 알림을 캐시해 resolve 때 선에 묻지 않음 → 질의 없음. 1장 iPhone 이름이 보인 이유.
- AI 가 client avahi 재기동(캐시 비움) 후 재현 확인 → 학습자 재실행 c8b: 3(IPv6)·4 질의 .137 → 224.0.0.251 · 5 응답 .20 → 224.0.0.251, A 10.55.0.20. 제3자 client2 에 모두 찍힘.
- 학습자 예측: 목적지 멀티캐스트 그룹(맞음), 누가 답하는지 모름, 답은 제3자에게 안 찍힌다(틀림 — 답도 멀티캐스트).
- 게이트: 답이 그룹으로 간 것을 스스로 발견하고 이유를 물음 → AI 설명(모두의 캐시 갱신·이름 충돌 감지, 기본은 멀티캐스트). 답한 주체 = 이름 주인(.20).

## 묶음 C 요약
- 닫힌 것: 이름 실패(SYN 없음) 대 연결 실패(SYN→RST, 커널 TCP 스택) · CNAME 은 서버가 따라가 한 응답에 · TC 는 실패가 아니라 잘림, EDNS(0) 의 OPT 가 한계를 올림 · http.time 은 요청→응답(3-way 제외) — Phase 1 잘못 알던 인과 해소 · 재조립 켬=마지막 바이트/끔=첫 바이트 · ClientHello 는 터널 안에서도 평문, TLS 상대는 CONNECT 문맥으로 · DoT 는 누구에게 물었나만 보이고 무엇을 물었나는 가림 · mDNS 는 질의·응답·알림 모두 멀티캐스트.
- 막힌 축: 표 열 읽기(response False/True, truncated, content_type vs opaque_type), http.time 끝점, SNI 가 IP 접속에서 비는 이유(힌트 두 번).
- AI 실수: C3 열 밀림(look.sh 로 해결), C6 pgrep 자기 매칭, C6 표의 content_type 칸 선택.
