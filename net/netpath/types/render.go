package types

import (
	"fmt"
	"strings"
	"time"
)

const (
	stageLine  = "%-4s %9s  %s\n"
	descColumn = 16 // stageLine 에서 설명이 시작하는 칸
)

// Result 만 읽으므로 표현을 types/ 에 둔다 — 의존 방향이 안 뒤집혀 2국면 exporter 도 빌려 쓸 수 있다.
func (r *Result) Waterfall() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", r.Target)
	for _, s := range r.Stages {
		fmt.Fprintf(&b, stageLine, strings.ToUpper(string(s.Stage)), durationCell(s), describe(s, r))
	}
	// Total 을 시간 열이 아닌 설명 칸에서 시작한다 — 같은 열이면 합계처럼 보인다.
	fmt.Fprintf(&b, "%s%s\n", strings.Repeat(" ", descColumn), "Total "+formatMillis(r.Total))

	b.WriteString(unmeasuredNote(r))
	if r.FailureStage != "" {
		fmt.Fprintf(&b, "\n실패 단계: %s (%s)\n", r.FailureStage, r.ErrorKind)
	}
	return b.String()
}

// 1국면은 TLS 를 안 재 https 는 handshake 시간이 어디에도 안 들어간다.
// 실패한 프로브엔 안 낸다 — 끝나지 않은 단계는 duration 0 이라 gap 이 부풀려지는데, TLS 가 아니라 기다린 시간이다.
func unmeasuredNote(r *Result) string {
	if r.FailureStage != "" || !strings.HasPrefix(r.Target, "https://") {
		return ""
	}
	var measured time.Duration
	for _, s := range r.Stages {
		measured += s.Duration
	}
	gap := r.Total - measured
	if gap <= 0 {
		return ""
	}
	return fmt.Sprintf("\n주의: TLS 단계는 아직 재지 않는다 (1.5국면). %s 가 어느 단계에도 잡히지 않았다.\n",
		formatMillis(gap))
}

// 0 은 "잰 적 없음" — skipped 와 hung(시작만 함) 둘 다 해당. hung 은 status 가 failed 라
// status 로만 걸면 3초 기다린 타임아웃이 "0.0ms 실패" 로 찍힌다.
func durationCell(s StageResult) string {
	if s.Duration == 0 {
		return "-"
	}
	return formatMillis(s.Duration)
}

// Duration.String() 은 단위가 바뀌어(120µs, 3ms, 1.5s) 열이 어긋나므로 ms 로 고정한다.
func formatMillis(d time.Duration) string {
	ms := float64(d) / float64(time.Millisecond)
	if ms >= 1 {
		return fmt.Sprintf("%.0fms", ms)
	}
	return fmt.Sprintf("%.1fms", ms)
}

func describe(s StageResult, r *Result) string {
	switch {
	case s.Status == StatusSkipped:
		return "실행되지 않음"
	case s.Status == StatusFailed && s.Error == "":
		return fmt.Sprintf("실패: %s", s.ErrorKind)
	case s.Status == StatusFailed:
		return fmt.Sprintf("실패: %s (%s)", s.ErrorKind, s.Error)
	}

	switch {
	case s.Stage == StageDNS && r.DNS != nil && len(r.DNS.Addresses) > 0:
		return "성공: " + strings.Join(r.DNS.Addresses, ", ")
	case s.Stage == StageTCP && r.TCP != nil && r.TCP.RemoteAddr != "":
		if r.TCP.Attempts > 1 {
			// 성공했지만 정상은 아니다 — 앞 주소들이 죽어 있었다.
			return fmt.Sprintf("성공: %s (시도 %d회)", r.TCP.RemoteAddr, r.TCP.Attempts)
		}
		return "성공: " + r.TCP.RemoteAddr
	case s.Stage == StageHTTP && r.HTTP != nil:
		return fmt.Sprintf("성공 (%d, %s)", r.HTTP.StatusCode, r.HTTP.Proto)
	}
	return "성공"
}
