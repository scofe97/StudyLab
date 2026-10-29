// Package scan 은 포트마다 말을 걸어 열림·닫힘·막힘을 판정한다.
// 판정 규칙과 정확도·속도의 맞교환은 학습 문서 04-01 이 설명한다.
package scan

import (
	"context"
	"sync"
	"time"
)

// Status 는 포트 하나의 판정이다.
type Status int

const (
	StatusUnknown  Status = iota
	StatusOpen            // 듣는 프로그램이 있다
	StatusClosed          // 호스트에는 닿았지만 듣는 프로그램이 없다
	StatusFiltered        // 중간에서 막혀 판단할 수 없다
)

func (s Status) String() string {
	switch s {
	case StatusOpen:
		return "open"
	case StatusClosed:
		return "closed"
	case StatusFiltered:
		return "filtered"
	}
	return "unknown"
}

// errTimeout 은 아무것도 돌아오지 않았다는 표시다. 재시도는 이 경우에만 한다.
const errTimeout = "timeout"

// Result 는 포트 하나를 물은 결과다.
type Result struct {
	Port     int
	Status   Status
	Errno    string // 판정 근거: ECONNREFUSED·EHOSTUNREACH·timeout 등. open 이면 비어 있다
	Attempts int    // 이 포트에 보낸 탐침 수
}

// Probe 는 포트 하나를 한 번 묻는다. 방식(CONNECT·SYN·UDP)마다 하나씩 둔다.
type Probe func(ctx context.Context, host string, port int, timeout time.Duration) Result

// Options 는 스캔 전체를 어떻게 돌릴지다.
type Options struct {
	Workers int           // 동시에 기다리는 탐침 수의 상한
	Rate    int           // 초당 보내는 탐침 수의 상한. 0 이하면 제한 없음
	Retries int           // 무응답일 때 더 물을 횟수
	Timeout time.Duration // 탐침 하나가 기다리는 시간
}

// Run 은 ports 를 Workers 개의 goroutine 에 나눠 묻고 결과를 ports 순서대로 돌려준다.
func Run(ctx context.Context, host string, ports []int, probe Probe, opt Options) []Result {
	opt.Workers = max(opt.Workers, 1)

	// rate limit: 티커 하나를 모든 worker 가 나눠 받는다. 신호를 가져간 worker 만 탐침을 보내므로
	// worker 가 몇 개든 전체 탐침은 초당 Rate 개를 넘지 않는다.
	var tick <-chan time.Time
	if opt.Rate > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(opt.Rate))
		defer ticker.Stop()
		tick = ticker.C
	}

	// worker pool: 같은 일을 하는 goroutine 여러 개가 jobs 에서 포트 자리 번호를 꺼내 간다.
	// 한 worker 가 응답을 기다리는 동안 다른 worker 가 다른 포트를 묻는다.
	results := make([]Result, len(ports))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range opt.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs { // jobs 가 닫히면 끝난다
				results[i] = probeWithRetry(ctx, tick, host, ports[i], probe, opt)
			}
		}()
	}

	// 이 goroutine 이 jobs 의 owner 다. 다 넣었거나 ctx 가 끝나면 닫아 worker 들을 끝낸다.
feed:
	for i := range ports {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	return results
}

// probeWithRetry 는 포트 하나를 묻고, 아무것도 돌아오지 않았을 때만 Retries 번 더 묻는다.
// RST·ICMP 처럼 무언가 돌아온 판정은 다시 물어도 같으므로 바로 끝낸다.
func probeWithRetry(ctx context.Context, tick <-chan time.Time, host string, port int, probe Probe, opt Options) Result {
	r := Result{Port: port, Status: StatusUnknown, Errno: "canceled"}
	for attempt := 1; attempt <= 1+opt.Retries; attempt++ {
		if tick != nil {
			select {
			case <-tick:
			case <-ctx.Done():
				return r
			}
		}
		r = probe(ctx, host, port, opt.Timeout)
		r.Attempts = attempt
		if r.Errno != errTimeout {
			return r
		}
	}
	return r
}
