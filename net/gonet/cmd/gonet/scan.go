package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gonet/internal/scan"
)

// runScan 은 gonet scan 을 처리하고 종료 코드를 돌려준다.
func runScan(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	ports := fs.String("ports", "1-1024", "물을 포트: 1-1024,8080 처럼 범위와 쉼표")
	workers := fs.Int("workers", 25, "동시에 기다리는 탐침 수의 상한")
	rate := fs.Int("rate", 1000, "초당 보내는 탐침 수의 상한 (0 이면 제한 없음)")
	retries := fs.Int("retries", 1, "무응답일 때 더 물을 횟수")
	timeout := fs.Duration("timeout", time.Second, "탐침 하나가 기다리는 시간")
	syn := fs.Bool("s", false, "SYN 스캔 (raw 소켓, root·CAP_NET_RAW, Linux 전용)")
	verbose := fs.Bool("v", false, "포트마다 판정을 찍는다 (기본은 open 만)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gonet scan [flags] <host>  (예: gonet scan -ports 1-1024,8080 127.0.0.1)")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	list, err := parsePorts(*ports)
	if fs.NArg() != 1 || err != nil {
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fs.Usage()
		return 2
	}
	host := fs.Arg(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	opt := scan.Options{Workers: *workers, Rate: *rate, Retries: *retries, Timeout: *timeout}
	mode := "connect"
	if *syn {
		mode = "syn"
	}
	fmt.Printf("scan host=%s mode=%s ports=%d workers=%d rate=%d retries=%d timeout=%s\n",
		host, mode, len(list), opt.Workers, opt.Rate, opt.Retries, opt.Timeout)

	start := time.Now()
	var results []scan.Result
	if *syn {
		results, err = scan.RunSYN(ctx, host, list, opt)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	} else {
		results = scan.Run(ctx, host, list, scan.ProbeConnect, opt)
	}
	printResults(os.Stdout, results, *verbose, time.Since(start))
	return 0
}

// printResults 는 포트별 판정과 판정·errno·시도 수 분포를 찍는다.
func printResults(w io.Writer, results []scan.Result, verbose bool, elapsed time.Duration) {
	status := map[scan.Status]int{}
	errnos := map[string]int{}
	attempts := map[int]int{}
	for _, r := range results {
		status[r.Status]++
		if r.Errno != "" {
			errnos[r.Errno]++
		}
		attempts[r.Attempts]++
		if verbose || r.Status == scan.StatusOpen {
			fmt.Fprintf(w, "%-8s %5d  %s (시도 %d)\n", r.Status, r.Port, r.Errno, r.Attempts)
		}
	}
	fmt.Fprintf(w, "--- open %d  closed %d  filtered %d  unknown %d  (%s)\n",
		status[scan.StatusOpen], status[scan.StatusClosed], status[scan.StatusFiltered], status[scan.StatusUnknown],
		elapsed.Round(time.Millisecond))
	fmt.Fprintf(w, "errno    %s\n", countLine(errnos))
	tries := map[string]int{}
	for n, c := range attempts {
		tries[strconv.Itoa(n)+"번"] = c
	}
	fmt.Fprintf(w, "시도     %s\n", countLine(tries))
}

func countLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, " ")
}

// parsePorts 는 "1-1024,8080" 을 포트 목록으로 바꾼다.
func parsePorts(s string) ([]int, error) {
	var ports []int
	for _, part := range strings.Split(s, ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return nil, fmt.Errorf("포트 %q", part)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil {
				return nil, fmt.Errorf("포트 %q", part)
			}
		}
		if a < 1 || b > 65535 || a > b {
			return nil, fmt.Errorf("포트 범위 %q (1–65535)", part)
		}
		for p := a; p <= b; p++ {
			ports = append(ports, p)
		}
	}
	return ports, nil
}
