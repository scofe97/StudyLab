// phase1-idle-clients 는 Phase 1 실험 도구다. addr 에 TCP 연결 n 개를 열고 아무것도 보내지 않은 채 들고 있다
// (학습 문서 01-01 의 "조용한 클라이언트"). Ctrl-C(SIGINT)·SIGTERM 을 받으면 연 연결을 모두 닫고 끝난다.
// 연결의 주인은 main 이고, main 이 conns 를 닫는다.
//
// 실행: GOOS=linux GOARCH=arm64 go build -o bin/idle-clients ./experiments/phase1-idle-clients
//
//	orb -m ubuntu ./bin/idle-clients localhost:8080 100
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: idle-clients <addr> <n>")
		os.Exit(2)
	}
	addr := os.Args[1]
	n, err := strconv.Atoi(os.Args[2])
	if err != nil || n < 1 {
		fmt.Fprintln(os.Stderr, "n 은 1 이상의 정수")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 연결이 막혀도 끝없이 기다리지 않도록 한 번의 connect 를 3 초로 끊는다.
	d := net.Dialer{Timeout: 3 * time.Second}
	conns := make([]net.Conn, 0, n)
	failed := 0
	for i := 0; i < n && ctx.Err() == nil; i++ {
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			failed++
			fmt.Printf("dial #%d: %v\n", i+1, err)
			continue
		}
		conns = append(conns, c)
	}
	fmt.Printf("opened=%d failed=%d pid=%d  (Ctrl-C 로 모두 닫고 종료)\n", len(conns), failed, os.Getpid())

	<-ctx.Done()
	for _, c := range conns {
		c.Close()
	}
	fmt.Printf("closed=%d\n", len(conns))
}
