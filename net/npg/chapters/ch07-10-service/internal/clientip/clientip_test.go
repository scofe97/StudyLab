package clientip

import (
	"testing"
)

// 신뢰 대역이 빈 판의 결과는 edge 창과의 계약이라 단언한다.
// 나머지 조합의 결과는 S7 의 예측 대상이라 찍기만 한다.

func TestEmptyTrust(t *testing.T) {
	// Given — 신뢰 대역이 없는 Resolver
	r, err := New(nil, "rightmost")
	if err != nil {
		t.Fatal(err)
	}

	// When — X-Forwarded-For 를 붙인 요청의 클라이언트 IP 를 정한다
	got := r.ClientIP("127.0.0.1:50000", []string{"203.0.113.7"})

	// Then — 계약 값과 같다
	if got.String() != "127.0.0.1" {
		t.Fatalf("client_ip = %s", got)
	}
}

func TestParsePrefixes(t *testing.T) {
	// Given — 대역과 맨 주소가 섞인 목록
	in := "127.0.0.1, 10.0.0.0/8 ,::1"

	// When — 대역 목록으로 바꾼다
	got, err := ParsePrefixes(in)

	// Then — 세 개가 되고 맨 주소는 단일 주소 대역이 된다
	if err != nil || len(got) != 3 || got[0].String() != "127.0.0.1/32" || got[2].String() != "::1/128" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestClientIPCombinations(t *testing.T) {
	// Given — 신뢰 설정 세 가지 × 요청 네 가지
	trusts := []struct{ name, cidrs, pick string }{
		{"신뢰 없음", "", "rightmost"},
		{"127.0.0.1/32 · rightmost", "127.0.0.1/32", "rightmost"},
		{"127.0.0.1/32 · leftmost", "127.0.0.1/32", "leftmost"},
		{"0.0.0.0/0 · rightmost", "0.0.0.0/0", "rightmost"},
	}
	reqs := []struct {
		remote string
		xff    []string
	}{
		{"127.0.0.1:40000", nil},
		{"127.0.0.1:40000", []string{"198.51.100.20"}},
		{"127.0.0.1:40000", []string{"203.0.113.7, 198.51.100.20"}},
		{"127.0.0.2:40000", []string{"203.0.113.7"}},
	}

	for _, tr := range trusts {
		p, err := ParsePrefixes(tr.cidrs)
		if err != nil {
			t.Fatal(err)
		}
		r, err := New(p, tr.pick)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range reqs {
			// When — 클라이언트 IP 를 정한다
			got := r.ClientIP(q.remote, q.xff)

			// Then — 조합과 결과를 남긴다
			t.Logf("%-26s remote=%-16s xff=%-32q → client_ip=%s", tr.name, q.remote, q.xff, got)
		}
	}
}
