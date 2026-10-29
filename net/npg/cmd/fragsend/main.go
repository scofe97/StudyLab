//go:build linux

// Command fragsend 는 장애 리허설용으로 크기를 정한 HELLO 모양 데이터그램 하나를 보내고 ACK 를 기다립니다.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/scofe97/runners-lab/net/npg/internal/announce"
)

func main() {
	addr := flag.String("addr", "[::1]:7100", "보낼 provd UDP 주소")
	size := flag.Int("size", 100, "UDP 페이로드 바이트 수(헤더 10바이트 포함)")
	df := flag.Bool("df", false, "단편화를 금지(DF)하고 보냄(IPv6 주소에서만 동작)")
	wait := flag.Duration("wait", 2*time.Second, "ACK 를 기다리는 시간")
	flag.Parse()

	if *size < announce.HeaderSize {
		fmt.Fprintf(os.Stderr, "size 는 헤더 %d바이트 이상이어야 합니다\n", announce.HeaderSize)
		os.Exit(2)
	}

	conn, err := net.Dial("udp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dial 실패: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if *df {
		if err := setDontFragment(conn.(*net.UDPConn)); err != nil {
			fmt.Fprintf(os.Stderr, "DF 설정 실패: %v\n", err)
			os.Exit(1)
		}
	}

	// announce.Message.Encode 는 상한 초과를 거절하므로 바이트를 직접 만든다.
	pkt := make([]byte, *size)
	pkt[0] = announce.Version
	pkt[1] = byte(announce.TypeHello)
	binary.BigEndian.PutUint64(pkt[2:], 1)
	for i := announce.HeaderSize; i < len(pkt); i++ {
		pkt[i] = 'x'
	}

	n, err := conn.Write(pkt)
	if err != nil {
		fmt.Printf("%s 전송 실패: %v\n", time.Now().Format("15:04:05.000"), err)
		os.Exit(1)
	}
	fmt.Printf("%s 전송 %d바이트\n", time.Now().Format("15:04:05.000"), n)

	conn.SetReadDeadline(time.Now().Add(*wait))
	buf := make([]byte, 64)
	rn, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("%s ACK 없음: %v\n", time.Now().Format("15:04:05.000"), err)
		return
	}
	fmt.Printf("%s ACK %d바이트\n", time.Now().Format("15:04:05.000"), rn)
}

// setDontFragment 는 IPv6 소켓이 단편화 없이 보내도록(경로 MTU 탐색 DO) 설정합니다.
func setDontFragment(conn *net.UDPConn) error {
	rc, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var serr error
	err = rc.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_MTU_DISCOVER, syscall.IPV6_PMTUDISC_DO)
	})
	if err != nil {
		return err
	}
	return serr
}
