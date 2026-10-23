package tftp

import (
	"strconv"
	"time"
)

// RFC 2348, 2349 가 정한 옵션 값의 범위입니다.
const (
	minBlksize    = 8
	maxBlksize    = 65464
	minTimeoutSec = 1
	maxTimeoutSec = 255
)

// params는 전송 하나에 쓰는 협상 결과입니다.
type params struct {
	blksize int
	timeout time.Duration
	// oack는 받아들인 옵션입니다. 비어 있으면 OACK 없이 바로 DATA 1 을 보냅니다.
	oack map[string]string
}

// negotiate는 RRQ 의 옵션 중 받아들일 것만 골라 전송 값을 정합니다 (RFC 2347~2349).
// 모르는 옵션이나 범위를 벗어난 값은 OACK 에서 뺍니다.
func (s *Server) negotiate(opts map[string]string, size int64) params {
	// 파람 설정
	p := params{blksize: DefaultBlksize, timeout: s.Timeout, oack: map[string]string{}}

	// bliskize 검사
	if v, err := strconv.Atoi(opts["blksize"]); err == nil && v >= minBlksize && v <= maxBlksize {
		p.blksize = min(v, s.MaxBlksize)
		p.oack["blksize"] = strconv.Itoa(p.blksize)
	}
	// timeout 검사
	if v, err := strconv.Atoi(opts["timeout"]); err == nil && v >= minTimeoutSec && v <= maxTimeoutSec {
		p.timeout = time.Duration(v) * time.Second
		p.oack["timeout"] = strconv.Itoa(v)
	}
	// tsize 검사
	if opts["tsize"] == "0" {
		p.oack["tsize"] = strconv.FormatInt(size, 10)
	}
	return p
}
