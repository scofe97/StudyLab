# runners-lab

`runners-lab`은 개념 학습(`runners-high`)을 거쳐 구현과 관측으로 직접 검증하는 Phase 3 실습 코드들을 도메인 계층별로 통합 관리하는 공개 모노레포다. Bazel(bzlmod) 기반 빌드 시스템을 채택하여 패키지 간의 명확한 경계, 세밀한 캐싱, 변경 영향에 따른 선별적 테스트 환경을 제공한다.

## 실습 목록 (Labs)

| 경로 | 한 줄 설명 | 원래 이름 |
|---|---|---|
| `net/gonet` | Go 로 네트워크 primitive 를 직접 구현하고, 실패시키고, 관측하면서 TCP/UDP 에서 Reverse Tunnel, P2P Overlay, Linux Networking, eBPF 까지 단계적으로 내려가는 학습 저장소입니다. | `gonet-lab` |
| `net/podwire` | 기본 CNI 를 끈 kind 클러스터에 podwire 를 깔면 CoreDNS 를 포함한 모든 Pod 가 뜹니다. 그리고 모든 패킷 결정이 어느 층에서 왜 내려졌는지 flow 로그로 남습니다. | `podwire` |
| `net/netpath` | Pod 에서 목적지까지 요청을 보내고 `DNS → TCP → HTTP` 각 단계의 지연과 실패를 관측해, 어디서 왜 실패했는지 설명하는 도구. | `netpath-lab` |
| `net/paw` | 설명 없음 | `paw-lab` |
| `net/cntd-control-plane` | Kurose 『Computer Networking』 5장(제어 평면)의 정독 노트가 적은 것을 정말 그런지 확인합니다. | `cntd-control-plane-lab` |
| `net/npg` | 설명 없음 (이관 예정) | `npg-lab` · `npg-practice` |
| `k8s/k8s-in-action` | 《Kubernetes in Action, 2판》(Marko Lukša, Manning)을 따라가며 손으로 만든 실습 코드 저장소입니다. | `k8s_in_action` |
| `k8s/networking-and-kubernetes` | 노트 문서 `01-04.패킷 캡처 실습` 의 다섯 절을 이 맥북에서 실제로 돌리기 위한 폴더입니다. | `networking-and-kubernetes-lab` |
| `jvm/jvm-practice` | 《JVM 밑바닥까지 파헤치기》(저우즈밍) 챕터별 실습 모듈을 모은다. | `jvm-practice` |
| `arch/gateway` | HTTP 리버스 프록시에서 시작해 API 게이트웨이까지, raw Netty 위에 단계별로 짓는 학습 랩입니다. | `gateway-lab` |
| `arch/hub` | 두 외부 시스템 사이에서 요청을 옮기는 연동 허브를 단계별로 짓는 학습 랩입니다. | `hub-lab` |
| `ai/flats` | gosuda/flats(v0.9.1) 실습 저장소. | `flats-lab` |
| `ai/infra-ai` | 설명 없음 | `infra_ai` |

## 빌드 및 시험

```bash
bazel build //...
bazel test //...
```

## 규칙 및 가이드

저장소 구조 규칙, 도메인 축 배치 기준, 네이밍 및 빌드 컨벤션, 공개 저장소 준수 사항 등 자세한 규칙은 [AGENTS.md](AGENTS.md)를 참고한다.
