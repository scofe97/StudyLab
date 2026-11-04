package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	// Given — 1..100ms
	var s []time.Duration
	for i := 1; i <= 100; i++ {
		s = append(s, time.Duration(i)*time.Millisecond)
	}
	sortDurations(s)

	// When / Then
	for p, want := range map[float64]time.Duration{
		50: 50 * time.Millisecond, 90: 90 * time.Millisecond, 99: 99 * time.Millisecond, 100: 100 * time.Millisecond,
	} {
		if got := percentile(s, p); got != want {
			t.Errorf("p%v = %v, want %v", p, got, want)
		}
	}
	if percentile(nil, 50) != 0 {
		t.Error("empty sample must give 0")
	}
}

func TestClassify(t *testing.T) {
	// Given / When / Then
	if got := classify(os.ErrDeadlineExceeded); got != "timeout" {
		t.Errorf("got %s", got)
	}
	if got := classify(errors.New("boom")); got != "other" {
		t.Errorf("got %s", got)
	}
}

func TestParseEventNs(t *testing.T) {
	// Given / When / Then
	cases := map[string]struct {
		ns int64
		ok bool
	}{
		"data: 1730000000123456789\n": {1730000000123456789, true},
		"data:42\r\n":                 {42, true},
		"\n":                          {0, false},
		": comment\n":                 {0, false},
		"data: abc\n":                 {0, false},
	}
	for line, want := range cases {
		ns, ok := parseEventNs(line)
		if ns != want.ns || ok != want.ok {
			t.Errorf("parseEventNs(%q) = %d,%v want %d,%v", line, ns, ok, want.ns, want.ok)
		}
	}
}
