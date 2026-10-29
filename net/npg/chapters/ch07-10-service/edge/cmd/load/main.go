// load 는 한 URL 에 일정 시간 동안 요청을 보내고, 응답 상태·전송 오류를 초 단위로 센다.
// 외부 도구(hey·wrk) 없이 E1·E4 의 실패 수를 세려고 만들었다.
//
//	go run ./ch07-10-service/edge/cmd/load -url http://127.0.0.1:8000/api/v1/rates/usd -d 20s
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8000/api/v1/rates/usd", "요청 URL")
	conc := flag.Int("c", 8, "동시 워커 수")
	dur := flag.Duration("d", 15*time.Second, "부하 시간")
	method := flag.String("method", http.MethodGet, "HTTP 메서드")
	keepalive := flag.Bool("keepalive", true, "false 면 요청마다 새 TCP 연결")
	timeout := flag.Duration("timeout", 3*time.Second, "요청 하나의 제한 시간")
	header := flag.String("header", "", "값을 세어 볼 응답 헤더 이름 (예: X-Upstream)")
	flag.Parse()

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			DisableKeepAlives:   !*keepalive,
			MaxIdleConnsPerHost: *conc,
		},
	}

	start := time.Now()
	rec := newRecorder()
	var wg sync.WaitGroup
	for range *conc {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(start) < *dur {
				req, _ := http.NewRequest(*method, *url, nil)
				resp, err := client.Do(req)
				elapsed := time.Since(start)
				sec := int(elapsed / time.Second)
				if err != nil {
					rec.fail(sec, elapsed, classify(err), err)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				hv := ""
				if *header != "" {
					hv = resp.Header.Get(*header)
				}
				rec.add(sec, fmt.Sprintf("%d", resp.StatusCode), hv)
			}
		}()
	}
	wg.Wait()
	rec.print(os.Stdout, *header)
}

// classify 는 전송 오류를 몇 갈래로 묶는다. 자세한 문자열은 첫 번째 것만 따로 남긴다.
func classify(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "reset"
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		return "addrnotavail"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "eof"
	case errors.As(err, &ne) && ne.Timeout():
		return "timeout"
	}
	return "other"
}

type recorder struct {
	mu      sync.Mutex
	perSec  map[int]map[string]int
	total   map[string]int
	headers map[string]int

	// 전송 오류 갈래별 첫 오류 원문과, 첫·마지막 오류 시각
	firstErr  map[string]string
	firstFail time.Duration
	lastFail  time.Duration
	anyFail   bool
}

func newRecorder() *recorder {
	return &recorder{perSec: map[int]map[string]int{}, total: map[string]int{}, headers: map[string]int{}, firstErr: map[string]string{}}
}

func (r *recorder) fail(sec int, at time.Duration, class string, err error) {
	r.add(sec, "err:"+class, "")
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.firstErr[class]; !ok {
		r.firstErr[class] = err.Error()
	}
	if !r.anyFail || at < r.firstFail {
		r.firstFail = at
	}
	if at > r.lastFail {
		r.lastFail = at
	}
	r.anyFail = true
}

func (r *recorder) add(sec int, outcome, headerValue string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.perSec[sec] == nil {
		r.perSec[sec] = map[string]int{}
	}
	r.perSec[sec][outcome]++
	r.total[outcome]++
	if headerValue != "" || outcome[0] != 'e' {
		r.headers[headerValue]++
	}
}

func (r *recorder) print(w io.Writer, header string) {
	secs := make([]int, 0, len(r.perSec))
	for s := range r.perSec {
		secs = append(secs, s)
	}
	sort.Ints(secs)
	fmt.Fprintln(w, "초별 결과 (t=시작 후 초)")
	for _, s := range secs {
		fmt.Fprintf(w, "  t=%02d  %s\n", s, joined(r.perSec[s]))
	}
	fmt.Fprintf(w, "합계  %s\n", joined(r.total))
	if header != "" {
		fmt.Fprintf(w, "%s 분포  %s\n", header, joined(r.headers))
	}
	if r.anyFail {
		fmt.Fprintf(w, "전송 오류 구간  %s ~ %s (%s)\n",
			r.firstFail.Round(time.Millisecond), r.lastFail.Round(time.Millisecond),
			(r.lastFail - r.firstFail).Round(time.Millisecond))
		for class, msg := range r.firstErr {
			fmt.Fprintf(w, "  첫 %s: %s\n", class, msg)
		}
	}
}

func joined(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		label := k
		if label == "" {
			label = "(없음)"
		}
		parts = append(parts, fmt.Sprintf("%s=%d", label, m[k]))
	}
	return strings.Join(parts, " ")
}
