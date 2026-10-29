# ch06-tftp — TFTP 실무판

『Network Programming with Go』 6장의 TFTP 서버를 현장에서 만나는 모습으로 다시 짠 랩입니다. TFTP 는 지금도 데이터센터 서버의 PXE 네트워크 부팅(랙 단위 OS 설치), 스위치·라우터의 설정 백업과 펌웨어 올리기, IP 전화기와 임베디드 장비의 부트로더(U-Boot)에서 씁니다. 이 랩의 시나리오는 그 현장에서 올라오는 티켓 한 줄에서 출발합니다.

UDP 에서 패킷이 사라지고 순서가 바뀐다는 것은 gonet-lab 에서 이미 netem 으로 확인했습니다. 여기서는 그 사실을 다시 보여 주지 않고, TFTP 가 그 위에서 **무엇을 결정하는가**(전송 모드, 블록 번호 필드 폭, 타이머와 재시도, 창 크기, 옵션 협상, 파일 이름 해석)에 시간을 씁니다.

## 이 프로젝트로 배우는 것

1. 전송 모드(octet·netascii)와 SHA512/256 대조로 "받은 파일이 깨졌다"는 신고를 원인별로 가를 수 있습니다.
2. 블록 번호 필드 폭(uint16)이 한 번에 보낼 수 있는 파일 크기를 정한다는 것을 계산하고, 넘칠 때의 운영 규칙(거절·롤오버·블록 크기 키우기)을 근거를 들어 고를 수 있습니다.
3. 정지-대기 처리량을 블록 크기와 RTT 로 계산해 실측과 맞추고, blksize·windowsize 협상이 그 식의 어느 항을 바꾸는지 설명할 수 있습니다.
4. 중복 ACK 에 다시 보내는 루프와 타임아웃에서만 다시 보내는 루프를 로그 숫자로 구별하고, 손실·RTT 조건에서 타임아웃·재시도 값을 측정으로 고를 수 있습니다.
5. 요청한 파일 이름을 그대로 경로로 여는 서버가 어디까지 열리는지 확인하고, `os.Root` 로 루트 밖 접근을 막을 수 있습니다.
6. 요청마다 전용 소켓을 여는 구조가 동시 요청을 어떻게 받아 내는지 소켓 목록과 걸린 시간으로 확인할 수 있습니다.

| 시나리오 | 현장의 증상 | 이어지는 목표 |
|---|---|---|
| T1 | 받은 펌웨어가 장비에서 깨졌다고 한다 | 1 |
| T2 | 40MB 이미지만 받다가 멈춘다 | 2 |
| T3 | 원격 지사에서는 부팅이 수십 분 걸린다 | 3 |
| T4 | 로그에 같은 블록이 두 번씩 나간다 | 4 |
| T5 | 타임아웃·재시도 값을 정해 달라는 요청 | 4 |
| T6 | 보안 점검에서 지적받았다 | 5 |
| T7 | 랙 하나를 한꺼번에 재부팅했다 | 6 |

시나리오마다 **실행 전에 적을 예측**을 먼저 적고 나서 돌립니다. 이 README 와 코드 어디에도 예측의 답은 없습니다.



## 준비

모든 명령은 이 폴더에서 실행합니다.

```bash
cd ~/study/npg-practice/ch06-tftp
NOTES=~/runners-high/write/02_os/book/npg_network-programming-with-go

# macOS 용 바이너리와 OrbStack(Linux arm64) 용 바이너리
go build -o bin/ ./cmd/...
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/linux/ ./cmd/...

# 시나리오용 파일 (work/ 아래, 약 110MB)
./scripts/mkroot.sh

# 불변식 테스트 (인코딩 왕복·해시 일치·형식 오류 거절)
go test ./... -count=1
```

- 포트는 TFTP 표준 69 대신 **6969** 를 씁니다. 실제 클라이언트에는 포트를 따로 적어 줍니다.
- 실제 클라이언트 셋을 씁니다.
  - macOS `tftp`(BSD 계열): `tftp 127.0.0.1 6969` 로 띄우고 표준 입력으로 명령을 줍니다. 띄운 뒤 `connect` 로 붙는 방식은 이 머신에서 RRQ 를 보내지 못했습니다. 모드는 `binary`·`ascii` 명령으로 바꿉니다.
  - `curl`: `curl tftp://127.0.0.1:6969/<파일> -o <저장>` · 모드는 URL 끝 `;mode=netascii` · 블록 크기는 `--tftp-blksize`.
  - `tftp-hpa`(OrbStack `ubuntu`): `sudo apt install tftp-hpa` 로 설치하고 `tftp -v -m <binary|netascii> 127.0.0.1 6969 -c get <파일> <저장>`.
- Linux 가 필요한 것(netem·tcpdump·tftp-hpa)은 OrbStack `ubuntu` 에서 돌립니다. 이 폴더에서 `orb -m ubuntu <명령>` 을 부르면 같은 경로에서 실행됩니다. 머신 안의 `~` 는 `/home/...` 이라 macOS 경로와 다릅니다.
- 지연·손실은 `scripts/netns.sh` 가 만드는 **네임스페이스 둘과 veth 쌍에만** 겁니다. 머신의 `eth0` 은 건드리지 않습니다(다른 실습이 같은 머신을 씁니다).

```bash
orb -m ubuntu sudo ./scripts/netns.sh up             # tftp-srv 10.69.0.1 <-> tftp-cli 10.69.0.2
orb -m ubuntu sudo ./scripts/netns.sh set 10ms 0%    # 방향마다 지연 10ms, 손실 0%
orb -m ubuntu sudo ./scripts/netns.sh status
orb -m ubuntu sudo ./scripts/netns.sh down           # 끝나면 반드시 걷기
```



## 구성

| 경로 | 하는 일 |
|---|---|
| `tftp/packet.go` | RRQ·WRQ·DATA·ACK·ERROR·OACK 인코딩·디코딩, 블록 번호 롤오버 정책 |
| `tftp/netascii.go` | netascii 변환(LF↔CR LF, CR↔CR NUL) |
| `tftp/server.go` | 읽기 전용 서버 — 요청마다 고루틴과 전용 소켓, 창 단위 전송·타임아웃 재전송·재시도 상한, 옵션 협상, `os.Root` |
| `tftp/client.go` | 클라이언트 — 옵션 요청, 창 단위 수신, TID 확인, ACK 지연 주입 |
| `tftp/*_test.go` | 불변식 테스트와 T4 관찰 테스트(`TestDelayedAckObserve`, 숫자는 `-v` 로 봅니다) |
| `cmd/tftpd` | 서버 명령 |
| `cmd/tftpget` | 파일 하나를 받는 클라이언트 명령 |
| `cmd/tftpbench` | 같은 파일을 여러 번(동시에도) 받아 CSV 한 줄로 요약 |
| `scripts/mkroot.sh` · `netns.sh` · `t5-grid.sh` | 파일 준비 · 네임스페이스와 netem · T5 격자 측정 |

### tftpd 플래그

| 플래그 | 기본값 | 뜻 |
|---|---|---|
| `-root` | `work/root` | 내줄 파일이 있는 폴더 |
| `-listen` | `127.0.0.1:6969` | 요청을 받는 UDP 주소 |
| `-timeout` | `6s` | DATA(창)를 보낸 뒤 ACK 를 기다리는 시간 (책과 같은 값) |
| `-retries` | `10` | 같은 블록(창)을 보내는 최대 횟수 (책과 같은 값) |
| `-options` | `true` | RFC 2347 옵션 협상(blksize·windowsize·tsize·timeout) |
| `-max-blksize` · `-max-window` | `65464` · `64` | 협상에서 받아들일 상한 |
| `-rollover` | `zero` | 블록 번호 65535 다음: `zero`(0 으로) · `one`(1 로) · `refuse`(넘칠 전송은 시작 전에 ERROR) |
| `-netascii` | `convert` | netascii 요청: `convert`(RFC 대로 변환) · `raw`(모드를 무시하고 그대로) · `reject`(ERROR) |
| `-legacy-dup-ack` | `false` | 책 Listing 루프 — 지난 ACK 를 받으면 현재 블록을 곧바로 다시 보내고 재시도 한 번을 쓴다 |
| `-unsafe-open` | `false` | `os.Root` 대신 `filepath.Join(root, 이름)` 을 `os.Open` 으로 연다 |

서버는 전송이 끝날 때마다(성공·실패·거절 모두) 한 줄을 남깁니다.

| 칸 | 세는 것 |
|---|---|
| `mode` · `blksize` · `window` | 요청 모드와 협상 결과 |
| `bytes` · `blocks` | ACK 를 받은 파일 바이트와 블록 수 |
| `sent` | 보낸 DATA 패킷 수(재전송 포함) |
| `timeouts` · `resend_timeout` | 타임아웃 횟수와 그때 다시 보낸 DATA 수 |
| `resend_dupack` | 지난 ACK 를 받고 곧바로 다시 보낸 DATA 수 |
| `resend_seqerr` | 창 중간까지만 ACK 가 와서 나머지를 다시 보낸 DATA 수 (RFC 7440 §4) |
| `dupack` · `bad` | 현재 창에 해당하지 않는 ACK 수, 해석할 수 없는 패킷 수 |
| `elapsed` · `rate` · `result` | 걸린 시간, 처리량, `ok`·`error: …`·`refused: …` |

### tftpget 과 tftpbench

`tftpget [플래그] <원격파일>` 은 받은 뒤 클라이언트 쪽 숫자(`dup_data` 같은 블록을 다시 받은 수, `unexpected` 순서가 어긋난 수, `acks`, `timeouts`, `strangers` 다른 TID 에서 온 패킷 수)를 한 줄로 남깁니다. `-o -` 는 저장하지 않고 버리며, `-sum` 은 `shasum -a 512256` 과 같은 해시를 찍습니다. 장애 주입은 `-ack-delay-block N -ack-delay D` 로, N 번 블록의 첫 ACK 만 D 만큼 늦춥니다.

`tftpbench` 는 `-n` 번 받고(`-parallel` 개씩 동시에) `-orig` 원본과 해시까지 맞으면 성공으로 셉니다. 출력은 `label,n,parallel,ok,fail,success_pct,min,p50,p95,max,mean_KiBps,client_timeouts,wall` CSV 한 줄이고, 실패 이유는 종류별 개수로 표준 에러에 남습니다.



## T1 받은 펌웨어가 장비에서 깨졌다고 한다

**증상** — "스위치에 TFTP 로 올린 펌웨어 이미지가 장비의 무결성 검증에 실패합니다. 서버에 있는 원본 이미지는 정상입니다."

**실행 전에 적을 예측**

1. macOS `tftp`·`curl`·`tftp-hpa` 는 모드를 따로 지정하지 않으면 각각 어떤 모드로 요청할까요?
2. 서버 `-netascii` 를 `convert`·`raw`·`reject` 로 바꿔 가며 macOS `tftp`·`curl`·`tftp-hpa` 가 netascii 로 `fw.bin` 을 받으면, 클라이언트마다 받은 파일의 해시는 원본과 같을까요?
3. 같은 조건에서 텍스트 파일 `switch01.cfg`(줄 끝 LF, CR 없음)는 어떨까요? 펌웨어와 결과가 다르다면 바이트의 어떤 차이 때문일까요?
4. netascii 전송에서 서버 로그 `bytes=` 는 원본 파일 크기(1,048,576)와 같을까요?

**돌리는 법** — 터미널 하나에 서버, 다른 터미널에서 클라이언트를 돌립니다. 서버 정책마다 서버를 다시 띄웁니다.

```bash
./bin/tftpd -netascii convert          # raw, reject 로 바꿔 가며
# macOS
printf 'get fw.bin work/dl/fw.bsd\nquit\n' | tftp 127.0.0.1 6969
printf 'ascii\nget fw.bin work/dl/fw.bsd-ascii\nquit\n' | tftp 127.0.0.1 6969
curl tftp://127.0.0.1:6969/fw.bin -o work/dl/fw.curl
curl "tftp://127.0.0.1:6969/fw.bin;mode=netascii" -o work/dl/fw.curl-ascii
./bin/tftpget -mode netascii -o work/dl/fw.ours-ascii fw.bin
# OrbStack (서버도 그 안에서: orb -m ubuntu ./bin/linux/tftpd -netascii convert)
orb -m ubuntu tftp -v 127.0.0.1 6969 -c get fw.bin work/dl/fw.hpa-default
orb -m ubuntu tftp -v -m binary 127.0.0.1 6969 -c get fw.bin work/dl/fw.hpa-binary
shasum -a 512256 work/root/fw.bin work/dl/fw.*
```

**볼 것** — 서버 로그의 `mode=` 칸(클라이언트가 실제로 보낸 모드), `bytes=` 칸, 그리고 `shasum` 줄의 해시가 원본과 같은지. 클라이언트가 출력하는 "받은 바이트 수"와 `ls -l` 의 파일 크기도 나란히 적어 둡니다.

**이어 읽을 노트** — `$NOTES/06-02…` §3 "무결성 검증과 SHA512/256 해시의 선택 이유", `$NOTES/06-01…` §2 "RRQ 패킷 구조와 구분자 배치"(mode 필드). RFC 1350 의 netascii 정의는 Telnet(RFC 854)의 NVT ASCII 를 빌려 옵니다.



## T2 40MB 이미지만 받다가 멈춘다

**증상** — "작은 부트 파일은 문제없는데 40MB 짜리 설치 이미지만 받다가 멈춥니다."

**실행 전에 적을 예측**

1. 33,553,919 · 33,553,920 · 40,000,000 바이트 파일은 512바이트 블록으로 각각 몇 블록일까요? 블록 번호가 65,535 를 넘는 파일은 어느 것일까요?
2. 서버 기본값(`-rollover zero`)에서 `curl`·macOS `tftp`·`tftp-hpa`·`tftpget` 은 `big.img` 를 끝까지 받을까요?
3. 서버를 `-rollover one` 으로 바꾸면 클라이언트마다 어떻게 될까요? 각 클라이언트의 마지막 출력 문구, 종료 코드, 받은 파일의 크기·해시를 따로 예측합니다.
4. `-rollover refuse` 서버에 `-blksize 1428` 로 요청하면 `big.img` 는 어떻게 될까요?
5. 현장 서버라면 셋 중 무엇을 기본값으로 두고, 클라이언트 쪽에서는 무엇을 확인하게 하겠습니까?

**돌리는 법**

```bash
./bin/tftpd -rollover zero -timeout 1s -retries 3      # one, refuse 로 바꿔 가며
curl --max-time 120 tftp://127.0.0.1:6969/big.img -o work/dl/big.curl; echo "exit=$?"
printf 'get big.img work/dl/big.bsd\nquit\n' | tftp 127.0.0.1 6969; echo "exit=$?"
./bin/tftpget -o work/dl/big.ours big.img; echo "exit=$?"
./bin/tftpget -rollover one -o work/dl/big.ours-one big.img     # 클라이언트 기대값을 바꿔 보기
./bin/tftpget -blksize 1428 -o work/dl/big.ours-1428 big.img
# OrbStack: orb -m ubuntu ./bin/linux/tftpd -rollover … 를 띄운 뒤
orb -m ubuntu bash -c 'tftp -v -m binary 127.0.0.1 6969 -c get big.img work/dl/big.hpa; echo "exit=$?"'
ls -l work/dl/big.*; shasum -a 512256 work/root/big.img work/dl/big.*
```

macOS `tftp` 에는 `rollover 0`·`rollover 1` 명령도 있으니 서버 정책과 맞춰 한 번 더 받아 봅니다. 한 판에 수 초가 걸립니다.

**볼 것** — 서버 로그의 `blocks=`·`sent=`·`dupack=`·`result=`, 클라이언트의 마지막 줄과 `exit=`, 파일 크기와 해시. 셋(문구·종료 코드·해시)을 따로 기록합니다.

**이어 읽을 노트** — `$NOTES/06-01…` §3(데이터 패킷과 블록 번호), RFC 2348(blksize).



## T3 원격 지사에서는 부팅이 수십 분 걸린다

**증상** — "본사 랙에서는 PXE 부팅이 금방 끝나는데, RTT 50ms 인 지사에서는 커널·initrd 를 받는 데만 한참 걸립니다. 회선은 1Gbps 입니다."

**실행 전에 적을 예측**

1. RTT 0·20·50·100ms 에서 `boot-128k.bin`(131,072바이트)을 기본 설정(512바이트, 창 1)으로 받는 데 각각 몇 초가 걸릴까요? 계산식을 함께 적습니다.
2. `-blksize 1428`, `-windowsize 8`, 둘 다를 켜면 위 식의 어느 항이 바뀌고, RTT 50ms 에서 몇 초가 될까요?
3. `-blksize 8192` 는 veth MTU 1500 위에서 IP 계층에서 무슨 일을 일으킬까요? 1428 보다 빠를까요, 느릴까요?
4. 이 결과를 TCP 슬라이딩 윈도와 BDP 로 설명하면 어떻게 될까요?

**돌리는 법** — `netns.sh up` 뒤에 OrbStack 에서. `set` 의 값은 방향마다의 지연이라 RTT 는 두 배입니다.

```bash
orb -m ubuntu sudo bash -c '
for d in 0ms 10ms 25ms 50ms; do
  ./scripts/netns.sh set $d 0%
  ip netns exec tftp-srv ./bin/linux/tftpd -listen 10.69.0.1:6969 2>>work/t3-server.log & pid=$!; sleep 0.3
  for a in "" "-blksize 1428" "-windowsize 8" "-blksize 1428 -windowsize 8" "-blksize 8192"; do
    echo "delay=$d [$a]"; ip netns exec tftp-cli ./bin/linux/tftpget -server 10.69.0.1:6969 -o - $a boot-128k.bin
  done
  kill $pid; wait $pid
done'
```

`blksize 8192` 로 받는 동안 다른 터미널에서 tcpdump 를 봅니다.

```bash
orb -m ubuntu sudo ip netns exec tftp-cli tcpdump -i veth-cli -nn -c 20 'udp or (ip[6:2] & 0x1fff != 0)'
```

**볼 것** — 클라이언트 줄의 `elapsed=`·`rate=`·`blocks=` 와 예측 1·2 의 계산값. tcpdump 에 찍히는 줄의 종류와 개수.

**이어 읽을 노트** — `$NOTES/06-02…` §2 "TCP 슬라이딩 윈도와의 대비", `$NOTES/03-01…` §3(창), `$NOTES/04-03…`(BDP), `$NOTES/05-02…` §2, RFC 2348·RFC 7440.



## T4 로그에 같은 블록이 두 번씩 나간다

**증상** — "혼잡한 시간대 TFTP 서버 로그를 보면 어느 순간부터 블록마다 패킷이 두 번씩 나가고, 회선 사용량도 그만큼 늘었습니다."

**실행 전에 적을 예측**

1. 서버 타임아웃 200ms, 10번 블록의 ACK 만 300ms 늦게 도착합니다. `-legacy-dup-ack` 서버에서 `big.img`(78,126블록)를 보낼 때 `sent=` 와 `resend_dupack=` 은 얼마쯤일까요? 클라이언트의 `dup_data=` 는?
2. 기본(fixed) 서버에서는 세 숫자가 어떻게 될까요?
3. 두 판의 `elapsed=` 는 어느 쪽이 길고, 왜일까요?
4. 이 반복은 스스로 멈출까요? 멈춘다면 무엇이 멈추게 할까요?

**돌리는 법**

```bash
./bin/tftpd -timeout 200ms -legacy-dup-ack          # 한 번은 이것으로
./bin/tftpd -timeout 200ms                          # 한 번은 이것으로
./bin/tftpget -o - -ack-delay-block 10 -ack-delay 300ms big.img
# 같은 장면을 테스트 하나로
go test ./tftp -run TestDelayedAckObserve -v -count=1
```

**볼 것** — 서버 줄의 `sent=`·`timeouts=`·`resend_timeout=`·`resend_dupack=`·`dupack=`·`elapsed=`, 클라이언트 줄의 `dup_data=`·`acks=`. 고친 루프는 `tftp/server.go` 의 `run` 에서 `s.legacy` 분기 앞뒤를 읽습니다.

**이어 읽을 노트** — `$NOTES/06-02…` §2 "이중 루프로 구현된 handle 제어문", RFC 1123 §4.2.3.1.



## T5 타임아웃·재시도 값을 정해 달라는 요청

**증상** — "지사 회선이 가끔 손실이 납니다. TFTP 서버의 timeout·retries 를 얼마로 해야 하는지 근거를 달라고 합니다."

**실행 전에 적을 예측**

1. 방향마다 손실 10% 일 때 DATA 한 번과 그 ACK 한 번이 모두 살아남을 확률은 얼마일까요? 64블록 파일(`boot-32k.bin`)을 받는 동안 타임아웃은 몇 번쯤 날까요?
2. RTT 20ms 에서 서버 timeout 200ms 와 1s 는 성공률과 걸린 시간을 각각 어떻게 바꿀까요? retries 3 과 10 은?
3. timeout 을 RTT 보다 짧게 잡으면 어떤 숫자가 늘어날까요?
4. 손실 20% 에서 retries 3 으로 64블록을 끝까지 보낼 확률을 대략 계산할 수 있을까요?

**돌리는 법** — `netns.sh up` 뒤에. 기본 격자는 24칸이라 오래 걸리므로 먼저 몇 칸만 돌려 봅니다.

```bash
orb -m ubuntu sudo env DELAYS="10ms" LOSSES="10%" TIMEOUTS="200ms 1s" RETRIES="3 10" N=5 \
  ./scripts/t5-grid.sh > work/t5-small.csv
orb -m ubuntu sudo ./scripts/t5-grid.sh > work/t5.csv        # 전체 격자
```

스크립트는 칸마다 netem 을 바꾸고, `tftp-srv` 에 `tftpd -timeout T -retries R` 을 띄우고, `tftp-cli` 에서 `tftpbench` 를 `N` 번 돌린 뒤 서버를 내립니다. 클라이언트는 `CLI_TIMEOUT=3s`·`CLI_RETRIES=3` 으로 서버보다 오래 버티게 두었습니다. 클라이언트도 자기 타임아웃에 마지막 ACK 를 다시 보내므로, 그 값을 바꾸면 결과가 함께 움직입니다.

**볼 것** — CSV 의 `success_pct`·`p50`·`max`·`client_timeouts`, 실패 이유(표준 에러의 `fail x…`), `work/t5-server.log` 의 `timeouts=`·`result=`. 손실 자체가 어떻게 생기는지는 다시 보지 않습니다.

**이어 읽을 노트** — `$NOTES/06-02…` §2(재시도 상한과 고정 6초 타임아웃), RFC 7440 §6(재시도 상한과 예시 값), gonet-lab `~/runners-high/write/02_os/project/gonet-lab/02-03…` §4(netem 손실), `02-02…`(TCP 의 손실 판정과 RTO).



## T6 보안 점검에서 지적받았다

**증상** — "취약점 점검 보고서: TFTP 서비스가 경로 순회(path traversal)로 서버의 임의 파일을 내준다. 쓰기 요청 처리 여부도 확인 바람."

**실행 전에 적을 예측**

1. `-unsafe-open` 서버에 `../../../../../../../../etc/passwd` 를 요청하면 어떻게 될까요? `filepath.Join` 이 `..` 을 정리해 준다는 사실은 이 질문의 답을 바꿀까요?
2. 기본(`os.Root`) 서버는 같은 요청에 어떤 오류 코드를 보내고, 서버 로그에는 무엇이 남을까요?
3. `/etc/passwd` 처럼 `/` 로 시작하는 이름은 두 판에서 각각 어떻게 될까요?
4. 루트 안의 `link-out`(루트 밖 `work/outside/secret.txt` 를 가리키는 심볼릭 링크)은 두 판에서 각각 어떻게 될까요?
5. macOS `tftp` 의 `put`(WRQ)에 클라이언트는 무엇을 출력하고, 서버 로그의 `result=` 에는 무엇이 남을까요?

**돌리는 법**

```bash
./bin/tftpd                       # 한 번은 기본
./bin/tftpd -unsafe-open          # 한 번은 막지 않은 판
P='../../../../../../../../etc/passwd'
./bin/tftpget -timeout 1s -retries 1 -o - "$P"
./bin/tftpget -timeout 1s -retries 1 -o - /etc/passwd
./bin/tftpget -timeout 1s -retries 1 -o - link-out
curl --path-as-is "tftp://127.0.0.1:6969/$P"
printf 'get %s work/dl/passwd.bsd\nquit\n' "$P" | tftp 127.0.0.1 6969
printf 'put work/root/switch01.cfg evil.cfg\nquit\n' | tftp 127.0.0.1 6969
```

`curl` 은 `--path-as-is` 없이는 URL 의 `..` 을 먼저 정리해서 보냅니다. `work/dl/` 의 결과물은 실습이 끝난 뒤 지웁니다.

**볼 것** — 클라이언트의 `bytes=` 와 `error:` 문구, 서버 로그의 `file=` 과 `result=` 칸. 두 쪽의 문구를 나란히 적고 `tftp/server.go` 의 `open`·`handle` 을 읽어 각 문구가 어디서 나오는지 찾습니다.

**이어 읽을 노트** — `$NOTES/06-02…` §1 "Server 타입의 필드와 기본값"(책 서버는 이름을 쓰지 않음), RFC 1350 Error Codes, RFC 7440 §7(서버 프로세스 권한), Go 문서 `os.Root`(`go doc os.Root`).



## T7 랙 하나를 한꺼번에 재부팅했다

**증상** — "랙 하나(서버 50대)를 동시에 재부팅하자 PXE 서버 하나에 요청이 몰렸습니다. 한 대씩 할 때와 비교해 얼마나 걸리는지 알고 싶습니다."

**실행 전에 적을 예측**

1. RTT 20ms 에서 `boot-128k.bin` 을 50개 동시에 받을 때 서버 프로세스가 연 UDP 소켓은 몇 개일까요? 각 소켓의 상대 주소는?
2. 50개 동시의 `p50` 은 한 개만 받을 때의 시간과 비교해 어떨까요? 왜 그럴까요?
3. 서버가 6969 리스너 하나로 데이터까지 보냈다면(책 6장이 피한 설계) 무엇이 달라질까요?

**돌리는 법** — `netns.sh up` 뒤에.

```bash
orb -m ubuntu sudo ./scripts/netns.sh set 10ms 0%
orb -m ubuntu sudo ip netns exec tftp-srv ./bin/linux/tftpd -listen 10.69.0.1:6969 &
orb -m ubuntu sudo ip netns exec tftp-cli ./bin/linux/tftpbench -server 10.69.0.1:6969 \
  -file boot-128k.bin -orig work/root/boot-128k.bin -n 50 -parallel 50 -label rack -header
# 전송 도중 다른 터미널에서
orb -m ubuntu sudo ip netns exec tftp-srv ss -uanp
orb -m ubuntu sudo ip netns exec tftp-cli ./bin/linux/tftpbench -server 10.69.0.1:6969 \
  -file boot-128k.bin -orig work/root/boot-128k.bin -n 1 -label single
```

**볼 것** — `ss` 출력의 줄 수와 `Local Address:Port`·`Peer Address:Port`, CSV 의 `p50`·`max`·`wall`.

**이어 읽을 노트** — `$NOTES/06-02…` §1 "요청마다 고루틴과 net.Dial 을 분리하는 이유", `$NOTES/05-02…` §1, gonet-lab `02-03…` §3.



## 정리

```bash
orb -m ubuntu sudo ./scripts/netns.sh down
orb -m ubuntu ip netns list                 # 빈 출력
orb -m ubuntu tc qdisc show | grep netem    # 빈 출력
pgrep -fl tftpd; orb -m ubuntu pgrep -fl tftpd
```

`work/` 는 다시 만들 수 있는 실습 파일입니다. 지울 때는 절대 경로로 지웁니다.



## 책과 다르게 한 것

- 책의 서버는 파일 이름과 상관없이 메모리의 페이로드 하나를 내줍니다. 여기서는 루트 폴더의 파일을 이름으로 내주고, `os.Root`(Go 1.24+)로 루트 밖을 막습니다.
- 책은 octet 만 받습니다. 여기서는 netascii 를 정책 세 가지로 다룹니다(T1).
- 책의 `Data` 는 `io.Reader` 를 들고 다니며 블록을 만들고, 여기서는 창 단위 재전송을 위해 만든 패킷을 창이 끝날 때까지 들고 있습니다.
- 책의 재전송 루프는 `-legacy-dup-ack` 로 남겨 두고, 기본은 RFC 1123 §4.2.3.1 을 따르는 루프입니다(T4).
- RFC 2347·2348·2349·7440 옵션 협상을 더했습니다. 서버는 요청 값보다 큰 값으로 답하지 않고, 알 수 없는 옵션은 OACK 에서 뺍니다.
- 응답은 리스너가 묶인 IP 에서 나가도록 전용 소켓의 출발 주소를 고정합니다. 리스너가 `0.0.0.0` 이면 커널이 고릅니다.
- 테스트는 불변식(인코딩 왕복, 받은 파일 해시 = 원본, 형식 오류 거절, 루트 밖 거절)만 단언하고, 관찰값은 `t.Logf` 로 남깁니다.
