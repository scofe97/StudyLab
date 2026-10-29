// Package upstream 은 외부 환율 제공자를 흉내 내는 가짜 API 다.
//
// 업무 엔드포인트 두 개(환율 조회, 환전 생성)와 관리 엔드포인트 두 개를 둔다.
// 장애는 PUT /_faults?키=값 으로 켜고 DELETE /_faults 로 되돌린다.
// 요청·연결 수는 GET /_stats 와 1초마다 찍는 보고 줄로 본다.
package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Faults 는 장애 주입 손잡이 묶음이다. 값이 0 이거나 false 면 그 손잡이는 꺼진다.
type Faults struct {
	// 요청마다 작업자 하나를 붙잡고 머무는 처리 시간.
	Latency time.Duration
	// 동시에 처리하는 요청 수 상한. 0 이면 상한이 없다.
	Workers int
	// 작업자를 기다리는 요청 수 상한. 넘치면 바로 ErrorStatus 로 답한다. 0 이면 상한이 없다.
	QueueLimit int
	// 작업자를 얻은 뒤 클라이언트가 이미 떠났으면 처리를 건너뛴다.
	HonorCancel bool
	// 상태 줄과 헤더를 보낸 뒤 본문을 보내기 전에 멈추는 시간.
	HeaderStall time.Duration
	// 본문을 절반만 보내고 연결을 끊는다.
	BodyCut bool
	// 이 비율(0~1)의 요청에 ErrorStatus 로 답한다.
	ErrorRate float64
	// 에러 응답과 대기열 넘침 응답의 상태 코드.
	ErrorStatus int
	// 에러 응답에 붙이는 Retry-After 초. 0 이면 붙이지 않는다.
	RetryAfter int
	// 이 비율(0~1)의 요청에서 응답 없이 연결을 RST 로 끊는다.
	ResetRate float64
	// 연결마다 N 번째 요청에서 응답 없이 연결을 RST 로 끊는다. 0 이면 끈다.
	ResetNth int
	// 끊는 시점. before 는 처리 전, after 는 처리(환전이면 기록)까지 마친 뒤.
	ResetWhen string
	// Content-Length 없이 청크 인코딩으로 보낸다.
	Chunked bool
	// JSON 뒤에 덧붙이는 공백 바이트 수.
	Trailing int
	// 환율 응답의 padding 필드에 채우는 바이트 수.
	BodySize int
	// 환율 응답에 ETag 를 붙이고 If-None-Match 를 비교한다.
	ETag bool
	// Cache-Control: max-age 초. 0 이면 no-cache 를 보낸다.
	MaxAge int
	// 환율 값이 바뀌는 주기. 0 이면 바뀌지 않는다.
	Rotate time.Duration
}

// DefaultFaults 는 DELETE /_faults 가 되돌리는 값이다.
func DefaultFaults() Faults {
	return Faults{
		Latency:     10 * time.Millisecond,
		ErrorStatus: http.StatusServiceUnavailable,
		ResetWhen:   "before",
		ETag:        true,
		Rotate:      30 * time.Second,
	}
}

// Apply 는 query 에 있는 키만 바꾼다. 모르는 키나 잘못된 값이 있으면 아무것도 바꾸지 않는다.
func (f Faults) Apply(q url.Values) (Faults, error) {
	out := f
	for key, vals := range q {
		v := vals[len(vals)-1]
		var err error
		switch key {
		case "latency":
			out.Latency, err = time.ParseDuration(v)
		case "workers":
			out.Workers, err = strconv.Atoi(v)
		case "queue_limit":
			out.QueueLimit, err = strconv.Atoi(v)
		case "honor_cancel":
			out.HonorCancel, err = strconv.ParseBool(v)
		case "header_stall":
			out.HeaderStall, err = time.ParseDuration(v)
		case "body_cut":
			out.BodyCut, err = strconv.ParseBool(v)
		case "error_rate":
			out.ErrorRate, err = strconv.ParseFloat(v, 64)
		case "error_status":
			out.ErrorStatus, err = strconv.Atoi(v)
		case "retry_after":
			out.RetryAfter, err = strconv.Atoi(v)
		case "reset_rate":
			out.ResetRate, err = strconv.ParseFloat(v, 64)
		case "reset_nth":
			out.ResetNth, err = strconv.Atoi(v)
		case "reset_when":
			if v != "before" && v != "after" {
				err = fmt.Errorf("before 또는 after")
			}
			out.ResetWhen = v
		case "chunked":
			out.Chunked, err = strconv.ParseBool(v)
		case "trailing":
			out.Trailing, err = strconv.Atoi(v)
		case "body_size":
			out.BodySize, err = strconv.Atoi(v)
		case "etag":
			out.ETag, err = strconv.ParseBool(v)
		case "max_age":
			out.MaxAge, err = strconv.Atoi(v)
		case "rotate":
			out.Rotate, err = time.ParseDuration(v)
		default:
			err = fmt.Errorf("모르는 키")
		}
		if err != nil {
			return f, fmt.Errorf("%s=%q: %w", key, v, err)
		}
	}
	return out, nil
}

// View 는 사람이 읽기 좋은 형태로 바꾼다.
func (f Faults) View() map[string]any {
	return map[string]any{
		"latency":      f.Latency.String(),
		"workers":      f.Workers,
		"queue_limit":  f.QueueLimit,
		"honor_cancel": f.HonorCancel,
		"header_stall": f.HeaderStall.String(),
		"body_cut":     f.BodyCut,
		"error_rate":   f.ErrorRate,
		"error_status": f.ErrorStatus,
		"retry_after":  f.RetryAfter,
		"reset_rate":   f.ResetRate,
		"reset_nth":    f.ResetNth,
		"reset_when":   f.ResetWhen,
		"chunked":      f.Chunked,
		"trailing":     f.Trailing,
		"body_size":    f.BodySize,
		"etag":         f.ETag,
		"max_age":      f.MaxAge,
		"rotate":       f.Rotate.String(),
	}
}

// Rates 는 환율 응답 본문이다.
type Rates struct {
	Base    string             `json:"base"`
	Version int64              `json:"version"`
	AsOf    string             `json:"as_of"`
	Rates   map[string]float64 `json:"rates"`
	Padding string             `json:"padding,omitempty"`
}

// ConversionRequest 는 환전 생성 요청 본문이다.
type ConversionRequest struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

// Conversion 은 업스트림이 기록한 환전 한 건이다.
type Conversion struct {
	ID             string  `json:"id"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Amount         float64 `json:"amount"`
	Rate           float64 `json:"rate"`
	Result         float64 `json:"result"`
	IdempotencyKey string  `json:"idempotency_key,omitempty"`
}

var currencies = []string{"USD", "EUR", "JPY", "KRW", "GBP", "CNY"}

func knownCurrency(c string) bool {
	for _, k := range currencies {
		if k == c {
			return true
		}
	}
	return false
}

type counters struct {
	requests     atomic.Int64
	connsNew     atomic.Int64
	connsOpen    atomic.Int64
	inflight     atomic.Int64
	maxInflight  atomic.Int64
	queued       atomic.Int64
	maxQueued    atomic.Int64
	queueReject  atomic.Int64
	skipped      atomic.Int64
	wasted       atomic.Int64
	resets       atomic.Int64
	cuts         atomic.Int64
	notModified  atomic.Int64
	bodyBytes    atomic.Int64
	convCreated  atomic.Int64
	convReplayed atomic.Int64

	mu       sync.Mutex
	byStatus map[int]int64
}

func (c *counters) status(code int) {
	c.mu.Lock()
	c.byStatus[code]++
	c.mu.Unlock()
}

func storeMax(m *atomic.Int64, v int64) {
	for {
		old := m.Load()
		if v <= old || m.CompareAndSwap(old, v) {
			return
		}
	}
}

// Server 는 가짜 업스트림의 상태(장애 설정·작업자·통계·환전 기록)를 갖는다.
type Server struct {
	start time.Time

	mu     sync.RWMutex
	faults Faults
	sem    chan struct{}

	st counters

	convMu  sync.Mutex
	convSeq int64
	byKey   map[string]Conversion
}

// New 는 기본 장애 설정으로 Server 를 만든다.
func New() *Server {
	s := &Server{start: time.Now(), byKey: map[string]Conversion{}}
	s.st.byStatus = map[int]int64{}
	s.setFaults(DefaultFaults())
	return s
}

func (s *Server) setFaults(f Faults) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Workers != s.faults.Workers || s.sem == nil {
		if f.Workers > 0 {
			s.sem = make(chan struct{}, f.Workers)
		} else {
			s.sem = nil
		}
	}
	s.faults = f
}

// SetFaults 는 장애 설정을 통째로 바꾼다. 테스트가 쓴다.
func (s *Server) SetFaults(f Faults) { s.setFaults(f) }

// Faults 는 지금 장애 설정을 돌려준다.
func (s *Server) Faults() Faults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.faults
}

func (s *Server) snapshot() (Faults, chan struct{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.faults, s.sem
}

type connReqKey struct{}

// Configure 는 http.Server 에 핸들러와 연결 훅을 건다.
func (s *Server) Configure(hs *http.Server) {
	hs.Handler = s.Handler()
	hs.ConnState = s.connState
	hs.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
		return context.WithValue(ctx, connReqKey{}, new(atomic.Int64))
	}
}

func (s *Server) connState(_ net.Conn, st http.ConnState) {
	switch st {
	case http.StateNew:
		s.st.connsNew.Add(1)
		s.st.connsOpen.Add(1)
	case http.StateClosed, http.StateHijacked:
		s.st.connsOpen.Add(-1)
	}
}

// Handler 는 업무·관리 엔드포인트를 묶은 핸들러다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/rates/{base}", s.business(s.rates))
	mux.HandleFunc("POST /v1/conversions", s.business(s.conversions))
	mux.HandleFunc("GET /_faults", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.Faults().View())
	})
	mux.HandleFunc("PUT /_faults", func(w http.ResponseWriter, r *http.Request) {
		f, err := s.Faults().Apply(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.setFaults(f)
		writeJSON(w, http.StatusOK, f.View())
	})
	mux.HandleFunc("DELETE /_faults", func(w http.ResponseWriter, _ *http.Request) {
		s.setFaults(DefaultFaults())
		writeJSON(w, http.StatusOK, s.Faults().View())
	})
	mux.HandleFunc("GET /_stats", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.Stats())
	})
	mux.HandleFunc("DELETE /_stats", func(w http.ResponseWriter, _ *http.Request) {
		s.ResetStats()
		writeJSON(w, http.StatusOK, s.Stats())
	})
	return mux
}

// statusWriter 는 업무 응답의 상태 코드와 본문 바이트 수를 센다.
type statusWriter struct {
	http.ResponseWriter
	code  int
	bytes int64
}

func (w *statusWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// business 는 업무 핸들러 앞에 공통 장애(대기열·처리 시간·에러·끊기)를 붙인다.
//
// h 의 quiet 가 true 면 처리(환전 기록)까지만 하고 응답은 쓰지 않는다. 그 뒤 business 가 연결을 끊는다.
func (s *Server) business(h func(w http.ResponseWriter, r *http.Request, f Faults, quiet bool) bool) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		f, sem := s.snapshot()
		s.st.requests.Add(1)
		w := &statusWriter{ResponseWriter: rw}
		outcome := 0
		defer func() {
			s.st.bodyBytes.Add(w.bytes)
			if v := recover(); v != nil {
				s.st.cuts.Add(1)
				panic(v)
			}
			if outcome != 0 {
				return
			}
			if w.code == 0 {
				w.code = http.StatusOK
			}
			s.st.status(w.code)
		}()

		nth := int64(0)
		if c, ok := r.Context().Value(connReqKey{}).(*atomic.Int64); ok {
			nth = c.Add(1)
		}
		resetNow := (f.ResetRate > 0 && rand.Float64() < f.ResetRate) ||
			(f.ResetNth > 0 && nth == int64(f.ResetNth))
		if resetNow && f.ResetWhen == "before" {
			outcome = 1
			s.reset(w)
			return
		}

		if sem != nil {
			if f.QueueLimit > 0 && s.st.queued.Load() >= int64(f.QueueLimit) {
				s.st.queueReject.Add(1)
				s.writeError(w, f)
				return
			}
			storeMax(&s.st.maxQueued, s.st.queued.Add(1))
			sem <- struct{}{}
			s.st.queued.Add(-1)
			defer func() { <-sem }()
			// 기다리는 동안 바뀐 설정을 처리에 쓴다.
			f, _ = s.snapshot()
			if f.HonorCancel && r.Context().Err() != nil {
				s.st.skipped.Add(1)
				outcome = 1
				return
			}
		}

		storeMax(&s.st.maxInflight, s.st.inflight.Add(1))
		defer s.st.inflight.Add(-1)
		if f.Latency > 0 {
			if f.HonorCancel {
				select {
				case <-time.After(f.Latency):
				case <-r.Context().Done():
				}
			} else {
				time.Sleep(f.Latency)
			}
		}
		if r.Context().Err() != nil {
			s.st.wasted.Add(1)
		}

		if f.ErrorRate > 0 && rand.Float64() < f.ErrorRate {
			s.writeError(w, f)
			return
		}
		resetAfter := resetNow && f.ResetWhen == "after"
		if !h(w, r, f, resetAfter) {
			return
		}
		if resetAfter {
			outcome = 1
			s.reset(w)
		}
	}
}

func (s *Server) writeError(w http.ResponseWriter, f Faults) {
	if f.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(f.RetryAfter))
	}
	code := f.ErrorStatus
	if code == 0 {
		code = http.StatusServiceUnavailable
	}
	http.Error(w, http.StatusText(code), code)
}

// reset 은 응답을 보내지 않고 연결을 가로채 SO_LINGER 0 으로 닫는다.
func (s *Server) reset(w http.ResponseWriter) {
	s.st.resets.Add(1)
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		panic(http.ErrAbortHandler)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetLinger(0)
	}
	conn.Close()
}

// RatesFor 는 base 와 version 으로 정해지는 환율 표를 만든다.
func RatesFor(base string, version int64) map[string]float64 {
	out := map[string]float64{}
	for _, c := range currencies {
		if c == base {
			continue
		}
		h := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%d", base, c, version)))
		frac := float64(binary.BigEndian.Uint16(h[:2])) / 65535
		out[c] = float64(int((0.5+frac*2)*10000)) / 10000
	}
	return out
}

func (s *Server) version(f Faults) int64 {
	if f.Rotate <= 0 {
		return 0
	}
	return int64(time.Since(s.start) / f.Rotate)
}

func etagFor(base string, version int64, size int) string {
	return fmt.Sprintf(`"%s-v%d-%d"`, base, version, size)
}

func matchETag(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		p = strings.TrimPrefix(p, "W/")
		if p == "*" || p == etag {
			return true
		}
	}
	return false
}

func (s *Server) rates(w http.ResponseWriter, r *http.Request, f Faults, quiet bool) bool {
	base := r.PathValue("base")
	if !knownCurrency(base) {
		http.Error(w, "unknown currency", http.StatusNotFound)
		return false
	}
	if quiet {
		return true
	}
	v := s.version(f)
	if f.MaxAge > 0 {
		w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(f.MaxAge))
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if f.ETag {
		tag := etagFor(base, v, f.BodySize)
		w.Header().Set("ETag", tag)
		if inm := r.Header.Get("If-None-Match"); inm != "" && matchETag(inm, tag) {
			s.st.notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	body := Rates{
		Base:    base,
		Version: v,
		AsOf:    s.start.Add(time.Duration(v) * f.Rotate).UTC().Format(time.RFC3339),
		Rates:   RatesFor(base, v),
		Padding: strings.Repeat("x", f.BodySize),
	}
	b, _ := json.Marshal(body)
	b = append(b, '\n')
	if f.Trailing > 0 {
		b = append(b, []byte(strings.Repeat(" ", f.Trailing))...)
	}
	s.writeBody(w, f, b)
	return true
}

func (s *Server) conversions(w http.ResponseWriter, r *http.Request, f Faults, quiet bool) bool {
	var req ConversionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return false
	}
	if !knownCurrency(req.From) || !knownCurrency(req.To) || req.From == req.To {
		http.Error(w, "bad currency pair", http.StatusUnprocessableEntity)
		return false
	}
	key := r.Header.Get("Idempotency-Key")

	s.convMu.Lock()
	if prev, ok := s.byKey[key]; ok && key != "" {
		s.convMu.Unlock()
		s.st.convReplayed.Add(1)
		if quiet {
			return true
		}
		w.Header().Set("Idempotent-Replayed", "true")
		b, _ := json.Marshal(prev)
		s.writeBody(w, f, append(b, '\n'))
		return true
	}
	s.convSeq++
	rate := RatesFor(req.From, s.version(f))[req.To]
	c := Conversion{
		ID:             fmt.Sprintf("cv-%06d", s.convSeq),
		From:           req.From,
		To:             req.To,
		Amount:         req.Amount,
		Rate:           rate,
		Result:         req.Amount * rate,
		IdempotencyKey: key,
	}
	if key != "" {
		s.byKey[key] = c
	}
	s.convMu.Unlock()
	s.st.convCreated.Add(1)

	if quiet {
		return true
	}
	b, _ := json.Marshal(c)
	w.Header().Set("Location", "/v1/conversions/"+c.ID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)+1))
	w.WriteHeader(http.StatusCreated)
	w.Write(append(b, '\n'))
	return true
}

// writeBody 는 청크·헤더 멈춤·본문 끊기 손잡이를 적용해 200 본문을 쓴다.
func (s *Server) writeBody(w http.ResponseWriter, f Faults, b []byte) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "application/json")
	if !f.Chunked {
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	}
	w.WriteHeader(http.StatusOK)
	if f.HeaderStall > 0 {
		rc.Flush()
		time.Sleep(f.HeaderStall)
	}
	if f.BodyCut {
		w.Write(b[:len(b)/2])
		rc.Flush()
		panic(http.ErrAbortHandler)
	}
	if f.Chunked {
		half := len(b) / 2
		w.Write(b[:half])
		rc.Flush()
		w.Write(b[half:])
		return
	}
	w.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// Stats 는 누적 통계를 돌려준다.
func (s *Server) Stats() map[string]any {
	s.st.mu.Lock()
	by := map[string]int64{}
	for k, v := range s.st.byStatus {
		by[strconv.Itoa(k)] = v
	}
	s.st.mu.Unlock()
	return map[string]any{
		"requests":             s.st.requests.Load(),
		"by_status":            by,
		"conns_new":            s.st.connsNew.Load(),
		"conns_open":           s.st.connsOpen.Load(),
		"inflight":             s.st.inflight.Load(),
		"max_inflight":         s.st.maxInflight.Load(),
		"queued":               s.st.queued.Load(),
		"max_queued":           s.st.maxQueued.Load(),
		"queue_rejected":       s.st.queueReject.Load(),
		"skipped_after_cancel": s.st.skipped.Load(),
		"worked_after_cancel":  s.st.wasted.Load(),
		"resets":               s.st.resets.Load(),
		"body_cuts":            s.st.cuts.Load(),
		"not_modified":         s.st.notModified.Load(),
		"body_bytes":           s.st.bodyBytes.Load(),
		"conversions_created":  s.st.convCreated.Load(),
		"conversions_replayed": s.st.convReplayed.Load(),
		"uptime":               time.Since(s.start).Truncate(time.Millisecond).String(),
	}
}

// ResetStats 는 누적 통계를 0 으로 되돌린다. 열린 연결 수·처리 중·대기 중은 지금 값이라 그대로 둔다.
func (s *Server) ResetStats() {
	for _, c := range []*atomic.Int64{
		&s.st.requests, &s.st.connsNew, &s.st.maxInflight, &s.st.maxQueued,
		&s.st.queueReject, &s.st.skipped, &s.st.wasted, &s.st.resets, &s.st.cuts,
		&s.st.notModified, &s.st.bodyBytes, &s.st.convCreated, &s.st.convReplayed,
	} {
		c.Store(0)
	}
	s.st.mu.Lock()
	s.st.byStatus = map[int]int64{}
	s.st.mu.Unlock()
}

// Report 는 every 마다 지난 구간의 요청 수와 지금 상태를 한 줄로 쓴다.
func (s *Server) Report(ctx context.Context, w io.Writer, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	var lastReq, lastNew, lastReset int64
	lastBy := map[int]int64{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		req, nw, rs := s.st.requests.Load(), s.st.connsNew.Load(), s.st.resets.Load()
		s.st.mu.Lock()
		codes := make([]int, 0, len(s.st.byStatus))
		for k := range s.st.byStatus {
			codes = append(codes, k)
		}
		sort.Ints(codes)
		var parts []string
		for _, k := range codes {
			if d := s.st.byStatus[k] - lastBy[k]; d > 0 {
				parts = append(parts, fmt.Sprintf("%d=%d", k, d))
			}
			lastBy[k] = s.st.byStatus[k]
		}
		s.st.mu.Unlock()
		if req < lastReq {
			lastReq, lastNew, lastReset = 0, 0, 0
		}
		fmt.Fprintf(w, "upstream %s req=%d [%s] resets=%d inflight=%d queued=%d conns_open=%d conns_new=%d\n",
			time.Now().Format("15:04:05"), req-lastReq, strings.Join(parts, " "), rs-lastReset,
			s.st.inflight.Load(), s.st.queued.Load(), s.st.connsOpen.Load(), nw-lastNew)
		lastReq, lastNew, lastReset = req, nw, rs
	}
}
