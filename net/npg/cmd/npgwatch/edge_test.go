package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeRun 은 run.sh 호출을 기록만 하고 아무것도 띄우지 않는다.
type fakeRun struct {
	mu    sync.Mutex
	calls []string
	fail  string        // 이 하위 명령이면 실패
	gate  chan struct{} // 비어 있지 않으면 풀릴 때까지 기다린다
}

func (f *fakeRun) run(_ context.Context, args ...string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, strings.Join(args, " "))
	f.mu.Unlock()
	if f.gate != nil {
		<-f.gate
	}
	if args[0] == f.fail {
		return "curl: (7) Failed to connect\n", errors.New("exit status 7")
	}
	return "", nil
}

func (f *fakeRun) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func testEdge(f *fakeRun) *edge {
	e := newEdge("/caddy", "127.0.0.1:8000", "scenarios/x.json")
	e.run = f.run
	e.reachable = func(string) bool { return false }
	e.sleep = func(context.Context, time.Duration) {}
	return e
}

func waitIdle(t *testing.T, e *edge) {
	t.Helper()
	require.Eventually(t, func() bool { return e.view().Busy == "" }, time.Second, time.Millisecond)
}

func TestEdgeOffWhenFlagEmpty(t *testing.T) {
	// Given — -caddy-dir 를 주지 않은 lab
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")

	// When
	lb.enableEdge("", "127.0.0.1:8000", "x.json")
	st := lb.state()

	// Then — 엣지 칸이 없고 e1 장면은 목록에도, 시작에서도 빠진다
	require.Nil(t, lb.edge)
	require.Nil(t, st.Edge)
	for _, s := range st.Scenes {
		require.NotEqual(t, "e1", s.ID)
	}
	require.Error(t, lb.startScene("e1"))
}

func TestEdgeOnShowsE1Scene(t *testing.T) {
	// Given — -caddy-dir 를 준 lab
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")

	// When
	lb.enableEdge("/caddy", "127.0.0.1:8000", "x.json")
	st := lb.state()

	// Then — 엣지 상태가 실리고 e1 장면이 보인다
	require.NotNil(t, st.Edge)
	require.Equal(t, "127.0.0.1:8000", st.Edge.Addr)
	ids := []string{}
	for _, s := range st.Scenes {
		ids = append(ids, s.ID)
	}
	require.Contains(t, ids, "e1")
}

func TestSceneEdgePlaceholder(t *testing.T) {
	// Given — {E} 를 쓰는 장면과, 실행 파일이 없는 lab(띄우기는 실패해도 인자는 만들어진다)
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	lb.enableEdge("/caddy", "127.0.0.1:8000", "x.json")

	// When
	_ = lb.startScene("e1")

	// Then — 부하의 -url 은 엣지 주소로 바뀌고 프록시 주소는 쓰지 않는다
	p := lb.procs["load:get"]
	require.NotNil(t, p)
	require.Contains(t, p.args, "http://127.0.0.1:8000/v1/devices/7")
	require.NotContains(t, strings.Join(p.args, " "), "{E}")
	require.NotContains(t, strings.Join(p.args, " "), "127.0.0.1:2")
}

func TestEdgeRouteGuard(t *testing.T) {
	// Given — 가짜 run.sh 를 끼운 엣지 칸
	f := &fakeRun{}
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	lb.enableEdge("/caddy", "127.0.0.1:8000", "x.json")
	lb.edge.run = f.run
	lb.edge.sleep = func(context.Context, time.Duration) {}
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

	// When / Then — 헤더 없는 POST 는 run.sh 를 부르지 않고 거절, 없는 동작도 거절
	require.Equal(t, http.StatusForbidden, post("/api/edge/down", false))
	require.Equal(t, http.StatusBadRequest, post("/api/edge/rm", true))
	require.Empty(t, f.got())
	require.Equal(t, http.StatusNoContent, post("/api/edge/down", true))
	waitIdle(t, lb.edge)
	require.Equal(t, []string{"down"}, f.got())
}

func TestEdgeRouteOffRejects(t *testing.T) {
	// Given — 엣지 칸이 꺼진 lab
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	mux := http.NewServeMux()
	lb.routes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/edge/up", nil)
	req.Header.Set("X-NPG", "1")
	rec := httptest.NewRecorder()

	// When
	mux.ServeHTTP(rec, req)

	// Then
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEdgeActions(t *testing.T) {
	t.Run("재시작은 down 뒤 쉬고 up, 설정만 바꾸기는 load", func(t *testing.T) {
		// Given
		f := &fakeRun{}
		e := testEdge(f)
		var slept time.Duration
		e.sleep = func(_ context.Context, d time.Duration) { slept = d }

		// When — 재시작 뒤 설정 교체
		require.NoError(t, e.do(context.Background(), "restart"))
		waitIdle(t, e)
		require.NoError(t, e.do(context.Background(), "load"))
		waitIdle(t, e)

		// Then — 설정 경로는 -caddy-config 값, 쉬는 시간은 기본 2초, 기록에 시작·끝이 남는다
		require.Equal(t, []string{"down", "up scenarios/x.json", "load scenarios/x.json"}, f.got())
		require.Equal(t, 2*time.Second, slept)
		ev := e.view().Events
		require.Len(t, ev, 2)
		require.Equal(t, "restart", ev[0].Kind)
		require.NotZero(t, ev[0].End)
		require.GreaterOrEqual(t, ev[0].End, ev[0].Start)
	})

	t.Run("도는 중에 다른 동작을 누르면 거절한다", func(t *testing.T) {
		// Given — run.sh 가 끝나지 않는 상태
		f := &fakeRun{gate: make(chan struct{})}
		e := testEdge(f)
		require.NoError(t, e.do(context.Background(), "up"))

		// When
		err := e.do(context.Background(), "down")

		// Then
		require.ErrorIs(t, err, errRunning)
		require.Equal(t, "up", e.view().Busy)
		close(f.gate)
		waitIdle(t, e)
	})

	t.Run("실패하면 기록에 마지막 줄이 남는다", func(t *testing.T) {
		// Given — load 가 실패하는 run.sh
		f := &fakeRun{fail: "load"}
		e := testEdge(f)

		// When
		require.NoError(t, e.do(context.Background(), "load"))
		waitIdle(t, e)

		// Then
		ev := e.view().Events
		require.Len(t, ev, 1)
		require.Contains(t, ev[0].Err, "Failed to connect")
	})
}

func TestEdgeViewReachable(t *testing.T) {
	// Given — 포트에 접속되는지를 알려 주는 함수만 바꾼 엣지
	e := testEdge(&fakeRun{})
	e.reachable = func(addr string) bool { return addr == "127.0.0.1:8000" }

	// When / Then
	require.True(t, e.view().Up)
}

func TestParseLoadLineKinds(t *testing.T) {
	// Given — 재시작 중 한 줄: 성공, refused, 본문 읽다 끊김, HTTP 502 가 섞였다
	line := "16:47:48 get inflight=10 n=40 200:20 refused:12 body_reset:3 eof:2 502:3 p50=2ms p99=9ms max=10ms"

	// When
	_, st, ok := parseLoadLine(line)

	// Then — 상태 코드 없는 실패는 종류별 수가 남고 합계(other)와 같다
	require.True(t, ok)
	require.Equal(t, map[string]float64{"refused": 12, "body_reset": 3, "eof": 2}, st.kinds)
	require.Equal(t, 17.0, st.other)
}

func TestLoadRawErrGroups(t *testing.T) {
	// Given — 에러 종류가 섞인 줄 하나
	now := time.Unix(1000, 0)
	lb := &lab{loads: map[string]loadStat{
		"get": {at: now, n: 40, ok: 20, c5xx: 3, other: 17,
			kinds: map[string]float64{"refused": 12, "body_reset": 3, "eof": 1, "timeout": 1}},
	}}
	out := map[string]float64{}

	// When
	lb.loadRaw(now, out)

	// Then — body_reset 은 reset 으로, 나머지(timeout)는 etc 로 묶이고, HTTP 5xx 는 따로 센다
	require.Equal(t, 12.0, out["load.all.err.refused"])
	require.Equal(t, 3.0, out["load.all.err.reset"])
	require.Equal(t, 1.0, out["load.all.err.eof"])
	require.Equal(t, 1.0, out["load.all.err.etc"])
	require.Equal(t, 3.0, out["load.all.err.5xx"])
}
