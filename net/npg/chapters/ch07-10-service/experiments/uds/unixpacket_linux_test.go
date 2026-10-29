//go:build linux

package uds

import (
	"bytes"
	"net"
	"path/filepath"
	"testing"
)

// TestUnixpacketSmallBuffer 는 64 바이트 메시지 두 개를 보내고 16 바이트 버퍼로 두 번 읽는다.
// unixpacket(연결형)과 unixgram(비연결형)을 같은 조건으로 돌린다.
func TestUnixpacketSmallBuffer(t *testing.T) {
	msgA := bytes.Repeat([]byte("A"), 64)
	msgB := bytes.Repeat([]byte("B"), 64)

	t.Run("unixpacket", func(t *testing.T) {
		// Given — unixpacket 리스너와 연결 하나
		path := filepath.Join(shortDir(t), "p.sock")
		l, err := net.Listen("unixpacket", path)
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		accepted := make(chan net.Conn, 1)
		go func() {
			c, _ := l.Accept()
			accepted <- c
		}()
		cli, err := net.Dial("unixpacket", path)
		if err != nil {
			t.Fatal(err)
		}
		defer cli.Close()
		srv := <-accepted
		defer srv.Close()

		// When — 두 메시지를 보내고 16 바이트 버퍼로 두 번 읽는다
		cli.Write(msgA)
		cli.Write(msgB)
		buf := make([]byte, 16)
		for i := 1; i <= 2; i++ {
			n, err := srv.Read(buf)
			// Then — 읽힌 바이트와 에러를 남긴다
			t.Logf("Read %d: n=%d data=%q err=%v", i, n, buf[:n], err)
		}
	})

	t.Run("unixgram", func(t *testing.T) {
		// Given — unixgram 소켓 두 개
		dir := shortDir(t)
		srv, err := net.ListenPacket("unixgram", filepath.Join(dir, "g.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer srv.Close()
		cli, err := net.Dial("unixgram", filepath.Join(dir, "g.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer cli.Close()

		// When — 두 메시지를 보내고 16 바이트 버퍼로 두 번 읽는다
		cli.Write(msgA)
		cli.Write(msgB)
		buf := make([]byte, 16)
		for i := 1; i <= 2; i++ {
			n, _, err := srv.ReadFrom(buf)
			// Then — 읽힌 바이트와 에러를 남긴다
			t.Logf("ReadFrom %d: n=%d data=%q err=%v", i, n, buf[:n], err)
		}
	})
}
