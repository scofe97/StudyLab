// Command agent 는 provd 에 제어 채널로 붙어 하트비트를 보내고, 끊기면 다시 붙습니다.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "접속할 provd 주소")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info().Str("addr", *addr).Msg("agent 시작")
	ctrl.RunAgent(ctx, *addr, ctrl.HeartbeatInterval, log)
	log.Info().Msg("agent 종료")
}
