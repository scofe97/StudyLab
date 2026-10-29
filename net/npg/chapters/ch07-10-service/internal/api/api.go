// Package api 는 사내 「환율 조회 API」 의 핸들러와 미들웨어다.
//
// 의존성(업스트림 클라이언트, 클라이언트 IP 계산기, 로거)은 App 구조체 필드로 받는다.
// 요청은 바깥부터 아래 순서로 미들웨어를 지난다.
//
//	recover → requestID → clientIP → [accessLog: outer] → inflight → deny → restrictPrefix(/debug/)
//	  → [TimeoutHandler] → [accessLog: inner] → ServeMux
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/clientip"
	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/upclient"
)

// Options 는 App 이 받는 의존성과 설정이다.
type Options struct {
	Upstream *upclient.Client
	IPs      *clientip.Resolver
	// 이 대역의 클라이언트 IP 는 403 으로 막는다.
	DenyIPs []netip.Prefix
	// /debug/ 아래는 이 대역의 클라이언트 IP 만 들어온다.
	DebugAllow []netip.Prefix
	// 0 보다 크면 ServeMux 를 http.TimeoutHandler 로 감싼다.
	HandlerTimeout time.Duration
	// 접근 로그 미들웨어 자리. outer 는 TimeoutHandler 바깥, inner 는 안쪽.
	LogPosition string
	// true 면 업스트림 호출에 요청 context 를 넘기고, false 면 취소를 떼어 낸 context 를 넘긴다.
	PropagateCtx bool
	// 응답 헤더 X-Instance 에 싣는 이름.
	Instance string
	Logger   *slog.Logger
	// /debug/vars 에 함께 싣는 설정 요약.
	ConfigView map[string]any
	// /debug/vars 의 listener 칸을 채우는 함수. nil 이면 칸을 비운다.
	ListenerView func() map[string]any
}

// App 은 핸들러들이 공유하는 상태다.
type App struct {
	opt      Options
	log      *slog.Logger
	started  time.Time
	draining atomic.Bool
	inflight atomic.Int64
	conns    connGauge
}

// New 는 App 을 만든다.
func New(opt Options) (*App, error) {
	if opt.LogPosition == "" {
		opt.LogPosition = "outer"
	}
	if opt.LogPosition != "outer" && opt.LogPosition != "inner" {
		return nil, fmt.Errorf("log position: %q", opt.LogPosition)
	}
	if opt.Logger == nil {
		opt.Logger = slog.New(slog.DiscardHandler)
	}
	if opt.IPs == nil {
		opt.IPs, _ = clientip.New(nil, "rightmost")
	}
	return &App{opt: opt, log: opt.Logger, started: time.Now(), conns: connGauge{states: map[net.Conn]http.ConnState{}}}, nil
}

// StartDrain 은 /healthz 를 503 draining 으로 바꾼다.
func (a *App) StartDrain() { a.draining.Store(true) }

// Draining 은 드레인 중인지 알려 준다.
func (a *App) Draining() bool { return a.draining.Load() }

// Inflight 는 처리 중인 요청 수다.
func (a *App) Inflight() int64 { return a.inflight.Load() }

// ConnState 는 http.Server.ConnState 에 건다.
func (a *App) ConnState(c net.Conn, st http.ConnState) { a.conns.set(c, st) }

// Handler 는 미들웨어로 감싼 전체 핸들러다.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /whoami", a.whoami)
	mux.HandleFunc("GET /api/v1/rates/{base}", a.rates)
	mux.HandleFunc("POST /api/v1/conversions", a.conversions)
	mux.HandleFunc("GET /debug/vars", a.debugVars)

	var h http.Handler = mux
	if a.opt.LogPosition == "inner" {
		h = a.accessLog("inner", h)
	}
	if a.opt.HandlerTimeout > 0 {
		h = http.TimeoutHandler(h, a.opt.HandlerTimeout, "handler timeout\n")
	}
	h = restrictPrefix("/debug/", a.opt.DebugAllow, h)
	h = a.deny(h)
	h = a.countInflight(h)
	if a.opt.LogPosition == "outer" {
		h = a.accessLog("outer", h)
	}
	h = a.clientIP(h)
	h = requestID(h)
	h = a.recover(h)
	return h
}

// ---- 문맥 값 ----

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyClientIP
)

// RequestID 는 요청 ID 를 꺼낸다.
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(keyRequestID).(string)
	return s
}

// ClientIP 는 clientIP 미들웨어가 정한 IP 를 꺼낸다.
func ClientIP(ctx context.Context) netip.Addr {
	a, _ := ctx.Value(keyClientIP).(netip.Addr)
	return a
}

// ---- 미들웨어 ----

// recorder 는 상태 코드·바이트 수·첫 쓰기 에러를 기록한다.
type recorder struct {
	http.ResponseWriter
	code     int
	bytes    int64
	writeErr error
}

func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	if err != nil && r.writeErr == nil {
		r.writeErr = err
	}
	return n, err
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (a *App) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			a.log.Error("panic", "req_id", RequestID(r.Context()), "value", fmt.Sprint(v))
			http.Error(w, "internal error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

func validRequestID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c == '-' || c == '_' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = upclient.NewIdempotencyKey()[:16]
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyRequestID, id)))
	})
}

func (a *App) clientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := a.opt.IPs.ClientIP(r.RemoteAddr, r.Header.Values("X-Forwarded-For"))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyClientIP, ip)))
	})
}

func (a *App) accessLog(position string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.code == 0 {
			rec.code = http.StatusOK
		}
		attrs := []any{
			"log_at", position, "req_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path,
			"status", rec.code, "bytes", rec.bytes, "dur", time.Since(start).Truncate(time.Microsecond),
			"client_ip", ClientIP(r.Context()).String(), "remote", r.RemoteAddr,
		}
		if rec.writeErr != nil {
			attrs = append(attrs, "write_err", rec.writeErr.Error())
		}
		if err := r.Context().Err(); err != nil {
			attrs = append(attrs, "ctx_err", err.Error())
		}
		a.log.Info("access", attrs...)
	})
}

func (a *App) countInflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.inflight.Add(1)
		defer a.inflight.Add(-1)
		next.ServeHTTP(w, r)
	})
}

func (a *App) deny(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := ClientIP(r.Context()); ip.IsValid() && clientip.Contains(a.opt.DenyIPs, ip) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// restrictPrefix 는 prefix 로 시작하는 경로를 allow 대역의 클라이언트에게만 연다.
func restrictPrefix(prefix string, allow []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, prefix) {
			ip := ClientIP(r.Context())
			if !ip.IsValid() || !clientip.Contains(allow, ip) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---- 핸들러 ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)+1))
	w.WriteHeader(code)
	w.Write(append(b, '\n'))
}

func (a *App) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Instance", a.opt.Instance)
	if a.Draining() {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, "draining\n")
		return
	}
	io.WriteString(w, "ok\n")
}

func (a *App) whoami(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r.Context())
	cip := ""
	if ip.IsValid() {
		cip = ip.String()
	}
	w.Header().Set("X-Instance", a.opt.Instance)
	writeJSON(w, http.StatusOK, map[string]string{
		"remote_addr":     r.RemoteAddr,
		"client_ip":       cip,
		"x_forwarded_for": strings.Join(r.Header.Values("X-Forwarded-For"), ", "),
	})
}

func (a *App) upstreamCtx(r *http.Request) context.Context {
	if a.opt.PropagateCtx {
		return r.Context()
	}
	return context.WithoutCancel(r.Context())
}

func upstreamErrorStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

func validCurrency(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func (a *App) rates(w http.ResponseWriter, r *http.Request) {
	base := strings.ToUpper(r.PathValue("base"))
	if !validCurrency(base) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base 는 통화 코드 세 글자"})
		return
	}
	if a.opt.Upstream == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "upstream 미설정"})
		return
	}
	rates, res, err := a.opt.Upstream.GetRates(a.upstreamCtx(r), base, RequestID(r.Context()))
	w.Header().Set("X-Instance", a.opt.Instance)
	w.Header().Set("X-Upstream-Attempts", strconv.Itoa(res.Attempts))
	w.Header().Set("X-Upstream-Cache", res.Cache)
	if err != nil {
		writeJSON(w, upstreamErrorStatus(err), map[string]string{"error": "upstream: " + err.Error()})
		return
	}
	switch res.Status {
	case http.StatusOK:
		writeJSON(w, http.StatusOK, rates)
	case http.StatusNotFound:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown currency"})
	default:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream status " + strconv.Itoa(res.Status)})
	}
}

func (a *App) conversions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body"})
		return
	}
	if a.opt.Upstream == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "upstream 미설정"})
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = upclient.NewIdempotencyKey()
	}
	res, err := a.opt.Upstream.CreateConversion(a.upstreamCtx(r), body, key, RequestID(r.Context()))
	w.Header().Set("X-Instance", a.opt.Instance)
	w.Header().Set("X-Upstream-Attempts", strconv.Itoa(res.Attempts))
	if err != nil {
		writeJSON(w, upstreamErrorStatus(err), map[string]string{"error": "upstream: " + err.Error()})
		return
	}
	if v := res.Header.Get("Idempotent-Replayed"); v != "" {
		w.Header().Set("Idempotent-Replayed", v)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.Status)
	w.Write(res.Body)
}

// CountFDs 는 이 프로세스가 연 파일 디스크립터 수를 센다. 셀 수 없으면 -1 이다.
func CountFDs() int {
	ents, err := os.ReadDir("/dev/fd")
	if err != nil {
		return -1
	}
	return len(ents) - 1
}

// Vars 는 /debug/vars 본문이다. admin status 도 같은 값을 쓴다.
func (a *App) Vars() map[string]any {
	up := map[string]int64{}
	if a.opt.Upstream != nil {
		up = a.opt.Upstream.Snapshot()
	}
	var lv map[string]any
	if a.opt.ListenerView != nil {
		lv = a.opt.ListenerView()
	}
	return map[string]any{
		"instance":     a.opt.Instance,
		"pid":          os.Getpid(),
		"uptime":       time.Since(a.started).Truncate(time.Millisecond).String(),
		"draining":     a.Draining(),
		"inflight":     a.Inflight(),
		"goroutines":   runtime.NumGoroutine(),
		"fds":          CountFDs(),
		"server_conns": a.conns.snapshot(),
		"listener":     lv,
		"upstream":     up,
		"config":       a.opt.ConfigView,
	}
}

func (a *App) debugVars(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(a.Vars())
}

// ---- 서버 쪽 연결 계측 ----

type connGauge struct {
	mu       sync.Mutex
	states   map[net.Conn]http.ConnState
	newTotal int64
}

func (g *connGauge) set(c net.Conn, st http.ConnState) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch st {
	case http.StateNew:
		g.newTotal++
		g.states[c] = st
	case http.StateClosed, http.StateHijacked:
		delete(g.states, c)
	default:
		g.states[c] = st
	}
}

func (g *connGauge) snapshot() map[string]int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := map[string]int64{"open": int64(len(g.states)), "accepted_total": g.newTotal}
	for _, st := range g.states {
		out[st.String()]++
	}
	return out
}
