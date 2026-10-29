# eBGP · iBGP 실습 — FRR 컨테이너 3대

01-04 의 캡처 방식(관측 지점을 고르고 tcpdump 로 확인)을 라우팅 프로토콜로 이어 가는 실습입니다.
BGP 는 책 5장(MetalLB) 소재지만, 여기서는 K8s 없이 **순수 BGP 만** 봅니다.

## 토폴로지

```
        AS 65001                      AS 65002
   ┌──────────────────────┐        ┌───────────┐
   │  r1 ────iBGP──── r2  │        │    r3     │
   │ .0.2            .0.3 │        │   .0.4    │
   └───┬──────────────────┘        └─────┬─────┘
       └───────────eBGP───────────────────┘

   loopback:  r1 10.0.0.1  r2 10.0.0.2  r3 10.0.0.3
   대역:      172.30.0.0/24 (docker network bgplab)
```

**r2 는 r3 와 직접 피어링하지 않습니다.** 일부러 그렇게 뒀습니다. r2 가 r3 의 경로를
어떻게 얻는지, 그때 next-hop 이 어떻게 되는지가 이 실습의 핵심입니다.

## 실행

```bash
bash up.sh          # FRR 3대 기동 (세션 수립까지 ~12초)
bash verify.sh      # eBGP/iBGP 차이 자동 확인
bash capture.sh open        # 세션 수립 순간의 OPEN 메시지
bash capture.sh keepalive   # 3초 주기 KEEPALIVE
bash down.sh        # 정리
```

`up.sh` 는 `bgplab` 네트워크와 r1~r3 만 만듭니다. **kind `k8s-lab` 클러스터와
`sq-jenkins` 는 건드리지 않습니다.**

## 무엇을 보는가

### 1. eBGP 와 iBGP 는 같은 정보를 다르게 싣는다

`show bgp ipv4 unicast <prefix>` 의 출력을 나란히 놓으면 갈립니다.

| | eBGP 로 배운 경로 (10.0.0.3) | iBGP 로 배운 경로 (10.0.0.2) |
|---|---|---|
| AS_PATH | `65002` — AS 를 건너며 붙음 | `Local` — 비어 있음 |
| 표시 | `external` | `internal` |
| localpref | 없음 | `100` |

AS_PATH 가 AS 경계를 넘을 때만 늘어나는 것이 BGP 가 루프를 막는 방식입니다.
자기 AS 번호가 이미 들어 있는 경로를 받으면 버립니다.

### 2. eBGP 경로를 iBGP 로 넘길 때 next-hop 이 안 바뀐다

r2 에서 `show bgp ipv4 unicast 10.0.0.3/32` 를 보면:

```
  65002
    172.30.0.4 from 172.30.0.2 (10.0.0.1)
```

**`from` 은 r1 인데 next-hop 은 r3(172.30.0.4)** 입니다. r1 이 자기를 next-hop 으로
바꿔 주지 않았습니다. 이게 BGP 입문에서 가장 흔히 걸리는 함정입니다.

이 실습에서는 세 라우터가 같은 172.30.0.0/24 에 있어 r2 가 172.30.0.4 로 직접 갈 수
있으므로 통신이 됩니다. 실무처럼 r2 가 그 대역을 모르면 경로는 테이블에 있는데
`inaccessible` 로 뜨고 트래픽은 안 흐릅니다. 그때 쓰는 것이
`neighbor <ip> next-hop-self` 입니다.

직접 확인하려면:

```bash
docker exec r1 vtysh -c 'configure terminal' -c 'router bgp 65001' \
  -c 'address-family ipv4 unicast' -c 'neighbor 172.30.0.3 next-hop-self'
docker exec r1 vtysh -c 'clear bgp * soft out'
docker exec r2 vtysh -c 'show bgp ipv4 unicast 10.0.0.3/32' | grep from
```

r2 의 next-hop 이 172.30.0.4 에서 172.30.0.2(r1)로 바뀝니다 — 실측 확인.
`next-hop-self` 는 `address-family ipv4 unicast` 안에 넣어야 먹습니다.

### 3. "iBGP 는 재전달하지 않는다" 의 정확한 경계

01-03 노트는 iBGP 가 배운 경로를 다른 iBGP 이웃에게 다시 전달하지 않는다고 적었습니다.
그런데 이 실습에서 r2 는 r1 을 거쳐 r3(AS65002)의 경로를 받았습니다. 모순처럼 보이지만
아닙니다. **금지되는 것은 iBGP 로 배운 것을 다시 iBGP 로 넘기는 경우뿐**입니다.

| r1 이 배운 경로 | 배운 곳 | r2(iBGP)에게 전달 | r3(eBGP)에게 전달 |
|---|---|---|---|
| 10.0.0.3 | eBGP(r3) | O — 금지 대상 아님 | — |
| 10.0.0.2 | iBGP(r2) | X — 금지 | O |

r1 에서 확인할 수 있습니다:

```bash
docker exec r1 vtysh -c 'show bgp ipv4 unicast 10.0.0.2/32' | grep -A2 Advertised
```

`Advertised to non peer-group peers: 172.30.0.4` — iBGP 로 배운 10.0.0.2 를
eBGP 상대인 r3 에게만 광고하고 있습니다. 풀메시가 필요한 이유가 이 규칙이고,
이 실습의 3노드 구성이 그 경계를 최소 크기로 보여 줍니다.

### 4. OPEN 메시지는 평문이다

`bash capture.sh open` 출력:

```
Open Message (1), length: 124
  Version 4, my AS 65001, Holdtime 9s, ID 10.0.0.1
```

AS 번호·Holdtime·Router ID 가 그대로 읽힙니다. 01-04 §5 에서 TLS 의 SNI 가 평문으로
읽힌 것과 같은 종류의 관찰입니다. BGP 는 기본적으로 암호화가 없고, 보호가 필요하면
TCP-AO 나 MD5 인증을 따로 겁니다.

### 5. KEEPALIVE 는 19바이트다

`bash capture.sh keepalive` 를 보면 `length: 19` 가 3초마다 양방향으로 오갑니다.
설정의 `timers bgp 3 9` 가 그것입니다 — 3초마다 살아 있다고 알리고, 9초 동안
소식이 없으면 세션을 내립니다. 기본값은 60/180 인데 실습에서 기다리기 지루해
줄였습니다.

BGP 메시지 헤더는 19바이트(마커 16 + 길이 2 + 타입 1)라, KEEPALIVE 는 헤더만 있고
본문이 없는 메시지입니다.

## 캡처는 왜 사이드카로 하나

macOS 의 Docker Desktop 은 리눅스 VM 안에서 돌아, 호스트에서 `tcpdump` 를 떠도
`br-*` 브리지가 보이지 않습니다. 그래서 `--net=container:r1` 로 netshoot 컨테이너를
r1 의 네트워크 네임스페이스에 붙여 그 안에서 캡처합니다.

01-04 §1 의 "관측 지점이 보이는 계층을 결정한다" 가 여기서도 그대로입니다.
관측 지점을 잘못 고르면 트래픽이 있는데도 안 보입니다.

## 파일

| 파일 | 역할 |
|---|---|
| `up.sh` / `down.sh` | 토폴로지 기동·정리 |
| `verify.sh` | eBGP/iBGP 차이 자동 검증. FAIL 이면 exit 1 |
| `capture.sh` | 179 포트 캡처 (`open` \| `keepalive`) |
| `r1,r2,r3/frr.conf` | 각 라우터 BGP 설정 |
| `r1,r2,r3/daemons` | bgpd 활성화 (FRR 기본은 `bgpd=no`) |

## 더 해 볼 것

- `next-hop-self` 를 넣고 r2 의 next-hop 변화 관찰 (위 2번)
- r2 에도 r3 와의 eBGP 를 추가해 경로가 둘이 될 때 어느 쪽이 best 로 뽑히는지
- `timers bgp` 를 늘리고 KEEPALIVE 간격이 실제로 바뀌는지 캡처로 확인
- r3 를 정지시켜 holdtime 9초 후 세션이 내려가는 과정 캡처
