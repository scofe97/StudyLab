package main

import (
	"regexp"
	"strconv"
	"strings"
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

func TestCountTimeWait(t *testing.T) {
	// Given — 머리줄 + 9090(0x2382)로 간 TIME_WAIT 2개, 9090 으로 간 ESTABLISHED 1개, 다른 포트 TIME_WAIT 1개, IPv6 줄 1개
	table := `  sl  local_address rem_address   st tx_queue rx_queue
   0: D08BA8C0:9C40 D08BA8C0:2382 06 00000000:00000000
   1: D08BA8C0:9C41 D08BA8C0:2382 06 00000000:00000000
   2: D08BA8C0:9C42 D08BA8C0:2382 01 00000000:00000000
   3: 0100007F:9C43 0100007F:46A0 06 00000000:00000000
   4: 0000000000000000FFFF00000100007F:9C44 0000000000000000FFFF00000100007F:2382 06 00000000:00000000
`

	// When
	n, err := countTimeWait(strings.NewReader(table), 9090)

	// Then — 상태 06 이면서 상대 포트가 9090 인 줄만 센다
	require.NoError(t, err)
	require.Equal(t, 3, n)
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

// 화면 크기 규칙(사용자 규칙): 20 이하는 2의 배수, 40 이하는 4의 배수, 그 위는 8의 배수.
// 글자는 14px 이상. 1px 테두리(머리카락 선)만 예외로 둔다.
func TestIndexHTMLSizeRule(t *testing.T) {
	// Given — 바이너리에 들어가는 index.html 의 모든 px 값과 글자 크기
	px := regexp.MustCompile(`(\d+(?:\.\d+)?)px`)
	font := regexp.MustCompile(`(?:font(?:-size)?:\s*|font = ")(\d+)px`)
	hairline := regexp.MustCompile(`border[a-z-]*:\s*1px`)
	html := string(indexHTML)

	// When — 규칙에 어긋나는 값을 모은다
	var bad []string
	for _, m := range px.FindAllStringSubmatchIndex(html, -1) {
		v, _ := strconv.ParseFloat(html[m[2]:m[3]], 64)
		start := max(0, m[0]-16)
		if hairline.MatchString(html[start:m[1]]) {
			continue
		}
		step := 8.0
		switch {
		case v <= 20:
			step = 2
		case v <= 40:
			step = 4
		}
		if v != float64(int(v)) || int(v)%int(step) != 0 {
			bad = append(bad, html[m[0]:m[1]]+" (배수 규칙)")
		}
	}
	for _, m := range font.FindAllStringSubmatch(html, -1) {
		if v, _ := strconv.Atoi(m[1]); v < 14 {
			bad = append(bad, m[0]+" (최소 14px)")
		}
	}

	// Then
	require.Empty(t, bad)
}
