// Package lab 은 Stage 0 실습 토폴로지를 세우고 허문다.
//
// 재현 대상은 손으로 지었던 그 구성이다.
//
//	netns ns1 ─ veth1 ┐
//	                  ├─ br0 (10.10.1.1/24) ─ MASQUERADE ─ eth0 ─ 바깥
//	netns ns2 ─ veth2 ┘
//
// 같은 결과가 나오는지 verify 로 대조하는 것이 Stage 0 의 완료 조건이다.
package lab

import (
	"errors"
	"fmt"
	"io"
)

var ErrTODO = errors.New("podwire: 구현 전")

// Config 는 실습 토폴로지의 모든 값을 한자리에 모은다.
// 하드코딩을 피해야 나중에 크로스호스트(ubuntu ↔ ubuntu2)로 늘릴 때 값만 바꾸면 된다.
type Config struct {
	Bridge     string   // 브리지 이름
	BridgeCIDR string   // 브리지에 붙일 주소, 게이트웨이 역할
	Namespaces []NSSpec // 만들 네임스페이스들
	Uplink     string   // MASQUERADE 를 걸 바깥 인터페이스 (예: eth0)
}

// NSSpec 은 네임스페이스 하나와 그 안에 놓일 veth 끝을 적는다.
type NSSpec struct {
	Name     string // netns 이름
	IfName   string // 네임스페이스 안쪽 장치 이름
	PeerName string // 호스트 쪽(브리지에 꽂힐) 장치 이름
	CIDR     string // 안쪽 장치에 붙일 주소
}

// Default 는 02-04 실습과 같은 값을 돌려준다.
func Default() Config {
	return Config{
		Bridge:     "br0",
		BridgeCIDR: "10.10.1.1/24",
		Uplink:     "eth0",
		Namespaces: []NSSpec{
			{Name: "ns1", IfName: "veth1", PeerName: "veth1-br", CIDR: "10.10.1.11/24"},
			{Name: "ns2", IfName: "veth2", PeerName: "veth2-br", CIDR: "10.10.1.12/24"},
		},
	}
}

// Up 은 토폴로지를 세운다.
//
// 순서가 중요하다. 브리지를 먼저 올리지 않고 veth 를 꽂으면 캐리어가 없어 DOWN 에 머문다.
//  1. 브리지 생성 + 주소 + UP
//  2. 네임스페이스마다: netns 생성 → veth 쌍 생성 → 한쪽을 netns 로 이동
//     → 호스트 쪽을 브리지에 부착 + UP → 안쪽에서 주소 부여 + UP + lo UP + 기본 경로
//  3. ip_forward 켜기 + Uplink 로 MASQUERADE
//
// TODO(Stage 0)
func Up(cfg Config) error { return ErrTODO }

// Down 은 Up 이 만든 것을 되돌린다. 이미 없는 것을 지우는 것은 오류가 아니다.
// TODO(Stage 0)
func Down(cfg Config) error { return ErrTODO }

// Verify 는 현재 상태를 읽어 w 에 적는다.
// 손 실습의 verify.sh 출력과 눈으로 대조할 수 있는 형태여야 한다.
//
// 최소한 이 넷을 확인한다.
//   - 기본 네임스페이스에서 호스트 쪽 veth 끝만 보이고 안쪽 끝은 안 보인다
//   - veth1@ifN ↔ veth1-br@ifM 상호 참조 인덱스가 맞는다
//   - ns1 → ns2 ping 이 통하고 TTL 이 64 그대로다 (브리지는 홉이 아니다)
//   - 바깥으로 나가면 TTL 이 하나 줄고 conntrack 에 MASQUERADE 튜플이 생긴다
//
// TODO(Stage 0)
func Verify(cfg Config, w io.Writer) error {
	fmt.Fprintln(w, "TODO(Stage 0): verify 미구현")
	return ErrTODO
}
