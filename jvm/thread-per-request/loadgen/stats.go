package main

import (
	"errors"
	"net"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

// percentile 은 정렬된 표본에서 nearest-rank 방식으로 p(0~100) 값을 고른다. 표본이 없으면 0.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p/100*float64(len(sorted))+0.999999999) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func sortDurations(d []time.Duration) {
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
}

// classify 는 요청 오류를 "timeout" "refused" "reset" "other" 로 묶는다.
func classify(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "refused"
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE):
		return "reset"
	case errors.Is(err, os.ErrDeadlineExceeded), (errors.As(err, &ne) && ne.Timeout()),
		strings.Contains(err.Error(), "Client.Timeout"):
		return "timeout"
	default:
		return "other"
	}
}
