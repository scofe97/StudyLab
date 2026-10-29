// gonet 은 네트워크 primitive 를 하나씩 직접 구현하고 관측하는 학습용 CLI 다.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/scofe97/runners-lab/net/gonet/internal/tcp"
	"github.com/scofe97/runners-lab/net/gonet/internal/udp"
)

const usage = `usage:
  gonet listen <addr> [idle]  TCP echo 서버. idle 동안 입력이 없는 접속은 끊는다 (기본 30s, 예: gonet listen :8080 10s)
  gonet connect <addr>        stdin 을 보내고 응답을 stdout 에 쓴다 (예: gonet connect localhost:8080)
  gonet udp-listen <addr> [buf]
                              UDP echo 서버. 데이터그램마다 받은 바이트 수와 보낸 쪽 주소를 남긴다
                              (기본 버퍼 65536 바이트, 예: gonet udp-listen :8080 3)
  gonet udp-send <addr> <wait> <msg>...
                              msg 를 데이터그램 하나씩 보내고 wait 동안 echo 를 기다린다
                              (예: gonet udp-send localhost:8080 1s hello world)
  gonet dns [flags] <name>    질의를 직접 바이트로 지어 묻고 보낸·받은 바이트와 풀어낸 답을 찍는다
                              (예: gonet dns -type TXT google.com, 플래그는 gonet dns -h)
  gonet scan [flags] <host>   CONNECT 스캔. 포트마다 open·closed·filtered 와 근거 errno 를 찍는다
                              (예: gonet scan -ports 1-1024,8080 127.0.0.1, 플래그는 gonet scan -h)`

func main() {
	// dns 는 주소 대신 플래그와 이름을 받으므로 아래 <cmd> <addr> 형식보다 먼저 가른다.
	if len(os.Args) >= 2 && os.Args[1] == "dns" {
		os.Exit(runDNS(os.Args[2:]))
	}
	if len(os.Args) >= 2 && os.Args[1] == "scan" {
		os.Exit(runScan(os.Args[2:]))
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, addr, rest := os.Args[1], os.Args[2], os.Args[3:]

	// Ctrl-C(SIGINT)·SIGTERM 을 ctx 취소로 바꿔 graceful close 경로를 탄다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stderr, "gonet ", log.LstdFlags|log.Lmicroseconds)

	var err error
	switch cmd {
	case "listen":
		idle := 30 * time.Second
		if len(rest) > 1 {
			usageExit()
		}
		if len(rest) == 1 {
			d, perr := time.ParseDuration(rest[0])
			if perr != nil || d <= 0 {
				usageExit()
			}
			idle = d
		}
		err = listen(ctx, addr, idle, logger)
	case "connect":
		if len(rest) != 0 {
			usageExit()
		}
		err = tcp.Connect(ctx, addr, os.Stdin, os.Stdout)
	case "udp-listen":
		buf := 64 * 1024
		if len(rest) > 1 {
			usageExit()
		}
		if len(rest) == 1 {
			n, perr := strconv.Atoi(rest[0])
			if perr != nil || n <= 0 {
				usageExit()
			}
			buf = n
		}
		err = udpListen(ctx, addr, buf, logger)
	case "udp-send":
		if len(rest) < 2 {
			usageExit()
		}
		wait, perr := time.ParseDuration(rest[0])
		if perr != nil || wait <= 0 {
			usageExit()
		}
		err = udp.Send(ctx, addr, rest[1:], wait, os.Stdout)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		logger.Printf("error: %v", err)
		os.Exit(1)
	}
}

func listen(ctx context.Context, addr string, idle time.Duration, logger *log.Logger) error {
	// net.Listen 한 줄 아래에서 socket() → bind() → listen() 이 일어난다.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	logger.Printf("listen addr=%s pid=%d idle=%s", ln.Addr(), os.Getpid(), idle)
	err = tcp.Serve(ctx, ln, idle, logger)
	logger.Printf("shutdown")
	return err
}

func udpListen(ctx context.Context, addr string, bufSize int, logger *log.Logger) error {
	// net.ListenPacket 는 socket() → bind() 까지만 한다. listen()·accept() 가 없고 LISTEN 상태도 없다.
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	logger.Printf("udp-listen addr=%s pid=%d buf=%d", pc.LocalAddr(), os.Getpid(), bufSize)
	err = udp.Serve(ctx, pc, bufSize, logger)
	logger.Printf("shutdown")
	return err
}

func usageExit() {
	fmt.Fprintln(os.Stderr, usage)
	os.Exit(2)
}
