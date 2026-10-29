// Command npgload 는 장애 훈련용 HTTP 부하 도구입니다.
//
// 열린 루프(-rate, 초당 요청 수를 지킨다)와 닫힌 루프(-c, 동시 작업자 수를 지킨다)를 고릅니다.
// 1초마다 상태 분포·p50·p99·처리 중 요청 수를 찍고, 끝나면 요약을 찍습니다.
// 구조는 net/npg/chapters/ch07-10-service 의 cmd/load 를 참고했습니다.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type headers []string

func (h *headers) String() string     { return strings.Join(*h, ", ") }
func (h *headers) Set(v string) error { *h = append(*h, v); return nil }

type result struct {
	status int    // 0 이면 응답을 받지 못함
	kind   string // 에러 종류
	dur    time.Duration
}

type window struct {
	mu   sync.Mutex
	res  []result
	all  []result
	sent atomic.Int64
	live atomic.Int64
}

func (w *window) add(r result) {
	w.mu.Lock()
	w.res = append(w.res, r)
	w.all = append(w.all, r)
	w.mu.Unlock()
}

func (w *window) take() []result {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.res
	w.res = nil
	return r
}

func errKind(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "reset"
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		return "no_port"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "eof"
	}
	return "other"
}

func summarize(rs []result) string {
	if len(rs) == 0 {
		return "n=0"
	}
	codes := map[string]int{}
	var durs []time.Duration
	for _, r := range rs {
		k := r.kind
		if r.status != 0 {
			k = strconv.Itoa(r.status)
		}
		codes[k]++
		durs = append(durs, r.dur)
	}
	slices.Sort(durs)
	pct := func(p float64) time.Duration { return durs[min(len(durs)-1, int(p*float64(len(durs))))] }
	keys := make([]string, 0, len(codes))
	for k := range codes {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s:%d ", k, codes[k])
	}
	return fmt.Sprintf("n=%d %sp50=%s p99=%s max=%s", len(rs), b.String(),
		pct(0.50).Round(time.Millisecond), pct(0.99).Round(time.Millisecond), durs[len(durs)-1].Round(time.Millisecond))
}

func main() {
	target := flag.String("url", "http://127.0.0.1:8080/v1/devices/1", "요청할 URL")
	method := flag.String("method", http.MethodGet, "HTTP 메서드")
	body := flag.String("body", "", "요청 본문, %d 는 요청 순번으로 바뀐다 (예: '{\"serial\":\"SN-%d\"}')")
	var hs headers
	flag.Var(&hs, "H", "요청 헤더 'Name: value', 여러 번 줄 수 있다. 값의 %d 는 요청 순번")
	rate := flag.Float64("rate", 0, "열린 루프: 초당 요청 수 (0 이면 -c 닫힌 루프)")
	poisson := flag.Bool("poisson", false, "열린 루프에서 요청 간격을 지수 분포로 흩는다(실제 트래픽처럼 몰렸다 비었다 한다)")
	conc := flag.Int("c", 10, "닫힌 루프: 동시 작업자 수")
	dur := flag.Duration("d", 10*time.Second, "부하를 거는 시간")
	timeout := flag.Duration("timeout", 30*time.Second, "요청 하나의 클라이언트 쪽 기한 (사용자가 기다려 주는 시간)")
	every := flag.Duration("report", time.Second, "보고 줄 간격, 0 이면 요약만")
	label := flag.String("label", "", "보고 줄 앞에 붙일 이름 (부하를 둘 띄울 때 구분용)")
	flag.Parse()

	// 부하 도구 자신이 병목이 되지 않게 연결 풀을 넉넉히 둔다
	tr := &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 4096,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: tr, Timeout: *timeout}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *dur)
	defer cancel()

	var seq atomic.Int64
	var w window
	one := func() {
		n := seq.Add(1)
		var rd io.Reader
		if *body != "" {
			rd = strings.NewReader(strings.ReplaceAll(*body, "%d", strconv.FormatInt(n, 10)))
		}
		req, err := http.NewRequest(*method, *target, rd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "npgload:", err)
			os.Exit(2)
		}
		if rd != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, h := range hs {
			k, v, _ := strings.Cut(h, ":")
			req.Header.Set(strings.TrimSpace(k), strings.ReplaceAll(strings.TrimSpace(v), "%d", strconv.FormatInt(n, 10)))
		}
		w.sent.Add(1)
		w.live.Add(1)
		defer w.live.Add(-1)
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			w.add(result{kind: errKind(err), dur: time.Since(start)})
			return
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		r := result{status: resp.StatusCode, dur: time.Since(start)}
		if err != nil {
			r = result{kind: "body_" + errKind(err), dur: r.dur}
		}
		w.add(r)
	}

	prefix := time.Now().Format("15:04:05")
	if *label != "" {
		prefix = *label
	}
	if *every > 0 {
		go func() {
			t := time.NewTicker(*every)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-t.C:
					p := now.Format("15:04:05")
					if *label != "" {
						p += " " + *label
					}
					fmt.Fprintf(os.Stderr, "%s inflight=%d %s\n", p, w.live.Load(), summarize(w.take()))
				}
			}
		}()
	}

	var wg sync.WaitGroup
	if *rate > 0 {
		gap := time.Duration(float64(time.Second) / *rate)
		next := time.Now()
	loop:
		for {
			d := gap
			if *poisson {
				d = time.Duration(rand.ExpFloat64() * float64(gap))
			}
			next = next.Add(d)
			select {
			case <-ctx.Done():
				break loop
			case <-time.After(time.Until(next)):
			}
			wg.Go(one)
		}
	} else {
		for range *conc {
			wg.Go(func() {
				for ctx.Err() == nil {
					one()
				}
			})
		}
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "%s 요약 sent=%d %s\n", prefix, w.sent.Load(), summarize(w.all))
}
