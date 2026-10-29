package upstream

import (
	"errors"
	"io"
	"net/http"
	"sync"
)

// ErrOverflow 는 풀의 칸과 줄이 모두 차서 요청을 보내지 않았다는 뜻이다 (ADR 0012 결정 6, envoy UO).
var ErrOverflow = errors.New("upstream: pool and queue full")

// limit 은 한 풀에 동시에 들어가는 요청을 칸 + 줄 개수로 묶는다.
// 칸을 넘는 요청은 안쪽 Transport 가 기다리게 하므로, 이 층은 "기다리는 수"를 줄 길이 안으로 막기만 한다.
type limit struct {
	next  http.RoundTripper
	slots chan struct{} // 버퍼 크기 = 칸 + 줄. 하나 넣으면 입장, 하나 빼면 퇴장
}

// newLimit 은 next 앞에 칸 conns + 줄 pending 개의 입장권을 둔다.
func newLimit(next http.RoundTripper, conns, pending int) limit {
	return limit{next: next, slots: make(chan struct{}, conns+pending)}
}

// RoundTrip 은 입장권을 얻으면 next 에 넘기고, 못 얻으면 보내지 않고 ErrOverflow 를 낸다.
func (l limit) RoundTrip(r *http.Request) (*http.Response, error) {
	select {
	case l.slots <- struct{}{}: // 자리가 있으면 들어간다
	default: // 다 찼으면 기다리지 않고 바로 거절
		if r.Body != nil {
			r.Body.Close() // RoundTripper 는 에러를 낼 때도 요청 본문을 닫아야 한다
		}
		return nil, ErrOverflow
	}

	resp, err := l.next.RoundTrip(r)
	if err != nil {
		<-l.slots
		return nil, err
	}

	// 연결은 본문을 다 읽고 닫을 때까지 쓰인다. 그래서 칸도 그때 돌려준다.
	resp.Body = &releaseBody{ReadCloser: resp.Body, release: func() { <-l.slots }}
	return resp, nil
}

// releaseBody 는 응답 본문을 닫을 때 칸을 한 번만 돌려준다.
type releaseBody struct {
	io.ReadCloser // 끼워 넣기: Read·Close 를 그대로 물려받는다
	once          sync.Once
	release       func()
}

func (b *releaseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}
