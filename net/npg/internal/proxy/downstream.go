package proxy

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"
)

// 손님 쪽 기한 (ADR 0014 결정 1·2). 전체 시계 대신 틈 시계를 쓴다.
// 틈 시계는 읽기·쓰기가 한 번 될 때마다 다시 잰다(nginx client_body_timeout·send_timeout, caddy read_idle_timeout·write_idle_timeout).
// 그래서 업스트림을 기다리는 동안은 쓰기 시계가 돌지 않고(J2), 꾸준히 오는 큰 업로드는 잘리지 않는다.
const (
	ClientReadGap  = 10 * time.Second // 몸통 읽기와 읽기 사이 최대 간격
	ClientWriteGap = 10 * time.Second // 응답 쓰기와 쓰기 사이 최대 간격
	ClientMinRate  = 1024             // 몸통 평균 속도 하한(초당 바이트). ClientReadGap 이 지난 뒤부터 잰다 (caddy read_min_rate)
)

// clientLimits 는 손님 쪽 기한 묶음이다. 시험이 짧은 값을 넣을 수 있게 값으로 넘긴다.
type clientLimits struct {
	readGap  time.Duration
	writeGap time.Duration
	minRate  int64
}

var defaultClientLimits = clientLimits{readGap: ClientReadGap, writeGap: ClientWriteGap, minRate: ClientMinRate}

// errTooSlow 는 몸통이 최소 속도보다 느리게 올 때 읽기가 돌려주는 오류다.
// 2초에 1바이트씩 흘리는 손님(slow POST)은 틈이 짧아 틈 시계만으로는 못 잡는다 (J1).
var errTooSlow = errors.New("client body slower than minimum rate")

// clientDeadlines 는 손님 몸통과 응답 쓰기에 틈 시계를 건다. accessLog 안쪽에 두어 기록장(reqInfo)을 쓴다.
func clientDeadlines(lim clientLimits, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctl := http.NewResponseController(w)

		// keep-alive 연결은 앞 요청이 건 쓰기 기한을 그대로 들고 온다. 지난 기한이 남아 있으면
		// 이번 응답(예: 15초 뒤의 504)을 못 쓴다. 그래서 요청마다 먼저 지운다.
		_ = ctl.SetWriteDeadline(time.Time{})

		if r.Body != nil && r.Body != http.NoBody {
			b := &clientBody{rc: r.Body, ctl: ctl, lim: lim, start: time.Now()}
			if info := infoFrom(r.Context()); info != nil {
				info.body = b // [4b] classify 가 기한 초과를 손님 이탈(DC)과 가려낸다
			}
			r.Body = b
		}
		next.ServeHTTP(&gapWriter{ResponseWriter: w, ctl: ctl, gap: lim.writeGap}, r)
	})
}

// clientBody 는 손님 몸통을 감싼 마디다. 모든 읽기가 여기를 지나므로 틈 시계를 다시 걸고, 처음 난 오류를 적어 둔다.
type clientBody struct {
	rc    io.ReadCloser
	ctl   *http.ResponseController
	lim   clientLimits
	start time.Time
	n     int64 // 지금까지 읽은 바이트
	err   error // 처음 난 읽기 오류 (EOF 제외)
}

func (b *clientBody) Read(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	_ = b.ctl.SetReadDeadline(time.Now().Add(b.lim.readGap))
	n, err := b.rc.Read(p)
	b.n += int64(n)

	if err == nil && b.lim.minRate > 0 {
		if el := time.Since(b.start); el >= b.lim.readGap && float64(b.n)/el.Seconds() < float64(b.lim.minRate) {
			err = errTooSlow
			_ = b.ctl.SetReadDeadline(time.Now()) // 서버가 남은 몸통을 버리며 읽을 때 기다리지 않게 바로 끝낸다
		}
	}

	switch {
	case errors.Is(err, io.EOF):
		// 몸통을 다 읽었다. 기한을 지운다. 남겨 두면 Go 서버가 손님 이탈을 살피려고 거는 뒤쪽 읽기가
		// 이 기한에 걸려 연결 context 를 취소하고, 업스트림을 오래 기다리는 POST 가 DC 로 끝난다.
		_ = b.ctl.SetReadDeadline(time.Time{})
	case err != nil:
		b.err = err
	}
	return n, err
}

func (b *clientBody) Close() error { return b.rc.Close() }

// timedOut 은 손님이 느려서(틈 시계나 최소 속도에 걸려) 끊겼는지 알려 준다.
// 기한에 걸려도 net/http 는 연결 context 를 그냥 취소하므로 context 만 보면 손님 이탈과 같아 보인다 (J1).
func (b *clientBody) timedOut() bool {
	return errors.Is(b.err, os.ErrDeadlineExceeded) || errors.Is(b.err, errTooSlow)
}

// gapWriter 는 응답 쓰기마다 쓰기 틈 시계를 다시 건다. 상태 줄만 쓰는 응답(ErrorHandler 의 504)도
// 실제 전송은 핸들러가 끝날 때 일어나므로 WriteHeader 에서도 건다.
type gapWriter struct {
	http.ResponseWriter
	ctl *http.ResponseController
	gap time.Duration
}

func (g *gapWriter) arm() { _ = g.ctl.SetWriteDeadline(time.Now().Add(g.gap)) }

func (g *gapWriter) WriteHeader(code int) {
	g.arm()
	g.ResponseWriter.WriteHeader(code)
}

func (g *gapWriter) Write(p []byte) (int, error) {
	g.arm()
	return g.ResponseWriter.Write(p)
}

// FlushError 는 ReverseProxy 가 스트리밍 응답을 밀어낼 때 ResponseController 로 부른다.
func (g *gapWriter) FlushError() error {
	g.arm()
	return g.ctl.Flush()
}

// Unwrap 은 안쪽 writer 를 내준다. ResponseController 가 SetReadDeadline 같은 메서드를 찾을 때 따라간다.
func (g *gapWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }
