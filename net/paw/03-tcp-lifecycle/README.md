# 03-tcp-lifecycle

『Packet Analysis with Wireshark』 3장 실습입니다. 2026-09-16 에 `~/pcap-practice/` 를 통째로 `~/study/paw-lab/` 으로 옮겼습니다. 다른 실습 랩이 모두 `~/study` 아래에 있어서입니다. 1·2장 캡처는 옆 폴더 `../00-setup-verify/` 에 있습니다. 노트는 `runners-high` 저장소의 `write/02_os/book/paw_packet-analysis-wireshark/` 에 있는 `03-01.TCP 연결의 생애.md` 와 `03-02.TCP가 어긋날 때.md` 입니다.

## 왜 리눅스 컨테이너인가

원서는 리눅스(CentOS)를 전제로 `ss`·`iptables`·`sysctl net.ipv4.tcp_*` 를 씁니다. 맥에는 `ss` 가 없고, 손실이나 지연을 넣으려면 호스트 방화벽(`pf`·`dnctl`)을 sudo 로 바꿔야 합니다. OrbStack 리눅스 컨테이너 둘을 쓰면 원서 명령을 거의 그대로 쓸 수 있고, `tc netem`·`iptables` 는 컨테이너 네트워크 네임스페이스 안에서만 적용되어 호스트를 건드리지 않습니다.

## 원서 프로그램과의 대응

원서는 파일 이름과 `socket.close()` 조각만 싣고 소스 전문을 싣지 않아, 서술된 동작만 맞춰 새로 썼습니다.

| 원서 | 여기 |
|---|---|
| `TCPServer01.java`, `Server0302.java` | `src/SimpleTCPServer01.java` (항상 서버가 먼저 닫습니다) |
| `Client0301.java` | `src/SimpleClient0301.java` |
| `Client0302.java` (소켓을 안 닫음) | `src/SimpleClient0301.java --close-after never` |

Go 판도 같은 옵션으로 있습니다. `src-go/server`, `src-go/client` 를 `./build.sh` 로 리눅스 arm64 바이너리로 빌드해 `bin/` 에 두고, 컨테이너에는 Go 를 넣지 않습니다.

```bash
./build.sh                                            # 소스를 고칠 때마다
docker exec tcp-server /lab/bin/server --mode normal
docker exec tcp-client /lab/bin/client --close-after never
```

## 올리기와 내리기

```bash
cd ~/study/paw-lab/03-tcp-lifecycle
docker compose up -d --build     # tcp-server, tcp-client 두 컨테이너
docker compose ps

docker compose down              # 끝나면 내립니다 — netem·iptables 설정도 컨테이너와 함께 사라집니다
```

캡처는 컨테이너 안 `tcpdump` 가 `pcap/` 에 쓰고, 분석은 호스트의 `tshark`·Wireshark 로 합니다.

Java 를 실행할 때마다 `NOTE: Picked up JDK_JAVA_OPTIONS: -Djava.net.preferIPv4Stack=true` 가 한 줄 찍힙니다. Java 소켓은 기본이 IPv6 이중 스택(`[::ffff:…]`)이라 원서의 `ss -nt4 state CLOSE-WAIT` 에 잡히지 않아서 compose 에서 IPv4 로 고정했습니다. 단일 파일 실행 모드는 `main` 전에 네트워크 클래스를 초기화하므로 코드 안에서 `System.setProperty` 로 거는 것은 효과가 없었습니다(2026-09-15 확인).

## 단계

| 묶음 | 단계 | 시나리오 |
|---|---|---|
| A. 연결의 생애 | 1 | 정상 연결, 서버가 먼저 닫음 |
| | 2 | 클라이언트가 끝내 닫지 않음 |
| | 3 | 클라이언트가 20초 뒤 닫음 |
| B. 연결 실패 | 4 | 서버 없이 접속 |
| | 5 | handshake 뒤 클라이언트가 강제 중단 (`--abort`) |
| | 6 | 원서의 `iptables` 규칙을 그대로 걸고 접속 |
| C. 느림 | 7 | `tc netem delay` 와 서버 처리 지연(`--mode slow`) 비교 |
| | 8 | 받는 쪽이 한동안 읽지 않음 (`--read-delay`, `--rcvbuf`) |
| | 9 | 대량 전송 중 `tc netem loss` |

단계별 명령은 학습 세션에서 예측을 받은 뒤에 하나씩 진행합니다.
