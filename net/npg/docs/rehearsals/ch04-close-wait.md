# CLOSE_WAIT 누적

- 장: ch04
- 날짜: 2026-10-21

## 장면

OrbStack `ubuntu` 안에서 실험용 서버 `cmd/leakd` 를 띄운다. leakd 는 연결마다 상대가 닫을 때까지 읽다가, 끝나도 `conn.Close()` 를 부르지 않는다. 연결 참조는 슬라이스에 붙들어 둔다(참조를 버리면 GC finalizer 가 소켓을 대신 닫아 누수가 가려질 수 있다는 Go 런타임 동작은 이번에 실험으로 확인하지 않았다). `cmd/connchurn` 이 연결 20개를 열고 곧바로 닫는다.

## 증상

닫힘을 통보받은 쪽이 `Close` 를 부르지 않으면 소켓이 `CLOSE-WAIT` 에 머문다. 어느 쪽에 몇 개 쌓이는지, 프로세스가 끝나면 어떻게 되는지가 관찰 대상이다.

## 실행 전 예측

- 1. `connchurn` 이 닫은 뒤 `CLOSE-WAIT` 은 **connchurn 쪽 20개**.
- 2. leakd 를 `Ctrl+C` 로 끝내면 쌓인 소켓은 **"defer 가 없으면 계속 남음"**.

## 실행 (사용자가 친 명령)

```bash
# 모두 orb -m ubuntu 안에서
# 터미널 1
./bin/leakd-linux
# 터미널 2
./bin/connchurn-linux -n 20
ss -tan '( sport = :7100 or dport = :7100 )' | awk 'NR>1{c[$1]++} END{for(s in c) print s, c[s]}'
# 터미널 1 에서 Ctrl+C 한 뒤 터미널 2 에서 같은 ss 를 한 번 더
```

## 관찰

| 시각 | 무슨 일 |
|---|---|
| 14:13:14 | connchurn 이 연결 20개를 열고 닫음. leakd 가 20개를 받고 모두 "상대가 닫음, Close 를 부르지 않음" 을 찍음 |
| 14:13:24 | `LISTEN` 1, `CLOSE-WAIT` 20, `FIN-WAIT-2` 20. `CLOSE-WAIT` 행의 로컬 주소는 `127.0.0.1:7100` (서버 쪽) |
| 14:13:54 | leakd 종료 뒤. `TIME-WAIT` 20, `CLOSE-WAIT` 0 (`Ctrl+C` 를 친 시각은 기록하지 않았다) |

## 해석

- 예측 1 은 빗나갔다. `CLOSE-WAIT` 은 먼저 닫은 쪽이 아니라 **닫힘을 통보받은 쪽**의 상태이다. connchurn 이 FIN 을 보내고 leakd 의 커널이 ACK 하며 `CLOSE-WAIT` 으로 바꿨다. 먼저 닫은 connchurn 쪽은 상대의 FIN 을 기다리는 `FIN-WAIT-2` 에 머물렀다.
- 예측 2 도 빗나갔다. 프로세스가 끝나면 커널이 열린 fd 를 모두 닫아 FIN 이 나가고, 쌓인 `CLOSE-WAIT` 이 사라졌다. 이어 connchurn 쪽 `FIN-WAIT-2` 가 `TIME-WAIT` 으로 넘어갔다고 TCP 상태 전이로 설명되지만, `TIME-WAIT` 소켓의 로컬 포트는 출력에 없어 직접 확인하지 못했다.
- 그러므로 `CLOSE-WAIT` 누수는 프로세스가 살아 있는 동안만 쌓인다. 서버는 계속 살아 있어야 하니 사실상 영구이며, 소켓마다 fd 를 차지하므로 결국 새 연결을 못 받을 수 있다. fd 한계 도달은 이번에 재현하지 않았다.
- 현재 provd 는 `serve` 첫 줄 `defer conn.Close()` 로 이 누수가 없다(`cmd/provd/main.go`). 3장 리허설의 `CLOSE-WAIT` 은 `Close` 누락이 아니라 provd 를 얼려 `serve` 가 돌지 못한 경우로, 증상이 같고 원인이 다르다.
- 점검 수단: `ss -tan state close-wait '( sport = :7100 )'` 로 서버 쪽 `CLOSE-WAIT` 개수를 센다.

## 이어 읽을 노트

- 04-03.TCPConn 손잡이와 흔한 버그 — keepalive·linger·버퍼·zero window·CLOSE_WAIT.md
- 겹치는 개념(CLOSE_WAIT·zero window): gonet-lab 학습 문서
- ADR 0005
