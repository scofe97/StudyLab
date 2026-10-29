// tftpget 은 파일 하나를 받는 TFTP 클라이언트다. 장애 주입(ACK 늦추기)과 옵션 요청을 플래그로 고른다.
//
//	go run ./cmd/tftpget -server 127.0.0.1:6969 -sum fw.bin
//
// 받은 뒤 표준 에러에 클라이언트 쪽 숫자를 한 줄로 남긴다.
package main

import (
	"crypto/sha512"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/scofe97/runners-lab/net/npg/chapters/ch06-tftp/tftp"
)

func main() {
	var (
		server     = flag.String("server", "127.0.0.1:6969", "서버 주소")
		out        = flag.String("o", "", "저장할 경로 (기본: 원격 이름의 마지막 부분, - 면 버림)")
		mode       = flag.String("mode", "octet", "전송 모드: octet | netascii")
		blksize    = flag.Int("blksize", 0, "0 이 아니면 blksize 옵션을 요청한다")
		window     = flag.Int("windowsize", 0, "0 이 아니면 windowsize 옵션을 요청한다")
		tsize      = flag.Bool("tsize", false, "tsize 옵션으로 파일 크기를 묻는다")
		timeoutOpt = flag.Int("timeout-opt", 0, "0 이 아니면 timeout 옵션(초)을 요청한다")
		timeout    = flag.Duration("timeout", 5*time.Second, "다음 패킷을 기다리는 시간")
		retries    = flag.Int("retries", 5, "연달아 타임아웃이 나도 버티는 횟수")
		rollover   = flag.String("rollover", "zero", "블록 번호 65535 다음으로 기대하는 값: zero | one")
		delayBlk   = flag.Uint64("ack-delay-block", 0, "이 블록(1부터)의 첫 ACK 를 늦춘다")
		delay      = flag.Duration("ack-delay", 0, "-ack-delay-block 의 ACK 를 늦출 시간")
		printSum   = flag.Bool("sum", false, "받은 바이트의 SHA512/256 을 출력한다 (shasum -a 512256 과 같은 값)")
	)
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: tftpget [flags] <remote-file>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	remote := flag.Arg(0)
	ro, err := tftp.ParseRollover(*rollover)
	if err != nil {
		log.Fatal(err)
	}

	var w io.Writer = io.Discard
	dest := *out
	if dest == "" {
		dest = path.Base(remote)
	}
	if dest != "-" {
		f, err := os.Create(dest)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		w = f
	}
	h := sha512.New512_256()
	w = io.MultiWriter(w, h)

	st, err := tftp.Get(*server, remote, tftp.ClientOptions{
		Mode:          *mode,
		BlockSize:     *blksize,
		WindowSize:    *window,
		TSize:         *tsize,
		TimeoutOption: *timeoutOpt,
		Timeout:       *timeout,
		Retries:       *retries,
		Rollover:      ro,
		AckDelayBlock: *delayBlk,
		AckDelay:      *delay,
	}, w)

	rate := 0.0
	if st.Elapsed > 0 {
		rate = float64(st.Bytes) / 1024 / st.Elapsed.Seconds()
	}
	fmt.Fprintf(os.Stderr, "file=%q bytes=%d blocks=%d blksize=%d window=%d tsize=%d oack=%v "+
		"dup_data=%d unexpected=%d acks=%d timeouts=%d strangers=%d elapsed=%s rate=%.1fKiB/s\n",
		remote, st.Bytes, st.Blocks, st.BlockSize, st.WindowSize, st.TSize, st.OAck,
		st.DupData, st.Unexpected, st.AcksSent, st.Timeouts, st.Strangers,
		st.Elapsed.Round(time.Millisecond), rate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *printSum {
		fmt.Printf("%s  %s\n", hex.EncodeToString(h.Sum(nil)), dest)
	}
}
