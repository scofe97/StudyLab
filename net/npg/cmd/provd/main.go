// Command provd 는 agent 의 제어 채널 연결을 받아 하트비트로 생존을 확인합니다.
package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "제어 채널을 받을 주소")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", *addr).Msg("listen 실패")
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	log.Info().Str("addr", *addr).Msg("provd 시작")

	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			log.Info().Msg("provd 종료")
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("accept 실패")
			continue
		}
		go serve(conn, log)
	}
}

func serve(conn net.Conn, log zerolog.Logger) {
	defer conn.Close()

	remote := conn.RemoteAddr().String()
	log.Info().Str("remote", remote).Msg("agent 연결됨")

	err := ctrl.ServeHeartbeats(conn, ctrl.ReadDeadline)
	log.Warn().Err(err).Str("remote", remote).Msg("agent 연결 종료")
}
