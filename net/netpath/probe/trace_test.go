package probe

import (
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"
)

// 0ms 에 끝난 단계와 시작만 하고 안 끝난 단계는 duration 이 둘 다 0 이다.
// 그 둘을 가르는 것이 stageSpan 을 시각 두 개로 둔 이유다.
func TestStageSpanDistinguishesHungFromInstant(t *testing.T) {
	// Given — 시작만 하고 끝나지 않은 단계, 그리고 같은 시각에 끝난 단계
	now := time.Now()
	hung := stageSpan{start: now}
	instant := stageSpan{start: now, end: now}

	// Then — duration 은 둘 다 0 이다
	if hung.duration() != 0 || instant.duration() != 0 {
		t.Fatalf("duration: hung=%v instant=%v, 둘 다 0 이어야 한다", hung.duration(), instant.duration())
	}

	// Then — 그래도 상태는 갈린다
	if !hung.hung() {
		t.Error("시작만 한 단계가 hung 이 아니라고 나온다. 에러를 귀속시킬 곳을 못 찾는다")
	}
	if instant.hung() {
		t.Error("0ms 에 끝난 단계를 hung 으로 본다. 성공을 실패로 보고한다")
	}

	// Given · Then — 시작도 안 한 단계는 hung 이 아니다 (그냥 skipped 다)
	if (stageSpan{}).hung() {
		t.Error("시작조차 안 한 단계를 hung 으로 본다")
	}
}

func TestTracerCollectsStagesOnRealRequest(t *testing.T) {
	// Given — 평문 HTTP 서버. URL 이 127.0.0.1 이라 DNS 도 거치지 않는다
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := &tracer{}
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("요청 생성: %v", err)
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), tr.clientTrace()))

	// When
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("요청: %v", err)
	}
	defer resp.Body.Close()

	got := tr.snapshot()

	// Then — TCP 는 시작과 끝이 다 잡혔다
	if !got.conn.started() || !got.conn.finished() {
		t.Errorf("TCP span 이 비었다: %+v", got.conn)
	}

	// Then — IP 로 직접 접속했으니 DNS 훅은 아예 불리지 않았다
	if got.dns.started() {
		t.Error("DNS 가 측정됐다. IP 직접 접속이므로 훅이 불리지 않아야 한다")
	}

	// Then — 연결의 신원이 채워졌다
	if got.remoteAddr == "" || got.localAddr == "" {
		t.Errorf("주소가 비었다: remote=%q local=%q", got.remoteAddr, got.localAddr)
	}
	if got.reused {
		t.Error("첫 요청인데 reused 가 true 다")
	}
	if !got.http.started() || !got.http.finished() {
		t.Errorf("HTTP span 이 비었다: %+v", got.http)
	}
	if got.connAttempts != 1 {
		t.Errorf("connAttempts=%d, 단일 주소이므로 1 이어야 한다", got.connAttempts)
	}

	// Then — HTTP 는 연결이 끝난 뒤에 시작한다
	if got.http.start.Before(got.conn.end) {
		t.Error("HTTP 시작이 연결 완료보다 앞이다. 단계 경계가 뒤집혔다")
	}
}
