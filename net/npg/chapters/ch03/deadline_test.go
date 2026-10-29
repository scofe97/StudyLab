package ch03

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// 실험 3 — 지난 데드라인과 바쁜 루프 (원문 Listing 3-9 응용).
//
// 클라이언트는 연결만 맺고 아무것도 보내지 않는다. 서버는 1초 동안
// 에러가 나도 연결을 닫지 않고 Read 를 계속 다시 부르며, 몇 번 돌았는지 센다.
// 두 판의 차이는 데드라인을 "한 번만" 거느냐, "Read 직전마다" 다시 거느냐뿐이다.

const (
	idle   = 200 * time.Millisecond // 데드라인 길이
	window = time.Second            // 관찰 시간
)

// readLoop 는 window 동안 Read 를 반복하며 호출 횟수와 마지막 에러를 돌려준다.
// resetEachRead 가 true 면 Read 직전마다 데드라인을 "지금 + idle" 로 민다.
func readLoop(conn net.Conn, resetEachRead bool) (calls int, lastErr error) {
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(idle)) // 처음 한 번은 두 판 모두 건다

	begin := time.Now()
	for time.Since(begin) < window {
		if resetEachRead {
			conn.SetReadDeadline(time.Now().Add(idle))
		}
		_, lastErr = conn.Read(buf)
		calls++
		// 에러가 나도 닫지 않고 다시 돈다 (Q8 의 상황)
	}
	return calls, lastErr
}

// silentPair 는 서로 연결된 서버 쪽 conn 과, 아무것도 보내지 않는 클라이언트 conn 을 만든다.
func silentPair(t *testing.T) (server net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })

	server, err = listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	return server
}

func logResult(t *testing.T, calls int, err error) {
	t.Helper()
	t.Logf("1초 동안 Read 호출 = %d 번", calls)
	t.Logf("마지막 err         = %v", err)
	t.Logf("errors.Is(err, os.ErrDeadlineExceeded) = %v", errors.Is(err, os.ErrDeadlineExceeded))
}

func TestDeadlineSetOnce(t *testing.T) {
	// Given — 말없는 클라이언트와 연결된 서버 conn
	server := silentPair(t)

	// When — 데드라인을 처음에 한 번만 걸고 1초 동안 Read 를 반복한다
	calls, err := readLoop(server, false)

	// Then — 몇 번 돌았는지와 에러 종류를 남긴다
	logResult(t, calls, err)
}

func TestDeadlineResetEachRead(t *testing.T) {
	// Given — 말없는 클라이언트와 연결된 서버 conn
	server := silentPair(t)

	// When — Read 직전마다 데드라인을 지금+200ms 로 밀며 1초 동안 반복한다
	calls, err := readLoop(server, true)

	// Then — 몇 번 돌았는지와 에러 종류를 남긴다
	logResult(t, calls, err)
}
