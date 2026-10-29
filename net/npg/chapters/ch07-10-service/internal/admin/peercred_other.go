//go:build !linux

package admin

import "net"

const peerCredSupported = false

// readPeerCred 는 Linux 가 아니면 자격 증명을 읽지 않는다.
// macOS 에도 LOCAL_PEERCRED 가 있지만 표준 라이브러리 syscall 에는 그 호출이 없다.
func readPeerCred(_ *net.UnixConn, _ string) (*Cred, error) {
	return nil, errUnsupported
}
