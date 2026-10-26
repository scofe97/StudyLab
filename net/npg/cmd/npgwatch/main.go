// Command npgwatch 는 장애 리허설용 관측 화면입니다.
//
// npgproxy 관리 포트의 /debug/vars 와 fakeinv 의 /_stats 를 1초마다 긁어 최근 5분을 들고 있다가,
// 브라우저 화면(http://127.0.0.1:8090)에 그래프로 보여 줍니다. 원인은 보여 주지 않고 증상(숫자)만 보여 줍니다.
// 실무에서 Prometheus 가 긁고 Grafana 가 그리는 일을 한 파일로 줄인 것입니다.
//
//	npgwatch -proxy http://127.0.0.1:8081/debug/vars -backend http://127.0.0.1:9090/_stats
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

//go:embed index.html
var indexHTML []byte

// sample 은 한 번 긁은 원시 값입니다. 키는 "proxy.requests", "proxy.reasons.UT", "backend.inflight" 꼴로 펼칩니다.
type sample struct {
	at        time.Time
	raw       map[string]float64
	proxyUp   bool
	backendUp bool
}

// point 는 화면에 보낼 한 시점의 값입니다. 카운터는 초당 변화량으로, 게이지는 그대로 바꿉니다.
type point struct {
	T         int64              `json:"t"` // 유닉스 밀리초
	V         map[string]float64 `json:"v"`
	ProxyUp   bool               `json:"proxy_up"`
	BackendUp bool               `json:"backend_up"`
}

// 카운터(오르기만 함): 화면에는 초당 변화량을 그립니다. 키는 화면 이름, 값은 원시 키.
var counters = map[string]string{
	"proxy_rps":       "proxy.requests",
	"proxy_conns_new": "proxy.conns_new",
	"proxy_conns_reu": "proxy.conns_reused",
	"UT":              "proxy.reasons.UT",
	"UF":              "proxy.reasons.UF",
	"UC":              "proxy.reasons.UC",
	"DC":              "proxy.reasons.DC",
	"backend_rps":     "backend.requests",
	"backend_5xx":     "backend.status_5xx",
	"backend_rejects": "backend.rejected_queue_full",
	"backend_after":   "backend.worked_after_cancel",
	"backend_cnew":    "backend.conns_new",
}

// 게이지(오르내림): 그대로 그립니다.
var gauges = map[string]string{
	"proxy_inflight":   "proxy.inflight",
	"backend_inflight": "backend.inflight",
	"backend_queued":   "backend.queued",
	"backend_open":     "backend.conns_open",
}

// derive 는 앞뒤 두 샘플로 화면 값 하나를 만듭니다.
// 카운터가 줄었으면(프로세스 재시작, DELETE /_stats) 그 구간은 0 으로 둡니다. Prometheus rate() 도 같은 경우를 리셋으로 봅니다.
func derive(prev, cur sample) point {
	p := point{T: cur.at.UnixMilli(), V: map[string]float64{}, ProxyUp: cur.proxyUp, BackendUp: cur.backendUp}
	dt := cur.at.Sub(prev.at).Seconds()
	for name, key := range counters {
		c, ok1 := cur.raw[key]
		b, ok2 := prev.raw[key]
		switch {
		case !ok1:
			// 이번에 못 긁었으면 값을 비워 그래프에 빈칸으로 보이게 한다
		case !ok2 || c < b || dt <= 0:
			p.V[name] = 0
		default:
			p.V[name] = (c - b) / dt
		}
	}
	for name, key := range gauges {
		if v, ok := cur.raw[key]; ok {
			p.V[name] = v
		}
	}
	return p
}

// flatten 은 JSON 객체를 "앞.키" 꼴의 숫자 표로 펼칩니다. 숫자가 아닌 값은 버립니다.
func flatten(prefix string, v any, out map[string]float64) {
	switch t := v.(type) {
	case float64:
		out[prefix] = t
	case map[string]any:
		for k, sub := range t {
			flatten(prefix+"."+k, sub, out)
		}
	}
}

// fetch 는 url 의 JSON 을 받아 pick 경로 아래만 prefix 로 펼칩니다. pick 이 비면 전체를 씁니다.
func fetch(ctx context.Context, c *http.Client, url, pick, prefix string, out map[string]float64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	var v any = doc
	if pick != "" {
		var ok bool
		if v, ok = doc[pick]; !ok {
			return fmt.Errorf("%s: %q 가 없습니다", url, pick)
		}
	}
	flatten(prefix, v, out)
	return nil
}

// ring 은 최근 n 개 화면 값을 들고 있습니다.
type ring struct {
	mu     sync.Mutex
	n      int
	points []point
	last   *sample
}

func (r *ring) add(s sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last != nil {
		r.points = append(r.points, derive(*r.last, s))
		if len(r.points) > r.n {
			r.points = r.points[len(r.points)-r.n:]
		}
	}
	r.last = &s
}

func (r *ring) snapshot() []point {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]point(nil), r.points...)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "화면을 띄울 주소")
	proxyURL := flag.String("proxy", "http://127.0.0.1:8081/debug/vars", "npgproxy 관리 포트의 /debug/vars")
	backendURL := flag.String("backend", "http://127.0.0.1:9090/_stats", "fakeinv 의 /_stats, 빈 값이면 끈다")
	every := flag.Duration("every", time.Second, "긁는 간격")
	keep := flag.Duration("keep", 5*time.Minute, "화면에 남길 기간")
	flag.Parse()

	r := &ring{n: int(*keep / *every)}
	// 긁는 쪽 기한은 간격보다 짧게: 대상이 멈춰도 화면은 "수집 실패" 로 계속 움직여야 한다
	client := &http.Client{Timeout: *every * 8 / 10}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(*every)
		defer t.Stop()
		for {
			s := sample{at: time.Now(), raw: map[string]float64{}}
			s.proxyUp = fetch(ctx, client, *proxyURL, "npgproxy", "proxy", s.raw) == nil
			if *backendURL != "" {
				s.backendUp = fetch(ctx, client, *backendURL, "", "backend", s.raw) == nil
			}
			r.add(s)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/points", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.snapshot())
	})
	hs := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	fmt.Fprintf(os.Stderr, "%s npgwatch http://%s  proxy=%s backend=%s\n", time.Now().Format("15:04:05"), *addr, *proxyURL, *backendURL)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "npgwatch:", err)
		os.Exit(1)
	}
}
