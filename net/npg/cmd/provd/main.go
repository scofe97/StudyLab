// Command provd 는 agent 의 제어 채널 연결을 받아 하트비트로 생존을 확인합니다.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/announce"
	"github.com/scofe97/npg-lab/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "제어 채널을 받을 주소")
	announceAddr := flag.String("announce-addr", ctrl.DefaultAddr, "UDP 공지를 받을 주소")
	flag.Parse()

	// 제로로그 설정
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()

	// 백그라운드 중단, 종료시 완료 처리
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

	pc, err := net.ListenPacket("udp", *announceAddr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", *announceAddr).Msg("UDP listen 실패")
	}
	go func() {
		err := announce.Serve(ctx, pc, log, func(deviceID string, from net.Addr) {
			log.Info().Str("device", deviceID).Str("from", from.String()).Msg("장비 공지 받음")
		})
		if err != nil {
			log.Error().Err(err).Msg("공지 수신 중단")
		}
	}()

	log.Info().Str("addr", *addr).Str("announce", *announceAddr).Msg("provd 시작")

	// 통신 진행(5ms 재시도 기본값)
	var delay time.Duration
	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			log.Info().Msg("provd 종료")
			return
		}
		if err != nil {
			delay = min(max(delay*2, 5*time.Millisecond), time.Second)
			log.Error().Err(err).Str("retry_in", delay.String()).Msg("accept 실패")

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}
		delay = 0
		go serve(conn, log)
	}
}

func serve(conn net.Conn, log zerolog.Logger) {
	defer conn.Close()

	// 연결
	remote := conn.RemoteAddr().String()
	log.Info().Str("remote", remote).Msg("agent 연결됨")

	// 하트비트 진행
	err := ctrl.ServeHeartbeats(conn, ctrl.ReadDeadline)
	if errors.Is(err, io.EOF) {
		log.Info().Str("remote", remote).Msg("agent 연결 종료")
		return
	}

	log.Warn().Err(err).Str("remote", remote).Msg("agent 연결 끊김")
}
