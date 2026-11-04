package main

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpoints(t *testing.T) {
	// Given
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// When / Then
	cases := []struct {
		path string
		want int // 본문 길이, -1 이면 길이 검사 안 함
	}{
		{"/small", len(`{"ok":true}`)},
		{"/big?kb=4", 4096},
	}
	for _, c := range cases {
		resp, err := http.Get(srv.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(body) != c.want {
			t.Errorf("%s: status=%d len=%d want len %d", c.path, resp.StatusCode, len(body), c.want)
		}
	}
}

func TestSlowWaits(t *testing.T) {
	// Given
	srv := httptest.NewServer(newMux())
	defer srv.Close()
	start := time.Now()

	// When
	resp, err := http.Get(srv.URL + "/slow?ms=50")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Then
	if time.Since(start) < 50*time.Millisecond {
		t.Errorf("slow returned too early: %v", time.Since(start))
	}
}

func TestSSESendsFirstEvent(t *testing.T) {
	// Given
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// When
	resp, err := http.Get(srv.URL + "/sse?interval=50")
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	resp.Body.Close()

	// Then
	if err != nil || !strings.HasPrefix(line, "data: ") {
		t.Errorf("first line = %q, err = %v", line, err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content-type = %q", ct)
	}
}
