// tftpd 는 루트 폴더의 파일을 이름으로 내주는 읽기 전용 TFTP 서버다.
//
//	go run ./cmd/tftpd -root work/root -listen 127.0.0.1:6969
//
// 전송이 끝날 때마다(성공·실패·거절 모두) 표준 에러에 한 줄을 남긴다.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/scofe97/runners-lab/net/npg/chapters/ch06-tftp/tftp"
)

func main() {
	var (
		root      = flag.String("root", "work/root", "내줄 파일이 있는 폴더")
		listen    = flag.String("listen", "127.0.0.1:6969", "요청을 받을 UDP 주소 (TFTP 표준 포트는 69)")
		timeout   = flag.Duration("timeout", tftp.DefaultTimeout, "DATA(창)를 보낸 뒤 ACK 를 기다리는 시간")
		retries   = flag.Int("retries", tftp.DefaultRetries, "같은 블록(창)을 보내는 최대 횟수")
		options   = flag.Bool("options", true, "RFC 2347 옵션 협상(blksize·windowsize·tsize·timeout)을 한다")
		maxBlk    = flag.Int("max-blksize", tftp.MaxBlockSize, "blksize 협상 상한")
		maxWin    = flag.Int("max-window", 64, "windowsize 협상 상한")
		rollover  = flag.String("rollover", "zero", "블록 번호 65535 다음: zero | one | refuse")
		netascii  = flag.String("netascii", "convert", "netascii 모드 요청: convert | raw | reject")
		legacy    = flag.Bool("legacy-dup-ack", false, "책 Listing 루프: 지난 ACK 를 받으면 현재 블록을 곧바로 다시 보낸다")
		unsafeOpn = flag.Bool("unsafe-open", false, "os.Root 대신 filepath.Join(root, 이름) 을 os.Open 으로 연다")
	)
	flag.Parse()

	ro, err := tftp.ParseRollover(*rollover)
	if err != nil {
		log.Fatal(err)
	}
	na, err := tftp.ParseNetasciiPolicy(*netascii)
	if err != nil {
		log.Fatal(err)
	}

	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
	s := &tftp.Server{
		Root:          *root,
		Timeout:       *timeout,
		Retries:       *retries,
		Options:       *options,
		MaxBlockSize:  *maxBlk,
		MaxWindowSize: *maxWin,
		Rollover:      ro,
		Netascii:      na,
		LegacyDupAck:  *legacy,
		UnsafeOpen:    *unsafeOpn,
		Log:           logger,
	}

	pc, err := net.ListenPacket("udp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	logger.Printf("listening on %s root=%s timeout=%s retries=%d options=%t max-blksize=%d max-window=%d "+
		"rollover=%s netascii=%s legacy-dup-ack=%t unsafe-open=%t",
		pc.LocalAddr(), *root, *timeout, *retries, *options, *maxBlk, *maxWin, ro, na, *legacy, *unsafeOpn)

	// Ctrl-C 나 kill 을 받으면 리스너를 닫아 Serve 를 끝낸다
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		pc.Close()
	}()

	if err := s.Serve(pc); err != nil {
		log.Fatal(err)
	}
}
