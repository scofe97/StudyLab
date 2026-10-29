//go:build linux

package admin

import (
	"net"
	"syscall"
)

const peerCredSupported = true

// readPeerCred 는 연결된 상대 프로세스의 자격 증명을 SO_PEERCRED 로 읽는다.
//
//	rawconn   — SyscallConn().Control 안에서 원래 fd 로 getsockopt 를 부른다.
//	file      — File() 로 fd 를 복제해 getsockopt 를 부르고 복제본을 닫는다.
//	file-noclose — File() 로 fd 를 복제해 getsockopt 를 부르고 복제본을 닫지 않는다.
func readPeerCred(c *net.UnixConn, method string) (*Cred, error) {
	var uc *syscall.Ucred
	var err error
	switch method {
	case "file", "file-noclose":
		f, ferr := c.File()
		if ferr != nil {
			return nil, ferr
		}
		if method == "file" {
			defer f.Close()
		}
		uc, err = syscall.GetsockoptUcred(int(f.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	default:
		raw, rerr := c.SyscallConn()
		if rerr != nil {
			return nil, rerr
		}
		cerr := raw.Control(func(fd uintptr) {
			uc, err = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		})
		if cerr != nil {
			return nil, cerr
		}
	}
	if err != nil {
		return nil, err
	}
	return &Cred{PID: uc.Pid, UID: uc.Uid, GID: uc.Gid}, nil
}
