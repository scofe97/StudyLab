// Package netns 는 네트워크 네임스페이스를 오가는 최소 도구를 담는다.
//
// 여기서 다루는 함정 하나가 이 프로젝트 Stage 0 의 목적이다.
// setns(2) 는 프로세스가 아니라 "호출한 OS 스레드"의 네임스페이스를 바꾼다.
// Go 런타임은 고루틴을 스레드 사이로 자유롭게 옮기므로,
// runtime.LockOSThread 로 고정하지 않으면 setns 직후 이어지는 코드가
// 엉뚱한 네임스페이스에서 실행될 수 있다. 그러면 veth 가 호스트에 생기거나
// 주소가 엉뚱한 곳에 붙는데, 재현이 들쭉날쭉해서 원인을 찾기 어렵다.
package netns

import "errors"

// ErrTODO 는 아직 구현하지 않은 자리를 표시한다.
var ErrTODO = errors.New("podwire: 구현 전")

// Handle 은 네트워크 네임스페이스 하나를 가리킨다.
// Path 는 보통 /var/run/netns/<이름> 이거나 /proc/<pid>/ns/net 이다.
type Handle struct {
	Path string
}

// FromPath 는 경로로 핸들을 만든다.
func FromPath(path string) Handle { return Handle{Path: path} }

// Do 는 fn 을 h 네임스페이스 안에서 실행하고, 끝나면 원래 네임스페이스로 돌아온다.
//
// 구현 순서:
//  1. runtime.LockOSThread() — 반드시 먼저. defer UnlockOSThread 는 복귀 실패 시 스레드를 버려야 하므로 조건부다
//  2. 현재 네임스페이스 fd 를 열어 둔다 (/proc/self/ns/net)
//  3. h.Path 를 열어 setns
//  4. fn() 실행
//  5. 저장해 둔 fd 로 setns 해 복귀
//
// TODO(Stage 0): vishvananda/netns 를 쓰거나 직접 unix.Setns 로 구현한다.
func (h Handle) Do(fn func() error) error {
	return ErrTODO
}

// Create 는 이름 있는 네임스페이스를 만든다 (ip netns add 와 같은 일).
// TODO(Stage 0)
func Create(name string) (Handle, error) {
	return Handle{}, ErrTODO
}

// Delete 는 이름 있는 네임스페이스를 지운다.
// TODO(Stage 0)
func Delete(name string) error {
	return ErrTODO
}
