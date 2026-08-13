# 11-02 문서 보강 후보 (BOOSTER-NOTES)

> 2026-07-20 학습 세션(11-02 외부 노출)에서 수집. 대상 노트:
> `write/08_cloud/book/kubernetes-in-action/11-02.외부 노출 — NodePort·LoadBalancer·트래픽 정책.md`
> 이 파일은 *후보 수집*만. 본문 반영은 별도 세션에서 승인 후(학습-문서 본문 불변 원칙).

---

## ⚠️ 선결 판단 — 새 networking 문서는 쓰지 않는다

사용자가 "SNAT/DNAT/NAT 리눅스 네트워크 보강, 분량이면 `02_os/networking`에 새 문서" 요청.
**결론: 새 문서 작성 안 함.** 이유 (MEMORY "신규 편 작성 전 기존 문서 grep" + "기존 SSOT 카테고리엔 위임·교차참조"):

- `02_os/networking/01-02.K8s 패킷 여정 — netfilter·conntrack·라우팅.md`(23.2K)가 이미 SNAT/DNAT를 깊게 다룸:
  - §1.2 netfilter 5훅 (PREROUTING=DNAT, POSTROUTING=SNAT) + "DNAT은 PREROUTING, SNAT은 POSTROUTING에 자리잡는 이유"
  - §2.2 kube-proxy의 정체 / §2.3 5테이블 + `KUBE-SERVICES→KUBE-SVC-*→KUBE-SEP-*` 체인 점프
  - §3 conntrack 양방향 장부 / §5 CNI 실습
- grep 확인: `externalTrafficPolicy` / Local 정책 / masquerade / healthCheckNodePort 는 01-02에 **없음** → 그것만 갭.

→ **조치: (a) 11-01·11-02에서 `01-02`로 교차 링크, (b) externalTrafficPolicy(Cluster/Local, SNAT, healthCheckNodePort)만 11-02 §3에 보강.**
   `01-02`(Service 레벨 정책은 원래 그 문서 범위 밖)에는 손대지 않음.

---

## 후보 ① §1 NodePort 6포트 흐름 시각화 + kube-proxy 규칙

- **무엇**: `외부 → 노드IP:30080 → (iptables NodePort 규칙) → ClusterIP:80 → (DNAT) → 파드IP:8080` 흐름도 SVG.
- **왜**: Q1/Q3에서 6포트는 자답했으나, "모든 노드에 30080이 열리는 실제 기전 = kube-proxy가 각 노드 iptables에 NodePort 규칙을 심음 → 파드 없는 노드도 DNAT"은 1단계 보정 필요했음.
- **연결**: 흐름도에서 `KUBE-NODEPORTS → KUBE-SVC-* → KUBE-SEP-*` 체인은 `01-02` §2.3으로 교차 링크(중복 서술 금지, 링크만).

## 후보 ② §2 LoadBalancer = NodePort 확장 계층도 + 프로바이더/MetalLB

- **무엇**: `LoadBalancer(공용 IP) └ NodePort(각 노드 30000~32767) └ ClusterIP └ 파드` 3층 계층 SVG.
- **왜**: Q4에서 "LB는 NodePort 앞에 얹은 분배 계층 → 대체가 아니라 감쌈 → 그래서 확장"이 명확화됨. `kubectl get svc`에서 LB 타입도 `80:31234`처럼 nodePort가 찍힌다는 관찰 포인트.
- **추가**: `<pending>` = 클라우드 프로바이더 컨트롤러가 없어서(요청만 하고 채워줄 주체 없음), 온프렘은 MetalLB가 그 대역. 실습 kind에서 `<pending>` 재현 캡처.

## 후보 ③ §3 externalTrafficPolicy — Cluster vs Local (⭐ 유일한 실제 갭, 우선순위 최상)

- **무엇**: 표 + 흐름도. Cluster(DNAT+SNAT, Client IP=노드 IP, 파드 없는 노드도 홉 1회로 전달) vs Local(DNAT만, 원본 IP 보존, 파드 없는 노드 타임아웃).
- **왜**: Q2에서 목적지(DNAT)/출발지(SNAT) 혼동, Q5(불균등 분산 + healthCheckNodePort)는 설명 필요했던 유일 막힘. `_mistakes.md` 2026-07-20 첫 항목과 짝.
- **핵심 서술**:
  - SNAT(masquerade)이 왜 필요한가 = 비대칭 경로 방지(응답이 받은 노드로 되돌아오게 출발지 위조) → 대가로 Client IP 손실.
  - Local 불균등 = LB는 **노드 단위** 균등 분배, 파드는 노드마다 수가 달라 파드당 부하 어긋남(A:3파드→11%, B:1파드→33%).
  - healthCheckNodePort = 로컬 파드 있으면 200/없으면 503 → LB가 파드 없는 노드를 분배에서 제외 → 타임아웃 우회.
  - Local을 쓰는 이유 = 원본 Client IP 필요(감사 로깅·geo-blocking·IP allowlist·rate limiting). Cluster는 IP가 뭉개져 못 함.
- **교차 링크**: SNAT/masquerade 기전 뿌리는 `01-02` §1.2·§2로 링크. 11-02에는 "Service 정책 관점"만.

## 후보 ④ 11-01·11-02 → `02_os/networking/01-02` 교차 링크 삽입

- **무엇**: 11-01 §1/§5(NAT·DNAT·kube-proxy), 11-02 §3(SNAT)에서 `01-02.K8s 패킷 여정`으로 "리눅스 netfilter/iptables 관점 상세" 링크 한 줄씩.
- **왜**: 사용자 3세션 공통 약점 "자동으로 채워지는 것의 내부 기전"의 뿌리 문서가 01-02. 학습 순서상 01-02를 다음에 읽으면 지금 막힌 SNAT/DNAT/kube-proxy가 밑단까지 이어짐.
- **양방향 권장**: `01-02`에도 "이 netfilter 흐름이 Service에서 어떻게 쓰이나 → 11-01/11-02" 역링크 한 줄(01-02 본문은 사용자 승인 후에만).
