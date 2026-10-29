// Command npgproxy 는 요청을 업스트림 하나로 넘기는 리버스 프록시입니다 (ADR 0011).
//
// 손님용 주소(-addr)와 관리용 주소(-admin)를 따로 엽니다. 관리용 주소에서는
// GET /debug/vars 로 카운터(npgproxy.requests·inflight·conns_new·conns_reused·reasons)를 봅니다.
// envoy 가 관리 포트(기본 9901)를 손님 포트와 나누는 것과 같은 까닭입니다:
// 손님 경로와 겹치지 않고, 관리 포트는 밖에 열지 않을 수 있습니다.
//
//	npgproxy -addr 127.0.0.1:8080 -upstream http://127.0.0.1:9090 -admin 127.0.0.1:8081
package main

import (
	"context"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/scofe97/npg-lab/internal/proxy"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "손님 요청을 받을 주소")
	admin := flag.String("admin", "127.0.0.1:8081", "/debug/vars 를 내보낼 관리 주소, 빈 값이면 끈다")
	upstream := flag.String("upstream", "http://127.0.0.1:9090", "요청을 넘길 업스트림 주소")
	jsonLog := flag.Bool("json", false, "로그를 사람용 대신 JSON 한 줄로 쓴다 (기계로 집계할 때)")
	flag.Parse()

	u, err := url.Parse(*upstream)
	if err != nil || u.Scheme == "" || u.Host == "" {
		fmt.Fprintf(os.Stderr, "npgproxy: -upstream 은 http://host:port 꼴이어야 합니다: %q\n", *upstream)
		os.Exit(2)
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano // 기본은 초 단위라 같은 초 안의 요청 순서가 안 보인다
	var log zerolog.Logger
	if *jsonLog {
		log = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}).With().Timestamp().Logger()
	}

	h, vars := proxy.New(u, log)
	expvar.Publish("npgproxy", vars)

	// 손님용 서버. ReadHeaderTimeout 은 헤더를 질질 끌며 연결을 무는 손님(slowloris)을 끊는다.
	front := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	servers := []*http.Server{front}
	if *admin != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /debug/vars", expvar.Handler())
		servers = append(servers, &http.Server{Addr: *admin, Handler: mux, ReadHeaderTimeout: 5 * time.Second})
	}

	// Ctrl-C·SIGTERM 이 오면 새 연결을 받지 않고, 처리 중인 요청을 2초까지 기다렸다 닫는다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for _, s := range servers {
			_ = s.Shutdown(sctx)
		}
	}()

	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.ListenAndServe() }()
	}
	log.Info().Str("addr", *addr).Str("admin", *admin).Str("upstream", u.String()).Msg("npgproxy listening")

	// 서버 하나라도 닫히면(종료 신호든 포트 충돌이든) 전체를 내린다.
	err = <-errc
	stop()
	// ListenAndServe 는 Shutdown 이 시작되자마자 돌아온다. 처리 중인 요청이 다 나가는 건
	// Shutdown 이 끝날 때라서 그것까지 기다려야 한다 (net/http server.go 의 Shutdown 주석).
	<-drained
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("npgproxy")
		os.Exit(1)
	}
}
