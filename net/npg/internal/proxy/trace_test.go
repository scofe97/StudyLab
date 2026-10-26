package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sendTraced 는 기록장을 단 요청 하나를 traceConn 을 거쳐 보내고 본문까지 읽어 연결을 풀에 돌려놓습니다.
func sendTraced(t *testing.T, m *metrics, tr *http.Transport, url string) *reqInfo {
	t.Helper()
	ctx, info := withInfo(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(m.traceConn(req))
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return info
}

func TestTraceConn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	t.Run("첫 요청은 새 연결, 다음 요청은 풀에서 쉬던 연결을 다시 쓴다", func(t *testing.T) {
		// Given — 빈 풀을 가진 Transport
		m := newMetrics()
		tr := &http.Transport{}
		defer tr.CloseIdleConnections()

		// When — 같은 업스트림에 두 번 보내고, 사이에 잠깐 쉰다
		first := sendTraced(t, m, tr, srv.URL)
		time.Sleep(20 * time.Millisecond)
		second := sendTraced(t, m, tr, srv.URL)

		// Then — 카운터와 기록장이 새 연결 1 · 재사용 1 을 말한다
		require.Equal(t, int64(1), m.connsNew.Value())
		require.Equal(t, int64(1), m.connsReused.Value())
		require.False(t, first.reused)
		require.True(t, second.reused)
		require.GreaterOrEqual(t, second.idle, 20*time.Millisecond)
	})

	t.Run("keep-alive 를 끄면 매번 새 연결", func(t *testing.T) {
		// Given — 연결을 풀에 돌려놓지 않는 Transport
		m := newMetrics()
		tr := &http.Transport{DisableKeepAlives: true}

		// When
		sendTraced(t, m, tr, srv.URL)
		second := sendTraced(t, m, tr, srv.URL)

		// Then
		require.Equal(t, int64(2), m.connsNew.Value())
		require.Equal(t, int64(0), m.connsReused.Value())
		require.False(t, second.reused)
	})

	t.Run("기록장이 없는 요청도 카운터는 센다", func(t *testing.T) {
		// Given — withInfo 를 거치지 않은 요청
		m := newMetrics()
		tr := &http.Transport{}
		defer tr.CloseIdleConnections()
		req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
		require.NoError(t, err)

		// When
		resp, err := tr.RoundTrip(m.traceConn(req))
		require.NoError(t, err)
		resp.Body.Close()

		// Then — nil 기록장에 적다가 panic 나지 않고 카운터만 오른다
		require.Equal(t, int64(1), m.connsNew.Value())
	})
}
