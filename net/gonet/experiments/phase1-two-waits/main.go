// phase1-two-waits 는 Phase 1 실험이다. 같은 goroutine 100 개를 두 방식으로 기다리게 하고
// OS 스레드 수와 goroutine 덤프의 기다림 이름표를 센다(학습 문서 01-01 §1).
//
//	net  : TCP Read 에서 기다림 (EAGAIN → netpoller → 스레드 반납)
//	pipe : 블로킹 pipe fd 를 syscall.Read (스레드를 쥔 채 커널 안에서 기다림)
//
// 두 모드 모두 goroutine 이 끝나지 않도록 일부러 만든 코드라 AGENTS.md 의
// "종료 경로가 없는 goroutine 을 만들지 않는다" 를 의도적으로 어긴다. 실험 밖에서 쓰지 않는다.
//
// 실행: GOOS=linux GOARCH=arm64 go build -o bin/two-waits ./experiments/phase1-two-waits
//
//	orb -m ubuntu ./bin/two-waits net   (또는 pipe)
package main

import (
	"fmt"
	"net"
	"os"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"
)

const n = 100

func main() {
	mode := os.Args[1]
	switch mode {
	case "net":
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		go func() {
			for {
				c, _ := ln.Accept()
				// 받기만 하고 아무것도 보내지 않는다
				_ = c
			}
		}()
		for i := 0; i < n; i++ {
			c, _ := net.Dial("tcp", ln.Addr().String())
			go func() {
				buf := make([]byte, 1)
				// 데이터가 오지 않으니 영원히 기다린다
				c.Read(buf)
			}()
		}
	case "pipe":
		for i := 0; i < n; i++ {
			// syscall.Pipe 는 블로킹 fd 를 돌려준다. os.File 로 감싸지 않았으니 netpoller 를 거치지 않는다.
			var p [2]int
			syscall.Pipe(p[:])
			go func() {
				buf := make([]byte, 1)
				// 쓰는 쪽이 없으니 커널 안에서 멈춘다
				syscall.Read(p[0], buf)
			}()
		}
	}
	time.Sleep(time.Second)

	threads := pprof.Lookup("threadcreate").Count()
	fmt.Printf("mode=%s goroutines=%d threads=%d\n", mode, pprof.Lookup("goroutine").Count(), threads)
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "Threads:") {
				fmt.Println("/proc/self/status", l)
			}
		}
	}
	// 기다림의 이름표: goroutine 덤프의 [ ] 안 문자열
	var sb strings.Builder
	pprof.Lookup("goroutine").WriteTo(&sb, 2)
	counts := map[string]int{}
	for _, l := range strings.Split(sb.String(), "\n") {
		if strings.HasPrefix(l, "goroutine ") {
			s := l[strings.Index(l, "[")+1 : strings.Index(l, "]")]
			if i := strings.Index(s, ","); i >= 0 {
				s = s[:i]
			}
			counts[s]++
		}
	}
	fmt.Println("wait reasons:", counts)
}
