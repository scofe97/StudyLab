# gonet-lab

Go 로 네트워크 primitive 를 직접 구현하고, 실패시키고, 관측하면서 TCP/UDP 에서 Reverse Tunnel, P2P Overlay, Linux Networking, eBPF 까지 단계적으로 내려가는 학습 저장소입니다.

## 상태

**Phase 0~2 구현·실험 완료, Phase 3·4 는 학습 중이고 코드는 아직 없습니다.** `gonet listen`·`gonet connect` 로 TCP echo 를 주고받고(idle timeout 포함), `gonet udp-listen`·`gonet udp-send` 로 UDP echo 를 주고받습니다. macOS 와 OrbStack Linux 양쪽에서 동작을 확인했습니다. 학습 진행의 정본은 runners-high `write/02_os/project/gonet-lab/STATE.md` 입니다.

```bash
make build                               # bin/gonet
./bin/gonet listen 127.0.0.1:8080        # 터미널 A
./bin/gonet connect 127.0.0.1:8080       # 터미널 B, 입력 후 Ctrl-D
make build-linux                         # bin/gonet-linux → orb -m ubuntu 로 실행
make check                               # vet + gofmt + test -race
```

## 진행 방식

한 Phase 는 늘 같은 사이클을 밟습니다. 기능이 돌아가는 데서 멈추지 않고, 일부러 깨뜨린 뒤 `ss`·`tcpdump`·`strace`·metric 으로 무엇이 남는지 보고, 그 원인을 말로 설명할 수 있어야 끝납니다.

```bash
구현 → 정상 동작 → 장애 주입 → 관측 → 원인 설명 → 문서화
```

## Phase

| Phase | 주제 | 상태 |
|---|---|---|
| 0 | Network CLI (`listen`·`connect`) | 구현·실험 완료 (`experiments/phase0-*`) |
| 1 | TCP Echo / Chat — multi-client, timeout, backpressure | 구현·실험 완료 (idle timeout, `experiments/phase1-*`) |
| 2 | UDP — datagram, loss, `tc netem` | 구현·실험 완료 (`internal/udp`, `experiments/phase2-loss`) |
| 3 | DNS Client — 직접 encode/decode | 학습 중, 코드 전 |
| 4 | Port Scanner — CONNECT·SYN·UDP | 계획 완료 (`docs/03-02`), 코드 전 |
| 5 | TCP Proxy — 양방향 복사, half-close | 대기 |
| 6 | SOCKS5 Proxy | 대기 |
| 7 | HTTP CONNECT Proxy | 대기 |
| 8 | Reverse Tunnel (첫 핵심 milestone) | 대기 |
| 9 | Stream Multiplexing | 대기 |
| 10 | Discovery (multicast·mDNS) | 대기 |
| 11 | P2P Overlay (relay, 선택: hole punching) | 대기 |
| 12 | Mini DHT / Kademlia | 대기 |
| 13 | IP Overlay (TUN) | 대기 |
| 14 | Failure / Chaos Lab | 대기 |
| 15 | Observability (metric·log·trace) | 대기 |
| 16 | eBPF Network Observer | 대기 |

## 구조

```bash
gonet-lab/
├── cmd/gonet/       # CLI 진입점
├── internal/tcp/    # Phase 0·1 — Serve(echo, idle timeout), Connect
├── internal/udp/    # Phase 2 — UDP echo, 송신
├── experiments/     # 장애 주입 실험 (phase0~phase2)
└── docs/            # 로드맵, OrbStack 환경, Phase 별 계획
```

## 문서

- `docs/01-01.gonet-lab-roadmap.md` — 전체 로드맵 (scofe97/ai-context 원본 전사)
- `docs/02-01.orbstack-linux-setup.md` — Linux 관측 환경
- `docs/03-01.gonet-lab-phase0-plan.md` — Phase 0 범위와 완료 조건
- `docs/03-02.gonet-lab-phase4-plan.md` — Phase 4 범위·실험·naabu 대응표
- `AGENTS.md` — 코드 규칙
- 학습 문서와 진행 상태: `~/runners-high/write/02_os/project/gonet-lab/`
