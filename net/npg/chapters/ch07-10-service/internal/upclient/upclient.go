// Package upclient 는 api 가 업스트림(환율 제공자)을 부르는 HTTP 클라이언트다.
//
// 한 번의 호출(Call)은 시도(attempt) 여러 번으로 이루어질 수 있다.
//
//	Call ─ 전체 기한(TotalTimeout)
//	  ├─ attempt 1 ─ 시도당 기한(AttemptTimeout), Transport.ResponseHeaderTimeout
//	  ├─ 기다림(백오프·지터·Retry-After)
//	  └─ attempt 2 …
//
// 시도마다 httptrace 로 연결을 새로 맺었는지, 풀에서 꺼냈는지, 다 쓴 연결을 풀에 돌려놓았는지를 센다.
package upclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	mrand "math/rand/v2"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config 는 클라이언트 설정이다. 시간·개수 값이 0 이면 Go 의 http.DefaultTransport 값을 그대로 둔다.
type Config struct {
	BaseURL string

	// Transport 손잡이
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	MaxConnsPerHost       int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	DisableKeepAlives     bool

	// context 로 거는 기한. 0 이면 걸지 않는다.
	AttemptTimeout time.Duration
	TotalTimeout   time.Duration

	// 응답 본문을 다 쓴 뒤 처리. drain(끝까지 읽고 닫기) | close(닫기만) | leak(닫지 않기)
	BodyMode string

	// 재시도
	Retries         int           // 첫 시도 뒤 더 하는 시도 수
	Backoff         string        // none | exp
	BackoffBase     time.Duration // exp 의 첫 대기
	BackoffMax      time.Duration // exp 의 대기 상한
	Jitter          string        // none | full
	RetryBudget     float64       // 요청 하나가 쌓는 재시도 토큰. 0 이면 예산을 쓰지 않는다
	HonorRetryAfter bool          // Retry-After 가 있으면 그만큼은 기다린다
	RetryPOST       bool          // POST 도 재시도한다
	IdempotencyKey  bool          // POST 에 Idempotency-Key 헤더를 붙인다

	// 캐시. off | etag
	Cache string

	Logger *slog.Logger
}

// Rates 는 업스트림 환율 응답에서 api 가 쓰는 필드다.
type Rates struct {
	Base    string             `json:"base"`
	Version int64              `json:"version"`
	AsOf    string             `json:"as_of"`
	Rates   map[string]float64 `json:"rates"`
}

// Result 는 호출 한 번의 결과다.
type Result struct {
	Status   int
	Attempts int
	Cache    string // off | miss | fresh | revalidated
	Header   http.Header
	Body     []byte // GetRates 에서는 비어 있다
}

// metrics 는 누적 계수다. /debug/vars 로 나간다.
type metrics struct {
	calls, attempts, retries, retryDenied       atomic.Int64
	errors, attemptTimeouts                     atomic.Int64
	connNew, connReused, connPutIdle, connWaits atomic.Int64
	gotConnExtra                                atomic.Int64
	cacheFresh, cacheRevalidated, cacheMiss     atomic.Int64
	bodyBytes                                   atomic.Int64
	inflight                                    atomic.Int64
}

// Client 는 업스트림 클라이언트다.
type Client struct {
	cfg    Config
	tr     *http.Transport
	hc     *http.Client
	log    *slog.Logger
	m      metrics
	cache  *etagCache
	budget *retryBudget
}

// New 는 Config 로 Client 를 만든다.
func New(cfg Config) (*Client, error) {
	switch cfg.BodyMode {
	case "":
		cfg.BodyMode = "drain"
	case "drain", "close", "leak":
	default:
		return nil, fmt.Errorf("body mode: %q", cfg.BodyMode)
	}
	if cfg.Backoff == "" {
		cfg.Backoff = "none"
	}
	if cfg.Jitter == "" {
		cfg.Jitter = "none"
	}
	if cfg.Cache == "" {
		cfg.Cache = "off"
	}
	if cfg.Backoff != "none" && cfg.Backoff != "exp" {
		return nil, fmt.Errorf("backoff: %q", cfg.Backoff)
	}
	if cfg.Jitter != "none" && cfg.Jitter != "full" {
		return nil, fmt.Errorf("jitter: %q", cfg.Jitter)
	}
	if cfg.Cache != "off" && cfg.Cache != "etag" {
		return nil, fmt.Errorf("cache: %q", cfg.Cache)
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	if cfg.DialTimeout > 0 {
		tr.DialContext = (&net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second}).DialContext
	}
	if cfg.ResponseHeaderTimeout > 0 {
		tr.ResponseHeaderTimeout = cfg.ResponseHeaderTimeout
	}
	if cfg.MaxConnsPerHost > 0 {
		tr.MaxConnsPerHost = cfg.MaxConnsPerHost
	}
	if cfg.MaxIdleConnsPerHost > 0 {
		tr.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}
	if cfg.IdleConnTimeout > 0 {
		tr.IdleConnTimeout = cfg.IdleConnTimeout
	}
	tr.DisableKeepAlives = cfg.DisableKeepAlives

	c := &Client{
		cfg: cfg,
		tr:  tr,
		hc:  &http.Client{Transport: tr},
		log: cfg.Logger,
	}
	if cfg.Cache == "etag" {
		c.cache = &etagCache{m: map[string]cacheEntry{}}
	}
	if cfg.RetryBudget > 0 {
		c.budget = newRetryBudget(cfg.RetryBudget, 10)
	}
	return c, nil
}

// Config 는 적용된 설정을 돌려준다.
func (c *Client) Config() Config { return c.cfg }

// CloseIdle 은 풀에 있는 유휴 연결을 닫는다.
func (c *Client) CloseIdle() { c.tr.CloseIdleConnections() }

// Snapshot 은 누적 계수를 돌려준다.
func (c *Client) Snapshot() map[string]int64 {
	return map[string]int64{
		"calls":             c.m.calls.Load(),
		"inflight":          c.m.inflight.Load(),
		"attempts":          c.m.attempts.Load(),
		"retries":           c.m.retries.Load(),
		"retry_denied":      c.m.retryDenied.Load(),
		"errors":            c.m.errors.Load(),
		"attempt_timeouts":  c.m.attemptTimeouts.Load(),
		"conn_new":          c.m.connNew.Load(),
		"conn_reused":       c.m.connReused.Load(),
		"conn_put_idle":     c.m.connPutIdle.Load(),
		"conn_waits":        c.m.connWaits.Load(),
		"got_conn_extra":    c.m.gotConnExtra.Load(),
		"cache_fresh":       c.m.cacheFresh.Load(),
		"cache_revalidated": c.m.cacheRevalidated.Load(),
		"cache_miss":        c.m.cacheMiss.Load(),
		"body_bytes":        c.m.bodyBytes.Load(),
	}
}

// NewIdempotencyKey 는 무작위 키 하나를 만든다.
func NewIdempotencyKey() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// GetRates 는 GET /v1/rates/{base} 를 부르고 본문을 Rates 로 읽는다.
func (c *Client) GetRates(ctx context.Context, base, requestID string) (Rates, Result, error) {
	path := "/v1/rates/" + base
	res := Result{Cache: c.cfg.Cache}

	var cached cacheEntry
	var haveCached bool
	if c.cache != nil {
		cached, haveCached = c.cache.get(base)
		if haveCached && time.Now().Before(cached.expires) {
			c.m.cacheFresh.Add(1)
			res.Cache = "fresh"
			res.Status = http.StatusOK
			return cached.rates, res, nil
		}
	}

	var out Rates
	hdr := http.Header{}
	if requestID != "" {
		hdr.Set("X-Request-ID", requestID)
	}
	if haveCached && cached.etag != "" {
		hdr.Set("If-None-Match", cached.etag)
	}
	read := func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}
		return json.NewDecoder(resp.Body).Decode(&out)
	}
	r, err := c.call(ctx, http.MethodGet, path, nil, hdr, read)
	r.Cache = res.Cache
	if err != nil {
		return Rates{}, r, err
	}
	if c.cache != nil {
		switch r.Status {
		case http.StatusNotModified:
			c.m.cacheRevalidated.Add(1)
			r.Cache = "revalidated"
			cached.expires = expiresFrom(r.Header)
			c.cache.put(base, cached)
			r.Status = http.StatusOK
			return cached.rates, r, nil
		case http.StatusOK:
			c.m.cacheMiss.Add(1)
			r.Cache = "miss"
			c.cache.put(base, cacheEntry{rates: out, etag: r.Header.Get("ETag"), expires: expiresFrom(r.Header)})
		}
	}
	return out, r, nil
}

// CreateConversion 은 POST /v1/conversions 를 부르고 응답 본문을 그대로 돌려준다.
func (c *Client) CreateConversion(ctx context.Context, body []byte, idemKey, requestID string) (Result, error) {
	hdr := http.Header{"Content-Type": {"application/json"}}
	if requestID != "" {
		hdr.Set("X-Request-ID", requestID)
	}
	if c.cfg.IdempotencyKey && idemKey != "" {
		hdr.Set("Idempotency-Key", idemKey)
	}
	var buf []byte
	read := func(resp *http.Response) error {
		var err error
		buf, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return err
	}
	r, err := c.call(ctx, http.MethodPost, "/v1/conversions", body, hdr, read)
	r.Body = buf
	return r, err
}

// retryable 은 상태 코드로 다시 시도할지 정한다.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// call 은 재시도 정책을 적용해 시도를 반복한다.
func (c *Client) call(ctx context.Context, method, path string, body []byte, hdr http.Header, read func(*http.Response) error) (Result, error) {
	c.m.calls.Add(1)
	c.m.inflight.Add(1)
	defer c.m.inflight.Add(-1)
	if c.cfg.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.TotalTimeout)
		defer cancel()
	}
	if c.budget != nil {
		c.budget.deposit()
	}

	var res Result
	for attempt := 1; ; attempt++ {
		res.Attempts = attempt
		status, header, retryAfter, err := c.attempt(ctx, attempt, method, path, body, hdr, read)
		res.Status, res.Header = status, header

		again := false
		switch {
		case ctx.Err() != nil:
		case err != nil:
			again = true
		case retryableStatus(status):
			again = true
		}
		if method == http.MethodPost && !c.cfg.RetryPOST {
			again = false
		}
		if !again || attempt > c.cfg.Retries {
			return res, err
		}
		if c.budget != nil && !c.budget.withdraw() {
			c.m.retryDenied.Add(1)
			return res, err
		}

		wait := c.backoff(attempt)
		if c.cfg.HonorRetryAfter && retryAfter > wait {
			wait = retryAfter
		}
		if wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return res, ctx.Err()
			}
		}
		c.m.retries.Add(1)
	}
}

func (c *Client) backoff(attempt int) time.Duration {
	if c.cfg.Backoff == "none" {
		return 0
	}
	d := c.cfg.BackoffBase << (attempt - 1)
	if c.cfg.BackoffMax > 0 && (d > c.cfg.BackoffMax || d <= 0) {
		d = c.cfg.BackoffMax
	}
	if c.cfg.Jitter == "full" && d > 0 {
		d = time.Duration(mrand.Int64N(int64(d)))
	}
	return d
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && s >= 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

// attemptTrace 는 시도 하나의 httptrace 기록이다.
type attemptTrace struct {
	start                    time.Time
	gotConn                  int
	reused, wasIdle          bool
	idleTime                 time.Duration
	getConn, connAt          time.Time
	connectStart, connectEnd time.Time
	wrote, firstByte         time.Time
	putIdle                  int
	putIdleErr               error
	mu                       sync.Mutex
}

func (t *attemptTrace) hooks(m *metrics) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn: func(string) {
			t.mu.Lock()
			if t.getConn.IsZero() {
				t.getConn = time.Now()
			}
			t.mu.Unlock()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.gotConn++
			t.reused, t.wasIdle, t.idleTime = info.Reused, info.WasIdle, info.IdleTime
			t.connAt = time.Now()
			t.mu.Unlock()
			if info.Reused {
				m.connReused.Add(1)
			} else {
				m.connNew.Add(1)
			}
		},
		ConnectStart: func(string, string) {
			t.mu.Lock()
			t.connectStart = time.Now()
			t.mu.Unlock()
		},
		ConnectDone: func(string, string, error) {
			t.mu.Lock()
			t.connectEnd = time.Now()
			t.mu.Unlock()
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.mu.Lock()
			t.wrote = time.Now()
			t.mu.Unlock()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			t.firstByte = time.Now()
			t.mu.Unlock()
		},
		PutIdleConn: func(err error) {
			t.mu.Lock()
			t.putIdle++
			t.putIdleErr = err
			t.mu.Unlock()
			if err == nil {
				m.connPutIdle.Add(1)
			}
		},
	}
}

func since(a, b time.Time) time.Duration {
	if a.IsZero() || b.IsZero() {
		return -1
	}
	return b.Sub(a).Truncate(time.Microsecond)
}

// countingReader 는 읽은 바이트 수를 센다.
type countingReader struct {
	r io.Reader
	n *atomic.Int64
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// attempt 는 요청 한 번을 보내고 read 로 본문을 읽은 뒤 BodyMode 대로 정리한다.
func (c *Client) attempt(ctx context.Context, n int, method, path string, body []byte, hdr http.Header, read func(*http.Response) error) (int, http.Header, time.Duration, error) {
	c.m.attempts.Add(1)
	actx := ctx
	if c.cfg.AttemptTimeout > 0 {
		var cancel context.CancelFunc
		actx, cancel = context.WithTimeout(ctx, c.cfg.AttemptTimeout)
		defer cancel()
	}
	tr := &attemptTrace{start: time.Now()}
	actx = httptrace.WithClientTrace(actx, tr.hooks(&c.m))

	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(actx, method, c.cfg.BaseURL+path, rd)
	if err != nil {
		return 0, nil, 0, err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}

	status := 0
	var header http.Header
	var retryAfter time.Duration
	resp, err := c.hc.Do(req)
	if err == nil {
		status, header = resp.StatusCode, resp.Header
		retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		body := countingReader{r: resp.Body, n: &c.m.bodyBytes}
		resp.Body = struct {
			io.Reader
			io.Closer
		}{body, resp.Body}
		err = read(resp)
		switch c.cfg.BodyMode {
		case "drain":
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		case "close":
			resp.Body.Close()
		case "leak":
		}
	}
	if err != nil {
		c.m.errors.Add(1)
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			c.m.attemptTimeouts.Add(1)
		}
	}

	tr.mu.Lock()
	if tr.gotConn > 1 {
		c.m.gotConnExtra.Add(int64(tr.gotConn - 1))
	}
	if !tr.getConn.IsZero() && !tr.connAt.IsZero() && tr.connAt.Sub(tr.getConn) > time.Millisecond {
		c.m.connWaits.Add(1)
	}
	attrs := []any{
		"attempt", n, "method", method, "path", path, "status", status,
		"got_conn", tr.gotConn, "reused", tr.reused, "was_idle", tr.wasIdle, "idle_time", tr.idleTime,
		"conn_wait", since(tr.getConn, tr.connAt), "connect", since(tr.connectStart, tr.connectEnd),
		"ttfb", since(tr.wrote, tr.firstByte), "total", time.Since(tr.start).Truncate(time.Microsecond),
		"put_idle", tr.putIdle, "body_mode", c.cfg.BodyMode,
	}
	tr.mu.Unlock()
	if err != nil {
		attrs = append(attrs, "err", err.Error())
	}
	c.log.Debug("upstream attempt", attrs...)
	return status, header, retryAfter, err
}

// cacheEntry 는 base 하나의 캐시 항목이다.
type cacheEntry struct {
	rates   Rates
	etag    string
	expires time.Time
}

type etagCache struct {
	mu sync.Mutex
	m  map[string]cacheEntry
}

func (c *etagCache) get(k string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	return e, ok
}

func (c *etagCache) put(k string, e cacheEntry) {
	c.mu.Lock()
	c.m[k] = e
	c.mu.Unlock()
}

// expiresFrom 은 Cache-Control 의 max-age 로 신선한 기간의 끝을 정한다. max-age 가 없으면 지금이다.
func expiresFrom(h http.Header) time.Time {
	now := time.Now()
	for _, d := range strings.Split(h.Get("Cache-Control"), ",") {
		d = strings.TrimSpace(d)
		if v, ok := strings.CutPrefix(d, "max-age="); ok {
			if s, err := strconv.Atoi(v); err == nil && s > 0 {
				return now.Add(time.Duration(s) * time.Second)
			}
		}
	}
	return now
}

// retryBudget 은 요청마다 ratio 만큼 토큰을 쌓고 재시도마다 1 을 쓴다. 상한은 max 다.
type retryBudget struct {
	mu     sync.Mutex
	ratio  float64
	max    float64
	tokens float64
}

func newRetryBudget(ratio, max float64) *retryBudget {
	return &retryBudget{ratio: ratio, max: max, tokens: max}
}

func (b *retryBudget) deposit() {
	b.mu.Lock()
	b.tokens += b.ratio
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.mu.Unlock()
}

func (b *retryBudget) withdraw() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
