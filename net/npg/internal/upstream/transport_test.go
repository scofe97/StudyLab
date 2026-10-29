package upstream

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// countingServer 는 맺어진 TCP 연결 수와 동시에 처리 중인 요청 수를 세는 시험용 백엔드다.
// release 가 닫히기 전까지 모든 요청을 붙잡아, 요청이 동시에 겹치게 만든다.
type countingServer struct {
	*httptest.Server
	newConns atomic.Int64  // 서버가 받은 새 연결 수
	entered  chan struct{} // 요청이 처리 코드에 들어올 때마다 하나씩
	release  chan struct{} // 닫으면 붙잡힌 요청이 모두 답한다
	once     sync.Once
}

// unblock 은 붙잡힌 요청을 모두 풀어 준다. 여러 번 불러도 된다.
func (s *countingServer) unblock() { s.once.Do(func() { close(s.release) }) }

func newCountingServer(t *testing.T) *countingServer {
	t.Helper()
	s := &countingServer{entered: make(chan struct{}, 100), release: make(chan struct{})}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.entered <- struct{}{}
		<-s.release
		io.WriteString(w, "ok")
	}))
	s.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			s.newConns.Add(1)
		}
	}
	s.Start()
	t.Cleanup(s.Close)
	t.Cleanup(s.unblock) // 단언이 중간에 실패해도 Close 가 붙잡힌 요청을 기다리며 멈추지 않게 먼저 푼다 (Cleanup 은 나중 것부터)
	return s
}

// waitEntered 는 요청 n 개가 처리 코드에 들어올 때까지 기다린다.
func (s *countingServer) waitEntered(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-s.entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("요청 %d/%d 가 서버에 닿지 않았다", i, n)
		}
	}
}

// fire 는 요청 n 개를 동시에 보내고, 연결을 풀에서 꺼내 다시 쓴 요청 수를 돌려주는 함수를 준다.
// 돌려받은 함수는 모든 요청이 끝날 때까지 기다린다.
func fire(t *testing.T, tr *http.Transport, url string, n int) (wait func() int) {
	t.Helper()
	var wg sync.WaitGroup
	var reused atomic.Int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			trace := &httptrace.ClientTrace{GotConn: func(c httptrace.GotConnInfo) {
				if c.Reused {
					reused.Add(1)
				}
			}}
			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
			resp, err := tr.RoundTrip(req)
			if err != nil {
				t.Errorf("RoundTrip: %v", err)
				return
			}
			io.Copy(io.Discard, resp.Body) // 본문을 비우고 닫아야 연결이 풀로 돌아간다
			resp.Body.Close()
		}()
	}
	return func() int { wg.Wait(); return int(reused.Load()) }
}

func TestNewTransport(t *testing.T) {
	t.Run("상한을 넘는 요청은 연결을 더 열지 않고 기다린다 (I1)", func(t *testing.T) {
		// Given: 상한 2 인 Transport 와 요청을 붙잡는 백엔드
		srv := newCountingServer(t)
		tr := newTransport(2)
		t.Cleanup(tr.CloseIdleConnections)

		// When: 요청 3개를 동시에 보낸다
		wait := fire(t, tr, srv.URL, 3)
		srv.waitEntered(t, 2)
		time.Sleep(100 * time.Millisecond) // 셋째가 새 연결을 열 틈을 준다

		// Then: 연결은 2개뿐이고 셋째는 서버에 닿지 않았다
		require.Equal(t, int64(2), srv.newConns.Load())
		require.Len(t, srv.entered, 0)

		srv.unblock()
		srv.waitEntered(t, 1) // 앞의 연결이 비면 셋째가 그 연결로 들어온다
		wait()
		require.Equal(t, int64(2), srv.newConns.Load())
	})

	t.Run("끝난 연결은 상한만큼 모두 풀로 돌아가 다음 요청이 다시 쓴다 (I3)", func(t *testing.T) {
		// Given: 상한 4 인 Transport 로 요청 4개를 동시에 보내 연결 4개를 연다
		srv := newCountingServer(t)
		tr := newTransport(4)
		t.Cleanup(tr.CloseIdleConnections)
		first := fire(t, tr, srv.URL, 4)
		srv.waitEntered(t, 4)
		srv.unblock()
		first()

		// When: 다시 4개를 동시에 보낸다
		reused := fire(t, tr, srv.URL, 4)()

		// Then: 4개 모두 풀의 연결을 다시 썼고 새 연결은 처음 4개뿐이다 (기본값 2 였다면 2개가 닫혀 새로 열린다)
		require.Equal(t, 4, reused)
		require.Equal(t, int64(4), srv.newConns.Load())
	})

	t.Run("환경 변수 프록시를 보지 않는다", func(t *testing.T) {
		// Given
		tr := newTransport(1)

		// When / Then: Proxy 가 nil 이면 HTTP_PROXY 가 있어도 -upstream 주소로 바로 간다
		require.Nil(t, tr.Proxy)
	})

	t.Run("기한은 ADR 0012 의 순서를 지킨다", func(t *testing.T) {
		// Given
		tr := newTransport(1)

		// Then: 빈 연결 보관은 백엔드(fakeinv -idle-timeout 기본 5s)보다 짧고, 요청 기한은 손님(npgload -timeout 기본 30s)보다 짧다
		require.Less(t, tr.IdleConnTimeout, 5*time.Second)
		require.Less(t, RequestTimeout, 30*time.Second)
		require.Equal(t, 32, searchConns+restConns) // 백엔드 창구 32 를 나눈 것
	})
}
