// Command tftpd 는 루트 폴더의 파일을 TFTP 로 내주는 읽기 전용 서버입니다.
package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/tftp"
)

func main() {
	// 재전송 간격을 로그로 읽으려면 시각이 밀리초까지 필요합니다
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000Z07:00"

	root := flag.String("root", "root", "내줄 파일이 있는 폴더")
	listen := flag.String("listen", "127.0.0.1:6969", "요청을 받을 UDP 주소 (TFTP 표준 포트는 69)")
	timeout := flag.Duration("timeout", tftp.DefaultTimeout, "첫 재전송까지의 대기 (재전송마다 2배)")
	retries := flag.Int("retries", tftp.DefaultRetries, "같은 블록을 보내는 최대 횟수")
	maxBlksize := flag.Int("max-blksize", tftp.DefaultMaxBlksize, "blksize 협상 상한")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}).With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := os.OpenRoot(*root)
	if err != nil {
		log.Fatal().Err(err).Str("root", *root).Msg("루트 폴더를 열지 못함")
	}
	defer dir.Close()

	pc, err := net.ListenPacket("udp", *listen)
	if err != nil {
		log.Fatal().Err(err).Str("addr", *listen).Msg("UDP listen 실패")
	}

	srv := &tftp.Server{
		Root:       dir,
		Timeout:    *timeout,
		Retries:    *retries,
		MaxBlksize: *maxBlksize,
		Log:        log,
	}
	log.Info().
		Str("addr", pc.LocalAddr().String()).
		Str("root", *root).
		Dur("timeout_ms", *timeout).
		Int("retries", *retries).
		Int("max_blksize", *maxBlksize).
		Msg("tftpd 시작")

	if err := srv.Serve(ctx, pc); err != nil {
		log.Fatal().Err(err).Msg("서버 중단")
	}
	log.Info().Msg("tftpd 종료")
}
