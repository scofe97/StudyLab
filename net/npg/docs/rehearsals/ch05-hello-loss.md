# 공지 유실 (HELLO 재전송)

- 장: ch05
- 날짜: 2026-10-21

## 장면

agent 가 부팅 때 UDP 로 HELLO 를 보내는데, 처음 몇 개가 provd 에 닿지 못하고 사라진다. ADR 0006 은 ACK 를 기다리며 1·2·4·8·16초(±25%)로 최대 5번 보내기로 했다. 그 재전송이 실제로 그 간격대로 나가고, 길이 열리면 ACK 로 멈추는지 본다.

UDP 가 메시지마다 독립적으로 유실되는 현상 자체(netem 손실 20% 등)는 gonet-lab 02-03 실험 4·5 에서 이미 관찰했으므로 다시 실험하지 않는다.

## 증상

provd 로그에는 HELLO 가 한 번만 찍히고, agent 에서는 시작 후 몇 초 뒤에야 `공지 ACK 받음` 이 찍힌다.

## 실행 전 예측

예측: "32초 지나서" (ACK 로그가 찍히는 시각에 대한 답이다)

## 실행 (사용자가 친 명령)

환경은 OrbStack `ubuntu`, 저장소 경로 `/Users/user/study/runners-lab/net/npg`.

T1 — 관찰
```bash
sudo tcpdump -i lo -n -tttt -l 'udp port 7100'
```

T2 — provd
```bash
./bin/provd-linux 2>&1 | while IFS= read -r l; do echo "$(date +%T) $l"; done
```

T3 — 규칙을 켜고 4.5초 뒤 끄면서 agent 를 시작
```bash
sudo iptables -I INPUT -i lo -p udp --dport 7100 -j DROP; echo "규칙 켬 $(date +%T)"
(sleep 4.5; sudo iptables -D INPUT -i lo -p udp --dport 7100 -j DROP; echo "규칙 끔 $(date +%T)") &
./bin/agent-linux -id rehearsal-1 2>&1 | while IFS= read -r l; do echo "$(date +%T) $l"; done
```

지나간 코드: `internal/announce/sender.go` 의 `Send` 재전송 루프(`for attempt`), `internal/announce/backoff.go` 의 `retryWait`.

## 관찰

tcpdump 로 본 HELLO 전송 시각(agent 시작 17:34:49.270 기준).

| 전송 | 시각 | 시작부터 | 직전과의 간격 | 기준 간격(허용 범위) |
|---|---|---|---|---|
| 1번째 | 17:34:49.270 | 0초 | - | - |
| 2번째 | 17:34:50.313 | 1.04초 | 1.04초 | 1초 (0.75~1.25) |
| 3번째 | 17:34:52.030 | 2.76초 | 1.72초 | 2초 (1.5~2.5) |
| 4번째 | 17:34:55.864 | 6.59초 | 3.83초 | 4초 (3~5) |

- 규칙을 켠 시각 17:34:49, 끈 시각 17:34:53 (`date +%T`, 초 단위).
- 4번째 HELLO 2.5ms 뒤에 provd 에서 agent 로 ACK(길이 10)가 갔다.
- provd 로그: `17:34:55 장비 공지 받음 device=rehearsal-1 from=127.0.0.1:47007` 한 번뿐. 출발 포트가 tcpdump 와 같다.
- agent 로그: `17:34:55 공지 ACK 받음` (시작 후 약 6.6초). 5번째 HELLO 는 나가지 않았다.
- TCP 제어 채널은 17:34:49 에 바로 붙었다(`agent 연결됨`).
- 정리 확인: `iptables -S INPUT` 에 규칙 없음, 남은 프로세스 없음, 7100/udp 소켓 없음, `tc qdisc` 는 `noqueue`.
- tcpdump 종료 요약이 `5 packets captured`, `10 packets received by filter` 로 달랐다. 이후 별도 시험(UDP 패킷 2개를 `lo` 로 보내 tcpdump)에서 `2 captured`, `4 received by filter` 로 항상 2배가 나와, 루프백에서는 같은 패킷이 송신 쪽과 수신 쪽에서 한 번씩 필터를 지나고 화면에는 한 번만 찍히는 것으로 읽었다(두 번 센다는 libpcap 소스는 확인하지 않았다).

## 해석

- 재전송 간격 세 개(1.04, 1.72, 3.83초)가 모두 ADR 0006 의 허용 범위 안이다. 기준 1·2·4초가 2배씩 늘고 ±25% 지터가 섞여 4번째 전송이 이론값 7초 대신 6.59초에 나갔다(가능한 범위 5.25~8.75초).
- 4번째 HELLO 가 통과한 이유는 DROP 규칙이 53초대에 꺼졌기 때문이다. 앞의 세 HELLO 는 provd 소켓에 닿기 전에 `iptables` INPUT 에서 버려졌고 provd 로그에 흔적이 없다.
- 예측("32초")은 빗나갔다. 31초는 5번 모두 실패했을 때 마지막 16초 대기가 끝나 포기하는 시각이고, ACK 시각은 아니다. ACK 가 오면 `Send` 가 바로 끝나므로 ACK 시각은 길이 열린 뒤 처음 나가는 HELLO 의 시각(이번에는 4번째, 6.6초)이다.
- 공지 실패 구간에도 TCP 제어 채널은 붙어 있었다. ADR 0006 의 "공지는 보조 경로, 등록의 정본은 TCP" 설계가 확인됐다.

## 재실행: 규칙을 끄지 않은 경우 (포기까지)

처음 실행에서 "언제 통과하는가"와 "언제 포기하는가"가 섞여 이해하기 어려웠다는 요청으로, 규칙을 끝까지 켜 둔 채 한 번 더 돌렸다.

### 재실행 예측

"5번 나가고, 31초 뒤까지 기다린 다음 포기" (HELLO 5번, 약 31초 뒤 포기)

### 재실행 명령

T1·T2 는 처음 실행과 같다. T3 만 다르다.

```bash
sudo iptables -I INPUT -i lo -p udp --dport 7100 -j DROP
sudo iptables -S INPUT
echo "규칙 켬 $(date +%T)"
./bin/agent-linux -id rehearsal-2 2>&1 | while IFS= read -r l; do echo "$(date +%T) $l"; done
```

끝낼 때 규칙을 지운다(`sudo iptables -D INPUT -i lo -p udp --dport 7100 -j DROP`).

### 재실행 관찰

agent 시작 17:52:42.130 기준 HELLO 전송 시각(tcpdump).

| 전송 | 시각 | 시작부터 | 직전과의 간격 | 기준 간격(허용 범위) |
|---|---|---|---|---|
| 1번째 | 17:52:42.130 | 0초 | - | - |
| 2번째 | 17:52:42.928 | 0.80초 | 0.80초 | 1초 (0.75~1.25) |
| 3번째 | 17:52:44.657 | 2.53초 | 1.73초 | 2초 (1.5~2.5) |
| 4번째 | 17:52:48.501 | 6.37초 | 3.84초 | 4초 (3~5) |
| 5번째 | 17:52:54.669 | 12.54초 | 6.17초 | 8초 (6~10) |

- agent 로그: `17:52:42 연결됨`(TCP), `17:53:08 WRN 공지 실패 error="공지 ACK를 받지 못했습니다"`. 시작 후 약 26초이고, 5번째 전송 뒤 약 13.3~14.3초(기준 16초, 허용 12~20초)를 기다렸다.
- provd 로그에는 `장비 공지 받음` 이 한 번도 없다. HELLO 5번이 모두 provd 소켓에 닿기 전에 DROP 규칙에 버려졌다.
- agent 는 재전송마다 로그를 남기지 않아 재시도는 tcpdump 로만 보인다.
- 사용자가 TCP 제어 채널을 8분쯤 유지한 뒤 종료했다(provd `18:00:26 agent 연결 종료`). 결과에는 영향이 없다.
- 정리 확인: `iptables -S INPUT` 은 `-P INPUT ACCEPT` 한 줄, 남은 프로세스와 7100/udp 소켓 없음, `lo` MTU 65536.

### 재실행 해석

- 재전송 횟수는 예측대로 5번이었다. 포기 시각은 예측(약 31초)과 달리 약 26초였다.
- 31초는 지터가 없을 때 대기의 합(1+2+4+8+16)이다. 각 대기가 ±25% 로 흔들리므로 합의 범위는 23.3~38.8초이고, 이번에는 대기가 짧은 쪽으로 나왔다. 5번째 전송도 이론값 15초가 아니라 12.54초에 나갔다.
- 공지가 끝내 실패한 동안에도 TCP 제어 채널은 17:52:42 에 정상적으로 붙었다(ADR 0006: 공지는 보조 경로).
- DROP 은 거절 신호(ICMP)를 보내지 않는 규칙이라 agent 는 5번 모두 응답 없음으로 기다렸다. provd 가 없어 포트가 닫힌 경우(`ECONNREFUSED`)와 같은 시간표로 끝나지만 원인이 다르며, 둘의 구분은 tcpdump 에 ICMP 가 보이는지로 한다.

## 이어 읽을 노트

- 05-01.UDP 는 연결 없이 보내고, 소켓 하나가 누구에게서든 받는다.md
- gonet-lab 02-03.UDP 실습 기록 - 경계·잘림·소켓·netem 손실과 순서.md (UDP 손실 자체의 관찰)
- ADR 0006
