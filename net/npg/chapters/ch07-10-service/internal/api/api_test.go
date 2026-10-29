package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"npg-practice/ch07-10-service/internal/upclient"
	"npg-practice/ch07-10-service/internal/upstream"
)

// edge 창과의 계약(/healthz, /whoami 형식, /api/v1/rates/{base} 형식)과 미들웨어 계약만 단언한다.

func newApp(t *testing.T, opt Options) *httptest.Server {
	t.Helper()
	if opt.Upstream == nil {
		up := upstream.New()
		f := upstream.DefaultFaults()
		f.Latency = 0
		up.SetFaults(f)
		uts := httptest.NewUnstartedServer(nil)
		up.Configure(uts.Config)
		uts.Start()
		t.Cleanup(uts.Close)
		c, err := upclient.New(upclient.Config{BaseURL: uts.URL})
		if err != nil {
			t.Fatal(err)
		}
		opt.Upstream = c
	}
	if opt.DebugAllow == nil {
		opt.DebugAllow = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	}
	app, err := New(opt)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(app.Handler())
	ts.Config.ConnState = app.ConnState
	ts.Start()
	t.Cleanup(ts.Close)
	t.Cleanup(func() { appByURL[ts.URL] = nil })
	appByURL[ts.URL] = app
	return ts
}

var appByURL = map[string]*App{}

func get(t *testing.T, url string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func TestHealthzContract(t *testing.T) {
	// Given — 막 뜬 App
	ts := newApp(t, Options{})

	// When — 드레인 전후로 /healthz 를 부른다
	before, b1 := get(t, ts.URL+"/healthz", nil)
	appByURL[ts.URL].StartDrain()
	after, b2 := get(t, ts.URL+"/healthz", nil)

	// Then — 평소 200 ok, 드레인 뒤 503 draining
	if before.StatusCode != 200 || strings.TrimSpace(b1) != "ok" {
		t.Fatalf("before: %d %q", before.StatusCode, b1)
	}
	if after.StatusCode != 503 || strings.TrimSpace(b2) != "draining" {
		t.Fatalf("after: %d %q", after.StatusCode, b2)
	}
}

func TestWhoamiContract(t *testing.T) {
	// Given — 신뢰 프록시를 비운 App
	ts := newApp(t, Options{})

	// When — X-Forwarded-For 를 붙여 /whoami 를 부른다
	resp, body := get(t, ts.URL+"/whoami", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	var v map[string]string
	err := json.Unmarshal([]byte(body), &v)

	// Then — 세 키가 있고, 신뢰 대역이 비었으니 client_ip 는 remote_addr 의 IP 다
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%d %q %v", resp.StatusCode, body, err)
	}
	for _, k := range []string{"remote_addr", "client_ip", "x_forwarded_for"} {
		if _, ok := v[k]; !ok {
			t.Fatalf("키 %q 가 없다: %v", k, v)
		}
	}
	if !strings.HasPrefix(v["remote_addr"], v["client_ip"]+":") || v["x_forwarded_for"] != "203.0.113.7" {
		t.Fatalf("whoami = %v", v)
	}
}

func TestRatesContract(t *testing.T) {
	// Given — 가짜 업스트림에 붙은 App
	ts := newApp(t, Options{})

	// When — /api/v1/rates/usd 를 부른다
	resp, body := get(t, ts.URL+"/api/v1/rates/usd", nil)
	var r upclient.Rates
	err := json.Unmarshal([]byte(body), &r)

	// Then — 200 이고 base 가 대문자로 맞춰져 있다
	if err != nil || resp.StatusCode != 200 || r.Base != "USD" || len(r.Rates) == 0 {
		t.Fatalf("%d %q %v", resp.StatusCode, body, err)
	}
	t.Logf("X-Upstream-Attempts=%s X-Upstream-Cache=%s", resp.Header.Get("X-Upstream-Attempts"), resp.Header.Get("X-Upstream-Cache"))
}

func TestRequestIDEcho(t *testing.T) {
	// Given — App
	ts := newApp(t, Options{})

	// When — X-Request-ID 를 붙인 요청과 붙이지 않은 요청을 보낸다
	with, _ := get(t, ts.URL+"/healthz", map[string]string{"X-Request-ID": "abc-123"})
	without, _ := get(t, ts.URL+"/healthz", nil)

	// Then — 붙인 값은 그대로 돌아오고, 없으면 새로 만든다
	if with.Header.Get("X-Request-ID") != "abc-123" || without.Header.Get("X-Request-ID") == "" {
		t.Fatalf("with=%q without=%q", with.Header.Get("X-Request-ID"), without.Header.Get("X-Request-ID"))
	}
}

func TestRecoverMiddleware(t *testing.T) {
	// Given — 패닉을 내는 핸들러를 recover 미들웨어로 감싼다
	a, _ := New(Options{})
	h := a.recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()

	// When — 요청 하나를 처리한다
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// Then — 500 으로 답한다
	if rec.Code != 500 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestDebugPrefixRestriction(t *testing.T) {
	// Given — /debug/ 허용 대역이 루프백인 App 과 아무 대역도 없는 App
	open := newApp(t, Options{})
	closed := newApp(t, Options{DebugAllow: []netip.Prefix{}})

	// When — 루프백에서 /debug/vars 를 부른다
	r1, _ := get(t, open.URL+"/debug/vars", nil)
	r2, _ := get(t, closed.URL+"/debug/vars", nil)

	// Then — 설정대로 열리고 닫힌다
	if r1.StatusCode != 200 || r2.StatusCode != 403 {
		t.Fatalf("open=%d closed=%d", r1.StatusCode, r2.StatusCode)
	}
}

func TestLimitListenerCap(t *testing.T) {
	// Given — 상한 3 인 리스너
	ts := httptest.NewUnstartedServer(http.NotFoundHandler())
	l := LimitListener(ts.Listener, 3)

	// Then — 자리 수 설정이 적용된다
	if l.Cap() != 3 || l.InUse() != 0 {
		t.Fatalf("cap=%d in_use=%d", l.Cap(), l.InUse())
	}
	l.Close()
}
