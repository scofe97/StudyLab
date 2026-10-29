# podwire — 컨테이너 네트워크 데이터패스를 Go 로 층층이 짓는 프로젝트 계획

- 작성: 2026-09-04. 진행하며 바뀐다 — 기능 목록은 확정이 아니라 현재 최선의 분해다.
- 완료 체크(`- [ ]`)는 *실물에서 무엇을 봐야 끝인가* 다. 코드가 컴파일되는 것은 완료가 아니다.

## 1. 왜 이 프로젝트인가

Networking and Kubernetes 정독(20편)과 2·3장 실습에서 netns·veth·브리지·conntrack·DNAT 을 손으로 쳐 봤다. 다음은 CoreDNS → Cilium → 암호화 → Istio ambient 순으로 읽는다. 읽는 개념을 한 번씩 코드로 굳힐 자리가 필요한데, 매번 새 토이 프로젝트를 만들면 개념 사이의 연결이 끊긴다.

그래서 **하나의 프로젝트가 독서 순서와 같은 순서로 자라게 한다.** 미니 CNI 로 시작해 Service·Policy·eBPF·암호화·L4/L7 메시를 층층이 얹는 "작은 Cilium". 각 기능은 그 시점에 읽는 장을 코드로 굳히고, 앞 기능이 다음 기능의 실험대가 된다.

세션에서 검토하고 버린 대안 — (b) 진단·설명 도구(`podpath trace A B`): 개념마다 probe 한 칸씩 붙는 구조라 확장은 쉽지만 "읽기"에 머문다. `F-23`·`F-24` flow 로그로 일부 흡수. (c) 기존 OSS 확장(CoreDNS 플러그인·Cilium 기여): Go 기초 단계에서 남의 코드베이스 규약이 학습을 가린다.

## 2. 합의된 결정

| 항목 | 결정 |
|---|---|
| 뼈대 | 데이터패스 빌더 — 미니 CNI → 작은 Cilium |
| 이름·위치 | `podwire`. 독립 저장소 `~/study/podwire`. 코드와 문서를 한 폴더에 둔다 |
| 기능 체계 | Stage 순서를 버리고 `F-01`~`F-28` 기능번호 + 선행 기능번호로 (§4). 순차 진행이 아니라 선행조건 조회 |
| 커널 인터페이스 | netlink + iptables 먼저. eBPF 는 `F-17` 에서 Service 경로를 교체 |
| 실행 대상 | `F-01`~`F-06` 은 OrbStack Ubuntu VM(맨 netns, `cnitool`) → 통과 후 kind(`disableDefaultCNI` + `extraMounts`) |
| 크로스노드 | 직접 라우팅 우선(kind 노드는 같은 docker 브리지). VXLAN 은 1b |
| 작업 방식 | 골격·인터페이스·테스트·TODO 스텁은 Claude, 함수 본문과 실습 실행은 사용자 (redpanda Ch05~11 방식, Phase 3 규칙 유지) |
| 산출물 | 코드 + 기능 묶음당 노트 1편(예측 → 실행 → 빗나간 것), 둘 다 이 저장소 안(`docs/`) |
| 기한 | 없음. 독서 Phase 진도에 종속 |
| 포지셔닝 | "안이 보이는 CNI". 프로덕션 대안이 아니라 층을 켜고 끄며 보는 학습·실험용 도구 |

## 3. 환경 사실 (2026-09-04 확인)

- Go 1.25.1 darwin/arm64. kind, kubectl, docker(OrbStack) 있음. `cilium` CLI 없음
- OrbStack Ubuntu 26.04 VM 2대 — `ubuntu`(192.168.139.208), `ubuntu2`(.238). 커널 `7.0.14-orbstack`
  - `ip`·`tc`·`conntrack`·`iptables`·`nft` 있음
  - `/sys/kernel/btf/vmlinux` 있음 → eBPF CO-RE 가능
  - `ip link add type wireguard` 성공 → `F-18` 커널 준비됨
  - `clang`·`llvm-strip`·`bpftool` 없음 → `F-16` 에서 설치
  - `/sys/fs/bpf` 는 root 전용
- 손 실습 스크립트: `~/study/networking-and-kubernetes-lab/netns-bridge/` (up·down·verify) — F-01~F-03 의 정답지
- 기존 Go 코드(참고용): `~/runners-high/project/flow-editor`(stdlib), `my-assistant`(1.7k줄), `poc/.../redpanda-go`(franz-go). netlink·client-go·netns 전환은 미경험
- 정독 노트: N&K 20편 완주, Learning CoreDNS 11편·Istio in Action 14편 초안 있음 (`~/runners-high/write/08_cloud/book/`)
- flow-editor 기존 흐름도: `k8s-packet-flow.json`, `k8s-ingress-flow.json`, `tls-handshake.json`

## 4. 기능 요구사항과 선행조건

Stage 순서를 기능번호로 바꿨다. 순차 목록이 아니라 **조회표**다 — 만들고 싶은 기능이 생기면
선행 기능번호와 선행 독서만 확인하면 지금 되는지가 판정된다.

`선행 기능` 은 코드 의존이고, `선행 독서` 는 만들기 전에 읽어야 뜻이 잡히는 것이다.
`✅` 는 지금 바로, `⚠` 는 부분 가능하거나 환경이 빠짐, `❌` 는 선행이 없어 불가.

| # | 기능 | 선행 기능 | 선행 독서 | 선행 환경 | 지금 |
|---|---|---|---|---|:---:|
| F-01 | netns·veth·브리지를 netlink 로 (`pw lab up/down/verify`) | — | N&K 2장 ✓ | — | ✅ |
| F-02 | 크로스호스트 배선 (ubuntu ↔ ubuntu2) | F-01 | ✓ | VM 2대 ✓ | ✅ |
| F-03 | `LockOSThread` 제거 시 깨지는 지점 재현 | F-01 | — | — | ✅ |
| F-04 | CNI spec `ADD`/`DEL`/`CHECK`/`VERSION` | F-01 | N&K 4장 ✓ | `cnitool` | ✅ |
| F-05 | 파일 기반 IPAM + 파일 잠금 | F-04 | ✓ | — | ✅ |
| F-06 | 노드별 static route 로 크로스노드 | F-04·F-05 | ✓ | kind 3노드 | ✅ |
| F-07 | Node watch 데몬 (client-go 첫 만남) | F-06 | ✓ | — | ✅ |
| F-08 | VXLAN 오버레이 | F-06 | N&K 03-02·04-03 ✓ | — | ✅ |
| F-09 | Service·EndpointSlice watch | F-07 | N&K 5장 ✓ | — | ✅ |
| F-10 | ClusterIP DNAT 3단 체인 (`PW-SERVICES`→`PW-SVC`→`PW-SEP`) | F-09 | ✓ | iptables ✓ | ✅ |
| F-11 | 확률 기반 백엔드 분산 | F-10 | ✓ (02-05 실측) | — | ✅ |
| F-12 | NodePort | F-10 | ✓ | — | ✅ |
| F-13 | hairpin · MASQUERADE | F-10 | ✓ | — | ✅ |
| F-14 | 내 DNS 서버 (Service A 레코드) | F-09 | CoreDNS 16편 ✓ | — | ✅ |
| F-15 | NetworkPolicy ingress/egress | F-10 | N&K 04-03 ✓ · Cilium 12장 ✗ | — | ⚠ |
| F-16 | eBPF 개발 환경 (`clang`·`llvm-strip`·`bpftool`) | — | — | 미설치 | ⚠ |
| F-17 | Service 경로를 eBPF 로 교체 | F-10·F-16 | eBPF ✗ · Cilium 5·6장 ✗ | BTF ✓ | ❌ |
| F-18 | 노드 간 WireGuard 암호화 | F-06 | RWC ✗ · Cilium 14장 ✗ | 커널 ✓ | ⚠ |
| F-19 | 자체 CA + SPIFFE 인증서 발급 | F-07 | Istio 19편 ✓ · ZTN ✗ | — | ⚠ |
| F-20 | L4 프록시 + iptables 리다이렉트 | F-10 | Istio in Action ✓ | — | ✅ |
| F-21 | 노드 간 mTLS + HBONE 터널 | F-19·F-20 | Sidecar-less ✗ | — | ⚠ |
| F-22 | 미니 waypoint (L7) | F-21 | Istio 03·05·06 ✓ | — | ⚠ |
| F-23 | flow 레코드 (iptables `NFLOG`) | F-10 | ✓ | — | ✅ |
| F-24 | flow 레코드 (eBPF 이벤트) | F-17 | — | — | ❌ |
| F-25 | CNI `ADD` 멱등성 (두 번 호출 안전) | F-04·F-05 | CNI spec ✓ | — | ✅ |
| F-26 | 부분 실패 롤백 (netns·veth·IP 정리) | F-04·F-05 | — | — | ✅ |
| F-27 | 노드 재시작 후 IPAM 복구·누수 회수 | F-05 | — | — | ✅ |
| F-28 | 에이전트 재시작 시 체인 재동기화 | F-10 | — | — | ✅ |

### 이번 라운드 범위 — F-01~F-07 + F-25

완료선은 **kind 3노드에서 내 CNI 로 CoreDNS 를 포함한 모든 Pod 가 `Running`, 다른 노드 Pod 끼리 `ping`** 이다.

`F-25` 를 함께 넣는 이유는 선택이 아니어서다. kubelet 은 `ADD` 실패 시 재시도하므로
멱등성이 없으면 `F-06` 완료 판정 자체를 통과 못 한다.

`F-26`·`F-27` 은 일부러 뒤로 뺐다. 롤백이 왜 필요한지는 `ADD` 가 중간에 죽어 veth 가 남은 걸
한 번 겪어야 손에 남는다. 미리 만들면 그럴듯한 코드가 되고, 겪고 만들면 그 문제를 푸는 코드가 된다.

### 무엇이 막고 있나

독서가 막는 것은 일곱이고, 실제로 막는 책은 **Cilium 과 Learning eBPF 두 권**이다. 둘 다 정독 노트 0편이다.
환경이 막는 것은 `F-16` 하나이고 VM 에 `clang`·`llvm-strip`·`bpftool` 을 깔면 끝난다.

### 이번 라운드 완료 판정

이전 판의 Stage 서술을 기능번호로 옮기는 과정에서 **F-08 이후의 상세 체크리스트를 잃었다**
(당시 커밋이 없어 복구 불가). 아래는 이번 범위분만 복원한 것이고, 나머지는 착수할 때 다시 쓴다.

**F-01** — `pw lab up|down|verify` 로 02-04 의 netns 2 + veth 쌍 2 + br0 + 주소·라우트·MASQUERADE 를 netlink 호출로 재현

- [ ] `verify` 출력이 손 실습 `verify.sh` 와 같다 (`ip link`·`ip route`·`ping` 결과)
- [ ] 정답지: `~/study/networking-and-kubernetes-lab/netns-bridge/`

**F-02** — 크로스호스트(ubuntu ↔ ubuntu2) 도 CLI 로 세워진다

**F-03** — `LockOSThread` 를 빼면 실제로 어디가 깨지는지 재현하고 노트에 적었다

**F-04·F-05** — `podwire` 바이너리. CNI spec 의 `ADD`/`DEL`/`CHECK`/`VERSION`.
stdin JSON 설정, 환경변수(`CNI_COMMAND`·`CNI_NETNS`·`CNI_IFNAME`), stdout 결과 JSON.
IPAM 은 자체 구현 — 노드 podCIDR 안에서 파일 기반 할당 + 파일 잠금

- [ ] `cnitool add` 결과 JSON 이 spec 형식이고 `cnitool del` 후 장치·IP 가 남지 않는다
- [ ] 실패 사례 하나를 일부러 만들고 `journalctl -u kubelet` 에서 내 플러그인 stderr 를 찾았다

**F-06** — 크로스노드는 노드별 static route

- [ ] kind 3노드에서 CoreDNS 포함 모든 Pod 가 `Running`, 다른 노드 Pod 끼리 `ping`

**F-07** — 노드 추가·삭제 시 static route 를 갱신하는 작은 데몬(Node watch). **client-go 첫 만남**

**F-25** — 같은 Pod 에 `ADD` 가 두 번 불려도 veth·IP 가 중복되지 않는다

- [ ] `cnitool add` 를 연속 두 번 실행해도 결과가 같고 장치가 하나다

## 5. Go 학습 지도 — 어느 기능에서 무엇을 만나나

| 기능 | Go 개념 |
|---|---|
| F-01~F-03 | 모듈·패키지 배치, 에러 처리(`errors.Is/As`), 시스템 콜 래퍼 사용, `runtime.LockOSThread` |
| F-04~F-07 | `encoding/json`, stdin/stdout 프로토콜, 파일 잠금, 크로스 빌드, 테이블 테스트 |
| F-09~F-13 | goroutine·channel·`context`, informer 패턴, 동기화(`sync.Mutex`), 재시도·백오프 |
| F-15 | 셀렉터 → 집합 변환 자료구조, 규칙 diff |
| F-17 | `go:generate`, cgo 없이 커널 객체 다루기, 바이너리 인코딩(`encoding/binary`) |
| F-18 | 키 관리, 어노테이션 기반 상태 공유 |
| F-19~F-21 | `crypto/tls`·`x509`, `net.Conn` 다루기, CONNECT 터널링 |
| F-22 | `net/http` 서버·리버스 프록시, HTTP/2, 타임아웃과 취소 전파 |

Go 학습 자체를 목표로 잡진 않았다. 위 표는 "이 기능에서 이 개념이 나올 것"을 미리 알아 두는 용도.

## 6. 라이브러리 후보 (실행 시점에 버전·API 를 1차 자료로 확인)

`github.com/vishvananda/netlink`, `github.com/vishvananda/netns`, `github.com/containernetworking/cni`(타입·`cnitool`), `github.com/containernetworking/plugins`(`pkg/ns` 참고), `github.com/coreos/go-iptables`, `k8s.io/client-go`, `github.com/cilium/ebpf`, `golang.zx2c4.com/wireguard/wgctrl`.

버전은 적지 않는다 — context7 preliminary 오독 사례가 있어 실행 시점에 pkg.go.dev 와 go.mod 로 확인한다.

## 7. 확인이 필요한 가정 (실행 전 점검)

- [ ] kind `networking.kubeProxyMode: "none"` 이 현재 kind 버전에서 지원되는지
- [ ] OrbStack docker 에서 macOS 경로 `extraMounts` 가 kind 노드 안 `/opt/cni/bin` 으로 들어가는지 (안 되면 `docker cp` 로 대체)
- [ ] kind 노드 이미지가 arm64 이고 `podCIDR` 가 노드마다 할당되는지 (`--allocate-node-cidrs`)
- [ ] OrbStack 커널 `7.0.14` 에서 TC eBPF attach 와 `bpf2go` 산출물이 도는지 (BTF 는 있음)
- [ ] kind 노드(컨테이너) 안에서 WireGuard 장치를 만들 수 있는지 — 노드 컨테이너의 capability 확인. 안 되면 F-18 는 OrbStack VM 2대 사이에서
- [x] ~~Go module 경로~~ — 해소. `~/study/podwire` 독립 저장소라 StduyWrite 와 무관하다 (2026-09-05 확인)

## 8. 폴더 배치

```
~/podwire/
├─ README.md · ROADMAP.md · Makefile · go.mod · .gitignore
├─ cmd/         pw(F-01~F-03 CLI) · podwire(CNI) · podwire-agent(데몬)
├─ internal/    netns link lab ipam cni controller
│              (datapath·wg·identity·ztunnel·waypoint·flow 는 해당 기능에 착수할 때 만든다)
├─ deploy/      kind 설정 · CNI conflist · install.sh
├─ docs/        기능 노트 (XX-YY.제목.md)
└─ test/        종단 검증 스크립트
```

- 기능 노트 파일명은 `XX-YY.제목.md`. 프론트매터(`title`·`tags`·`status`·`updated`)도 붙인다.
  나중에 `runners-high/write/` 로 옮길 수 있게 규약을 미리 맞춰 둔다.
- 도식은 `docs/_assets/` 에 정적 SVG 로. F-04·F-21 흐름도만 `flow-editor` 에 따로 만든다
  (`~/runners-high/project/flow-editor/examples/src/`, 작성 전 `schema/flow-schema.md` 제1원칙을 읽는다).
- 공개 저장소로 낼 경우 회사명·사내 IP·자격증명이 코드와 노트에 없는지 커밋 전 grep 으로 막는다.

## 9. 제외 범위

- Cilium 수준의 IPAM 모드(Cluster-pool·ENI 등), 멀티클러스터, 완전한 kube-proxy 호환(SessionAffinity·externalTrafficPolicy 전부)
- Istio 의 Envoy 필터·xDS·컨트롤 플레인(istiod). L7 은 waypoint 축소판까지
- (b) 진단 도구를 별도 제품으로 만드는 일 — F-23 로 갈음
- 프로덕션 품질(HA·무중단 업그레이드·CRD 검증)과 프로덕션 배포. Cilium·Calico 를 대체하지 않는다
- 다만 "학습·실험 클러스터에서 실제로 쓸 수 있는 것" 은 목표다. F-25~F-28 이 그 최소 조건이고,
  이건 운영 등급이 아니라 기본기다 — 두 번 불려도 안전한가, 재시작 후 회복되는가
