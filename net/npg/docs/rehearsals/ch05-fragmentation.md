# 공지 단편화 (MTU 경계)

- 장: ch05
- 날짜: 2026-10-22

## 장면

ADR 0007 은 공지 데이터를 1232바이트로 제한했다. 1232 는 IPv6 최소 MTU 1280 에서 IPv6 헤더 40 과 UDP 헤더 8 을 뺀 값이다. 이 상한을 1바이트만 넘기면 실제로 어떤 일이 벌어지는지, 그리고 상한을 어긴 공지를 받는 쪽이 어떻게 반응하는지 본다. `lo` 의 MTU 를 1280 으로 낮추고 IPv6(`::1`)로 크기를 바꿔 보낸다.

## 증상

상한보다 1바이트 큰 공지는 agent 에서 ACK 가 오지 않고 타임아웃으로 끝난다. 네트워크에서 패킷이 사라진 것처럼 보이지만 provd 는 받았다.

## 실행 전 예측

- 실험 A (1232바이트): "패킷 1개, ACK 왔음"
- 실험 B (1233바이트): "패킷 2개(단편화), ACK 안 옴"
- 실험 C (1233바이트, `-df`): "전송되지만 ACK 가 없을 것이다"

## 실행 (사용자가 친 명령)

환경은 OrbStack `ubuntu`, 저장소 경로 `/Users/user/study/runners-lab/net/npg`. 로그는 `tee` 로 저장소의 `.omc/scene2/`(gitignore)에 남겼다.

T1 — 관찰 (첫 실행의 필터)
```bash
sudo tcpdump -i lo -n -tttt -l 'ip6 and udp port 7100'
```

T2 — provd
```bash
./bin/provd-linux -announce-addr '[::1]:7100' 2>&1 | while IFS= read -r l; do echo "$(date +%T) $l"; done
```

T3 — MTU 를 낮추고 크기별로 전송
```bash
sudo ip link set dev lo mtu 1280
./bin/fragsend-linux -size 1232
./bin/fragsend-linux -size 1233
./bin/fragsend-linux -size 1233 -df
sudo ip link set dev lo mtu 65536
```

옵션: `-size` 는 UDP 데이터 크기, `-df` 는 Don't Fragment(`IPV6_PMTUDISC_DO`)로 커널이 쪼개지 못하게 한다.

지나간 코드: `cmd/fragsend/main.go`, `internal/announce/listener.go` 의 상한 검사(`n > MaxAnnounceSize`).

## 관찰

첫 실행 (10:13:00).

| 실험 | agent 쪽 | provd 로그 | tcpdump |
|---|---|---|---|
| 1232 | 전송, ACK 10바이트 (1ms 뒤) | `장비 공지 받음` | 패킷 1개 + ACK 1개 |
| 1233 | 전송, 2초 뒤 `ACK 없음` (i/o timeout) | `상한을 넘은 공지를 버림 size=1233` | 아무것도 안 찍힘 |
| 1233 `-df` | `write: message too long` | 없음 | 없음 |

필터 `udp port 7100` 이 조각 패킷을 잡지 못해 1233 의 패킷이 보이지 않았다. 조각이 있었는지 확인하려고 필터를 바꿔 1233 만 다시 보냈다.

T1 (재실행의 필터)
```bash
sudo tcpdump -i lo -n -tttt -l 'ip6 and (udp port 7100 or ip6[6] == 44)'
```

`ip6[6] == 44` 는 IPv6 헤더의 다음 헤더 종류가 44(조각 헤더)인 패킷이다.

재실행 (10:13:53).

| 시각 | tcpdump |
|---|---|
| 10:13:53.592543 | `frag (0\|1232) 39924 > 7100: UDP, length 1233` |
| 10:13:53.592554 | `frag (1232\|9)` |

- agent 는 2초 뒤 `ACK 없음`, provd 는 `상한을 넘은 공지를 버림 from=[::1]:39924 size=1233` 을 남겼다. 출발 포트(39924)가 tcpdump 와 같다.
- 정리 확인: `lo` MTU 65536 으로 복원, tcpdump 는 Ctrl+C 로 종료.

## 해석

- 실험 A 예측은 맞았다. 40+8+1232=1280 이 MTU 에 딱 맞아 한 패킷으로 갔고 ACK 가 왔다.
- 실험 B 예측은 맞았다. 커널이 1233바이트를 오프셋 0 의 1232바이트와 오프셋 1232 의 9바이트, 두 조각으로 나눠 보냈다(합 1241 = UDP 헤더 8 + 데이터 1233). 두 조각은 모두 도착했고 수신 커널이 합쳐서 provd 에 1233바이트 데이터그램으로 넘겼다. provd 는 ADR 0007 대로 상한 초과로 버렸고 ACK 를 보내지 않았다. 조각이 유실된 것이 아니라 재조립 뒤에 버려진 것이다.
- 실험 C 예측은 빗나갔다. `-df` 에서는 커널이 쪼개기를 거부해 `write` 자체가 `message too long`(EMSGSIZE)으로 실패했고, 패킷은 네트워크에 나가지 않았다. "전송되지만 ACK 없음"이 아니라 "전송 단계에서 실패"였다. 상한을 지키는 송신자(`Encode` 가 1232 를 넘기면 `ErrTooLarge`)는 이 경로를 만나지 않는다.
- 측정 교훈: tcpdump 의 `udp port N` 필터는 IPv6 조각을 놓친다. 조각이 의심될 때는 `ip6[6] == 44` 를 더해야 한다. 첫 실행에서 아무것도 안 찍힌 것은 조각이 없다는 뜻이 아니었다.
- ADR 0007 이 상한을 넘긴 공지에 오류 ACK 를 돌려주지 않기로 한 결과가 그대로 보였다. 송신자는 "받았는데 버려졌다"와 "유실됐다"를 구별하지 못하고, 둘 다 타임아웃으로 보인다.

## 이어 읽을 노트

- 05-02 (UDP 데이터그램 크기와 단편화 노트)
- ADR 0007
