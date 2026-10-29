package uds

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "npguds-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

// TestStreamThreeWrites 는 unix 스트림 소켓에 "ping" 을 세 번 쓰고, 받는 쪽 Read 가 몇 바이트씩 돌려주는지 찍는다.
// 쓰기 사이 간격을 0 과 10ms 두 가지로 둔다. 받는 쪽은 쓰기가 다 끝난 뒤 읽기 시작한다.
func TestStreamThreeWrites(t *testing.T) {
	for _, gap := range []time.Duration{0, 10 * time.Millisecond} {
		// Given — unix 스트림 리스너와 연결 하나
		path := filepath.Join(shortDir(t), "s.sock")
		l, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		accepted := make(chan net.Conn, 1)
		go func() {
			c, _ := l.Accept()
			accepted <- c
		}()
		cli, err := net.Dial("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		srv := <-accepted

		// When — 클라이언트가 세 번 쓰고 닫은 뒤 서버가 1024 바이트 버퍼로 읽는다
		for i := 0; i < 3; i++ {
			cli.Write([]byte("ping"))
			time.Sleep(gap)
		}
		cli.Close()
		buf := make([]byte, 1024)
		var reads []int
		for {
			n, err := srv.Read(buf)
			if n > 0 {
				reads = append(reads, n)
			}
			if err != nil {
				break
			}
		}

		// Then — Read 마다 받은 바이트 수를 남긴다
		t.Logf("gap=%-5s Read 결과 바이트 수 = %v", gap, reads)
		srv.Close()
		l.Close()
	}
}
