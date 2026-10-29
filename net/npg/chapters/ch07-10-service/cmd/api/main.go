// api 는 사내 「환율 조회 API」 를 띄운다.
//
//	go run ./ch07-10-service/cmd/api -addr 127.0.0.1:8081 -upstream http://127.0.0.1:9090 \
//	    -trusted-proxies 127.0.0.1/32 -admin-socket /tmp/npgsvc/api-8081.sock
//
// -profile 은 플래그 묶음이다. 명령줄에 직접 준 플래그는 묶음보다 앞선다.
// 플래그 기본값(-h 에 보이는 값)은 guarded 묶음의 값이다.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/admin"
	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/api"
	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/clientip"
	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/notify"
	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/upclient"
)

// bareProfile 은 Go 기본값에 가까운 묶음이다. 타임아웃·재시도·캐시·드레인을 걸지 않는다.
var bareProfile = map[string]string{
	"read-header-timeout": "0", "read-timeout": "0", "write-timeout": "0", "idle-timeout": "0",
	"handler-timeout": "0", "graceful": "false", "drain-delay": "0", "shutdown-timeout": "0",
	"dial-timeout": "0", "response-header-timeout": "0", "attempt-timeout": "0", "total-timeout": "0",
	"max-conns-per-host": "0", "max-idle-per-host": "0", "idle-conn-timeout": "0",
	"retries": "0", "backoff": "none", "jitter": "none", "retry-budget": "0",
	"honor-retry-after": "false", "retry-post": "false", "idempotency-key": "false", "cache": "off",
}

func main() {
	fs := flag.CommandLine
	// edge 창과의 계약 플래그
	addr := fs.String("addr", "127.0.0.1:8081", "들을 TCP 주소")
	upURL := fs.String("upstream", "http://127.0.0.1:9090", "업스트림 기준 URL")
	trusted := fs.String("trusted-proxies", "", "X-Forwarded-For 를 믿을 프록시 대역 (쉼표 목록)")
	adminSock := fs.String("admin-socket", "", "관리 소켓 경로 (비우면 관리 채널을 열지 않음)")

	profile := fs.String("profile", "guarded", "플래그 묶음: guarded | bare")
	instance := fs.String("instance", "", "X-Instance 헤더 값 (비우면 -addr)")
	logLevel := fs.String("log-level", "info", "debug | info | warn | error")
	xffPick := fs.String("xff-pick", "rightmost", "X-Forwarded-For 에서 고르는 방향: rightmost | leftmost")
	denyIPs := fs.String("deny-ips", "", "403 으로 막을 클라이언트 IP 대역 (쉼표 목록)")
	debugAllow := fs.String("debug-allow", "127.0.0.1/32,::1/128", "/debug/ 를 열어 줄 클라이언트 IP 대역")

	// 서버
	readHeaderTO := fs.Duration("read-header-timeout", 2*time.Second, "http.Server.ReadHeaderTimeout")
	readTO := fs.Duration("read-timeout", 10*time.Second, "http.Server.ReadTimeout")
	writeTO := fs.Duration("write-timeout", 15*time.Second, "http.Server.WriteTimeout")
	idleTO := fs.Duration("idle-timeout", 60*time.Second, "http.Server.IdleTimeout")
	maxConns := fs.Int("max-conns", 0, "동시에 받을 연결 수 상한 (0 이면 없음)")
	handlerTO := fs.Duration("handler-timeout", 3*time.Second, "http.TimeoutHandler 시간 (0 이면 감싸지 않음)")
	logPos := fs.String("log-position", "outer", "접근 로그 자리: outer | inner (TimeoutHandler 기준)")
	propagate := fs.Bool("propagate-ctx", true, "업스트림 호출에 요청 context 를 넘긴다")

	// 종료
	graceful := fs.Bool("graceful", true, "SIGTERM 을 받아 드레인 뒤 Shutdown 한다 (false 면 신호를 받지 않는다)")
	drainDelay := fs.Duration("drain-delay", 5*time.Second, "SIGTERM 뒤 Shutdown 을 부르기까지 기다리는 시간")
	shutdownTO := fs.Duration("shutdown-timeout", 15*time.Second, "Shutdown 에 주는 시간 (0 이면 끝날 때까지)")
	drainNoKA := fs.Bool("drain-close-keepalive", false, "드레인을 시작하면 SetKeepAlivesEnabled(false)")

	// 업스트림 클라이언트
	dialTO := fs.Duration("dial-timeout", 500*time.Millisecond, "연결 맺기 시한 (0 이면 Go 기본)")
	respHdrTO := fs.Duration("response-header-timeout", time.Second, "Transport.ResponseHeaderTimeout (0 이면 없음)")
	attemptTO := fs.Duration("attempt-timeout", time.Second, "시도당 context 기한 (0 이면 없음)")
	totalTO := fs.Duration("total-timeout", 2500*time.Millisecond, "호출 전체 context 기한 (0 이면 없음)")
	maxConnsPerHost := fs.Int("max-conns-per-host", 64, "Transport.MaxConnsPerHost (0 이면 없음)")
	maxIdlePerHost := fs.Int("max-idle-per-host", 64, "Transport.MaxIdleConnsPerHost (0 이면 Go 기본)")
	idleConnTO := fs.Duration("idle-conn-timeout", 90*time.Second, "Transport.IdleConnTimeout (0 이면 Go 기본)")
	noKA := fs.Bool("disable-keepalives", false, "Transport.DisableKeepAlives")
	bodyMode := fs.String("body-mode", "drain", "응답 본문 정리: drain | close | leak")
	retries := fs.Int("retries", 2, "첫 시도 뒤 더 하는 시도 수")
	backoff := fs.String("backoff", "exp", "none | exp")
	backoffBase := fs.Duration("backoff-base", 50*time.Millisecond, "exp 의 첫 대기")
	backoffMax := fs.Duration("backoff-max", 500*time.Millisecond, "exp 의 대기 상한")
	jitter := fs.String("jitter", "full", "none | full")
	budget := fs.Float64("retry-budget", 0.2, "요청 하나가 쌓는 재시도 토큰 (0 이면 예산 없음, 상한 10)")
	honorRA := fs.Bool("honor-retry-after", true, "Retry-After 만큼은 기다린다")
	retryPOST := fs.Bool("retry-post", false, "POST 도 재시도한다")
	idemKey := fs.Bool("idempotency-key", true, "업스트림 POST 에 Idempotency-Key 를 붙인다")
	cache := fs.String("cache", "etag", "off | etag")
	warmup := fs.String("warmup", "", "시작할 때 미리 받아 둘 통화 (쉼표 목록)")

	// 관리 소켓
	adminPerm := fs.String("admin-perm", "0660", "소켓 파일 권한 (8진수, 비우면 손대지 않음)")
	adminGroup := fs.String("admin-group", "", "소켓 파일 그룹")
	adminStale := fs.String("admin-stale", "probe", "남은 소켓 파일 처리: fail | remove | probe")
	adminAllow := fs.String("admin-allow-groups", "", "SO_PEERCRED 로 허용할 그룹 (쉼표 목록, 비우면 인가 끔, Linux 전용)")
	adminPeerCred := fs.String("admin-peercred", "rawconn", "자격 증명 읽기: rawconn | file | file-noclose")
	adminAuthzBy := fs.String("admin-authz-by", "uid-groups", "판정 기준: uid-groups | gid")

	// 준비 알림
	notifyLocal := fs.String("notify-local", "", "NOTIFY_SOCKET 으로 보낼 때 묶을 경로 (비우면 경로 없이)")
	notifyAck := fs.Duration("notify-ack-wait", 0, "READY=1 을 보낸 뒤 답장을 기다리는 시간 (0 이면 기다리지 않음)")

	fs.Parse(os.Args[1:])

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch *profile {
	case "guarded":
	case "bare":
		for k, v := range bareProfile {
			if !set[k] {
				fs.Set(k, v)
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "-profile: %q\n", *profile)
		os.Exit(2)
	}
	if *instance == "" {
		*instance = *addr
	}
	view := map[string]any{}
	fs.VisitAll(func(f *flag.Flag) { view[f.Name] = f.Value.String() })

	var level slog.LevelVar
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		fatal("-log-level: %v", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &level})).With("inst", *instance)

	up, err := upclient.New(upclient.Config{
		BaseURL:               strings.TrimRight(*upURL, "/"),
		DialTimeout:           *dialTO,
		ResponseHeaderTimeout: *respHdrTO,
		MaxConnsPerHost:       *maxConnsPerHost,
		MaxIdleConnsPerHost:   *maxIdlePerHost,
		IdleConnTimeout:       *idleConnTO,
		DisableKeepAlives:     *noKA,
		AttemptTimeout:        *attemptTO,
		TotalTimeout:          *totalTO,
		BodyMode:              *bodyMode,
		Retries:               *retries,
		Backoff:               *backoff,
		BackoffBase:           *backoffBase,
		BackoffMax:            *backoffMax,
		Jitter:                *jitter,
		RetryBudget:           *budget,
		HonorRetryAfter:       *honorRA,
		RetryPOST:             *retryPOST,
		IdempotencyKey:        *idemKey,
		Cache:                 *cache,
		Logger:                log,
	})
	if err != nil {
		fatal("upstream client: %v", err)
	}
	trustedList, err := clientip.ParsePrefixes(*trusted)
	if err != nil {
		fatal("-trusted-proxies: %v", err)
	}
	ips, err := clientip.New(trustedList, *xffPick)
	if err != nil {
		fatal("-xff-pick: %v", err)
	}
	deny, err := clientip.ParsePrefixes(*denyIPs)
	if err != nil {
		fatal("-deny-ips: %v", err)
	}
	dbg, err := clientip.ParsePrefixes(*debugAllow)
	if err != nil {
		fatal("-debug-allow: %v", err)
	}

	var limited *api.Limited
	app, err := api.New(api.Options{
		Upstream:       up,
		IPs:            ips,
		DenyIPs:        deny,
		DebugAllow:     dbg,
		HandlerTimeout: *handlerTO,
		LogPosition:    *logPos,
		PropagateCtx:   *propagate,
		Instance:       *instance,
		Logger:         log,
		ConfigView:     view,
		ListenerView: func() map[string]any {
			if limited == nil {
				return map[string]any{"max_conns": 0}
			}
			return map[string]any{"max_conns": limited.Cap(), "in_use": limited.InUse(), "accept_waiting": limited.Waiting()}
		},
	})
	if err != nil {
		fatal("app: %v", err)
	}

	for _, b := range strings.Split(*warmup, ",") {
		if b = strings.TrimSpace(strings.ToUpper(b)); b == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		start := time.Now()
		_, res, err := up.GetRates(ctx, b, "warmup")
		cancel()
		log.Info("warmup", "base", b, "status", res.Status, "cache", res.Cache, "dur", time.Since(start).Truncate(time.Millisecond), "err", errString(err))
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fatal("listen: %v", err)
	}
	if *maxConns > 0 {
		limited = api.LimitListener(ln, *maxConns)
		ln = limited
	}
	srv := &http.Server{
		Handler:           app.Handler(),
		ReadHeaderTimeout: *readHeaderTO,
		ReadTimeout:       *readTO,
		WriteTimeout:      *writeTO,
		IdleTimeout:       *idleTO,
		ConnState:         app.ConnState,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	drainReq := make(chan string, 1)
	requestDrain := func(why string) {
		select {
		case drainReq <- why:
		default:
		}
	}

	var adm *admin.Server
	if *adminSock != "" {
		var perm os.FileMode
		if *adminPerm != "" {
			p, err := strconv.ParseUint(*adminPerm, 8, 32)
			if err != nil {
				fatal("-admin-perm: %v", err)
			}
			perm = os.FileMode(p)
		}
		var groups []string
		for _, g := range strings.Split(*adminAllow, ",") {
			if g = strings.TrimSpace(g); g != "" {
				groups = append(groups, g)
			}
		}
		adm, err = admin.Listen(admin.Options{
			Path: *adminSock, Perm: perm, Group: *adminGroup, Stale: *adminStale,
			AllowGroups: groups, PeerCred: *adminPeerCred, AuthzBy: *adminAuthzBy,
		}, admin.Commands{
			Status: func() any { return app.Vars() },
			Drain:  func() { requestDrain("admin") },
			Level:  func() string { return level.Level().String() },
			SetLevel: func(s string) error {
				return level.UnmarshalText([]byte(s))
			},
		}, log)
		if err != nil {
			ln.Close()
			fatal("admin: %v", err)
		}
		go func() {
			if err := adm.Serve(); err != nil {
				log.Error("admin serve", "err", err.Error())
			}
		}()
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	log.Info("api listening", "addr", ln.Addr().String(), "profile", *profile, "upstream", *upURL,
		"trusted_proxies", *trusted, "admin_socket", *adminSock, "pid", os.Getpid())

	sender := notify.Sender{LocalPath: *notifyLocal, AckWait: *notifyAck}
	if ack, err := sender.Send("READY=1\nMAINPID=" + strconv.Itoa(os.Getpid())); err != nil || ack != "" {
		log.Info("notify READY=1", "ack", ack, "err", errString(err))
	}

	sig := make(chan os.Signal, 2)
	if *graceful {
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	}

	var why string
	select {
	case s := <-sig:
		why = s.String()
	case why = <-drainReq:
	case err := <-serveErr:
		fatal("serve: %v", err)
	}

	if !*graceful {
		log.Info("stop without drain", "why", why, "inflight", app.Inflight())
		srv.Close()
		if adm != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			adm.Shutdown(ctx)
			cancel()
		}
		os.Exit(0)
	}

	go func() {
		s := <-sig
		log.Warn("second signal, exit now", "signal", s.String(), "inflight", app.Inflight())
		os.Exit(1)
	}()

	begin := time.Now()
	log.Info("drain start", "why", why, "inflight", app.Inflight(), "drain_delay", drainDelay.String())
	app.StartDrain()
	if *drainNoKA {
		srv.SetKeepAlivesEnabled(false)
	}
	notify.Sender{}.Send("STOPPING=1\nSTATUS=draining")
	if *drainDelay > 0 {
		t := time.NewTicker(time.Second)
		end := time.After(*drainDelay)
	wait:
		for {
			select {
			case <-end:
				break wait
			case <-t.C:
				log.Info("draining", "elapsed", time.Since(begin).Truncate(time.Second), "inflight", app.Inflight())
			}
		}
		t.Stop()
	}

	ctx := context.Background()
	if *shutdownTO > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *shutdownTO)
		defer cancel()
	}
	log.Info("shutdown start", "inflight", app.Inflight())
	err = srv.Shutdown(ctx)
	log.Info("shutdown done", "err", errString(err), "inflight", app.Inflight(), "total", time.Since(begin).Truncate(time.Millisecond))
	if adm != nil {
		actx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		adm.Shutdown(actx)
		cancel()
	}
	up.CloseIdle()
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "api: "+format+"\n", args...)
	os.Exit(1)
}
