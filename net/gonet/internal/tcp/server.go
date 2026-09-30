// Package tcp 는 Phase 0 의 최소 TCP primitive 를 담는다.
// net.Listen/Accept/Dial 위에 추상화를 올리지 않고 그대로 드러내는 것이 목적이다.
package tcp

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// Serve 는 ln 에서 접속을 받아 받은 바이트를 그대로 돌려준다(echo).
// idle 동안 아무것도 받지 못한 접속은 서버가 끊는다(idle timeout).
// ctx 가 끝나면 새 접속을 멈추고, 열려 있는 접속을 닫은 뒤 모든 핸들러가 끝날 때까지 기다린다.
// listener 의 주인은 호출자지만, 종료 신호를 받으면 Serve 가 닫는다.
func Serve(ctx context.Context, ln net.Listener, idle time.Duration, logger *log.Logger) error {
	var (
		mu    sync.Mutex
		conns = make(map[net.Conn]struct{})
		wg    sync.WaitGroup
	)

	// Accept 는 blocking 이라 ctx 를 직접 보지 못한다. listener 를 닫아야 Accept 가 깨어난다.
	stop := context.AfterFunc(ctx, func() {
		ln.Close()
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	})
	defer stop()

	// Accept 가 연달아 실패할 때 쉬는 시간. 실패마다 두 배로 늘리고 1s 를 넘지 않는다(net/http 의 Server.Serve 와 같은 방식).
	var delay time.Duration
	for {
		conn, err := ln.Accept()
		if err != nil {
			// 종료 신호로 listener 가 닫혔다. 열린 접속이 끝나기를 기다린 뒤 끝낸다
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				wg.Wait()
				return nil
			}
			// 그 밖(EMFILE 등)은 접속이 끝나 FD 가 반납되면 풀린다. 끝내지 않고 쉬었다가 다시 받는다.
			// 곧바로 다시 부르면 Accept 가 즉시 실패를 반복해 CPU 를 태우고 로그를 쏟는다(busy loop).
			if delay == 0 {
				delay = 5 * time.Millisecond
			} else {
				delay *= 2
			}
			if delay > time.Second {
				delay = time.Second
			}
			logger.Printf("accept error: %v; retrying in %s", err, delay)
			select {
			case <-time.After(delay):
			// 쉬는 동안 종료 신호가 오면 바로 깨어나 다음 Accept 에서 ErrClosed 를 받는다
			case <-ctx.Done():
			}
			continue
		}
		delay = 0

		// 종료 직전에 accept 된 접속은 AfterFunc 의 정리 루프를 놓칠 수 있다.
		// ctx 는 AfterFunc 보다 먼저 취소되므로, 등록 시점에 한 번 더 보면 빠짐이 없다.
		mu.Lock()
		conns[conn] = struct{}{}
		if ctx.Err() != nil {
			conn.Close()
		}
		mu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				mu.Lock()
				delete(conns, conn)
				mu.Unlock()
			}()
			handleEcho(conn, idle, logger)
		}()
	}
}

func handleEcho(conn net.Conn, idle time.Duration, logger *log.Logger) {
	defer conn.Close()
	remote := conn.RemoteAddr()
	logger.Printf("accept remote=%s local=%s", remote, conn.LocalAddr())

	// io.Copy 는 읽기·쓰기 반복을 안에서 끝까지 돌려 사이에 deadline 을 다시 걸 틈이 없다.
	// 그래서 반복을 직접 쓰고, 읽기 직전마다 deadline 을 민다.
	buf := make([]byte, 32*1024)
	var n int64
	for {
		// deadline 은 시각이라 한 번만 걸면 연결이 열린 뒤 idle 에 무조건 끊긴다.
		// 읽기 직전마다 "지금부터 idle 뒤"로 밀어야 입력이 없는 시간만 잰다.
		conn.SetReadDeadline(time.Now().Add(idle))
		r, err := conn.Read(buf)
		if r > 0 {
			w, werr := conn.Write(buf[:r])
			n += int64(w)
			if werr != nil {
				logger.Printf("close remote=%s bytes=%d err=%v", remote, n, werr)
				return
			}
		}
		switch {
		case err == nil:
			continue
		// 상대가 FIN 을 보냈다. 정상 종료
		case errors.Is(err, io.EOF):
			logger.Printf("close remote=%s bytes=%d", remote, n)
		// idle 동안 아무것도 오지 않았다
		case errors.Is(err, os.ErrDeadlineExceeded):
			logger.Printf("idle timeout remote=%s bytes=%d idle=%s", remote, n, idle)
		// 종료 신호로 Serve 가 conn 을 닫았다
		case errors.Is(err, net.ErrClosed):
			logger.Printf("close remote=%s bytes=%d", remote, n)
		default:
			logger.Printf("close remote=%s bytes=%d err=%v", remote, n, err)
		}
		return
	}
}
