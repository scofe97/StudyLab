package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type sseResult struct {
	URL         string  `json:"url"`
	Mode        string  `json:"mode"`
	Conns       int     `json:"c"`
	RampSec     float64 `json:"ramp_sec"`
	DurationSec float64 `json:"duration_sec"`
	// 연결 상태. ok + stalled + notOpened + failed = c.
	OK         int            `json:"ok_conns"`      // 첫 이벤트를 받았고 끝까지 끊기지 않은 연결
	Stalled    int            `json:"stalled_conns"` // TCP 는 열렸지만 첫 이벤트를 못 받은 연결
	NotOpened  int            `json:"not_opened"`    // 끝까지 TCP 연결도 못 연 연결
	Failed     int            `json:"failed_conns"`  // 도중에 오류로 끊긴 연결
	ErrorKinds map[string]int `json:"error_kinds"`
	Events     int            `json:"events"`         // 측정 구간에 받은 이벤트 수
	EventsRPS  float64        `json:"events_per_sec"` // 측정 구간 초당 이벤트
	LatP50Ms   float64        `json:"lat_p50_ms"`     // 받은 시각 − 서버 송신 시각
	LatP99Ms   float64        `json:"lat_p99_ms"`
}

// parseEventNs 는 "data: <ns>" 줄에서 서버 송신 시각(epoch ns)을 꺼낸다.
func parseEventNs(line string) (int64, bool) {
	v, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "data:")
	if !ok {
		return 0, false
	}
	ns, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return ns, err == nil
}

// runSSE 는 n 개 연결을 ramp 동안 고르게 열고, ramp+settle 이후 d 동안의 이벤트를 표본으로 모은다.
// 서버와 같은 기기라 시계를 공유하므로 지연 = 받은 시각 − 서버가 적은 시각 이다.
func runSSE(url string, n int, ramp, settle, d time.Duration) {
	begin := time.Now()
	recordFrom := begin.Add(ramp + settle)
	deadline := recordFrom.Add(d)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	tr := &http.Transport{MaxIdleConns: 0, DisableCompression: true}
	client := &http.Client{Transport: tr} // 타임아웃 없음: 연결은 일부러 계속 열어 둔다.

	var mu sync.Mutex
	var lats []time.Duration
	kinds := map[string]int{}
	var ok, stalled, notOpened, failed, events atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(float64(ramp) * float64(i) / float64(n)))
			var opened, gotFirst bool
			trace := &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { opened = true }}
			req, _ := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, url, nil)
			resp, err := client.Do(req)
			var rerr error
			if err == nil {
				if resp.StatusCode != http.StatusOK {
					rerr = &statusError{resp.StatusCode}
				} else {
					r := bufio.NewReaderSize(resp.Body, 256)
					var local []time.Duration
					for {
						line, e := r.ReadString('\n')
						if sent, isData := parseEventNs(line); isData {
							recv := time.Now()
							gotFirst = true
							if !recv.Before(recordFrom) && recv.Before(deadline) {
								events.Add(1)
								local = append(local, time.Duration(recv.UnixNano()-sent))
							}
						}
						if e != nil {
							rerr = e
							break
						}
					}
					mu.Lock()
					lats = append(lats, local...)
					mu.Unlock()
				}
				resp.Body.Close()
			} else {
				rerr = err
			}
			switch {
			case time.Now().After(deadline.Add(-50*time.Millisecond)) || ctx.Err() != nil:
				// 측정 종료로 우리가 끊은 것: 상태는 첫 이벤트 여부로 가른다.
				switch {
				case gotFirst:
					ok.Add(1)
				case opened:
					stalled.Add(1)
				default:
					notOpened.Add(1)
				}
			default:
				failed.Add(1)
				mu.Lock()
				kinds[classify(rerr)]++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	sortDurations(lats)
	res := sseResult{
		URL: url, Mode: "sse", Conns: n, RampSec: ramp.Seconds(), DurationSec: d.Seconds(),
		OK: int(ok.Load()), Stalled: int(stalled.Load()), NotOpened: int(notOpened.Load()), Failed: int(failed.Load()),
		ErrorKinds: kinds, Events: int(events.Load()), EventsRPS: float64(events.Load()) / d.Seconds(),
		LatP50Ms: ms(percentile(lats, 50)), LatP99Ms: ms(percentile(lats, 99)),
	}
	json.NewEncoder(os.Stdout).Encode(res)
}

type statusError struct{ code int }

func (e *statusError) Error() string { return "http status " + strconv.Itoa(e.code) }
