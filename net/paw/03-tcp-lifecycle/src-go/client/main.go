// SimpleClient0301.java 의 Go 판. 옵션 이름과 동작을 자바 쪽과 맞췄다.
// 빌드: ./build.sh  실행: docker exec tcp-client /lab/bin/client --close-after never
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func main() {
	// 옵션
	host := flag.String("host", "server", "접속할 호스트")
	port := flag.Int("port", 8082, "접속할 포트")
	closeAfter := flag.String("close-after", "0", "0 | 초 | never")
	readDelay := flag.Int("read-delay", 0, "읽기 시작 전 대기 초")
	rcvbuf := flag.Int("rcvbuf", 0, "수신 버퍼 바이트")
	abort := flag.Bool("abort", false, "SO_LINGER 0 으로 연결 직후 중단")
	flag.Parse()

	// 연결
	logf("connect %s:%d", *host, *port)
	conn, err := net.Dial("tcp4", net.JoinHostPort(*host, strconv.Itoa(*port)))
	if err != nil {
		logf("connect 실패: %v", err)
		return
	}
	tcp := conn.(*net.TCPConn)
	logf("connected %s -> %s", tcp.LocalAddr(), tcp.RemoteAddr())

	// 수신 버퍼는 읽기 전에 걸어야 광고되는 윈도우에 반영된다
	if *rcvbuf > 0 {
		tcp.SetReadBuffer(*rcvbuf)
	}

	// 폐기 — FIN 대신 무엇이 나가는지 보는 경로
	if *abort {
		tcp.SetLinger(0)
		logf("SetLinger(0) 으로 Close() 호출")
		tcp.Close()
		return
	}

	// 받는 쪽이 못 읽어 가는 상황을 만든다
	if *readDelay > 0 {
		logf("읽기 전 %d초 대기", *readDelay)
		time.Sleep(time.Duration(*readDelay) * time.Second)
	}

	// 받은 데이터 읽기 — io.EOF 가 상대의 FIN 도착이다
	buf := make([]byte, 8192)
	var total int
	var head strings.Builder
	for {
		n, err := tcp.Read(buf)
		if n > 0 {
			if head.Len() < 64 {
				head.Write(buf[:min(n, 64)])
			}
			total += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			logf("read 중단: %v", err)
			return
		}
	}
	tail := ""
	if total <= 64 {
		tail = ", 내용 " + strings.TrimSpace(head.String())
	}
	logf("Read 가 EOF 반환 — 받은 바이트 %d%s", total, tail)

	// never 면 닫지 않고 멈춘다
	if *closeAfter == "never" {
		logf("Close() 를 부르지 않고 멈춰 있습니다 (Ctrl+C 로 종료)")
		select {}
	}

	// 지정한 초만큼 기다린 뒤 닫는다
	seconds, err := strconv.Atoi(*closeAfter)
	if err != nil {
		panic("close-after 는 0, 초, never 중 하나")
	}
	if seconds > 0 {
		logf("%d초 뒤 Close() 호출 예정", seconds)
		time.Sleep(time.Duration(seconds) * time.Second)
	}
	tcp.Close()
	logf("Close() 호출")
}

func logf(format string, a ...any) {
	fmt.Printf("[client %s] %s\n", time.Now().Format("15:04:05.000000"), fmt.Sprintf(format, a...))
}
