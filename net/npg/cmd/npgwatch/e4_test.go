package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newE4Lab 은 엣지와 proxy2 를 모두 켠 lab 이다. run.sh 는 가짜로 바꿔 끼운다.
func newE4Lab(t *testing.T, f *fakeRun) *lab {
	t.Helper()
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	lb.enableEdge("/caddy", "127.0.0.1:8000", "x.json")
	lb.enableProxy2("127.0.0.1:4", "127.0.0.1:5", 5*time.Second)
	lb.edge.run = f.run
	lb.edge.sleep = func(context.Context, time.Duration) {}
	lb.edge.reachable = func(string) bool { return false }
	return lb
}

func routePost(mux *http.ServeMux, path string, header bool) int {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if header {
		req.Header.Set("X-NPG", "1")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code
}

func TestProxy2OffWhenFlagEmpty(t *testing.T) {
	// Given — 엣지는 켜고 -proxy2-addr 는 비운 lab
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	lb.enableEdge("/caddy", "127.0.0.1:8000", "x.json")
	mux := http.NewServeMux()
	lb.routes(mux)

	// When
	lb.enableProxy2("", "127.0.0.1:5", 5*time.Second)
	st := lb.state()

	// Then — 두 번째 프록시도 e4 장면도 없고, 첫 프록시에는 -drain-delay 를 넘기지 않으며, E4 버튼과 proxy2 시작은 거절한다
	require.False(t, lb.proxy2)
	require.False(t, st.Proxy2)
	require.NotContains(t, lb.procs, "proxy2")
	require.NotContains(t, lb.procs["proxy"].args, "-drain-delay")
	for _, s := range st.Scenes {
		require.NotEqual(t, "e4", s.ID)
	}
	require.Error(t, lb.startScene("e4"))
	require.Equal(t, http.StatusBadRequest, routePost(mux, "/api/proc/proxy2/start", true))
	require.Equal(t, http.StatusBadRequest, routePost(mux, "/api/edge/board-both", true))
	require.Equal(t, http.StatusBadRequest, routePost(mux, "/api/edge/proxy1-restart", true))
}

func TestProxy2OnAddsProcAndDrain(t *testing.T) {
	// Given / When — 엣지와 proxy2 를 켠 lab
	lb := newE4Lab(t, &fakeRun{})
	st := lb.state()

	// Then — 두 프록시가 같은 fakeinv 를 보고 -drain-delay 5s 를 받는다. e4 장면이 보인다
	p2 := lb.procs["proxy2"]
	require.NotNil(t, p2)
	require.Contains(t, strings.Join(p2.args, " "), "-addr 127.0.0.1:4 -admin 127.0.0.1:5 -upstream http://127.0.0.1:1")
	for _, p := range []*proc{lb.procs["proxy"], p2} {
		require.Contains(t, strings.Join(p.args, " "), "-drain-delay 5s")
		require.Equal(t, 9*time.Second, p.waitDelay)
	}
	require.True(t, st.Proxy2)
	var ids []string
	for _, s := range st.Scenes {
		ids = append(ids, s.ID)
	}
	require.Contains(t, ids, "e4")
}

func TestBoardsRunBaseThenPieces(t *testing.T) {
	const load = "load scenarios/e4-two-proxies.json"
	const active = "admin POST /id/npg_proxy/health_checks scenarios/e4-active-health.json"
	const retry = "admin POST /id/npg_proxy/load_balancing scenarios/e4-try-duration.json"
	for _, tc := range []struct {
		kind string
		want []string
	}{
		{"board-base", []string{load}},
		{"board-active", []string{load, active}},
		{"board-retry", []string{load, retry}},
		{"board-both", []string{load, active, retry}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			// Given — 가짜 run.sh 를 끼운 E4 lab
			f := &fakeRun{}
			lb := newE4Lab(t, f)

			// When — 판 버튼
			require.NoError(t, lb.edge.do(context.Background(), tc.kind))
			waitIdle(t, lb.edge)

			// Then — 바탕 설정을 /load 로 되돌린 뒤 판의 조각만 관리 API 로 올리고, 지금 판이 그것으로 보인다
			require.Equal(t, tc.want, f.got())
			require.Equal(t, tc.kind, lb.edge.view().Board)
		})
	}
}

func TestBoardFailureClearsBoard(t *testing.T) {
	// Given — 먼저 기본 판이 올라가 있고, 다음 판의 조각을 올리는 admin 호출이 실패하는 lab
	f := &fakeRun{}
	lb := newE4Lab(t, f)
	require.NoError(t, lb.edge.do(context.Background(), "board-base"))
	waitIdle(t, lb.edge)
	require.Equal(t, "board-base", lb.edge.view().Board)
	f.fail = "admin"

	// When
	require.NoError(t, lb.edge.do(context.Background(), "board-active"))
	waitIdle(t, lb.edge)

	// Then — 어느 판인지 모르는 상태가 되고, 실패가 기록에 남는다
	v := lb.edge.view()
	require.Empty(t, v.Board)
	require.NotEmpty(t, v.Events[len(v.Events)-1].Err)
}

func TestAdminHTTPErrorIsFailure(t *testing.T) {
	// Given — curl 이 exit 0 으로 끝나지만 관리 API 가 400 으로 답한 run.sh 출력
	lb := newE4Lab(t, &fakeRun{})
	lb.edge.run = func(context.Context, ...string) (string, error) {
		return "{\"error\":\"loading config: bad\"}\n[HTTP 400]\n", nil
	}

	// When
	require.NoError(t, lb.edge.do(context.Background(), "board-active"))
	waitIdle(t, lb.edge)

	// Then — 실패로 읽어 판을 올린 것으로 치지 않는다
	v := lb.edge.view()
	require.Empty(t, v.Board)
	require.Contains(t, v.Events[len(v.Events)-1].Err, "HTTP 400")
}

func TestAdminHTTP200IsSuccess(t *testing.T) {
	// Given — 관리 API 가 200 으로 답한 출력
	lb := newE4Lab(t, &fakeRun{})
	lb.edge.run = func(context.Context, ...string) (string, error) { return "[HTTP 200]\n", nil }

	// When
	require.NoError(t, lb.edge.do(context.Background(), "board-both"))
	waitIdle(t, lb.edge)

	// Then
	require.Equal(t, "board-both", lb.edge.view().Board)
}

func TestProxy1RestartRouteGuard(t *testing.T) {
	// Given — 재시작 함수를 세기만 하는 것으로 바꾼 E4 lab
	lb := newE4Lab(t, &fakeRun{})
	var calls atomic.Int32
	lb.edge.restartProxy1 = func(context.Context) error { calls.Add(1); return nil }
	mux := http.NewServeMux()
	lb.routes(mux)

	// When / Then — X-NPG 없는 POST 는 거절하고 재시작을 부르지 않는다. 있으면 한 번 부른다
	require.Equal(t, http.StatusForbidden, routePost(mux, "/api/edge/proxy1-restart", false))
	require.Zero(t, calls.Load())
	require.Equal(t, http.StatusNoContent, routePost(mux, "/api/edge/proxy1-restart", true))
	waitIdle(t, lb.edge)
	require.EqualValues(t, 1, calls.Load())
}

func TestRestartSendsSIGTERMWaitsAndStartsAgain(t *testing.T) {
	// Given — SIGTERM 을 받으면 끝나는 셸을 띄워 둔 lab. SIGINT 로는 끝나지 않게 막아 SIGTERM 으로 끝났음을 구분한다
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")
	p := &proc{name: "sh", log: &lineLog{n: 50}, warn: &lineLog{n: 50}, slow: &lineLog{n: 50},
		args: []string{"/bin/sh", "-c", `trap '' INT; trap 'echo got-term; exit 0' TERM; echo ready; while :; do sleep 0.05; done`}}
	require.NoError(t, lb.start(p))
	first := p.view().PID
	t.Cleanup(func() { // INT 를 막아 둔 셸이라 멈춤 버튼으로는 안 끝난다. 시험이 끝나면 직접 죽인다
		p.mu.Lock()
		defer p.mu.Unlock()
		_ = p.cmd.Process.Kill()
	})
	ready := func() bool { return slices.Contains(p.log.tail(20), "ready") } // 시작 줄에도 글자가 있어 줄 전체로 맞춘다
	require.Eventually(t, ready, 3*time.Second, 5*time.Millisecond)          // 셸이 trap 을 건 뒤에 신호를 보낸다

	// When
	err := lb.restart(context.Background(), p, 5*time.Second)

	// Then — 옛 프로세스는 SIGTERM 을 받고 끝났고, 다시 떠서 pid 가 바뀌었다
	require.NoError(t, err)
	v := p.view()
	require.True(t, v.Running)
	require.NotEqual(t, first, v.PID)
	require.Contains(t, strings.Join(p.log.tail(20), "\n"), "got-term")
	require.Contains(t, strings.Join(p.log.tail(20), "\n"), "끝: exit status 0")
	require.Less(t, strings.Index(strings.Join(p.log.tail(20), "\n"), "끝:"), strings.LastIndex(strings.Join(p.log.tail(20), "\n"), "시작:"))
}

func TestRestartRejectsStoppedProc(t *testing.T) {
	// Given — 떠 있지 않은 프로그램
	lb := newLab(context.Background(), t.TempDir(), "", "127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3", "")

	// When
	err := lb.restart(context.Background(), lb.procs["proxy"], time.Second)

	// Then
	require.Error(t, err)
}

func TestLoadRawFiveXXOwnLane(t *testing.T) {
	// Given — 502 와 503 이 섞인 1초 줄 ("502:5 503:2")
	_, st, ok := parseLoadLine("16:47:48 get inflight=10 n=40 200:30 502:5 503:2 404:1 p50=2ms p99=9ms max=10ms")
	require.True(t, ok)
	now := time.Unix(1000, 0)
	st.at = now
	lb := &lab{loads: map[string]loadStat{"get": st}}
	out := map[string]float64{}

	// When
	lb.loadRaw(now, out)

	// Then — 5xx 갈래는 502+503, 4xx 는 그 밖 갈래에 들어간다
	require.Equal(t, 7.0, out["load.all.err.5xx"])
	require.Equal(t, 1.0, out["load.all.err.etc"])
	require.Equal(t, 30.0, out["load.all.ok"])
}
