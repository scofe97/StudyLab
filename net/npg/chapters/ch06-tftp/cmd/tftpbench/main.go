// tftpbench 는 같은 파일을 여러 번(동시에도) 받아 성공 수와 걸린 시간 분포를 CSV 한 줄로 남긴다.
//
//	go run ./cmd/tftpbench -server 127.0.0.1:6969 -file boot-32k.bin -orig work/root/boot-32k.bin -n 20
//
// -orig 를 주면 받은 바이트의 SHA512/256 이 원본과 같아야 성공으로 센다.
// 실패 이유는 종류별 개수로 표준 에러에 남긴다.
package main

import (
	"crypto/sha512"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"npg-practice/ch06-tftp/tftp"
)

type result struct {
	ok       bool
	elapsed  time.Duration
	bytes    int64
	timeouts int
	err      string
}

func main() {
	var (
		server   = flag.String("server", "127.0.0.1:6969", "서버 주소")
		file     = flag.String("file", "boot-32k.bin", "받을 원격 파일")
		orig     = flag.String("orig", "", "원본 파일 경로 (주면 해시까지 맞아야 성공)")
		n        = flag.Int("n", 10, "받는 횟수")
		parallel = flag.Int("parallel", 1, "동시에 받는 수")
		blksize  = flag.Int("blksize", 0, "0 이 아니면 blksize 옵션을 요청한다")
		window   = flag.Int("windowsize", 0, "0 이 아니면 windowsize 옵션을 요청한다")
		timeout  = flag.Duration("timeout", 5*time.Second, "클라이언트가 다음 패킷을 기다리는 시간")
		retries  = flag.Int("retries", 5, "클라이언트가 연달아 타임아웃을 버티는 횟수")
		label    = flag.String("label", "", "CSV 첫 칸에 넣을 조건 설명 (예: rtt=50ms,loss=5%)")
		header   = flag.Bool("header", false, "CSV 머리줄을 먼저 출력한다")
	)
	flag.Parse()

	var want []byte
	if *orig != "" {
		b, err := os.ReadFile(*orig)
		if err != nil {
			log.Fatal(err)
		}
		s := sha512.Sum512_256(b)
		want = s[:]
	}
	opt := tftp.ClientOptions{BlockSize: *blksize, WindowSize: *window, Timeout: *timeout, Retries: *retries}

	results := make([]result, *n)
	jobs := make(chan int)
	var wg sync.WaitGroup
	begin := time.Now()
	for w := 0; w < *parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = once(*server, *file, opt, want)
			}
		}()
	}
	for i := 0; i < *n; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	wall := time.Since(begin)

	var (
		ok       int
		times    []time.Duration
		bytes    int64
		timeouts int
		errs     = map[string]int{}
	)
	for _, r := range results {
		timeouts += r.timeouts
		if r.ok {
			ok++
			times = append(times, r.elapsed)
			bytes += r.bytes
		} else {
			errs[r.err]++
		}
	}
	slices.Sort(times)
	pick := func(q float64) string {
		if len(times) == 0 {
			return "-"
		}
		i := int(q * float64(len(times)-1))
		return times[i].Round(time.Millisecond).String()
	}
	mean := "-"
	if len(times) > 0 {
		var total time.Duration
		for _, d := range times {
			total += d
		}
		mean = fmt.Sprintf("%.1f", float64(bytes)/1024/total.Seconds())
	}

	if *header {
		fmt.Println("label,n,parallel,ok,fail,success_pct,min,p50,p95,max,mean_KiBps,client_timeouts,wall")
	}
	fmt.Printf("%s,%d,%d,%d,%d,%.0f,%s,%s,%s,%s,%s,%d,%s\n",
		*label, *n, *parallel, ok, *n-ok, 100*float64(ok)/float64(*n),
		pick(0), pick(0.5), pick(0.95), pick(1), mean, timeouts, wall.Round(time.Millisecond))

	keys := make([]string, 0, len(errs))
	for k := range errs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "fail x%d: %s\n", errs[k], k)
	}
}

func once(server, file string, opt tftp.ClientOptions, want []byte) result {
	h := sha512.New512_256()
	st, err := tftp.Get(server, file, opt, io.Writer(h))
	r := result{elapsed: st.Elapsed, bytes: st.Bytes, timeouts: st.Timeouts}
	switch {
	case err != nil:
		r.err = err.Error()
	case want != nil && !slices.Equal(h.Sum(nil), want):
		r.err = "checksum mismatch"
	default:
		r.ok = true
	}
	// 같은 종류의 실패를 한 줄로 묶으려고 주소·포트가 섞인 에러 문구를 줄인다
	if i := strings.Index(r.err, "->"); i >= 0 {
		r.err = "read/write error: " + r.err[strings.LastIndex(r.err, ":")+1:]
	}
	return r
}
