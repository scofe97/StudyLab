// Command forgeframe 은 장애 리허설용으로 위조된 프레임 헤더를 provd 에 보냅니다.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/scofe97/npg-lab/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "provd 주소")
	mode := flag.String("mode", "huge", "huge: 상한을 넘는 길이, short: 길이보다 짧은 본문 뒤 침묵")
	flag.Parse()

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dial 실패:", err)
		os.Exit(1)
	}
	defer conn.Close()

	var payload []byte
	switch *mode {
	case "huge":
		// 길이 필드에 uint32 최댓값을 적고 본문은 보내지 않는다.
		payload = make([]byte, ctrl.HeaderSize)
		binary.BigEndian.PutUint32(payload, 1<<32-1)
	case "short":
		// 길이 100 을 적고 본문은 10바이트만 보낸 뒤 멈춘다.
		payload = make([]byte, ctrl.HeaderSize+10)
		binary.BigEndian.PutUint32(payload, 100)
	default:
		fmt.Fprintln(os.Stderr, "알 수 없는 mode:", *mode)
		os.Exit(1)
	}

	start := time.Now()
	if _, err := conn.Write(payload); err != nil {
		fmt.Fprintln(os.Stderr, "write 실패:", err)
		os.Exit(1)
	}
	fmt.Printf("%s 보냄: mode=%s bytes=%d\n", time.Now().Format("15:04:05"), *mode, len(payload))

	// 서버가 연결을 끊을 때까지 기다리고 끊긴 시각과 경과 시간을 적는다.
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	_, err = conn.Read(make([]byte, 1))
	fmt.Printf("%s 서버가 끊음: %v (경과 %s)\n", time.Now().Format("15:04:05"), err, time.Since(start).Round(time.Millisecond))
}
