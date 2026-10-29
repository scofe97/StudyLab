package proxy

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// statusRecorder 는 손님에게 쓰는 ResponseWriter 를 감싸 상태 코드와 보낸 바이트 수를 엿본다.
// [4a] 응답 복사와 [4b] ErrorHandler 모두 이것을 거쳐 쓰므로 [5] 에서 둘 다 보인다.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

// WriteHeader 는 상태 코드를 적어 두고 그대로 넘긴다.
func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Write 는 본문을 넘기며 바이트 수를 센다. WriteHeader 없이 Write 하면 Go 가 200 을 보내므로 200 으로 적는다.
func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// Unwrap 은 안쪽 writer 를 내준다. ReverseProxy 는 ResponseController 로 Flush 하는데,
// 이게 없으면 감싼 writer 에서 Flush 를 못 찾아 스트리밍 응답이 버퍼에 묶인다.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// accessLog 는 요청마다 기록장을 달고, 끝나면 접근 로그 한 줄을 쓴다.
func (m *metrics) accessLog(log zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// [1] 들어올 때: 시간 재기 시작, 카운터, 빈 기록장
		start := time.Now()
		m.requests.Add(1)
		m.inflight.Add(1)
		defer m.inflight.Add(-1)

		ctx, info := withInfo(r.Context())
		rec := &statusRecorder{ResponseWriter: w}

		// [5] 나갈 때. defer 라서 본문 복사 중 panic 으로 빠져나가도 실행된다.
		defer func() {
			p := recover() // 진행 중인 panic 을 붙잡아 그 값을 돌려준다. panic 이 없으면 nil

			if p != nil && info.reason == ReasonNone {
				// 본문 도중 끊김: 손님이 떠났으면 DC, 아니면 업스트림이 끊은 UC
				info.reason = classify(ctx, http.ErrAbortHandler)
			}

			if info.reason != ReasonNone {
				m.reasons.Add(string(info.reason), 1)
			}

			if info.attempts > 1 {
				m.retries.Add(int64(info.attempts - 1)) // 첫 시도를 뺀 나머지가 다시 보낸 횟수
			}

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.RequestURI()).
				Int("status", rec.status).
				Str("flags", string(info.reason)).
				Dur("dur", time.Since(start)).
				Bool("reused", info.reused).
				Dur("idle", info.idle).
				Int("attempts", info.attempts).
				Int64("bytes", rec.bytes).
				Str("client", r.RemoteAddr).
				Send()

			if p != nil {
				panic(p) // 서버가 손님 연결을 끊도록 그대로 다시 던진다
			}
		}()

		next.ServeHTTP(rec, r.WithContext(ctx)) // [2]~[4] 가 이 안에서 일어난다
	})
}
