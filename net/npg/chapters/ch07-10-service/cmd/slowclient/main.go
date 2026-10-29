// slowclient 는 연결을 많이 열고 아주 천천히 쓰는 클라이언트다.
//
//	slowclient -addr 127.0.0.1:8081 -n 200 -mode header -every 1s -d 60s
//
// mode
//
//	header — 요청 줄과 헤더를 every 마다 한 바이트씩 보낸다. 헤더를 끝내는 빈 줄은 보내지 않는다.
//	idle   — GET /healthz 하나를 끝까지 보내고 응답을 읽은 뒤 연결을 연 채로 아무것도 보내지 않는다.
//
// 연결마다 서버가 닫을 때까지의 시간을 재고, 1초마다 열린 연결 수를 찍는다.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// kind 는 연결이 끝난 이유를 eof · reset · 그 밖의 에러 문자열로 묶는다.
func kind(err error) string {
	switch {
	case err == nil, errors.Is(err, io.EOF):
		return "eof"
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return "reset"
	}
	return err.Error()
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "서버 주소")
	n := flag.Int("n", 100, "열 연결 수")
	mode := flag.String("mode", "header", "header | idle")
	every := flag.Duration("every", time.Second, "header 모드에서 바이트 사이 간격")
	dur := flag.Duration("d", 30*time.Second, "버틸 시간")
	flag.Parse()
	if *mode != "header" && *mode != "idle" {
		fmt.Fprintln(os.Stderr, "-mode: header | idle")
		os.Exit(2)
	}

	var open, dialErr atomic.Int64
	var mu sync.Mutex
	var lifetimes []time.Duration
	var reasons = map[string]int{}
	stop := make(chan struct{})

	conn := func() {
		c, err := net.DialTimeout("tcp", *addr, 3*time.Second)
		if err != nil {
			dialErr.Add(1)
			return
		}
		open.Add(1)
		start := time.Now()
		closed := make(chan string, 1)
		go func() {
			r := bufio.NewReader(c)
			if *mode == "idle" {
				// 응답 상태 줄 하나를 읽는다.
				r.ReadString('\n')
			}
			_, err := io.Copy(io.Discard, r)
			closed <- kind(err)
		}()

		record := func(why string) {
			open.Add(-1)
			mu.Lock()
			lifetimes = append(lifetimes, time.Since(start))
			reasons[why]++
			mu.Unlock()
		}

		if *mode == "idle" {
			fmt.Fprintf(c, "GET /healthz HTTP/1.1\r\nHost: %s\r\n\r\n", *addr)
			select {
			case why := <-closed:
				record("server closed: " + why)
			case <-stop:
				c.Close()
				<-closed
			}
			return
		}

		payload := []byte(fmt.Sprintf("GET /healthz HTTP/1.1\r\nHost: %s\r\n", *addr))
		filler := []byte("X-Slow: a\r\n")
		t := time.NewTicker(*every)
		defer t.Stop()
		for i := 0; ; i++ {
			var b byte
			if i < len(payload) {
				b = payload[i]
			} else {
				b = filler[(i-len(payload))%len(filler)]
			}
			if _, err := c.Write([]byte{b}); err != nil {
				c.Close()
				<-closed
				record("write " + kind(err))
				return
			}
			select {
			case why := <-closed:
				c.Close()
				record("server closed: " + why)
				return
			case <-stop:
				c.Close()
				<-closed
				return
			case <-t.C:
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); conn() }()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	tick := time.NewTicker(time.Second)
	end := time.After(*dur)
	begin := time.Now()
loop:
	for {
		select {
		case <-tick.C:
			mu.Lock()
			closedN := len(lifetimes)
			mu.Unlock()
			fmt.Printf("t=%-4s open=%d closed_by_server=%d dial_err=%d\n", time.Since(begin).Round(time.Second), open.Load(), closedN, dialErr.Load())
		case <-end:
			break loop
		case <-sig:
			break loop
		}
	}
	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	sort.Slice(lifetimes, func(i, j int) bool { return lifetimes[i] < lifetimes[j] })
	fmt.Printf("== closed_by_server=%d dial_err=%d (of %d)\n", len(lifetimes), dialErr.Load(), *n)
	if len(lifetimes) > 0 {
		r := func(d time.Duration) string { return d.Round(time.Millisecond).String() }
		fmt.Printf("== lifetime min=%s median=%s max=%s\n", r(lifetimes[0]), r(lifetimes[len(lifetimes)/2]), r(lifetimes[len(lifetimes)-1]))
	}
	for k, v := range reasons {
		fmt.Printf("== reason %q: %d\n", k, v)
	}
}
