package restrictprefix

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// provisioned 는 Caddy 가 하는 순서(JSON 채움 → Provision → Validate)를 흉내 낸다.
func provisioned(t *testing.T, prefixes ...string) (*RestrictPrefix, error) {
	t.Helper()
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	t.Cleanup(cancel)

	p := &RestrictPrefix{Prefixes: prefixes}
	if err := p.Provision(ctx); err != nil {
		return nil, err
	}
	return p, p.Validate()
}

// 계약: Validate 는 세그먼트와 비교할 수 없는 접두사를 거부한다.
func TestValidateRejectsUnusablePrefix(t *testing.T) {
	cases := map[string][]string{
		"빈 문자열":       {".git", ""},
		"슬래시 포함":      {"admin/"},
		"경로 전체를 접두사로": {"/internal"},
	}
	for name, prefixes := range cases {
		t.Run(name, func(t *testing.T) {
			// Given — 잘못된 접두사가 섞인 설정
			// When — Caddy 와 같은 순서로 Provision·Validate
			_, err := provisioned(t, prefixes...)

			// Then — 거부되고, 그 메시지를 남긴다
			if err == nil {
				t.Fatalf("Validate 가 %q 를 받아들였다", prefixes)
			}
			t.Logf("거부 메시지: %v", err)
		})
	}
}

// 계약: 접두사를 하나도 주지 않으면 Provision 이 "." 하나로 채우고 Validate 를 통과한다.
func TestProvisionFillsDefault(t *testing.T) {
	// Given — prefixes 가 빠진 설정
	// When
	p, err := provisioned(t)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Prefixes, []string{"."}) {
		t.Fatalf("기본 접두사 = %q", p.Prefixes)
	}
}

// 계약: 걸린 요청은 다음 핸들러로 가지 않고 404 HandlerError 가 되며,
// 걸리지 않은 요청은 다음 핸들러로 그대로 간다.
func TestServeHTTPBlocksOrPasses(t *testing.T) {
	// Given — 기본 접두사 "." 로 준비한 모듈
	p, err := provisioned(t)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path    string
		blocked bool
	}{
		{"/index.html", false},
		{"/.env", true},
		{"/assets/.secrets/token.txt", true},
	} {
		// When
		called, err := serve(p, tc.path)

		// Then
		if tc.blocked {
			var herr caddyhttp.HandlerError
			if called || !errors.As(err, &herr) || herr.StatusCode != http.StatusNotFound {
				t.Fatalf("%s: next 호출=%v err=%v", tc.path, called, err)
			}
		} else if !called || err != nil {
			t.Fatalf("%s: next 호출=%v err=%v", tc.path, called, err)
		}
	}
}

// 예측용: 경계에 선 경로들이 어떻게 처리되는지 찍기만 한다.
// 실행 전에 README E6 의 예측 칸을 먼저 채운다.
func TestServeHTTPEdgePaths(t *testing.T) {
	// Given — 기본 접두사 "." 로 준비한 모듈
	p, err := provisioned(t)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"/.well-known/security.txt",
		"/%2eenv",
		"/docs/v1.2/index.html",
		"/a/../.env",
		"/..",
	} {
		// When
		called, err := serve(p, path)

		// Then — 단언하지 않고 기록만 남긴다
		t.Logf("%-28s next 호출=%-5v err=%v", path, called, err)
	}
}

// 계약: Caddyfile 의 인자가 그대로 Prefixes 가 된다.
func TestUnmarshalCaddyfile(t *testing.T) {
	// Given
	d := caddyfile.NewTestDispenser(`restrict_prefix .env .git`)
	var p RestrictPrefix

	// When
	err := p.UnmarshalCaddyfile(d)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Prefixes, []string{".env", ".git"}) {
		t.Fatalf("Prefixes = %q", p.Prefixes)
	}
}

// serve 는 path 로 요청 하나를 흘려 보내고 next 가 불렸는지와 반환 오류를 돌려준다.
func serve(p *RestrictPrefix, path string) (called bool, err error) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	next := caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error {
		called = true
		return nil
	})
	err = p.ServeHTTP(httptest.NewRecorder(), req, next)
	return called, err
}
