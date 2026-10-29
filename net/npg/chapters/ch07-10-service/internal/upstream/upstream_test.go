package upstream

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestServer(t *testing.T, f Faults) (*Server, *httptest.Server) {
	t.Helper()
	up := New()
	up.SetFaults(f)
	ts := httptest.NewUnstartedServer(nil)
	up.Configure(ts.Config)
	ts.Start()
	t.Cleanup(ts.Close)
	return up, ts
}

func TestRatesShape(t *testing.T) {
	// Given — 기본 장애 설정의 업스트림
	_, ts := newTestServer(t, DefaultFaults())

	// When — 환율을 조회한다
	resp, err := http.Get(ts.URL + "/v1/rates/USD")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r Rates
	err = json.NewDecoder(resp.Body).Decode(&r)

	// Then — 형식: 200, base, 다섯 통화의 환율, ETag 헤더
	if err != nil || resp.StatusCode != 200 || r.Base != "USD" || len(r.Rates) != len(currencies)-1 {
		t.Fatalf("status=%d base=%q rates=%d err=%v", resp.StatusCode, r.Base, len(r.Rates), err)
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatal("etag=true 인데 ETag 헤더가 없다")
	}
	t.Logf("ETag=%s Cache-Control=%s Content-Length=%s", resp.Header.Get("ETag"), resp.Header.Get("Cache-Control"), resp.Header.Get("Content-Length"))
}

func TestUnknownCurrency(t *testing.T) {
	// Given — 기본 업스트림
	_, ts := newTestServer(t, DefaultFaults())

	// When — 모르는 통화를 조회한다
	resp, err := http.Get(ts.URL + "/v1/rates/XYZ")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Then — 404
	if resp.StatusCode != 404 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestFaultsApply(t *testing.T) {
	// Given — 기본 설정
	f := DefaultFaults()

	// When — 알려진 키와 모르는 키를 각각 적용한다
	ok, err1 := f.Apply(url.Values{"latency": {"2s"}, "error_rate": {"0.5"}, "reset_when": {"after"}})
	_, err2 := f.Apply(url.Values{"nope": {"1"}})
	_, err3 := f.Apply(url.Values{"reset_when": {"sometimes"}})

	// Then — 알려진 키만 바뀌고, 모르는 키·잘못된 값은 에러
	if err1 != nil || ok.Latency.String() != "2s" || ok.ErrorRate != 0.5 || ok.ResetWhen != "after" {
		t.Fatalf("apply: %+v %v", ok, err1)
	}
	if err2 == nil || err3 == nil {
		t.Fatalf("잘못된 키·값이 통과했다: %v / %v", err2, err3)
	}
}

func TestFaultsEndpoint(t *testing.T) {
	// Given — 기본 업스트림
	up, ts := newTestServer(t, DefaultFaults())

	// When — PUT /_faults 로 손잡이를 켠다
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/_faults?latency=1ms&body_size=100", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Then — 설정이 적용된다
	if f := up.Faults(); resp.StatusCode != 200 || f.BodySize != 100 || f.Latency.String() != "1ms" {
		t.Fatalf("status=%d faults=%+v", resp.StatusCode, f)
	}
}

func TestConversionsLog(t *testing.T) {
	// Given — 기본 업스트림
	up, ts := newTestServer(t, DefaultFaults())
	post := func(key string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/conversions", strings.NewReader(`{"from":"USD","to":"KRW","amount":10}`))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var c Conversion
		json.NewDecoder(resp.Body).Decode(&c)
		return resp.StatusCode, c.ID
	}

	// When — 키 없이 두 번, 같은 키로 두 번 보낸다
	for _, k := range []string{"", "", "k1", "k1"} {
		code, id := post(k)
		// Then — 응답과 통계를 남긴다
		t.Logf("key=%-3q → %d id=%s", k, code, id)
	}
	st := up.Stats()
	t.Logf("conversions_created=%v conversions_replayed=%v", st["conversions_created"], st["conversions_replayed"])
}
