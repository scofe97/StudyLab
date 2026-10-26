// Command fakeinv 는 장애 훈련용 가짜 장비 인벤토리 백엔드입니다.
//
// npgproxy 뒤에 세워 두고 /_faults 손잡이로 느려지기·과부하·에러를 켭니다.
// 구조는 npg-practice ch07-10-service 의 internal/upstream(장애 손잡이·통계·보고 줄)을 참고했습니다.
//
//	GET  /v1/devices/{id}   장비 단건 조회 (get_latency)
//	GET  /v1/devices?q=     장비 검색 (search_latency)
//	POST /v1/devices        장비 등록, 본문 {"serial":"..."} (post_latency)
//	PUT·GET·DELETE /_faults 장애 손잡이 켜기·보기·되돌리기
//	GET·DELETE     /_stats  통계 보기·비우기
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// faults 는 /_faults 로 바꾸는 장애 손잡이입니다.
type faults struct {
	GetLatency    time.Duration // 단건 조회 처리 시간
	SearchLatency time.Duration // 검색 처리 시간
	PostLatency   time.Duration // 등록 처리 시간(등록은 처리가 끝난 뒤 기록된다)
	Workers       int           // 모든 경로가 함께 쓰는 동시 처리 상한, 0 이면 상한 없음
	QueueLimit    int           // 작업자를 기다리는 요청 수 상한, 넘치면 503, 0 이면 상한 없음
	HonorCancel   bool          // 클라이언트가 떠나면 대기·처리를 멈춘다
	ErrorRate     float64       // 이 비율(0~1)의 요청에 ErrorStatus 로 답한다
	ErrorStatus   int           // 에러 응답 상태 코드
	RetryAfter    int           // 에러 응답의 Retry-After 초, 0 이면 붙이지 않는다
	Idempotency   bool          // Idempotency-Key 가 같은 등록은 새로 만들지 않고 처음 응답을 돌려준다
	BodySize      int           // 응답 padding 필드 바이트 수
}

func defaultFaults() faults {
	return faults{
		GetLatency:    5 * time.Millisecond,
		SearchLatency: 30 * time.Millisecond,
		PostLatency:   10 * time.Millisecond,
		ErrorStatus:   http.StatusServiceUnavailable,
	}
}

func (f *faults) apply(q url.Values) error {
	for k, vs := range q {
		v := vs[len(vs)-1]
		var err error
		switch k {
		case "get_latency":
			f.GetLatency, err = time.ParseDuration(v)
		case "search_latency":
			f.SearchLatency, err = time.ParseDuration(v)
		case "post_latency":
			f.PostLatency, err = time.ParseDuration(v)
		case "workers":
			f.Workers, err = strconv.Atoi(v)
		case "queue_limit":
			f.QueueLimit, err = strconv.Atoi(v)
		case "honor_cancel":
			f.HonorCancel, err = strconv.ParseBool(v)
		case "error_rate":
			f.ErrorRate, err = strconv.ParseFloat(v, 64)
		case "error_status":
			f.ErrorStatus, err = strconv.Atoi(v)
		case "retry_after":
			f.RetryAfter, err = strconv.Atoi(v)
		case "idempotency":
			f.Idempotency, err = strconv.ParseBool(v)
		case "body_size":
			f.BodySize, err = strconv.Atoi(v)
		default:
			return fmt.Errorf("unknown fault %q", k)
		}
		if err != nil {
			return fmt.Errorf("fault %s=%q: %w", k, v, err)
		}
	}
	return nil
}

func (f faults) view() map[string]any {
	return map[string]any{
		"get_latency": f.GetLatency.String(), "search_latency": f.SearchLatency.String(),
		"post_latency": f.PostLatency.String(), "workers": f.Workers, "queue_limit": f.QueueLimit,
		"honor_cancel": f.HonorCancel, "error_rate": f.ErrorRate, "error_status": f.ErrorStatus,
		"retry_after": f.RetryAfter, "idempotency": f.Idempotency, "body_size": f.BodySize,
	}
}

// stats 는 /_stats 와 1초 보고 줄의 숫자입니다.
type stats struct {
	requests, inflight, queued, maxQueued     atomic.Int64
	s2xx, s4xx, s5xx, rejected                atomic.Int64
	connsNew, connsOpen, connsClosed          atomic.Int64
	workedAfterCancel, skippedCancel          atomic.Int64
	devicesCreated, devicesDuplicate, replays atomic.Int64
	getReq, searchReq, postReq                atomic.Int64
}

func (s *stats) view() map[string]int64 {
	return map[string]int64{
		"requests": s.requests.Load(), "inflight": s.inflight.Load(), "queued": s.queued.Load(),
		"max_queued": s.maxQueued.Load(), "status_2xx": s.s2xx.Load(), "status_4xx": s.s4xx.Load(),
		"status_5xx": s.s5xx.Load(), "rejected_queue_full": s.rejected.Load(),
		"conns_new": s.connsNew.Load(), "conns_open": s.connsOpen.Load(), "conns_closed": s.connsClosed.Load(),
		"worked_after_cancel": s.workedAfterCancel.Load(), "skipped_cancel": s.skippedCancel.Load(),
		"devices_created": s.devicesCreated.Load(), "devices_duplicate": s.devicesDuplicate.Load(),
		"idempotent_replays": s.replays.Load(),
		"get_requests": s.getReq.Load(), "search_requests": s.searchReq.Load(), "post_requests": s.postReq.Load(),
	}
}

func (s *stats) reset() {
	for _, c := range []*atomic.Int64{&s.requests, &s.maxQueued, &s.s2xx, &s.s4xx, &s.s5xx, &s.rejected,
		&s.connsNew, &s.connsClosed, &s.workedAfterCancel, &s.skippedCancel, &s.devicesCreated,
		&s.devicesDuplicate, &s.replays, &s.getReq, &s.searchReq, &s.postReq} {
		c.Store(0)
	}
}

type server struct {
	mu      sync.Mutex
	f       faults
	sem     chan struct{} // 작업자 자리, Workers 가 바뀌면 새로 만든다
	serials map[string]int
	idem    map[string][]byte
	nextID  atomic.Int64
	st      stats
}

func (s *server) faults() (faults, chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f, s.sem
}

func (s *server) setFaults(f faults) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Workers != s.f.Workers || s.sem == nil && f.Workers > 0 {
		s.sem = nil
		if f.Workers > 0 {
			s.sem = make(chan struct{}, f.Workers)
		}
	}
	s.f = f
}

// work 는 작업자 자리를 얻고 d 만큼 일합니다. 거절됐으면 false 를 돌려줍니다.
func (s *server) work(ctx context.Context, w http.ResponseWriter, f faults, sem chan struct{}, d time.Duration) bool {
	if sem != nil {
		q := s.st.queued.Add(1)
		for {
			m := s.st.maxQueued.Load()
			if q <= m || s.st.maxQueued.CompareAndSwap(m, q) {
				break
			}
		}
		if f.QueueLimit > 0 && q > int64(f.QueueLimit) {
			s.st.queued.Add(-1)
			s.st.rejected.Add(1)
			s.reply(w, f.ErrorStatus, map[string]string{"error": "queue full"})
			return false
		}
		var wait <-chan struct{}
		if f.HonorCancel {
			wait = ctx.Done()
		}
		select {
		case sem <- struct{}{}:
		case <-wait:
			s.st.queued.Add(-1)
			s.st.skippedCancel.Add(1)
			return false
		}
		s.st.queued.Add(-1)
		defer func() { <-sem }()
	}
	if f.HonorCancel && ctx.Err() != nil {
		s.st.skippedCancel.Add(1)
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	if f.HonorCancel {
		select {
		case <-t.C:
		case <-ctx.Done():
			s.st.skippedCancel.Add(1)
			return false
		}
	} else {
		<-t.C // 클라이언트가 떠나도 끝까지 일하는 백엔드
	}
	if ctx.Err() != nil {
		s.st.workedAfterCancel.Add(1)
	}
	return true
}

func (s *server) injectError(w http.ResponseWriter, f faults) bool {
	if f.ErrorRate <= 0 || rand.Float64() >= f.ErrorRate {
		return false
	}
	if f.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(f.RetryAfter))
	}
	s.reply(w, f.ErrorStatus, map[string]string{"error": "injected"})
	return true
}

func (s *server) reply(w http.ResponseWriter, code int, v any) {
	switch {
	case code < 300:
		s.st.s2xx.Add(1)
	case code < 500:
		s.st.s4xx.Add(1)
	default:
		s.st.s5xx.Add(1)
	}
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)+1))
	w.WriteHeader(code)
	_, _ = w.Write(append(b, '\n'))
}

func (s *server) track(counter *atomic.Int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.st.requests.Add(1)
		counter.Add(1)
		s.st.inflight.Add(1)
		defer s.st.inflight.Add(-1)
		h(w, r)
	}
}

func device(id, serial string, f faults) map[string]any {
	return map[string]any{"id": id, "serial": serial, "model": "edge-sw-24", "site": "hq",
		"padding": strings.Repeat("x", f.BodySize)}
}

func (s *server) getDevice(w http.ResponseWriter, r *http.Request) {
	f, sem := s.faults()
	if s.injectError(w, f) || !s.work(r.Context(), w, f, sem, f.GetLatency) {
		return
	}
	id := r.PathValue("id")
	s.reply(w, http.StatusOK, device(id, "SN-"+id, f))
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	f, sem := s.faults()
	if s.injectError(w, f) || !s.work(r.Context(), w, f, sem, f.SearchLatency) {
		return
	}
	q := r.URL.Query().Get("q")
	s.reply(w, http.StatusOK, map[string]any{"q": q, "items": []any{device("1", "SN-1", f), device("2", "SN-2", f)}})
}

func (s *server) createDevice(w http.ResponseWriter, r *http.Request) {
	f, sem := s.faults()
	var in struct {
		Serial string `json:"serial"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil || in.Serial == "" {
		s.reply(w, http.StatusBadRequest, map[string]string{"error": "body must be {\"serial\":\"...\"}"})
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if f.Idempotency && key != "" {
		s.mu.Lock()
		prev, ok := s.idem[key]
		s.mu.Unlock()
		if ok {
			s.st.replays.Add(1)
			s.st.s2xx.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(prev)
			return
		}
	}
	if s.injectError(w, f) || !s.work(r.Context(), w, f, sem, f.PostLatency) {
		return
	}
	// 처리를 마친 뒤 기록한다. 이 시점에 클라이언트가 떠났어도 기록은 남는다.
	id := strconv.FormatInt(s.nextID.Add(1), 10)
	out := device(id, in.Serial, f)
	b, _ := json.Marshal(out)
	b = append(b, '\n')
	s.mu.Lock()
	s.serials[in.Serial]++
	dup := s.serials[in.Serial] > 1
	if f.Idempotency && key != "" {
		s.idem[key] = b
	}
	s.mu.Unlock()
	s.st.devicesCreated.Add(1)
	if dup {
		s.st.devicesDuplicate.Add(1)
	}
	s.reply(w, http.StatusCreated, out)
}

func (s *server) handleFaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		f, _ := s.faults()
		if err := f.apply(r.URL.Query()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.setFaults(f)
	case http.MethodDelete:
		s.setFaults(defaultFaults())
	case http.MethodGet:
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	f, _ := s.faults()
	_ = json.NewEncoder(w).Encode(f.view())
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		s.st.reset()
		s.mu.Lock()
		s.serials = map[string]int{}
		s.idem = map[string][]byte{}
		s.mu.Unlock()
	case http.MethodGet:
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(s.st.view())
}

func (s *server) connState(_ net.Conn, st http.ConnState) {
	switch st {
	case http.StateNew:
		s.st.connsNew.Add(1)
		s.st.connsOpen.Add(1)
	case http.StateClosed, http.StateHijacked:
		s.st.connsOpen.Add(-1)
		s.st.connsClosed.Add(1)
	}
}

func (s *server) report(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	var prevReq, prevNew, prev5 int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		req, cn, s5 := s.st.requests.Load(), s.st.connsNew.Load(), s.st.s5xx.Load()
		fmt.Fprintf(os.Stderr, "%s fakeinv req/s=%d inflight=%d queued=%d conns_open=%d conns_new/s=%d 5xx/s=%d dup=%d\n",
			time.Now().Format("15:04:05"), req-prevReq, s.st.inflight.Load(), s.st.queued.Load(),
			s.st.connsOpen.Load(), cn-prevNew, s5-prev5, s.st.devicesDuplicate.Load())
		prevReq, prevNew, prev5 = req, cn, s5
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9090", "들을 주소")
	idle := flag.Duration("idle-timeout", 5*time.Second, "쉬는 keep-alive 연결을 닫기까지의 시간 (http.Server.IdleTimeout)")
	initial := flag.String("faults", "", "시작할 때 켤 장애 손잡이, 예: 'workers=32&search_latency=5s'")
	every := flag.Duration("report", time.Second, "보고 줄 간격, 0 이면 끈다")
	flag.Parse()

	s := &server{serials: map[string]int{}, idem: map[string][]byte{}}
	f := defaultFaults()
	if *initial != "" {
		q, err := url.ParseQuery(*initial)
		if err == nil {
			err = f.apply(q)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "fakeinv:", err)
			os.Exit(2)
		}
	}
	s.setFaults(f)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/devices/{id}", s.track(&s.st.getReq, s.getDevice))
	mux.HandleFunc("GET /v1/devices", s.track(&s.st.searchReq, s.search))
	mux.HandleFunc("POST /v1/devices", s.track(&s.st.postReq, s.createDevice))
	mux.HandleFunc("/_faults", s.handleFaults)
	mux.HandleFunc("/_stats", s.handleStats)

	hs := &http.Server{Addr: *addr, Handler: mux, IdleTimeout: *idle, ReadHeaderTimeout: 5 * time.Second, ConnState: s.connState}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *every > 0 {
		go s.report(ctx, *every)
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	fmt.Fprintf(os.Stderr, "%s fakeinv listening %s idle-timeout=%s faults=%v\n", time.Now().Format("15:04:05"), *addr, *idle, f.view())
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "fakeinv:", err)
		os.Exit(1)
	}
}
