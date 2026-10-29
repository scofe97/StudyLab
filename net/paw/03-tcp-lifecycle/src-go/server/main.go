// SimpleTCPServer01.java 의 Go 판. 동작은 같고, 항상 서버가 먼저 닫는다.
// 빌드: ./build.sh  실행: docker exec tcp-server /lab/bin/server --mode normal
package main

import (
	"flag"
	"fmt"
	"net"
	"time"
)

// 줄바꿈까지 29바이트 — 노트 03-01 §4 packet#4 의 29바이트와 대조하려고 맞췄다
const greeting = "ANISH NATH NORMAL CONNECTION\n"

func main() {
	// 옵션
	port := flag.Int("port", 8082, "listen port")
	mode := flag.String("mode", "normal", "normal | slow | bulk")
	delayMs := flag.Int("delay-ms", 3000, "slow 모드의 응답 전 지연")
	bytes := flag.Int("bytes", 5242880, "bulk 모드가 보낼 바이트 수")
	flag.Parse()

	// 듣기
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	logf("LISTEN :%d (mode=%s)", *port, *mode)

	// 연결마다 처리하고 서버가 먼저 닫는다
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		logf("accept %s", conn.RemoteAddr())
		handle(conn, *mode, *delayMs, *bytes)
	}
}

func handle(conn net.Conn, mode string, delayMs int, bulkBytes int) {
	defer func() {
		conn.Close()
		logf("Close() 호출")
	}()

	switch mode {
	case "normal":
		// 상대가 연결을 파기하면 write 가 실패한다. 로그만 남기고 서버는 계속 돈다
		if _, err := conn.Write([]byte(greeting)); err != nil {
			logf("write 실패: %v", err)
		}
	case "slow":
		// 서버 처리 지연 — handshake 는 빠르고 데이터만 늦는 경우를 만든다
		logf("응답 전 %dms 처리 지연", delayMs)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		conn.Write([]byte(greeting))
	case "bulk":
		chunk := make([]byte, 64*1024)
		sent := 0
		for sent < bulkBytes {
			n := len(chunk)
			if rest := bulkBytes - sent; rest < n {
				n = rest
			}
			written, err := conn.Write(chunk[:n])
			if err != nil {
				logf("write 중단: %v", err)
				return
			}
			sent += written
		}
		logf("보냄 %d bytes", sent)
	default:
		panic("알 수 없는 mode: " + mode)
	}
}

func logf(format string, a ...any) {
	fmt.Printf("[server %s] %s\n", time.Now().Format("15:04:05.000000"), fmt.Sprintf(format, a...))
}
