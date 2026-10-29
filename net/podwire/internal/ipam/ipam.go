// Package ipam 은 Pod 에 줄 IP 를 노드 안에서 배분한다.
//
// CNI 는 Pod 마다 프로세스로 새로 실행된다. 같은 노드에서 두 Pod 이 동시에 뜨면
// 두 프로세스가 같은 파일을 동시에 읽고 같은 IP 를 내줄 수 있다.
// 그래서 저장소는 파일 잠금이 필요하다 — 이 패키지의 핵심 난이도는 주소 계산이 아니라 그 잠금이다.
package ipam

import (
	"errors"
	"net"
)

var (
	ErrTODO      = errors.New("podwire: 구현 전")
	ErrExhausted = errors.New("podwire: 대역에 남은 주소 없음")
)

// Allocator 는 컨테이너 하나에 주소 하나를 대응시킨다.
// 같은 containerID 로 두 번 Allocate 하면 같은 주소가 나와야 한다(멱등).
// kubelet 이 실패한 ADD 를 재시도할 때 주소가 새로 새어 나가면 안 되기 때문이다.
type Allocator interface {
	Allocate(containerID string) (net.IP, error)
	Release(containerID string) error
	Gateway() net.IP
	Subnet() *net.IPNet
}

// FileStore 는 Dir 아래 파일 하나를 IP 하나로 삼는 가장 단순한 구현이다.
// 파일 이름이 IP, 내용이 containerID 다. host-local 플러그인이 쓰는 방식과 같다.
type FileStore struct {
	Dir    string     // 예: /var/lib/cni/networks/podwire
	CIDR   *net.IPNet // 이 노드에 할당된 podCIDR
	GwAddr net.IP     // 보통 대역의 첫 주소
}

// NewFileStore 는 podCIDR 문자열로 저장소를 만든다.
// TODO(Stage 1)
func NewFileStore(dir, podCIDR string) (*FileStore, error) { return nil, ErrTODO }

// Allocate 는 비어 있는 첫 주소를 잡아 준다.
// 잠금 → 디렉토리 스캔 → 빈 주소 찾기 → 파일 생성 → 잠금 해제 순서다.
// TODO(Stage 1)
func (s *FileStore) Allocate(containerID string) (net.IP, error) { return nil, ErrTODO }

// Release 는 주소를 반납한다. 없는 것을 반납하는 것은 오류가 아니다.
// TODO(Stage 1)
func (s *FileStore) Release(containerID string) error { return ErrTODO }

// Gateway 는 이 대역의 게이트웨이 주소다.
func (s *FileStore) Gateway() net.IP { return s.GwAddr }

// Subnet 은 이 노드가 맡은 대역이다.
func (s *FileStore) Subnet() *net.IPNet { return s.CIDR }
