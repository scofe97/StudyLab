package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/scofe97/runners-lab/net/npg/internal/upstream"
)

// waitDone 은 ctx 가 끝나길 기다린다. 기한이 안 걸려 있으면 시험이 멈추지 않게 2초 뒤 포기한다.
func waitDone(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
	}
}

func TestWithDeadline(t *testing.T) {
	t.Run("안쪽 핸들러는 d 가 지나면 DeadlineExceeded 로 끝나는 context 를 받는다", func(t *testing.T) {
		// Given
		var got context.Context
		h := withDeadline(50*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Context()
			waitDone(got)
		}))

		// When
		start := time.Now()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		// Then: classify 가 UT 로 고르는 에러다
		require.ErrorIs(t, got.Err(), context.DeadlineExceeded)
		require.Less(t, time.Since(start), time.Second)
	})

	t.Run("손님이 먼저 떠나면 Canceled 가 내려와 DC 로 남는다", func(t *testing.T) {
		// Given: 손님 요청 context 를 직접 취소할 수 있게 둔다
		parent, leave := context.WithCancel(context.Background())
		var got context.Context
		h := withDeadline(time.Hour, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Context()
			leave() // 처리 도중 손님이 떠난다
			waitDone(got)
		}))

		// When
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(parent))

		// Then
		require.ErrorIs(t, got.Err(), context.Canceled)
	})

	t.Run("핸들러가 일찍 끝나면 타이머를 바로 치운다", func(t *testing.T) {
		// Given
		var got context.Context
		h := withDeadline(time.Hour, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Context()
		}))

		// When
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		// Then: 한 시간을 기다리지 않고 이미 끝나 있다 (defer cancel)
		require.ErrorIs(t, got.Err(), context.Canceled)
	})
}

func TestProxyUpstreamLayer(t *testing.T) {
	t.Run("업스트림이 요청 전체 기한을 넘기면 504 와 UT 로 남는다 (I1)", func(t *testing.T) {
		t.Parallel() // 15초를 실제로 기다리므로 다른 시험과 겹쳐 돌린다

		// Given: 프록시가 끊을 때까지 답하지 않는 업스트림
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(upstream.RequestTimeout + 5*time.Second):
			}
		}))
		t.Cleanup(backend.Close)
		front, logs, vars := startProxy(t, backend.URL)

		// When
		start := time.Now()
		resp, err := http.Get(front.URL + "/v1/devices/1")
		require.NoError(t, err)
		resp.Body.Close()

		// Then: 손님 기한(30초)보다 먼저 프록시가 포기한다
		took := time.Since(start)
		require.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
		require.GreaterOrEqual(t, took, upstream.RequestTimeout)
		require.Less(t, took, upstream.RequestTimeout+2*time.Second)
		line := logs.accessLine(t)
		require.Equal(t, "UT", line["flags"])
		require.Contains(t, vars.String(), `"reasons": {"UT": 1}`)
	})

	t.Run("검색이 8칸을 다 차지해도 단건 조회는 다른 풀로 지나간다 (I1 격벽)", func(t *testing.T) {
		t.Parallel()

		// Given: 검색은 release 가 닫힐 때까지 붙잡고, 그 밖은 바로 답하는 업스트림
		var searching atomic.Int64
		release := make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("q") {
				searching.Add(1)
				<-release
			}
			io.WriteString(w, "ok")
		}))
		t.Cleanup(backend.Close)
		front, _, _ := startProxy(t, backend.URL)
		t.Cleanup(unblock) // 단언이 실패해도 붙잡힌 요청을 먼저 풀어 front·backend Close 가 멈추지 않게 (Cleanup 은 나중 것부터라 startProxy 뒤에 건다)

		// When: 검색 9개를 동시에 보낸다
		var wg sync.WaitGroup
		codes := make(chan int, 9)
		for range 9 {
			wg.Go(func() {
				resp, err := http.Get(front.URL + "/v1/devices?q=sw")
				if err != nil {
					t.Errorf("검색: %v", err)
					return
				}
				resp.Body.Close()
				codes <- resp.StatusCode
			})
		}
		require.Eventually(t, func() bool { return searching.Load() == 8 }, 2*time.Second, 10*time.Millisecond)
		time.Sleep(100 * time.Millisecond) // 아홉째가 9번째 연결을 열 틈을 준다

		// Then: 백엔드에 닿은 검색은 8개뿐이고(아홉째는 프록시 안에서 기다림), 단건 조회는 바로 성공한다
		require.Equal(t, int64(8), searching.Load())
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(front.URL + "/v1/devices/42")
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		unblock()
		wg.Wait()
		close(codes)
		for c := range codes {
			require.Equal(t, http.StatusOK, c) // 아홉째도 빈 칸이 생기면 지나간다
		}
		require.Equal(t, int64(9), searching.Load())
	})

	t.Run("검색 칸 8 + 줄 7 이 다 차면 열여섯째는 기다리지 않고 503 UO (ADR 0012 결정 6)", func(t *testing.T) {
		t.Parallel()

		// Given: 검색을 붙잡는 업스트림에 검색 15개를 먼저 보내 칸과 줄을 채운다
		var searching atomic.Int64
		release := make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			searching.Add(1)
			<-release
			io.WriteString(w, "ok")
		}))
		t.Cleanup(backend.Close)
		front, logs, vars := startProxy(t, backend.URL)
		t.Cleanup(unblock) // startProxy 뒤에 걸어야 front.Close 보다 먼저 돈다
		var wg sync.WaitGroup
		codes := make(chan int, 15)
		for range 15 {
			wg.Go(func() {
				resp, err := http.Get(front.URL + "/v1/devices?q=sw")
				if err != nil {
					t.Errorf("검색: %v", err)
					return
				}
				resp.Body.Close()
				codes <- resp.StatusCode
			})
		}
		require.Eventually(t, func() bool { return searching.Load() == 8 }, 2*time.Second, 10*time.Millisecond)
		// 15개 모두 프록시 핸들러에 들어왔는지 inflight 로 확인한다. 줄 입장은 기다림 없는 select 라 그 뒤 곧바로 끝난다
		require.Eventually(t, func() bool {
			var m struct{ Inflight int64 }
			return json.Unmarshal([]byte(vars.String()), &m) == nil && m.Inflight == 15
		}, 2*time.Second, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond) // 핸들러 입장에서 입장권까지의 짧은 틈

		// When: 열여섯째 검색
		start := time.Now()
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(front.URL + "/v1/devices?q=sw")
		require.NoError(t, err)
		resp.Body.Close()

		// Then: 바로 503, 접근 로그와 카운터에 UO. 백엔드에는 여전히 8개뿐이다
		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		require.Less(t, time.Since(start), 500*time.Millisecond)
		require.Equal(t, "UO", logs.accessLine(t)["flags"])
		require.Contains(t, vars.String(), `"reasons": {"UO": 1}`)
		require.Equal(t, int64(8), searching.Load())

		// 줄에 섰던 7개는 칸이 비면 모두 지나간다
		unblock()
		wg.Wait()
		close(codes)
		for c := range codes {
			require.Equal(t, http.StatusOK, c)
		}
		require.Equal(t, int64(15), searching.Load())
	})
}
