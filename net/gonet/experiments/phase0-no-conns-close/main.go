// phase0-no-conns-close 는 Phase 0 장애 주입 실험이다.
// internal/tcp.Serve 에서 종료 시 열린 접속을 닫는 conns 루프만 뺐다.
// listener 를 닫아도 연결 소켓은 살아남는다는 것(학습 문서 00-01 §3)을 관측하려고 일부러 깨뜨린 코드라,
// AGENTS.md 의 "종료 경로가 없는 goroutine 을 만들지 않는다" 를 의도적으로 어긴다. 실험 밖에서 쓰지 않는다.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: phase0-no-conns-close <addr>")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := log.New(os.Stderr, "exp ", log.LstdFlags|log.Lmicroseconds)

	ln, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("listen addr=%s pid=%d", ln.Addr(), os.Getpid())

	var wg sync.WaitGroup
	// 원본과 달리 listener 만 닫는다. 열린 접속은 아무도 닫지 않는다.
	stopClose := context.AfterFunc(ctx, func() { ln.Close() })
	defer stopClose()

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Printf("accept stopped: %v — waiting for handlers", err)
			wg.Wait()
			if errors.Is(err, net.ErrClosed) {
				logger.Printf("shutdown")
				return
			}
			logger.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			logger.Printf("accept remote=%s", conn.RemoteAddr())
			n, err := io.Copy(conn, conn)
			logger.Printf("close remote=%s bytes=%d err=%v", conn.RemoteAddr(), n, err)
		}()
	}
}
