package upstream

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stub 은 받은 요청 수를 세고, err 가 있으면 에러를, 없으면 빈 200 을 돌려주는 가짜 풀이다.
type stub struct {
	got int
	err error
}

func (s *stub) RoundTrip(*http.Request) (*http.Response, error) {
	s.got++
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
}

// closeSpy 는 Close 가 불렸는지 기록하는 요청 본문이다.
type closeSpy struct {
	io.Reader
	closed bool
}

func (c *closeSpy) Close() error { c.closed = true; return nil }

// get 은 rt 로 GET 하나를 보낸다. url 을 안 주면 가짜 풀용 주소를 쓴다.
func get(t *testing.T, rt http.RoundTripper, url ...string) (*http.Response, error) {
	t.Helper()
	target := "http://backend/"
	if len(url) > 0 {
		target = url[0]
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil { // 고루틴에서도 불리므로 require(FailNow) 대신 Errorf
		t.Errorf("요청 만들기: %v", err)
		return nil, err
	}
	return rt.RoundTrip(req)
}

func TestLimit(t *testing.T) {
	t.Run("칸 + 줄을 넘는 요청은 보내지 않고 ErrOverflow, 요청 본문은 닫는다", func(t *testing.T) {
		// Given: 칸 1 + 줄 1 = 입장권 2 장을 이미 다 쓴 상태
		next := &stub{}
		l := newLimit(next, 1, 1)
		_, err := get(t, l)
		require.NoError(t, err)
		_, err = get(t, l)
		require.NoError(t, err)
		body := &closeSpy{Reader: strings.NewReader("{}")}

		// When: 셋째가 본문을 들고 온다
		req := httptest.NewRequest(http.MethodPost, "http://backend/", body)
		resp, err := l.RoundTrip(req)

		// Then
		require.ErrorIs(t, err, ErrOverflow)
		require.Nil(t, resp)
		require.Equal(t, 2, next.got) // 셋째는 안쪽 풀에 닿지 않았다
		require.True(t, body.closed)
	})

	t.Run("응답 본문을 닫으면 칸이 돌아오고, 두 번 닫아도 한 번만 돌려준다", func(t *testing.T) {
		// Given: 입장권 1 장을 쓴 응답
		next := &stub{}
		l := newLimit(next, 1, 0)
		resp, err := get(t, l)
		require.NoError(t, err)
		_, err = get(t, l)
		require.ErrorIs(t, err, ErrOverflow) // 아직 닫지 않아 자리가 없다

		// When
		require.NoError(t, resp.Body.Close())
		require.NoError(t, resp.Body.Close())

		// Then: 한 장만 돌아와 하나는 들어가고 그다음은 다시 넘친다
		require.Len(t, l.slots, 0)
		_, err = get(t, l)
		require.NoError(t, err)
		_, err = get(t, l)
		require.ErrorIs(t, err, ErrOverflow)
	})

	t.Run("안쪽 풀이 에러를 내면 칸을 바로 돌려준다", func(t *testing.T) {
		// Given
		boom := errors.New("dial: refused")
		l := newLimit(&stub{err: boom}, 1, 0)

		// When
		_, err := get(t, l)

		// Then: 에러는 그대로 올라가고 자리는 비어 있다
		require.ErrorIs(t, err, boom)
		require.Len(t, l.slots, 0)
	})

	t.Run("실제 Transport 앞에서: 칸 1·줄 1 이면 하나는 백엔드, 하나는 줄, 셋째는 바로 넘친다", func(t *testing.T) {
		// Given: 요청을 붙잡는 백엔드 앞에 칸 1 Transport + 줄 1
		srv := newCountingServer(t)
		tr := newTransport(1)
		t.Cleanup(tr.CloseIdleConnections)
		l := newLimit(tr, 1, 1)
		done := make(chan error, 2)
		for range 2 {
			go func() {
				resp, err := get(t, l, srv.URL)
				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				done <- err
			}()
		}
		srv.waitEntered(t, 1)
		require.Eventually(t, func() bool { return len(l.slots) == 2 }, time.Second, 5*time.Millisecond)

		// When
		start := time.Now()
		_, err := get(t, l, srv.URL)

		// Then: 기다리지 않고 거절된다. 줄에 선 둘째는 칸이 비면 지나간다
		require.ErrorIs(t, err, ErrOverflow)
		require.Less(t, time.Since(start), 100*time.Millisecond)
		require.Equal(t, int64(1), srv.newConns.Load())
		srv.unblock()
		require.NoError(t, <-done)
		require.NoError(t, <-done)
		require.Len(t, l.slots, 0)
	})
}
