// Package link 는 veth·bridge 같은 네트워크 장치를 다룬다.
//
// ip link / ip addr / ip route 로 손으로 치던 것을 netlink 소켓 호출로 옮긴 자리다.
// 구현은 vishvananda/netlink 를 쓸 예정이고, 그 전까지는 인터페이스만 세워 둔다.
package link

import (
	"errors"

	"github.com/scofe97/runners-lab/net/podwire/internal/netns"
)

var ErrTODO = errors.New("podwire: 구현 전")

// Veth 는 veth 쌍 하나다. 두 끝은 각각 다른 네임스페이스에 놓일 수 있다.
type Veth struct {
	Name     string // 이쪽 끝 이름
	PeerName string // 반대쪽 끝 이름
	MTU      int    // 0 이면 기본값(1500)
}

// CreateVeth 는 veth 쌍을 만든다. 만든 직후 두 끝은 모두 현재 네임스페이스에 있고 DOWN 이다.
// TODO(Stage 0)
func CreateVeth(v Veth) error { return ErrTODO }

// CreateBridge 는 리눅스 브리지를 만든다.
// TODO(Stage 0)
func CreateBridge(name string) error { return ErrTODO }

// AttachToBridge 는 장치를 브리지에 꽂는다 (ip link set X master br0).
// TODO(Stage 0)
func AttachToBridge(name, bridge string) error { return ErrTODO }

// MoveToNetns 는 장치를 다른 네임스페이스로 옮긴다 (ip link set X netns Y).
// 옮기면 이름은 유지되지만 인덱스와 상태(UP/DOWN)는 그 네임스페이스 기준으로 다시 본다.
// TODO(Stage 0)
func MoveToNetns(name string, h netns.Handle) error { return ErrTODO }

// SetUp 은 장치를 UP 으로 올린다.
// TODO(Stage 0)
func SetUp(name string) error { return ErrTODO }

// AddAddr 은 CIDR 표기 주소를 장치에 붙인다 (예: "10.10.1.1/24").
// TODO(Stage 0)
func AddAddr(name, cidr string) error { return ErrTODO }

// AddDefaultRoute 는 기본 경로를 넣는다.
// 빈 네임스페이스에 없는 것이 바로 이 기본 경로다.
// TODO(Stage 0)
func AddDefaultRoute(gw string) error { return ErrTODO }

// AddRoute 는 목적지 대역을 특정 장치·게이트웨이로 보낸다.
// Stage 1 의 노드 간 직접 라우팅이 이걸 노드 수만큼 넣는 일이다.
// TODO(Stage 1)
func AddRoute(dstCIDR, gw, dev string) error { return ErrTODO }

// Delete 는 장치를 지운다. veth 는 한쪽만 지워도 쌍이 함께 사라진다.
// TODO(Stage 0)
func Delete(name string) error { return ErrTODO }
