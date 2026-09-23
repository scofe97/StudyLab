package probe

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/user/netpath-lab/types"
)

func stageOf(r *types.Result, s types.Stage) types.StageResult {
	for _, sr := range r.Stages {
		if sr.Stage == s {
			return sr
		}
	}
	return types.StageResult{}
}

func TestDoOnHealthyTarget(t *testing.T) {
	// Given — 200 을 돌려주는 평문 서버. URL 이 127.0.0.1 이라 DNS 는 거치지 않는다
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// When
	got, err := Do(t.Context(), srv.URL, Config{Timeout: 5 * time.Second})

	// Then — 측정에 성공했으므로 error 는 없다
	if err != nil {
		t.Fatalf("error = %v, 재는 데 성공했으면 nil 이어야 한다", err)
	}
	if len(got.Stages) != 3 {
		t.Fatalf("단계 %d개, 3개여야 한다: %+v", len(got.Stages), got.Stages)
	}

	// Then — DNS 는 해당 없음, TCP·HTTP 는 성공
	if s := stageOf(got, types.StageDNS); s.Status != types.StatusSkipped {
		t.Errorf("DNS status=%q, IP 직접 접속이므로 skipped 여야 한다", s.Status)
	}
	for _, st := range []types.Stage{types.StageTCP, types.StageHTTP} {
		s := stageOf(got, st)
		if s.Status != types.StatusOK {
			t.Errorf("%s status=%q error=%q, ok 여야 한다", st, s.Status, s.Error)
		}
		if s.Duration <= 0 {
			t.Errorf("%s duration=%v, 성공한 단계는 시간이 있어야 한다", st, s.Duration)
		}
	}

	// Then — 성공했으므로 실패 필드는 비어 있다
	if got.FailureStage != "" || got.ErrorKind != "" {
		t.Errorf("실패 필드가 채워졌다: stage=%q kind=%q", got.FailureStage, got.ErrorKind)
	}
	if got.TCP == nil || got.TCP.Attempts != 1 || got.TCP.Reused {
		t.Errorf("TCP 상세가 이상하다: %+v", got.TCP)
	}
	if got.HTTP == nil || got.HTTP.StatusCode != 200 {
		t.Errorf("HTTP 상세가 이상하다: %+v", got.HTTP)
	}
}

func TestDoOnRefusedPortSkipsRest(t *testing.T) {
	// Given · When — 아무도 듣지 않는 포트
	got, err := Do(t.Context(), "http://127.0.0.1:9", Config{Timeout: 5 * time.Second})

	// Then — 거부는 측정 결과다. error 가 아니다
	if err != nil {
		t.Fatalf("error = %v, tcp_refused 는 Result 에 담겨야 한다", err)
	}
	if got.ErrorKind != types.ErrTCPRefused || got.FailureStage != types.StageTCP {
		t.Errorf("kind=%q stage=%q, tcp_refused/tcp 여야 한다", got.ErrorKind, got.FailureStage)
	}

	// Then — 실패 이후 단계는 실행되지 않았다. 0ms 성공이 아니다
	if s := stageOf(got, types.StageHTTP); s.Status != types.StatusSkipped {
		t.Errorf("HTTP status=%q, TCP 가 실패했으므로 skipped 여야 한다", s.Status)
	}

	// Then — 원문 메시지는 사람용으로 남는다
	if s := stageOf(got, types.StageTCP); s.Error == "" {
		t.Error("TCP 단계에 원문 메시지가 없다")
	}
}

func TestDoOnServerErrorFailsOnlyHTTP(t *testing.T) {
	// Given — 500 을 돌려주는 서버. 경로 자체는 멀쩡하다
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// When
	got, err := Do(t.Context(), srv.URL, Config{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	// Then — 실패 단계가 http 라는 사실이 "앞 단계는 다 성공" 을 함께 말한다
	if got.FailureStage != types.StageHTTP || got.ErrorKind != types.ErrHTTPStatus {
		t.Errorf("stage=%q kind=%q, http/http_status 여야 한다", got.FailureStage, got.ErrorKind)
	}
	if s := stageOf(got, types.StageTCP); s.Status != types.StatusOK {
		t.Errorf("TCP status=%q, 경로는 살아 있으므로 ok 여야 한다", s.Status)
	}
	if got.HTTP == nil || got.HTTP.StatusCode != 500 {
		t.Errorf("상태 코드가 기록되지 않았다: %+v", got.HTTP)
	}
}

// Done Line 의 네 번째 줄. 이 프로젝트에서 가장 값진 한 줄이다.
// dial 이 타임아웃되면 ConnectDone 훅이 발화하지 않아 에러가 HTTP 단계로 흘러간다.
// 그대로 두면 "애플리케이션이 느리다" 고 보고하는데 실제로는 패킷이 도착조차 못 한 것이다.
func TestDoDialTimeoutBlamesTCPNotHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("응답 없는 주소의 타임아웃을 기다린다")
	}

	// Given · When — 아무 답이 없는 주소에 짧은 예산으로
	got, err := Do(t.Context(), "http://10.255.255.1:8080", Config{Timeout: 700 * time.Millisecond})
	if err != nil {
		t.Fatalf("error = %v, 타임아웃도 측정 결과다", err)
	}

	// Then — TCP 가 주인이다. http_timeout 이 아니다
	if got.ErrorKind != types.ErrTCPTimeout {
		t.Errorf("kind=%q, tcp_timeout 이어야 한다 (http_timeout 이면 hung 판정이 깨진 것)", got.ErrorKind)
	}
	if got.FailureStage != types.StageTCP {
		t.Errorf("stage=%q, tcp 여야 한다", got.FailureStage)
	}

	// Then — 끝나지 않은 단계라 시간이 없다. status 가 그 사실을 말한다
	if s := stageOf(got, types.StageTCP); s.Duration != 0 {
		t.Errorf("TCP duration=%v, 끝나지 않았으므로 0 이어야 한다", s.Duration)
	}
	if s := stageOf(got, types.StageHTTP); s.Status != types.StatusSkipped {
		t.Errorf("HTTP status=%q, skipped 여야 한다", s.Status)
	}
}

func TestDoRejectsUnprobeableTargets(t *testing.T) {
	tests := []struct{ name, target string }{
		{"scheme 이 없다", "://foo"},
		{"지원하지 않는 scheme", "ftp://example.com"},
		{"호스트가 없다", "http://"},
		{"괄호가 안 닫혔다", "http://[::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When — 재기 시작조차 할 수 없는 대상
			got, err := Do(t.Context(), tc.target, Config{Timeout: time.Second})

			// Then — 이때만 error 다. Result 는 없다
			if err == nil {
				t.Fatalf("error 가 nil 이다: %+v", got)
			}
			if got != nil {
				t.Errorf("Result 가 non-nil 이다: %+v", got)
			}
		})
	}
}
