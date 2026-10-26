package proxy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsVars(t *testing.T) {
	// Given — 카운터를 몇 번 올린 metrics
	m := newMetrics()
	m.requests.Add(3)
	m.inflight.Add(1)
	m.reasons.Add(string(ReasonTimeout), 2)

	// When — /debug/vars 가 내보낼 JSON
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(m.vars.String()), &got))

	// Then — 올린 값이 그대로 보이고, 안 올린 카운터도 0 으로 보인다
	require.Equal(t, map[string]any{
		"requests":     float64(3),
		"inflight":     float64(1),
		"conns_new":    float64(0),
		"conns_reused": float64(0),
		"reasons":      map[string]any{"UT": float64(2)},
	}, got)
}
