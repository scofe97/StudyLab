// Command agent 는 provd 에 제어 채널로 붙어 하트비트를 보내고, 끊기면 다시 붙습니다.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/announce"
	"github.com/scofe97/npg-lab/internal/ctrl"
)

func main() {
	// 플래그 처리
	addr := flag.String("addr", ctrl.DefaultAddr, "접속할 provd 주소")
	id := flag.String("id", "agent-1", "공지할 장비 id")
	flag.Parse()

	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info().Str("addr", *addr).Msg("agent 시작")
	// 공지는 보조 경로라 TCP 연결을 막지 않도록 따로 돌린다(ADR 0006).
	go func() {
		if err := announce.Send(ctx, *addr, *id); err != nil {
			log.Warn().Err(err).Msg("공지 실패")
			return
		}
		log.Info().Str("device", *id).Msg("공지 ACK 받음")
	}()

	ctrl.RunAgent(ctx, *addr, ctrl.HeartbeatInterval, log)
	log.Info().Msg("agent 종료")
}
