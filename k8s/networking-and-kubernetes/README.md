# 01-04 패킷 캡처 실습 — 실행 환경

노트 문서 [`01-04.패킷 캡처 실습`](../../runners-high/write/08_cloud/book/networking-and-kubernetes/) 의 다섯 절을
이 맥북에서 실제로 돌리기 위한 폴더입니다. 노트 문서는 수정하지 않았습니다.

저자 공식 예제(`~/study/networking-and-kubernetes`, @bd0e070)는 읽기 전용 참조로 두고,
이 폴더에 실행용 사본만 둡니다.

## 이 기기에서 문서와 다른 점 3가지

문서를 글자 그대로 따라 하면 두 절이 **에러 없이 틀린 결과**를 냅니다. 그래서 포트와 도메인을 바꿨습니다.

| 문서의 전제 | 이 기기의 사실 | 이 폴더의 대응 |
|---|---|---|
| 8080 은 Go 웹 서버 몫 | Docker `sq-jenkins` 가 8080 을 LISTEN 중 | 실습 포트를 **8081** 로 (`env.sh`). Jenkins 는 그대로 둠 |
| `example.com` 은 외부 도메인 | `/etc/hosts` 가 `127.0.0.1` 로 오버라이드했었음 | 그 토큰만 제거. 회사 인프라 항목은 무수정 |
| en0 하나가 실제 인터페이스 | VPN `utun*` 가 default 로 함께 있음 | §3·§5 경로는 en0 로 실측 확인됨. 캡처 시 `-i en0` 명시 |

세 번째는 실제 영향이 없지만, 인터페이스 목록에서 헷갈리기 쉬워 적어 둡니다.

## 시작하기

```bash
cd ~/study/networking-and-kubernetes-lab
sudo bash fix-hosts.sh     # 최초 1회만 — example.com 오버라이드 제거
bash preflight.sh          # 전 항목 PASS 여야 진행
sudo -v                    # 라이브 캡처 전 자격 증명 캐시
```

`preflight.sh` 는 FAIL 이 하나라도 있으면 exit 1 입니다. 통과 못 한 채로 시작하면
캡처가 비어 있거나 패킷 수가 어긋나는데, 원인을 찾느라 시간을 씁니다.

---

## §1 관측 지점 — sudo 불필요

**확인할 것**: 인터페이스가 보여 주는 계층이 갈리는 이유.

```bash
ifconfig lo0 | grep -cE '^[[:space:]]*ether'   # 0
ifconfig en0 | grep -cE '^[[:space:]]*ether'   # 1
ifconfig lo0 | grep -o 'mtu [0-9]*'            # mtu 16384
ifconfig en0 | grep -o 'mtu [0-9]*'            # mtu 1500
```

**기대**: `0` 과 `1`. lo0 에 MAC 이 없으니 ARP·Ethernet 은 여기서 볼 수 없습니다.
**어긋나면**: `preflight.sh` 4번 항목이 같은 것을 검사합니다. 거기가 FAIL 이면 문서 전제 자체가 이 기기와 다릅니다.

## §2 루프백 12패킷 — sudo 필요, 터미널 3개

**확인할 것**: 요청 한 번의 패킷 12개, 그중 데이터는 2개.

```bash
source env.sh

# 터미널 A — 캡처를 먼저 켭니다. 서버보다 늦으면 핸드셰이크를 놓칩니다
sudo tcpdump -i lo0 -n port "$LAB_PORT" -w "$LAB_PCAP"

# 터미널 B
go run web-server.go

# 터미널 C
curl localhost:8081
```

터미널 A 에서 `Ctrl+C` 후:

```bash
tcpdump -r /tmp/loop.pcap -n | wc -l    # 12
tcpdump -r /tmp/loop.pcap -n           # 플래그 확인
```

**기대**: 12. 구성은 수립 3 + 서버 ACK 1 + 데이터 2 + 데이터 ACK 2 + 종료 4.
**어긋나면**: 문서 §2 의 "숫자가 어긋날 때" 표를 봅니다. 많으면 다른 요청 혼입, 적으면 캡처 시점.
포트를 8080 으로 잘못 쓰면 Jenkins 응답이 잡혀 12 가 절대 안 나옵니다.

## §3 ARP — sudo 필요, 산출물 공개 금지

**확인할 것**: 브로드캐스트 질의와 유니캐스트 응답의 비대칭.

```bash
GW=$(netstat -rn -f inet | awk '$1=="default" && $NF=="en0"{print $2; exit}')

# 터미널 A — MAC 을 보려면 -e 가 필요합니다
sudo tcpdump -i en0 -e -n arp

# 터미널 B — 캐시를 지워 일부러 유발합니다
sudo arp -d "$GW"
ping -c 1 "$GW"
```

**기대**: 질의의 목적지가 `ff:ff:ff:ff:ff:ff`, 응답은 게이트웨이 MAC 에서 우리 MAC 으로 직접.

> ⛔ **이 절의 출력은 커밋하지 않습니다.** 게이트웨이 IP 와 양쪽 MAC 이 그대로 찍힙니다.
> 화면으로 확인만 하고 지웁니다.

## §4 TIME-WAIT / CLOSE-WAIT — sudo 불필요

**확인할 것**: 두 상태가 연결의 서로 다른 쪽에 생긴다는 것.

```bash
# TIME-WAIT — 먼저 닫는 쪽(curl)에 쌓입니다. §2 의 서버가 떠 있어야 합니다
for i in $(seq 1 20); do curl -s localhost:8081 > /dev/null; done
netstat -an -p tcp | grep 8081 | grep -c TIME_WAIT

# CLOSE-WAIT — 닫지 않는 서버가 필요합니다
go run close-wait-server.go     # 터미널 B
nc -w1 localhost 9090 < /dev/null
netstat -an -p tcp | grep 9090
```

**기대**: 마지막 출력에 같은 연결의 `CLOSE_WAIT`(서버)와 `FIN_WAIT_2`(클라이언트)가 함께.
**어긋나면**: TIME-WAIT 는 시간이 지나면 저절로 사라집니다. 호출 직후에 봐야 합니다.
**확인 후 `close-wait-server.go` 를 반드시 종료합니다.**

## §5 TLS 평문 경계 + UDP — TLS 만 sudo 필요

**확인할 것**: 암호화가 시작되는 지점, 그리고 UDP 2패킷과 TCP 12패킷의 대비.

```bash
# 터미널 A
sudo tcpdump -i en0 -n -A host example.com and port 443 -c 6
# 터미널 B
curl -s https://example.com > /dev/null
```

**기대**: ClientHello 안에 도메인 이름이 평문으로. 그 뒤부터 바이트가 깨져 보입니다.
**어긋나면**: 아무것도 안 잡히면 `/etc/hosts` 에 `example.com` 이 되살아난 것입니다. `preflight.sh` 3번이 잡습니다.

```bash
# UDP — sudo 불필요
dig @8.8.8.8 example.com +noall +answer +stats
```

**기대**: `Query time` 왕복 1회, `MSG SIZE rcvd` 72 바이트 근처. 핸드셰이크도 ACK 도 없습니다.

---

## 파일

| 파일 | 역할 |
|---|---|
| `env.sh` | 포트·도메인 SSOT. 8080 으로 되돌리려면 여기만 고칩니다 |
| `preflight.sh` | 실습 전 점검. FAIL 이면 exit 1 |
| `web-server.go` | 저자 예제에서 포트 한 줄만 8081 로 |
| `close-wait-server.go` | 문서 §4 코드 블록 그대로 |
| `fix-hosts.sh` | `/etc/hosts` 의 example.com 오버라이드만 제거 (sudo, 1회성) |

## 덧붙인 실습 — eBGP · iBGP

`bgp/` 에 FRR 컨테이너 3대로 eBGP·iBGP 를 관찰하는 실습을 뒀습니다.
01-04 의 캡처 방식을 라우팅 프로토콜로 이어 가며, 179 포트의 OPEN·KEEPALIVE 를
직접 뜹니다. `bgp/README.md` 참조.

```bash
cd bgp && bash up.sh && bash verify.sh
```

## 되돌리기

```bash
sudo cp /etc/hosts.bak.<timestamp> /etc/hosts   # hosts 복구
rm -rf ~/study/networking-and-kubernetes-lab    # 이 폴더 제거
```

원본 저장소와 노트 문서에는 영향이 없습니다.

---

## 나중에 — 2장 진입 시 처리할 것

`02-03.Linux 네트워크 진단 도구` 는 리눅스 문법으로 쓰여 있어 이 맥북과 어긋난다.
1장 실습에서 실제로 걸린 것만 적어 둔다. **1장 문서를 늘리지 않고 2장에서 처리한다.**

| 02-03 의 예시 | 이 맥북 | 비고 |
|---|---|---|
| `netstat -lp \| grep 3000` | `lsof -nP -iTCP:3000 -sTCP:LISTEN` | macOS netstat 에 `-l` 없음, `-p` 는 프로토콜 지정 |
| `netstat -an` 상태 열 | `netstat -an -p tcp \| grep <포트>` | `-p tcp` 가 프로토콜 필터 |

1장 실습에서 튀어나왔지만 문서에 없던 상황 셋 — 포트 점유(8080·8081), 엉뚱한 포트를
보고 있음(문서 8080 vs 서버 8081), 캐시가 관찰을 막음(ARP). 2장 도구 편의 상황 축 후보.
