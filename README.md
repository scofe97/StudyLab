# runners-lab

`runners-lab`은 개념 학습(`runners-high`)을 거쳐 구현과 관측으로 직접 검증하는 Phase 3 실습 코드들을 도메인 계층별로 통합 관리하는 공개 모노레포입니다. Bazel(bzlmod) 기반 빌드 시스템을 채택하여 패키지 간의 명확한 경계, 세밀한 캐싱, 변경 영향에 따른 선별적 테스트 환경을 제공합니다.

## 실습 목록 (Labs)

| 경로 | 한 줄 설명 | 원래 이름 |
|---|---|---|
| `net/podwire` | 기본 CNI 를 끈 kind 클러스터에 podwire 를 깔면 CoreDNS 를 포함한 모든 Pod 가 뜹니다. 그리고 모든 패킷 결정이 어느 층에서 왜 내려졌는지 flow 로그로 남습니다. | `podwire` |
| `net/paw` | 설명 없음 | `paw-lab` |
| `net/cntd-control-plane` | Kurose 『Computer Networking』 5장(제어 평면)의 정독 노트가 적은 것을 정말 그런지 확인합니다. | `cntd-control-plane-lab` |
| `net/npg` | 『Network Programming with Go』 3~12장을 장비 프로비저닝 서비스 한 벌(제어 채널·UDP 공지·TFTP·리버스 프록시)로 키우는 실습입니다. 장별 실습은 `chapters/` 에 있습니다. | `npg-lab` · `npg-practice` |
| `net/learning-coredns` | 『Learning CoreDNS』 1~4장을 사내 DNS 장애 일곱 개(존 파일 갱신·캐시·새 레코드 누락·서버 블록·상위 DNS·auto·hosts)로 재현하고 `dig` 로 원인을 찾는 실습입니다. | `learning-coredns-lab` |
| `net/ssh-bbs` | `ssh 닉@호스트` 로 어디서든 들어오는 레트로 BBS(채팅방·게시판·자료실·도어 게임)를 Go 로 만들며 SSH·터미널·포트 포워딩·노드 간 복제를 배우는 실습입니다. 지금은 로드맵만 있습니다. | (새 lab) |
| `k8s/k8s-in-action` | 《Kubernetes in Action, 2판》(Marko Lukša, Manning)을 따라가며 손으로 만든 실습 코드 저장소입니다. | `k8s_in_action` |
| `k8s/networking-and-kubernetes` | 노트 문서 `01-04.패킷 캡처 실습` 의 다섯 절을 이 맥북에서 실제로 돌리기 위한 폴더입니다. | `networking-and-kubernetes-lab` |
| `jvm/jvm-practice` | 《JVM 밑바닥까지 파헤치기》(저우즈밍) 챕터별 실습 모듈을 모은다. | `jvm-practice` |
| `jvm/thread-per-request` | 요청당 스레드(Spring 플랫폼·가상 스레드)와 Go 를 같은 부하(작은 요청 대량·느린 요청·큰 응답)로 재어 비교합니다. | `thread-per-request` |
| `arch/gateway` | HTTP 리버스 프록시에서 시작해 API 게이트웨이까지, raw Netty 위에 단계별로 짓는 학습 랩입니다. | `gateway-lab` |
| `arch/hub` | 두 외부 시스템 사이에서 요청을 옮기는 연동 허브를 단계별로 짓는 학습 랩입니다. | `hub-lab` |
| `ai/flats` | gosuda/flats(v0.9.1) 실습 저장소. | `flats-lab` |

## 빌드 및 시험

```bash
bazel build //...
bazel test //...
```

## 규칙 및 가이드

저장소 구조 규칙, 도메인 축 배치 기준, 네이밍 및 빌드 컨벤션, 공개 저장소 준수 사항 등 자세한 규칙은 [AGENTS.md](AGENTS.md)를 참고하시기 바랍니다.
