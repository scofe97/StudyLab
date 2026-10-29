// load 는 작은 HTTP 부하 도구다.
//
//	load -url http://127.0.0.1:8081/api/v1/rates/USD -c 50 -d 20s           닫힌 루프: 작업자 50 개가 응답을 받을 때마다 다음 요청
//	load -url http://127.0.0.1:8081/api/v1/rates/USD -rate 300 -d 30s       열린 루프: 응답과 상관없이 초당 300 개를 보낸다
//
// -report 간격마다 지난 구간을, 끝나면 전체를 요약한다.
// 에러는 timeout · refused · reset · eof · other 로 나누고 종류마다 첫 예시를 하나 찍는다.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type headerFlags []string

func (h *headerFlags) String() string     { return strings.Join(*h, ", ") }
func (h *headerFlags) Set(v string) error { *h = append(*h, v); return nil }

type window struct {
	mu      sync.Mutex
	sent    int64
	byCode  map[int]int64
	byErr   map[string]int64
	samples []time.Duration
}

func newWindow() *window { return &window{byCode: map[int]int64{}, byErr: map[string]int64{}} }

func (w *window) add(code int, kind string, d time.Duration) {
	w.mu.Lock()
	if kind != "" {
		w.byErr[kind]++
	} else {
		w.byCode[code]++
	}
	w.samples = append(w.samples, d)
	w.mu.Unlock()
}

func pct(s []time.Duration, p float64) time.Duration {
	if len(s) == 0 {
		return 0
	}
	i := int(float64(len(s)-1) * p)
	return s[i]
}

func (w *window) line(inflight int64) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	sort.Slice(w.samples, func(i, j int) bool { return w.samples[i] < w.samples[j] })
	codes := make([]int, 0, len(w.byCode))
	for c := range w.byCode {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	var parts []string
	for _, c := range codes {
		parts = append(parts, fmt.Sprintf("%d=%d", c, w.byCode[c]))
	}
	kinds := make([]string, 0, len(w.byErr))
	for k := range w.byErr {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", k, w.byErr[k]))
	}
	r := func(d time.Duration) string { return d.Round(time.Millisecond).String() }
	return fmt.Sprintf("sent=%d done=%d [%s] p50=%s p90=%s p99=%s max=%s inflight=%d",
		w.sent, len(w.samples), strings.Join(parts, " "),
		r(pct(w.samples, .50)), r(pct(w.samples, .90)), r(pct(w.samples, .99)), r(pct(w.samples, 1)), inflight)
}

var (
	exMu     sync.Mutex
	examples = map[string]string{}
)

func classify(err error) string {
	var ne net.Error
	kind := "other"
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		kind = "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		kind = "refused"
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		kind = "reset"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		kind = "eof"
	}
	exMu.Lock()
	if _, ok := examples[kind]; !ok {
		examples[kind] = err.Error()
	}
	exMu.Unlock()
	return kind
}

func main() {
	url := flag.String("url", "http://127.0.0.1:8081/api/v1/rates/USD", "요청 URL")
	method := flag.String("method", "GET", "HTTP 메서드")
	body := flag.String("body", "", "요청 본문")
	var headers headerFlags
	flag.Var(&headers, "header", "요청 헤더 'Name: value' (여러 번)")
	idem := flag.Bool("idem", false, "요청마다 새 Idempotency-Key 를 붙인다")
	conc := flag.Int("c", 10, "닫힌 루프 작업자 수 (-rate 가 0 일 때)")
	rate := flag.Float64("rate", 0, "열린 루프 초당 요청 수 (0 이면 닫힌 루프)")
	maxInflight := flag.Int64("max-inflight", 5000, "열린 루프에서 동시에 기다릴 수 있는 요청 수 상한 (넘치면 보내지 않고 dropped 로 센다)")
	dur := flag.Duration("d", 10*time.Second, "보낼 시간")
	timeout := flag.Duration("timeout", 5*time.Second, "요청 하나의 시한 (0 이면 없음)")
	keepAlive := flag.Bool("keepalive", true, "연결을 재사용한다")
	report := flag.Duration("report", time.Second, "구간 보고 간격 (0 이면 끔)")
	pause := flag.Duration("pause-on-error", 50*time.Millisecond, "닫힌 루프 작업자가 에러를 받은 뒤 쉬는 시간")
	flag.Parse()

	tr := &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   !*keepAlive,
	}
	hc := &http.Client{Transport: tr, Timeout: *timeout}

	total := newWindow()
	var win atomic.Pointer[window]
	win.Store(newWindow())
	var inflight, dropped, seq atomic.Int64

	one := func() (failed bool) {
		inflight.Add(1)
		defer inflight.Add(-1)
		var rd io.Reader
		if *body != "" {
			rd = bytes.NewReader([]byte(*body))
		}
		req, err := http.NewRequest(*method, *url, rd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, h := range headers {
			if k, v, ok := strings.Cut(h, ":"); ok {
				req.Header.Add(strings.TrimSpace(k), strings.TrimSpace(v))
			}
		}
		if *body != "" && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if *idem {
			req.Header.Set("Idempotency-Key", fmt.Sprintf("load-%d-%d", os.Getpid(), seq.Add(1)))
		}
		total.mu.Lock()
		total.sent++
		total.mu.Unlock()
		w := win.Load()
		w.mu.Lock()
		w.sent++
		w.mu.Unlock()
		start := time.Now()
		resp, err := hc.Do(req)
		code, kind := 0, ""
		if err != nil {
			kind = classify(err)
		} else {
			_, err = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			code = resp.StatusCode
			if err != nil {
				kind = classify(err)
			}
		}
		d := time.Since(start)
		total.add(code, kind, d)
		win.Load().add(code, kind, d)
		return kind != ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopSig
		cancel()
	}()

	begin := time.Now()
	if *report > 0 {
		go func() {
			t := time.NewTicker(*report)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
				old := win.Swap(newWindow())
				extra := ""
				if n := dropped.Swap(0); n > 0 {
					extra = fmt.Sprintf(" dropped=%d", n)
				}
				fmt.Printf("t=%-4s %s%s\n", time.Since(begin).Round(time.Second), old.line(inflight.Load()), extra)
			}
		}()
	}

	var wg sync.WaitGroup
	if *rate > 0 {
		interval := time.Duration(float64(time.Second) / *rate)
		t := time.NewTicker(interval)
	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case <-t.C:
				if inflight.Load() >= *maxInflight {
					dropped.Add(1)
					continue
				}
				wg.Add(1)
				go func() { defer wg.Done(); one() }()
			}
		}
		t.Stop()
	} else {
		for i := 0; i < *conc; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ctx.Err() == nil {
					if one() && *pause > 0 {
						time.Sleep(*pause)
					}
				}
			}()
		}
	}
	wg.Wait()

	fmt.Printf("== total %s (%s)\n", total.line(0), time.Since(begin).Round(time.Millisecond))
	exMu.Lock()
	for k, v := range examples {
		fmt.Printf("== example %s: %s\n", k, v)
	}
	exMu.Unlock()
}
