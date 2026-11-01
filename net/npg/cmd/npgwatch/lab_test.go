package main

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseLoadLine(t *testing.T) {
	t.Run("라벨·상태 코드·지연을 읽는다", func(t *testing.T) {
		// Given — 라벨 get, 200 과 502 가 섞이고 timeout 도 있는 1초 줄
		line := "16:47:48 get inflight=50 n=921 200:158 502:761 timeout:2 p50=32ms p99=1.5s max=2s"

		// When
		label, st, ok := parseLoadLine(line)

		// Then — 2xx 는 ok, 5xx 는 c5xx, 코드 없는 실패는 other, 지연은 밀리초
		require.True(t, ok)
		require.Equal(t, "get", label)
		require.Equal(t, 50.0, st.inflight)
		require.Equal(t, 921.0, st.n)
		require.Equal(t, 158.0, st.ok)
		require.Equal(t, 761.0, st.c5xx)
		require.Equal(t, 2.0, st.other)
		require.Equal(t, 32.0, st.p50)
		require.Equal(t, 1500.0, st.p99)
	})

	t.Run("라벨이 없으면 load, 끝난 요청이 없으면 지연은 NaN", func(t *testing.T) {
		// Given — -label 없이 돌린 npgload 의 n=0 줄
		line := "15:07:04 inflight=112 n=0"

		// When
		label, st, ok := parseLoadLine(line)

		// Then
		require.True(t, ok)
		require.Equal(t, "load", label)
		require.Equal(t, 112.0, st.inflight)
		require.True(t, math.IsNaN(st.p99))
	})

	t.Run("요약 줄과 4xx", func(t *testing.T) {
		// Given — 요약 줄에는 inflight 가 없다
		_, _, ok := parseLoadLine("16:47:10 get 요약 sent=88831 n=88831 200:51450 502:37381 p50=14ms")
		_, st, _ := parseLoadLine("16:47:11 inflight=1 n=3 404:3 p50=1ms p99=1ms max=1ms")

		// Then
		require.False(t, ok)
		require.Equal(t, 3.0, st.c4xx)
	})
}

func TestStripANSI(t *testing.T) {
	// Given — zerolog 사람용 출력 한 줄
	line := "\x1b[90m16:47:19.460\x1b[0m \x1b[33mWRN\x1b[0m \x1b[1mupstream error\x1b[0m"

	// When / Then
	require.Equal(t, "16:47:19.460 WRN upstream error", stripANSI(line))
}

func TestLineLogKeepsTail(t *testing.T) {
	// Given — 3줄만 남기는 기록
	l := &lineLog{n: 3}

	// When
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		l.add(s)
	}

	// Then — 마지막 줄들이 순서대로 남는다
	require.Equal(t, []string{"e", "f", "g"}, l.tail(3))
	require.Equal(t, []string{"g"}, l.tail(1))
}

func TestLoadRawDropsStale(t *testing.T) {
	// Given — get 은 방금, search 는 5초 전에 마지막 줄이 왔다
	now := time.Unix(1000, 0)
	lb := &lab{loads: map[string]loadStat{
		"get":    {at: now, inflight: 10, n: 50, ok: 48, c5xx: 2, p50: 5, p99: 9},
		"search": {at: now.Add(-5 * time.Second), inflight: 3, n: 1, ok: 1, p50: 30, p99: 30},
	}}
	out := map[string]float64{}

	// When
	lb.loadRaw(now, out)

	// Then — 멈춘 search 는 빠지고 합계에도 안 들어간다
	require.Equal(t, 9.0, out["load.get.p99"])
	require.NotContains(t, out, "load.search.p99")
	require.Equal(t, 48.0, out["load.all.ok"])
	require.Equal(t, 2.0, out["load.all.5xx"])
}

func TestLabRoutesGuard(t *testing.T) {
	// Given — 실행 파일이 없는 곳을 가리키는 lab
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	mux := http.NewServeMux()
	lb.routes(mux)
	post := func(path string, header bool) int {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		if header {
			req.Header.Set("X-NPG", "1")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	// When / Then — 헤더 없는 POST 는 막고, 정한 프로그램·장면 밖의 이름은 거절한다
	require.Equal(t, http.StatusForbidden, post("/api/proc/backend/start", false))
	require.Equal(t, http.StatusBadRequest, post("/api/proc/sh/start", true))
	require.Equal(t, http.StatusBadRequest, post("/api/scene/rm-rf/start", true))
	require.Equal(t, http.StatusNoContent, post("/api/load/stop", true))
}

func TestParseAccessLine(t *testing.T) {
	now := time.Unix(1000, 0)

	t.Run("접근 줄에서 메서드·상태·플래그·소요 시간을 읽는다", func(t *testing.T) {
		// Given — 30초 만에 504 로 끝난 POST 접근 줄
		line := "09:15:01.530 INF attempts=1 bytes=0 client=127.0.0.1:53457 dur=30000.592291 flags=UT idle=37.498666 method=POST path=/v1/devices reused=true status=504"

		// When
		d, ok := parseAccessLine(line, now)

		// Then — dur 는 밀리초 그대로, 시각은 읽은 시각
		require.True(t, ok)
		require.Equal(t, finished{T: now.UnixMilli(), Method: "POST", Status: 504, Flags: "UT", Dur: 30000.592291}, d)
	})

	t.Run("method 나 status 가 없는 줄은 접근 줄이 아니다", func(t *testing.T) {
		// Given — 따옴표 안에 공백이 있는 WRN 줄과 시작 안내 줄
		lines := []string{
			`09:15:01.530 WRN upstream error error="context deadline exceeded" flags=UT`,
			"09:15:00.001 INF listening addr=127.0.0.1:18080",
			"── 09:15:00 시작: ./npgproxy -addr x",
		}

		for _, line := range lines {
			// When
			_, ok := parseAccessLine(line, now)

			// Then
			require.False(t, ok, line)
		}
	})
}

func TestProxyLine(t *testing.T) {
	now := time.Unix(1000, 0)
	cases := []struct {
		name         string
		line         string
		access, keep bool
	}{
		{"빠른 성공 GET 은 기록만", "10:00:00.001 INF dur=2.1 flags=- method=GET path=/v1/devices/7 status=200", true, false},
		{"4xx 이상은 창에 남긴다", "10:00:00.002 INF dur=3.0 flags=- method=GET path=/x status=404", true, true},
		{"1초 이상이면 성공이어도 남긴다", "10:00:00.003 INF dur=1000.0 flags=- method=POST path=/v1/devices status=201", true, true},
		{"1초 미만 성공 POST 는 남기지 않는다", "10:00:00.004 INF dur=999.9 flags=- method=POST path=/v1/devices status=201", true, false},
		{"WRN 줄은 접근 줄이 아니어도 남긴다", `10:00:00.005 WRN upstream error error="context deadline exceeded" flags=UT`, false, true},
		{"그냥 INF 줄은 버린다", "10:00:00.006 INF listening", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given — 위 한 줄

			// When
			_, access, keep := proxyLine(c.line, now)

			// Then
			require.Equal(t, c.access, access)
			require.Equal(t, c.keep, keep)
		})
	}
}

func TestFinishedLog(t *testing.T) {
	t.Run("평범한 GET 은 솎고 POST·느린 줄은 모두 남긴다", func(t *testing.T) {
		// Given — 지금 시각에 빠른 GET 100개, 30초 걸린 POST 3개
		now := time.Now()
		var l finishedLog
		for i := 0; i < 100; i++ {
			l.add(finished{T: now.UnixMilli(), Method: "GET", Status: 200, Dur: 2})
		}
		for i := 0; i < 3; i++ {
			l.add(finished{T: now.UnixMilli(), Method: "POST", Status: 504, Dur: 30000})
		}

		// When
		got := l.snapshot(now)

		// Then — GET 은 10개에 1개(10개), POST 는 3개 전부
		require.Len(t, got, 13)
	})

	t.Run("5분이 지난 요청은 빠진다", func(t *testing.T) {
		// Given — 6분 전 POST 하나와 방금 POST 하나
		now := time.Now()
		var l finishedLog
		l.add(finished{T: now.Add(-6 * time.Minute).UnixMilli(), Method: "POST", Status: 504, Dur: 30000})
		l.add(finished{T: now.UnixMilli(), Method: "POST", Status: 504, Dur: 30000})

		// When
		got := l.snapshot(now)

		// Then
		require.Len(t, got, 1)
		require.Equal(t, now.UnixMilli(), got[0].T)
	})
}
