package tcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startServer 는 loopback 의 빈 포트에서 idle timeout 을 건 Serve 를 띄우고 주소와 종료 함수를 돌려준다.
func startServer(t *testing.T, idle time.Duration) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, idle, log.New(io.Discard, "", 0)) }()
	t.Cleanup(cancel)
	return ln.Addr().String(), cancel, done
}

func TestConnectEchoesInput(t *testing.T) {
	// Given — echo 서버와 보낼 입력
	addr, _, _ := startServer(t, time.Minute)
	in := strings.NewReader("hello\nworld\n")
	var out bytes.Buffer

	// When — 입력을 보내고 half-close 후 서버가 닫을 때까지 읽는다
	err := Connect(context.Background(), addr, in, &out)

	// Then — 보낸 바이트가 그대로 돌아온다
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got := out.String(); got != "hello\nworld\n" {
		t.Fatalf("echo = %q", got)
	}
}

func TestConnectReturnsWhenServerClosesFirst(t *testing.T) {
	// Given — 접속을 받자마자 닫는 서버와, 아무것도 보내지 않는 stdin
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			c.Close()
		}
	}()
	stdin, stdinW := io.Pipe()
	defer stdinW.Close()
	var out bytes.Buffer

	// When — 입력 없이 접속한 채로 서버가 먼저 닫는다
	done := make(chan error, 1)
	go func() { done <- Connect(context.Background(), ln.Addr().String(), stdin, &out) }()

	// Then — stdin 이 조용해도 Connect 가 서버 종료를 보고 돌아온다 (CLOSE-WAIT 로 남지 않는다)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Connect: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("서버가 닫았는데 Connect 가 stdin 을 기다리며 돌아오지 않음")
	}
}

func TestServeStopsAcceptingOnCancel(t *testing.T) {
	// Given — 떠 있는 서버
	addr, cancel, done := startServer(t, time.Minute)

	// When — 종료 신호
	cancel()

	// Then — Serve 가 nil 로 끝나고 같은 주소로 더는 접속되지 않는다
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve 가 종료되지 않음")
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		c.Close()
		t.Fatal("종료 후에도 접속됨")
	}
}

func TestServeClosesActiveConnOnCancel(t *testing.T) {
	// Given — 접속만 하고 아무것도 보내지 않는 클라이언트
	addr, cancel, done := startServer(t, time.Minute)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// accept 가 끝나 핸들러가 등록될 시간을 준다
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}

	// When — 종료 신호
	cancel()

	// Then — 서버가 열린 접속을 닫아 클라이언트 Read 가 끝나고, Serve 도 반환한다
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("서버 종료 후에도 Read 가 데이터를 받음")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve 가 열린 접속 때문에 종료되지 않음")
	}
}

func TestServeClosesIdleConn(t *testing.T) {
	// Given — idle 200ms 서버와, 접속만 하고 아무것도 보내지 않는 클라이언트
	const idle = 200 * time.Millisecond
	addr, _, _ := startServer(t, idle)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	start := time.Now()

	// When — 클라이언트가 조용히 기다린다
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Read(make([]byte, 1))

	// Then — idle 이 지난 뒤 서버가 닫아 EOF 를 받는다
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Read err = %v, want EOF (서버가 idle 연결을 닫지 않음)", err)
	}
	if elapsed := time.Since(start); elapsed < idle {
		t.Fatalf("idle(%s) 전에 끊김: %s", idle, elapsed)
	}
}

func TestServeKeepsActiveConnPastIdle(t *testing.T) {
	// Given — idle 300ms 서버
	const idle = 300 * time.Millisecond
	addr, _, _ := startServer(t, idle)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	buf := make([]byte, 1)

	// When — idle 보다 짧은 간격(100ms)으로 6 번, 합쳐서 idle 의 두 배 동안 보낸다
	for i := 0; i < 6; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := conn.Write([]byte("x")); err != nil {
			t.Fatalf("%d 번째 Write: %v", i+1, err)
		}
		conn.SetReadDeadline(time.Now().Add(time.Second))

		// Then — 매번 echo 가 돌아온다 (받을 때마다 deadline 이 밀려 끊기지 않는다)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("%d 번째 echo: %v (idle 보다 오래 붙어 있다고 끊김)", i+1, err)
		}
	}
}

// flakyListener 는 실제 loopback listener 앞에서 처음 fails 번의 Accept 만 EMFILE 로 실패시킨다.
// 소켓은 진짜이고, FD 고갈만 흉내 낸다(테스트 프로세스 전체의 FD 한도를 낮추지 않으려고).
type flakyListener struct {
	net.Listener
	fails int
}

func (l *flakyListener) Accept() (net.Conn, error) {
	if l.fails > 0 {
		l.fails--
		return nil, syscall.EMFILE
	}
	return l.Listener.Accept()
}

func TestServeRetriesTemporaryAcceptError(t *testing.T) {
	// Given — 처음 세 번의 Accept 가 EMFILE 로 실패하는 서버
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, &flakyListener{Listener: ln, fails: 3}, time.Minute, log.New(io.Discard, "", 0))
	}()

	// When — 클라이언트가 접속해 한 바이트를 보낸다
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = io.ReadFull(conn, buf)

	// Then — 실패 뒤에도 Serve 가 끝나지 않고 다시 받아 echo 가 돌아온다
	if err != nil {
		t.Fatalf("echo: %v (Accept 실패 뒤 다시 받지 않음)", err)
	}
	select {
	case err := <-done:
		t.Fatalf("일시적 Accept 실패로 Serve 가 끝남: %v", err)
	default:
	}
}
