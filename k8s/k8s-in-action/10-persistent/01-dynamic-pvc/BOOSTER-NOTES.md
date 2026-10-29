# 10-01 보강 메모 (학습 세션 중 수집 — 다른 세션에서 문서 반영)

> 2026-07-15~16 학습 4-Phase 재현 세션 + grill-me 추궁으로 확정한 보강 5건.
> 대상 문서: write/08_cloud/book/kubernetes-in-action/10-01.PV·PVC·StorageClass와 동적 프로비저닝.md
> 문서 본문 사실은 바꾸지 말고 보강만 (learning-doc body immutable). 저장 전 위치·섹션·시각화 방식 요약해 승인받을 것.

## 1. StorageClass 심화 — 실전 예시 다양화 (§7)

현재 §7은 GKE `standard-rwo` YAML 1개 + provisioner/parameters/volumeBindingMode 설명뿐. 보강:
- **여러 클래스를 나란히**: `fast`(SSD)/`slow`(HDD)처럼 같은 provisioner라도 `parameters`가 다르면 다른 종류 스토리지가 됨을 예시로. "StorageClass = 스토리지 종류를 고르는 다리"가 손에 잡히게.
- **kind 실측 대조**: 우리 kind `standard`(provisioner `rancher.io/local-path`, WaitForFirstConsumer) YAML을 GKE 예시와 나란히. `kubectl get sc -o yaml`로 실제 조회한 것 대조.
- 근거: Phase 2에서 사용자가 StorageClass를 RWO(access mode)와 혼동. 원인은 추상 개념이 아니라 구체 예시 부족 → 예시로 메움.

## 2. access mode 시각화 — 통합 비교도 + RWX·ROX 상세 (§3, §6)

Claude·GPT(Codex) 합의안. 현재 4행 표 + multi-pod-attach.svg(RWO 멀티노드 실패)만 있음.
- **통합 비교도 1개 (신규)**: 2×2 축 — (파드 제한 vs 노드 제한) × (단일 vs 다중). RWOP=파드1 / RWO=노드1 / RWX=여러노드 rw / ROX=여러노드 ro를 한눈에.
- **RWX·ROX 상세 도식 (신규)**: 사용자가 특히 원한 부분.
  - RWX: 여러 노드가 동시에 read-write. NFS 등 파일 스토리지 필요(블록 스토리지 대부분 불가).
  - ROX: 여러 노드 read-only. **빈 PV는 read-only로 못 쓰니** `dataSourceRef`로 기존 PVC를 소스 삼아 PV를 초기화(복제)한 뒤 마운트 — 이 메커니즘까지 도식에 포함해야 "왜 ROX가 특별한지" 완성. (§6 dataSourceRef 예시 활용)
- 기존 multi-pod-attach.svg(RWO Once=노드 함정)는 유지 — 핵심 함정 이미 커버.

## 3. CSI 드라이버 — 2컴포넌트 구조 도식 (§7 끝)

현재 문단만 있음(out-of-tree, 2컴포넌트, get csidrivers). 보강:
- **2컴포넌트 협업 도식 (신규)**: 컨트롤러(PV 동적 생성 담당) + 노드 레벨 에이전트(볼륨을 노드에 attach·mount)가 어떻게 나뉘어 협업하는지. PVC → 프로비저너(컨트롤러) → PV 생성 → 노드 에이전트가 그 노드에 attach 흐름.
- 근거: "왜 노드마다 CSI 에이전트가 있나"가 Phase 1 축6(WaitForFirstConsumer의 "노드에 붙는다")과 직결. 도식으로 역할 분담을 시각화.

## 4. PV 생명주기 상태 — Mermaid stateDiagram 신규 (§5)

현재 reclaim policy 표 + dynamic-provisioning-lifecycle.svg(생성 흐름)만 있어 Released·Failed 상태가 안 드러남.
- **Mermaid stateDiagram (신규)**: PV 4상태 전이.
  - `Available`(비어있음, 청구 대기) → `Bound`(PVC 묶임) → `Released`(PVC 삭제됨, 회수 전) → reclaim policy 분기: **Delete**면 PV·데이터 삭제 / **Retain**이면 보존(운영자 수동 회수) → 실패 시 `Failed`.
  - Phase 3에서 직접 본 `Bound→Released→삭제`를 그대로 담아 "PVC 지우면 왜 바로 안 사라지고 Released 거치나" 명확히.
- 도구: Mermaid stateDiagram (상태 전이는 SVG보다 자동 배치가 적합).

## 5. SVG 글자 잘림 점검 게이트 (전체)

- **확인된 잘림 후보**: `_assets/10-01-pv-pvc-storageclass.svg` — `"PV 가 실제 스토리지 대표"`(x=700, 한글 12자 우측 여백 빠듯), `Network storage`(rect width=180 중앙정렬) 등.
- **게이트**: 10-01의 **모든 SVG(기존 3개 + 이번 신규)** 에 오버플로 근사(한글 1.0·라틴 0.58·모노 0.62 × font-size) + **rsvg PNG 렌더 후 눈 확인** 적용. 사용자 룰 "SVG 가독성은 렌더+눈 확인 필수" 준수.
- 기존 3개: 10-01-pv-pvc-storageclass / 10-01-dynamic-provisioning-lifecycle / 10-01-multi-pod-attach.

## 산출 규약

- writing 스킬 로드, 06-second-brain-harness 진입 체크리스트 + 8부 grep 검증(어미·강조어·Mermaid·줄수) 적용.
- SVG 색상은 dev-standards 규칙(밝은 fill+어두운 글자 / 다크배경+밝은 글자, 중간밝기 판단은 렌더 확인).
- 문서 본문 사실 불변, 보강만. 저장 전 요약 승인.
