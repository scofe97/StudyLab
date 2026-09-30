# AGENTS.md

짧게, 행동 규칙만. 로드맵과 배경은 `docs/` 에 둔다.

## 개발 원칙

- 목표는 제품 완성이 아니라 네트워크 primitive 를 직접 구현하고 관측하는 것이다.
- 모든 Phase 는 구현 → 정상 동작 → 장애 주입 → 관측 → 원인 설명 → 문서화 순서를 밟는다.
- 추상화를 먼저 만들지 않는다. `TransportFactory`·`ConnectionManager` 같은 계층은 같은 변경 압력이 실제로 반복된 뒤에 뽑는다.
- 라이브러리가 대신해 주는 부분을 먼저 직접 짠다. reference 구현(frp·gost·quic-go·go-libp2p·cilium/ebpf)은 직접 짠 뒤 비교용으로 읽는다.
- 외부 의존은 표준 라이브러리로 안 되는 Phase(eBPF 등)에서만 들인다.

## 이 프로젝트의 규칙

- 네트워크 동작은 mock 으로 검증하지 않는다. 테스트는 loopback 실제 소켓을 쓴다.
- 테스트 본문은 `// Given`, `// When`, `// Then` 주석으로 나눈다.
- 모든 접속·goroutine 은 누가 닫는지(owner)가 코드에서 보여야 한다. 종료 경로가 없는 goroutine 을 만들지 않는다.
- 패키지 구조는 로드맵 `docs/01-01` 의 Repository Evolution 을 따라 필요할 때 늘린다.
- Linux 전용 관측은 `make build-linux` 후 OrbStack `ubuntu` 머신에서 한다 (`docs/02-01`).

## 문서 위치

- 로드맵(원본 전사): `docs/01-01.gonet-lab-roadmap.md`
- Phase 별 계획: `docs/03-NN.gonet-lab-phase<N>-plan.md`
- 학습 문서·상태: `~/runners-high/write/02_os/project/gonet-lab/` (이 저장소에 두지 않는다)

## 검증

- `make check` (vet + gofmt + `go test -race`) 를 커밋 전에 돌린다.
