// Command leakd 는 장애 리허설용 실험 서버입니다. 상대가 연결을 닫아도 Close 를 부르지 않아 CLOSE_WAIT 가 쌓입니다.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/scofe97/runners-lab/net/npg/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "받을 주소")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen 실패:", err)
		os.Exit(1)
	}
	fmt.Printf("%s leakd 시작 addr=%s\n", time.Now().Format("15:04:05"), *addr)

	// 연결 참조를 붙들어 둔다. 참조를 버리면 GC 의 finalizer 가 소켓을 대신 닫아 누수가 가려진다.
	var mu sync.Mutex
	var held []net.Conn

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "accept 실패:", err)
			return
		}
		mu.Lock()
		held = append(held, conn)
		n := len(held)
		mu.Unlock()
		fmt.Printf("%s 연결 받음 (누적 %d)\n", time.Now().Format("15:04:05"), n)

		go func() {
			// 상대가 닫을 때까지 읽는다. EOF 를 받고도 conn.Close() 를 부르지 않는다.
			io.Copy(io.Discard, conn)
			fmt.Printf("%s 상대가 닫음, Close 를 부르지 않음 remote=%s\n", time.Now().Format("15:04:05"), conn.RemoteAddr())
		}()
	}
}
