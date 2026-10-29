// Command tftpget 은 TFTP 서버에서 파일 하나를 받아 오고 결과 숫자를 한 줄로 남깁니다.
package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/scofe97/runners-lab/net/npg/internal/tftp"
)

func main() {
	server := flag.String("server", "127.0.0.1:6969", "서버 주소 (TFTP 표준 포트는 69)")
	timeout := flag.Duration("timeout", tftp.DefaultTimeout, "첫 재전송까지의 대기 (재전송마다 2배)")
	retries := flag.Int("retries", tftp.DefaultRetries, "같은 패킷을 보내는 최대 횟수")
	blksize := flag.Int("blksize", 0, "blksize 옵션을 요청한다 (0 이면 요청하지 않는다)")
	tsize := flag.Bool("tsize", false, "tsize 옵션으로 파일 크기를 묻는다")
	out := flag.String("o", "", "저장할 경로 (기본: 원격 파일 이름, - 이면 저장하지 않고 버린다)")
	sum := flag.Bool("sum", false, "받은 내용의 SHA512/256 해시를 표준 출력에 찍는다")
	ackDelayBlock := flag.Uint("ack-delay-block", 0, "ACK 를 늦출 블록 번호 (장애 주입, 0 이면 없음)")
	ackDelay := flag.Duration("ack-delay", 0, "그 블록의 ACK 를 늦추는 시간")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "사용법: %s [플래그] <원격파일>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	name := flag.Arg(0)

	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}).With().Timestamp().Logger()

	addr, err := net.ResolveUDPAddr("udp", *server)
	if err != nil {
		log.Fatal().Err(err).Str("server", *server).Msg("서버 주소를 해석하지 못함")
	}

	var w io.Writer = io.Discard
	if *out != "-" {
		dst := *out
		if dst == "" {
			dst = path.Base(name)
		}
		f, err := os.Create(dst)
		if err != nil {
			log.Fatal().Err(err).Str("path", dst).Msg("저장 파일을 만들지 못함")
		}
		defer f.Close()
		w = f
	}
	hasher := sha512.New512_256()
	w = io.MultiWriter(w, hasher)

	c := &tftp.Client{Timeout: *timeout, Retries: *retries, Blksize: *blksize, Tsize: *tsize}
	if *ackDelayBlock > 0 {
		// 장애 주입: 그 블록의 ACK 를 늦춘다 (ACK 는 이 콜백이 끝난 뒤 나간다)
		c.OnData = func(block uint16) {
			if uint(block) == *ackDelayBlock {
				time.Sleep(*ackDelay)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	res, err := c.Get(ctx, addr, name, w)
	ev := log.Info()
	if err != nil {
		ev = log.Error().Err(err)
	}
	ev.Str("file", name).
		Int("blksize", res.Blksize).
		Int64("tsize", res.Size).
		Int("blocks", res.Blocks).
		Int("bytes", res.Bytes).
		Int("dup_data", res.DupData).
		Int("timeouts", res.Timeouts).
		Int("strangers", res.Strangers).
		Dur("elapsed_ms", res.Elapsed).
		Msg("받기 끝")
	if err != nil {
		os.Exit(1)
	}
	if *sum {
		fmt.Println(hex.EncodeToString(hasher.Sum(nil)))
	}
}
