package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDerive(t *testing.T) {
	t0 := time.Unix(100, 0)

	t.Run("카운터는 초당 변화량, 게이지는 그대로", func(t *testing.T) {
		// Given — 2초 사이 requests 가 10 → 30, inflight 는 지금 7
		prev := sample{at: t0, raw: map[string]float64{"proxy.requests": 10, "proxy.inflight": 3}}
		cur := sample{at: t0.Add(2 * time.Second), raw: map[string]float64{"proxy.requests": 30, "proxy.inflight": 7}, proxyUp: true}

		// When
		p := derive(prev, cur)

		// Then
		require.Equal(t, 10.0, p.V["proxy_rps"])
		require.Equal(t, 7.0, p.V["proxy_inflight"])
		require.True(t, p.ProxyUp)
		require.Equal(t, cur.at.UnixMilli(), p.T)
	})

	t.Run("카운터가 줄면 리셋으로 보고 0", func(t *testing.T) {
		// Given — fakeinv 를 DELETE /_stats 로 비웠다
		prev := sample{at: t0, raw: map[string]float64{"backend.requests": 500}}
		cur := sample{at: t0.Add(time.Second), raw: map[string]float64{"backend.requests": 4}}

		// When
		p := derive(prev, cur)

		// Then — 음수 속도(-496/s)가 아니다
		require.Equal(t, 0.0, p.V["backend_rps"])
	})

	t.Run("이번에 못 긁은 값은 비우고, 처음 보인 카운터는 0", func(t *testing.T) {
		// Given — 프록시가 죽어 proxy.* 가 없고, reasons.UF 는 이번에 처음 생겼다
		prev := sample{at: t0, raw: map[string]float64{"proxy.requests": 3, "backend.requests": 1}}
		cur := sample{at: t0.Add(time.Second), raw: map[string]float64{"backend.requests": 2, "backend.status_5xx": 0}}
		cur2 := sample{at: t0.Add(2 * time.Second), raw: map[string]float64{"proxy.reasons.UF": 5}}

		// When
		p := derive(prev, cur)
		q := derive(cur, cur2)

		// Then — 빈칸은 그래프에서 끊기고, 처음 보인 5 를 한 번에 튄 속도로 그리지 않는다
		_, ok := p.V["proxy_rps"]
		require.False(t, ok)
		require.Equal(t, 1.0, p.V["backend_rps"])
		require.Equal(t, 0.0, q.V["UF"])
	})
}

func TestFlatten(t *testing.T) {
	// Given — expvar 처럼 맵 안에 맵, 숫자 아닌 값 섞임
	doc := map[string]any{"requests": 3.0, "reasons": map[string]any{"UF": 1.0}, "name": "x"}
	out := map[string]float64{}

	// When
	flatten("proxy", doc, out)

	// Then
	require.Equal(t, map[string]float64{"proxy.requests": 3, "proxy.reasons.UF": 1}, out)
}

func TestRingKeepsLastN(t *testing.T) {
	// Given — 2칸짜리 고리
	r := &ring{n: 2}
	t0 := time.Unix(0, 0)

	// When — 샘플 넷이면 점은 셋 생기고 마지막 둘만 남는다
	for i := range 4 {
		r.add(sample{at: t0.Add(time.Duration(i) * time.Second), raw: map[string]float64{"proxy.requests": float64(i * 10)}})
	}

	// Then
	got := r.snapshot()
	require.Len(t, got, 2)
	require.Equal(t, t0.Add(3*time.Second).UnixMilli(), got[1].T)
	require.Equal(t, 10.0, got[1].V["proxy_rps"])
}
