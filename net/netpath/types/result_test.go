package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// docs/result-schema.md 가 2국면 exporter 와 맺은 약속을 마샬링 결과로 고정한다.
// 문서만으로는 필드 이름과 형태가 바뀌는 것을 막을 수 없다.
func TestResultJSONContract(t *testing.T) {
	// Given — IP 를 직접 지정해 DNS 를 거치지 않고, TCP 에서 거부된 프로브
	r := Result{
		Target:    "http://127.0.0.1:9/",
		StartedAt: time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC),
		Stages: []StageResult{
			{Stage: StageDNS, Status: StatusSkipped},
			{
				Stage:     StageTCP,
				Status:    StatusFailed,
				Duration:  120 * time.Microsecond,
				ErrorKind: ErrTCPRefused,
				Error:     "connect: connection refused",
			},
			{Stage: StageHTTP, Status: StatusSkipped},
		},
		TCP:          &TCPInfo{RemoteAddr: "127.0.0.1:9", LocalAddr: "127.0.0.1:52814", Attempts: 1},
		Total:        200 * time.Microsecond,
		FailureStage: StageTCP,
		ErrorKind:    ErrTCPRefused,
	}

	// When — JSON 으로 내보낸다
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	// Then — 스키마가 약속한 형태여야 한다
	for _, want := range []string{
		`"started_at":"2026-09-13T10:00:00Z"`, // RFC3339
		`"duration_ns":120000`,                // time.Duration 이 나노초 정수로 나간다
		`"status":"skipped"`,                  // 상태는 읽을 수 있는 문자열
		`"error_kind":"tcp_refused"`,
		`"failure_stage":"tcp"`,
		`"attempts":1`, // 성공/실패와 별개로 시도 횟수를 남긴다
		`"tcp":{`,      // 정보를 남긴 단계의 상세는 나온다
	} {
		if !strings.Contains(got, want) {
			t.Errorf("빠졌다: %s\n실제: %s", want, got)
		}
	}

	// Then — 거치지 않은 단계의 상세는 키째로 없어야 한다.
	// `omitempty` 는 struct 를 비었다고 보지 않으므로 이 약속은 포인터가 지킨다.
	for _, absent := range []string{`"dns":`, `"http":`} {
		if strings.Contains(got, absent) {
			t.Errorf("있어선 안 된다: %s\n실제: %s", absent, got)
		}
	}
}
