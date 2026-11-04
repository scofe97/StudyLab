// loadgen 은 닫힌 루프 부하기다. 동시 c 개 워커가 끝나자마자 다음 요청을 보낸다.
// 결과는 JSON 한 줄로 stdout 에 낸다.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type result struct {
	URL         string         `json:"url"`
	Concurrency int            `json:"c"`
	DurationSec float64        `json:"duration_sec"`
	Success     int            `json:"success"`
	RPS         float64        `json:"rps"`
	P50Ms       float64        `json:"p50_ms"`
	P90Ms       float64        `json:"p90_ms"`
	P99Ms       float64        `json:"p99_ms"`
	Errors      int            `json:"errors"`
	ErrorKinds  map[string]int `json:"error_kinds"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// worker 는 deadline 까지 요청을 반복한다. record 가 false 인 동안(워밍업)에는 표본에 넣지 않는다.
func worker(client *http.Client, url string, deadline time.Time, recordFrom time.Time) (lat []time.Duration, errs map[string]int) {
	errs = map[string]int{}
	for time.Now().Before(deadline) {
		start := time.Now()
		resp, err := client.Get(url)
		kind := ""
		if err != nil {
			kind = classify(err)
		} else {
			_, cerr := io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if cerr != nil {
				kind = classify(cerr)
			} else if resp.StatusCode != 200 {
				kind = fmt.Sprintf("http_%d", resp.StatusCode)
			}
		}
		if start.Before(recordFrom) {
			continue
		}
		if kind != "" {
			errs[kind]++
		} else {
			lat = append(lat, time.Since(start))
		}
	}
	return
}

func main() {
	url := flag.String("url", "", "target url")
	c := flag.Int("c", 10, "concurrent workers")
	d := flag.Duration("d", 10*time.Second, "measured duration")
	warmup := flag.Duration("warmup", 2*time.Second, "warmup (not recorded)")
	timeout := flag.Duration("timeout", 15*time.Second, "per-request timeout")
	flag.Parse()
	if *url == "" {
		fmt.Fprintln(os.Stderr, "-url is required")
		os.Exit(2)
	}

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			// 연결을 재사용해 임시 포트 고갈로 결과가 오염되지 않게 한다.
			MaxIdleConns:        *c,
			MaxIdleConnsPerHost: *c,
			MaxConnsPerHost:     *c,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	begin := time.Now()
	recordFrom := begin.Add(*warmup)
	deadline := recordFrom.Add(*d)

	var mu sync.Mutex
	var all []time.Duration
	kinds := map[string]int{}
	var wg sync.WaitGroup
	for i := 0; i < *c; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lat, errs := worker(client, *url, deadline, recordFrom)
			mu.Lock()
			all = append(all, lat...)
			for k, v := range errs {
				kinds[k] += v
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	sortDurations(all)
	nerr := 0
	for _, v := range kinds {
		nerr += v
	}
	res := result{
		URL: *url, Concurrency: *c, DurationSec: d.Seconds(),
		Success: len(all), RPS: float64(len(all)) / d.Seconds(),
		P50Ms: ms(percentile(all, 50)), P90Ms: ms(percentile(all, 90)), P99Ms: ms(percentile(all, 99)),
		Errors: nerr, ErrorKinds: kinds,
	}
	json.NewEncoder(os.Stdout).Encode(res)
}
