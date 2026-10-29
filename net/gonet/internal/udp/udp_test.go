package udp

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"
)

// startServer 는 loopback 의 빈 포트에서 bufSize 버퍼로 Serve 를 띄우고 주소와 종료 채널을 돌려준다.
func startServer(t *testing.T, bufSize int) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, pc, bufSize, log.New(io.Discard, "", 0)) }()
	t.Cleanup(cancel)
	return pc.LocalAddr().String(), cancel, done
}

// readEchoes 는 conn 에서 데이터그램 want 개를 읽어 돌려준다.
func readEchoes(t *testing.T, conn net.Conn, want int) []string {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64*1024)
	var got []string
	for len(got) < want {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("%d 번째 echo: %v", len(got)+1, err)
		}
		got = append(got, string(buf[:n]))
	}
	return got
}

func TestServeKeepsDatagramBoundaries(t *testing.T) {
	// Given — 넉넉한 버퍼의 서버와 연달아 두 번 쓰는 클라이언트
	addr, _, _ := startServer(t, 64*1024)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — hello, world 를 데이터그램 둘로 보낸다
	conn.Write([]byte("hello"))
	conn.Write([]byte("world"))

	// Then — echo 도 이어 붙지 않고 5 바이트씩 둘로 돌아온다
	got := readEchoes(t, conn, 2)
	if strings.Join(got, "|") != "hello|world" {
		t.Fatalf("echo = %q, want [hello world]", got)
	}
}

func TestServeTruncatesWithoutError(t *testing.T) {
	// Given — 버퍼가 3 바이트인 서버
	addr, _, done := startServer(t, 3)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — 5 바이트 데이터그램 둘을 보낸다
	conn.Write([]byte("hello"))
	conn.Write([]byte("world"))

	// Then — 각 데이터그램의 앞 3 바이트만 돌아오고(남은 lo 는 다음 읽기로 넘어가지 않는다) Serve 는 에러 없이 계속 돈다
	got := readEchoes(t, conn, 2)
	if strings.Join(got, "|") != "hel|wor" {
		t.Fatalf("echo = %q, want [hel wor]", got)
	}
	select {
	case err := <-done:
		t.Fatalf("잘린 데이터그램 때문에 Serve 가 끝남: %v", err)
	default:
	}
}

func TestServeStopsOnCancel(t *testing.T) {
	// Given — 떠 있는 서버
	_, cancel, done := startServer(t, 64*1024)

	// When — 종료 신호
	cancel()

	// Then — blocking ReadFrom 이 깨어나 Serve 가 nil 로 끝난다
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve 가 종료되지 않음")
	}
}

func TestSendWritesEchoes(t *testing.T) {
	// Given — echo 서버
	addr, _, _ := startServer(t, 64*1024)
	var out bytes.Buffer

	// When — 두 메시지를 보내고 잠시 echo 를 기다린다
	err := Send(context.Background(), addr, []string{"hello", "world"}, 300*time.Millisecond, &out)

	// Then — 받은 echo 가 한 줄씩 찍힌다
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	for _, want := range []string{`echo r=5 data="hello"`, `echo r=5 data="world"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("출력에 %q 없음:\n%s", want, out.String())
		}
	}
}
