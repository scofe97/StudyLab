package upclient

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"npg-practice/ch07-10-service/internal/upstream"
)

// 여기의 테스트는 S2·S3·S8 의 축소판이다. 결과(재사용 여부, 시도 수, 캐시 결과)는 찍기만 한다.

func startUpstream(t *testing.T, f upstream.Faults) (*upstream.Server, string) {
	t.Helper()
	up := upstream.New()
	up.SetFaults(f)
	ts := httptest.NewUnstartedServer(nil)
	up.Configure(ts.Config)
	ts.Start()
	t.Cleanup(ts.Close)
	return up, ts.URL
}

func newClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	if testing.Verbose() {
		cfg.Logger = slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.CloseIdle)
	return c
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func TestConfigValidation(t *testing.T) {
	// Given — 잘못된 값이 하나씩 든 설정들
	bad := []Config{{BodyMode: "x"}, {Backoff: "linear"}, {Jitter: "half"}, {Cache: "lru"}}

	for _, cfg := range bad {
		// When — 클라이언트를 만든다
		_, err := New(cfg)

		// Then — 거절된다
		if err == nil {
			t.Fatalf("통과했다: %+v", cfg)
		}
	}
}

func TestBodyModes(t *testing.T) {
	shapes := []struct {
		name string
		f    func(upstream.Faults) upstream.Faults
	}{
		{"content-length", func(f upstream.Faults) upstream.Faults { return f }},
		{"content-length+trailing 64KiB", func(f upstream.Faults) upstream.Faults { f.Trailing = 64 << 10; return f }},
		{"chunked", func(f upstream.Faults) upstream.Faults { f.Chunked = true; return f }},
	}
	for _, sh := range shapes {
		for _, mode := range []string{"drain", "close", "leak"} {
			t.Run(sh.name+"/"+mode, func(t *testing.T) {
				// Given — 응답 모양이 정해진 업스트림과 본문 정리 방식이 정해진 클라이언트
				f := upstream.DefaultFaults()
				f.Latency = 0
				up, base := startUpstream(t, sh.f(f))
				c := newClient(t, Config{BaseURL: base, BodyMode: mode})

				// When — 같은 환율을 순서대로 다섯 번 조회한다
				for i := 0; i < 5; i++ {
					if _, _, err := c.GetRates(context.Background(), "USD", ""); err != nil {
						t.Fatalf("call %d: %v", i, err)
					}
				}

				// Then — 클라이언트 쪽 연결 계수와 업스트림 쪽 새 연결 수를 남긴다
				s := c.Snapshot()
				t.Logf("conn_new=%d conn_reused=%d conn_put_idle=%d upstream.conns_new=%v",
					s["conn_new"], s["conn_reused"], s["conn_put_idle"], up.Stats()["conns_new"])
			})
		}
	}
}

func TestRetriesLog(t *testing.T) {
	// Given — 모든 요청에 503 을 돌려주는 업스트림과 재시도 설정 둘
	f := upstream.DefaultFaults()
	f.Latency, f.ErrorRate = 0, 1
	cfgs := []Config{
		{Retries: 2},
		{Retries: 2, Backoff: "exp", BackoffBase: 20 * time.Millisecond, BackoffMax: 200 * time.Millisecond, Jitter: "full"},
	}
	for _, cfg := range cfgs {
		up, base := startUpstream(t, f)
		cfg.BaseURL = base
		c := newClient(t, cfg)

		// When — 한 번 호출한다
		start := time.Now()
		_, res, err := c.GetRates(context.Background(), "USD", "")

		// Then — 시도 수·걸린 시간·업스트림이 받은 요청 수를 남긴다
		t.Logf("backoff=%-4s status=%d attempts=%d err=%v took=%s upstream.requests=%v",
			c.Config().Backoff, res.Status, res.Attempts, err, time.Since(start).Truncate(time.Millisecond), up.Stats()["requests"])
	}
}

func TestResetOnSecondRequestLog(t *testing.T) {
	// Given — 연결마다 두 번째 요청에서 응답 없이 RST 로 끊는 업스트림
	cases := []struct {
		name   string
		method string
		key    bool
		when   string
	}{
		{"GET", "GET", false, "before"},
		{"POST 키 없음", "POST", false, "before"},
		{"POST 키 있음", "POST", true, "before"},
		{"POST 키 없음 · 처리 뒤 끊기", "POST", false, "after"},
		{"POST 키 있음 · 처리 뒤 끊기", "POST", true, "after"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := upstream.DefaultFaults()
			f.Latency, f.ResetNth, f.ResetWhen = 0, 2, tc.when
			up, base := startUpstream(t, f)
			c := newClient(t, Config{BaseURL: base, IdempotencyKey: tc.key})

			// When — 같은 클라이언트로 세 번 부른다 (재시도 정책은 끔)
			for i := 1; i <= 3; i++ {
				var status, attempts int
				var err error
				if tc.method == "GET" {
					var r Result
					_, r, err = c.GetRates(context.Background(), "USD", "")
					status, attempts = r.Status, r.Attempts
				} else {
					var r Result
					r, err = c.CreateConversion(context.Background(), []byte(`{"from":"USD","to":"KRW","amount":1}`), NewIdempotencyKey(), "")
					status, attempts = r.Status, r.Attempts
				}
				// Then — 호출마다 결과를 남긴다
				t.Logf("call %d: status=%d attempts=%d err=%v", i, status, attempts, err)
			}
			s, us := c.Snapshot(), up.Stats()
			t.Logf("got_conn_extra=%d conn_new=%d | upstream requests=%v resets=%v created=%v replayed=%v",
				s["got_conn_extra"], s["conn_new"], us["requests"], us["resets"], us["conversions_created"], us["conversions_replayed"])
		})
	}
}

func TestETagCacheLog(t *testing.T) {
	for _, maxAge := range []int{0, 1} {
		// Given — max-age 가 다른 업스트림과 etag 캐시를 켠 클라이언트
		f := upstream.DefaultFaults()
		f.Latency, f.MaxAge, f.BodySize = 0, maxAge, 4096
		up, base := startUpstream(t, f)
		c := newClient(t, Config{BaseURL: base, Cache: "etag"})

		// When — 0.4초 간격으로 다섯 번 조회한다
		var seq []string
		for i := 0; i < 5; i++ {
			_, r, err := c.GetRates(context.Background(), "USD", "")
			if err != nil {
				t.Fatal(err)
			}
			seq = append(seq, r.Cache)
			time.Sleep(400 * time.Millisecond)
		}

		// Then — 캐시 결과 순서와 업스트림 통계를 남긴다
		us := up.Stats()
		t.Logf("max_age=%d cache=%v | upstream requests=%v not_modified=%v body_bytes=%v",
			maxAge, seq, us["requests"], us["not_modified"], us["body_bytes"])
	}
}
