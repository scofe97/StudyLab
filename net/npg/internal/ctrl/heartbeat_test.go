package ctrl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestSendHeartbeatsWritesPingUntilCanceled(t *testing.T) {
	// Given — 버퍼에 쓰는 하트비트 송신기와 취소 가능한 context
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// When — 10ms 간격으로 돌리다가 약 55ms 뒤에 취소한다
	go func() { done <- SendHeartbeats(ctx, &buf, 10*time.Millisecond) }()
	time.Sleep(55 * time.Millisecond)
	cancel()

	// Then — nil 로 끝나고, ping 프레임만 여러 번 쓰였다
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("취소로 끝났는데 오류: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("취소했는데 SendHeartbeats 가 끝나지 않았다")
	}
	count := 0
	for {
		body, err := ReadFrame(&buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("프레임이 깨졌다: %v", err)
			}
			break
		}
		if string(body) != "ping" {
			t.Fatalf("ping 이 아닌 프레임이 섞였다: %q", body)
		}
		count++
	}
	if count < 2 {
		t.Fatalf("ping 프레임이 너무 적다: %d", count)
	}
}

// startServe 는 임시 포트에 서버를 열고, 첫 연결에 ServeHeartbeats 를 돌려 결과를 errCh 로 보낸다.
func startServe(t *testing.T, deadline time.Duration) (addr string, errCh chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	errCh = make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		errCh <- ServeHeartbeats(conn, deadline)
	}()
	return ln.Addr().String(), errCh
}

func TestServeHeartbeatsKeepsAliveWhilePingsArrive(t *testing.T) {
	// Given — 데드라인 100ms 인 서버와 연결된 클라이언트
	addr, errCh := startServe(t, 100*time.Millisecond)
	conn, err := Dial(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — 30ms 마다 ping 을 5번(총 150ms, 데드라인보다 길다) 보낸다
	for range 5 {
		if err := WriteFrame(conn, []byte("ping")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Then — 총 시간이 데드라인을 넘었어도 서버는 아직 끝나지 않았다
	select {
	case err := <-errCh:
		t.Fatalf("ping 이 오는 동안 서버가 끝났다: %v", err)
	default:
	}
}

func TestServeHeartbeatsFailsWhenSilent(t *testing.T) {
	// Given — 데드라인 100ms 인 서버와 연결만 해 두고 아무것도 보내지 않는 클라이언트
	addr, errCh := startServe(t, 100*time.Millisecond)
	conn, err := Dial(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — 데드라인이 지나도록 기다린다
	// Then — 서버가 데드라인 초과 오류로 끝난다
	select {
	case err := <-errCh:
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("got %v, want os.ErrDeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("조용한 연결을 1초 안에 끊지 못했다")
	}
}

func TestServeHeartbeatsRejectsUnknownMessage(t *testing.T) {
	// Given — 데드라인이 넉넉한 서버와 연결된 클라이언트
	addr, errCh := startServe(t, time.Second)
	conn, err := Dial(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — ping 이 아닌 프레임을 보낸다
	if err := WriteFrame(conn, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	// Then — 서버가 오류로 끝나고, 데드라인 초과는 아니다
	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("알 수 없는 메시지 오류를 기대했는데: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("알 수 없는 메시지에 서버가 끝나지 않았다")
	}
}

func TestServeHeartbeatsRepliesPongToPing(t *testing.T) {
	// Given — 데드라인이 넉넉한 서버와 연결된 클라이언트
	addr, _ := startServe(t, time.Second)
	conn, err := Dial(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// When — ping 프레임을 보내고 응답 프레임을 읽는다
	if err := WriteFrame(conn, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	body, err := ReadFrame(conn)

	// Then — 응답은 pong 이다
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "pong" {
		t.Fatalf("got %q, want %q", body, "pong")
	}
}
