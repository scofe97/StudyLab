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
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync/atomic"
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

// withHealthz 는 손님 주소에 GET /healthz 를 붙인다. 평소 200 ok, 드레인이 시작되면 503 draining 이다.
// 앞단(Caddy 의 능동 검사 등)이 이 한 줄로 "곧 내려간다"를 알아챈다. /healthz 는 업스트림으로 넘기지 않고
// 접근 로그에도 남기지 않는다. 1초마다 오는 검사 요청이 손님 요청 줄에 섞이지 않게 한다.
func withHealthz(next http.Handler, draining *atomic.Bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if draining.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, "draining\n")
				return
			}
			_, _ = io.WriteString(w, "ok\n")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// drainThenShutdown 은 종료 신호 뒤 순서를 정한다: /healthz 를 503 으로 바꾸고, delay 동안 듣는 소켓을 연 채
// 요청을 계속 처리한 뒤, shutdown 을 부른다. delay 가 0 이면 기다림 없이 바로 shutdown 이다(옛 동작).
// 앞단이 "곧 내려간다"를 알아채고 이 서버를 빼는 시간을 벌어 주는 것이 목적이다 (ADR 0018).
func drainThenShutdown(servers []*http.Server, draining *atomic.Bool, delay, timeout time.Duration) error {
	draining.Store(true)
	if delay > 0 {
		time.Sleep(delay)
	}
	return shutdown(servers, timeout)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "손님 요청을 받을 주소")
	admin := flag.String("admin", "127.0.0.1:8081", "/debug/vars 를 내보낼 관리 주소, 빈 값이면 끈다")
	upstream := flag.String("upstream", "http://127.0.0.1:9090", "요청을 넘길 업스트림 주소")
	jsonLog := flag.Bool("json", false, "로그를 사람용 대신 JSON 한 줄로 쓴다 (기계로 집계할 때)")
	trustedFlag := flag.String("trusted-proxies", "", "X-Forwarded-For 를 이어 써도 되는 바로 앞 홉 대역, 쉼표로 나눈 CIDR (예: 127.0.0.1/32). 비우면 아무도 안 믿는다 (ADR 0017)")
	drainDelay := flag.Duration("drain-delay", 0, "종료 신호 뒤 이 시간 동안 듣는 소켓을 연 채 /healthz 만 503 draining 으로 바꾸고, 지나면 닫는다. 0 이면 바로 닫는다 (ADR 0018)")
	flag.Parse()

	u, err := url.Parse(*upstream)
	if err != nil || u.Scheme == "" || u.Host == "" {
		fmt.Fprintf(os.Stderr, "npgproxy: -upstream 은 http://host:port 꼴이어야 합니다: %q\n", *upstream)
		os.Exit(2)
	}

	trusted, err := proxy.ParseTrustedProxies(*trustedFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "npgproxy: -trusted-proxies: %v\n", err)
		os.Exit(2)
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano // 기본은 초 단위라 같은 초 안의 요청 순서가 안 보인다
	var log zerolog.Logger
	if *jsonLog {
		log = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"}).With().Timestamp().Logger()
	}

	h, vars := proxy.New(u, log, trusted)
	expvar.Publish("npgproxy", vars)

	var draining atomic.Bool
	front := newFrontServer(*addr, withHealthz(h, &draining))

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

	// Ctrl-C·SIGTERM 이 오면 -drain-delay 동안 /healthz 만 503 으로 알린 뒤, 새 연결을 받지 않고
	// 처리 중인 요청을 shutdownTimeout 까지 기다렸다 닫는다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failed := make(chan struct{}) // 신호가 아니라 서버 오류(포트 충돌 등)로 내려가는 때는 드레인을 기다리지 않는다
	drained := make(chan error, 1)
	go func() {
		<-ctx.Done()
		delay := *drainDelay
		select {
		case <-failed:
			delay = 0
		default:
			log.Info().Dur("drain_delay", delay).Msg("npgproxy: 드레인 시작 (/healthz 503)")
		}
		drained <- drainThenShutdown(servers, &draining, delay, shutdownTimeout)
	}()

	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.ListenAndServe() }()
	}
	log.Info().Str("addr", *addr).Str("admin", *admin).Str("upstream", u.String()).Msg("npgproxy listening")

	// 서버 하나라도 닫히면(종료 신호든 포트 충돌이든) 전체를 내린다.
	err = <-errc
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		close(failed)
	}
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
