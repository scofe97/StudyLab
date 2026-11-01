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

	"github.com/scofe97/runners-lab/net/npg/internal/proxy"
	upstreampkg "github.com/scofe97/runners-lab/net/npg/internal/upstream" // 이름 upstream 은 아래 -upstream 플래그 변수가 쓴다
)

// shutdownTimeout 은 종료 신호 뒤 처리 중인 요청을 기다리는 시간이다.
// 업스트림 요청 기한(15초)보다 길고 쿠버네티스 기본 유예(30초)보다 짧게 둔다 (ADR 0014 결정 3).
const shutdownTimeout = 20 * time.Second

// newFrontServer 는 손님용 서버를 만든다 (ADR 0014).
// 몸통 읽기와 응답 쓰기는 전체 시계(ReadTimeout·WriteTimeout) 대신 proxy 의 틈 시계가 맡으므로 둘 다 0 으로 둔다.
// WriteTimeout 은 손님 헤더를 읽은 직후 시작해 업스트림 기한 15초와 겹쳤다(J2).
func newFrontServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 2 * time.Second,  // 헤더를 다 받기까지 (Slowloris 막기)
		IdleTimeout:       60 * time.Second, // keep-alive 로 다음 요청을 기다리는 시간. 0 이면 Go 는 ReadTimeout 을 쓴다
	}
}

// shutdown 은 서버들을 정상 종료하고, timeout 안에 처리 중인 요청을 다 끝내지 못하면 그 오류를 돌려준다.
// net/http 의 Shutdown 은 기한이 지나면 ctx.Err() 를 돌려준다. 이것을 버리면 끊긴 요청이 있어도 exit 0 이 된다(J4).
func shutdown(servers []*http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var errs []error
	for _, s := range servers {
		errs = append(errs, s.Shutdown(ctx))
	}
	return errors.Join(errs...)
}

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

	front := newFrontServer(*addr, h)

	servers := []*http.Server{front}
	// 관측 화면(npgwatch)이 손으로 적은 값 대신 실제 기한을 읽도록 초 단위로 내보낸다. 0 은 기한 없음이다.
	// 서버 구조체에서 바로 읽으므로 위 설정을 고치면 화면도 따라 바뀐다.
	expvar.Publish("npgproxy_limits", expvar.Func(func() any {
		return map[string]float64{
			"client_read_header": front.ReadHeaderTimeout.Seconds(),
			"client_read":        front.ReadTimeout.Seconds(),
			"client_write":       front.WriteTimeout.Seconds(),
			"client_idle":        front.IdleTimeout.Seconds(),
			"client_read_gap":    proxy.ClientReadGap.Seconds(),
			"client_write_gap":   proxy.ClientWriteGap.Seconds(),
			"client_min_rate":    proxy.ClientMinRate, // 초가 아니라 초당 바이트
			"upstream_request":   upstreampkg.RequestTimeout.Seconds(),
			"shutdown":           shutdownTimeout.Seconds(),
		}
	}))
	if *admin != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /debug/vars", expvar.Handler())
		servers = append(servers, &http.Server{Addr: *admin, Handler: mux, ReadHeaderTimeout: 5 * time.Second})
	}

	// Ctrl-C·SIGTERM 이 오면 새 연결을 받지 않고, 처리 중인 요청을 shutdownTimeout 까지 기다렸다 닫는다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	drained := make(chan error, 1)
	go func() {
		<-ctx.Done()
		drained <- shutdown(servers, shutdownTimeout)
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
	// 종료 대기 안에 다 못 끝냈으면 끊긴 요청 수를 남기고 exit 1 로 알린다. exit 0 이면 배포 도구가 장애를 못 본다.
	if derr := <-drained; derr != nil {
		inflight := vars.(*expvar.Map).Get("inflight")
		log.Error().Err(derr).Str("inflight", inflight.String()).Msg("npgproxy: 종료 대기 시간 안에 처리 중인 요청을 다 끝내지 못했습니다")
		os.Exit(1)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("npgproxy")
		os.Exit(1)
	}
}
