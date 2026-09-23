package types

import (
	"strings"
	"testing"
	"time"
)

func TestWaterfallNeverPrintsZeroForSkipped(t *testing.T) {
	// Given — TCP 에서 거부되어 HTTP 가 실행되지 않은 결과
	r := &Result{
		Target: "http://127.0.0.1:9/",
		Stages: []StageResult{
			{Stage: StageDNS, Status: StatusSkipped},
			{Stage: StageTCP, Status: StatusFailed, Duration: 120 * time.Microsecond,
				ErrorKind: ErrTCPRefused, Error: "dial tcp 127.0.0.1:9: connect: connection refused"},
			{Stage: StageHTTP, Status: StatusSkipped},
		},
		TCP:          &TCPInfo{RemoteAddr: "127.0.0.1:9", Attempts: 1},
		Total:        200 * time.Microsecond,
		FailureStage: StageTCP,
		ErrorKind:    ErrTCPRefused,
	}

	// When
	got := r.Waterfall()
	t.Logf("\n%s", got)

	// Then — skipped 줄에 숫자가 없어야 한다. 0ms 를 찍으면 "0ms 에 성공" 으로 읽힌다
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "실행되지 않음") && strings.Contains(line, "ms") {
			t.Errorf("skipped 줄에 시간이 찍혔다: %q", line)
		}
	}

	// Then — 1ms 미만은 소수 한 자리로
	if !strings.Contains(got, "0.1ms") {
		t.Errorf("120µs 가 0.1ms 로 안 나온다:\n%s", got)
	}

	// Then — 멈춘 지점을 마지막에 한 번 더 말한다
	if !strings.Contains(got, "실패 단계: tcp (tcp_refused)") {
		t.Errorf("실패 요약 줄이 없다:\n%s", got)
	}
}

func TestWaterfallOnSuccess(t *testing.T) {
	// Given — 세 단계가 다 성공했고 TCP 는 두 번째 주소에서 붙은 결과
	r := &Result{
		Target: "http://example.com/",
		Stages: []StageResult{
			{Stage: StageDNS, Status: StatusOK, Duration: 3 * time.Millisecond},
			{Stage: StageTCP, Status: StatusOK, Duration: 12 * time.Millisecond},
			{Stage: StageHTTP, Status: StatusOK, Duration: 1500 * time.Millisecond},
		},
		DNS:   &DNSInfo{Addresses: []string{"93.184.216.34", "93.184.216.35"}},
		TCP:   &TCPInfo{RemoteAddr: "93.184.216.35:80", Attempts: 2},
		HTTP:  &HTTPInfo{StatusCode: 200, Proto: "HTTP/1.1"},
		Total: 1515 * time.Millisecond,
	}

	// When
	got := r.Waterfall()
	t.Logf("\n%s", got)

	// Then — 성공해도 무엇을 얻었는지 말한다
	if !strings.Contains(got, "93.184.216.34, 93.184.216.35") {
		t.Errorf("DNS 가 찾은 주소가 안 보인다:\n%s", got)
	}
	if !strings.Contains(got, "성공 (200, HTTP/1.1)") {
		t.Errorf("HTTP 상태가 안 보인다:\n%s", got)
	}

	// Then — 시도가 여러 번이면 드러난다. 성공했지만 정상은 아니다
	if !strings.Contains(got, "시도 2회") {
		t.Errorf("attempts 가 안 보인다:\n%s", got)
	}

	// Then — 단위를 ms 로 고정해 크기를 자릿수로 비교하게 한다
	if !strings.Contains(got, "1500ms") {
		t.Errorf("1.5s 가 1500ms 로 안 나온다:\n%s", got)
	}

	// Then — 성공했으면 실패 요약 줄이 없다
	if strings.Contains(got, "실패 단계") {
		t.Errorf("성공인데 실패 줄이 있다:\n%s", got)
	}
}

// dial 타임아웃은 시작만 하고 끝나지 않아 duration 이 0 이다. status 는 failed 다.
// 조건을 status 로만 걸면 "0.0ms 실패" 로 찍혀서, 3초를 기다린 사실이 사라진다.
func TestWaterfallHungStageHasNoNumber(t *testing.T) {
	// Given — TCP 가 시작만 하고 끝나지 않은 결과
	r := &Result{
		Target: "http://10.255.255.1:8080",
		Stages: []StageResult{
			{Stage: StageDNS, Status: StatusSkipped},
			{Stage: StageTCP, Status: StatusFailed, Duration: 0,
				ErrorKind: ErrTCPTimeout, Error: "context deadline exceeded"},
			{Stage: StageHTTP, Status: StatusSkipped},
		},
		Total:        3 * time.Second,
		FailureStage: StageTCP,
		ErrorKind:    ErrTCPTimeout,
	}

	// When
	got := r.Waterfall()
	t.Logf("\n%s", got)

	// Then — 숫자가 아니라 '-' 여야 한다
	if strings.Contains(got, "0.0ms") {
		t.Errorf("끝나지 않은 단계에 0.0ms 가 찍혔다:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "TCP") && !strings.Contains(line, "-") {
			t.Errorf("TCP 줄에 '-' 가 없다: %q", line)
		}
	}
}

// 1국면은 TLS 를 재지 않으므로 https 대상에서는 handshake 시간이 어느 단계에도 안 들어간다.
// 화면이 침묵하면 설명되지 않는 시간을 감추게 된다.
func TestWaterfallTellsAboutUnmeasuredTLSTime(t *testing.T) {
	// Given — 단계 합이 79ms 인데 전체가 143ms 인 https 결과
	https := &Result{
		Target: "https://example.com",
		Stages: []StageResult{
			{Stage: StageDNS, Status: StatusOK, Duration: 5 * time.Millisecond},
			{Stage: StageTCP, Status: StatusOK, Duration: 33 * time.Millisecond},
			{Stage: StageHTTP, Status: StatusOK, Duration: 41 * time.Millisecond},
		},
		Total: 143 * time.Millisecond,
	}

	// When
	got := https.Waterfall()
	t.Logf("\n%s", got)

	// Then — 잡히지 않은 64ms 를 말한다
	if !strings.Contains(got, "64ms") {
		t.Errorf("측정되지 않은 시간을 안 알린다:\n%s", got)
	}

	// Given · When · Then — 평문은 그 공백이 없으므로 조용하다
	plain := &Result{
		Target: "http://example.com",
		Stages: []StageResult{{Stage: StageTCP, Status: StatusOK, Duration: 33 * time.Millisecond}},
		Total:  33 * time.Millisecond,
	}
	if strings.Contains(plain.Waterfall(), "주의") {
		t.Errorf("평문인데 주의가 붙었다:\n%s", plain.Waterfall())
	}

	// Given · When · Then — 실패한 프로브에는 내지 않는다.
	// 끝나지 않은 단계가 duration 0 이라 gap 이 부풀려지는데, 그건 TLS 시간이 아니다
	failed := &Result{
		Target:       "https://10.255.255.1",
		Stages:       []StageResult{{Stage: StageTCP, Status: StatusFailed, ErrorKind: ErrTCPTimeout}},
		Total:        3 * time.Second,
		FailureStage: StageTCP,
		ErrorKind:    ErrTCPTimeout,
	}
	if strings.Contains(failed.Waterfall(), "주의") {
		t.Errorf("실패한 프로브에 주의가 붙었다:\n%s", failed.Waterfall())
	}
}
