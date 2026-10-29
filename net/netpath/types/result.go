package types

import "time"

// 계층이 아니라 시간축이다 — 한 요청 안에서는 DNS 가 먼저 온다. TLS 는 1.5국면에서 추가.
type Stage string

const (
	StageDNS  Stage = "dns"
	StageTCP  Stage = "tcp"
	StageHTTP Stage = "http"
)

type StageStatus string

const (
	StatusOK     StageStatus = "ok"
	StatusFailed StageStatus = "failed"
	// 실행 안 됨 — "0ms 만에 성공" 과 다른 사실이라 따로 둔다.
	StatusSkipped StageStatus = "skipped"
)

type StageResult struct {
	Stage  Stage       `json:"stage"`
	Status StageStatus `json:"status"`
	// 나노초 정수로 마샬링된다. 0 은 측정값이 아니므로 차트에 그리면 안 된다.
	Duration  time.Duration `json:"duration_ns"`
	ErrorKind ErrorKind     `json:"error_kind,omitempty"`
	// 원문 메시지 — 사람용, 분기는 ErrorKind 로 한다.
	Error string `json:"error,omitempty"`
}

type DNSInfo struct {
	Addresses []string `json:"addresses"`
}

type TCPInfo struct {
	RemoteAddr string `json:"remote_addr,omitempty"`
	LocalAddr  string `json:"local_addr,omitempty"`
	// true 면 dial 을 건너뛰어 TCP 단계가 측정 안 된다 — 프로버는 막지만 사실은 남긴다.
	Reused bool `json:"reused"`
	// 2 이상이면 앞 주소들이 죽어 있었다는 뜻 — error_kind 가 비어도 이상을 드러낸다. omitempty 미부착.
	Attempts int `json:"attempts"`
}

type HTTPInfo struct {
	StatusCode int    `json:"status_code"`
	Proto      string `json:"proto"`
}

// 스키마 정본은 docs/result-schema.md — 필드 이름을 바꾸면 2국면 exporter 가 깨진다.
type Result struct {
	Target    string        `json:"target"`
	StartedAt time.Time     `json:"started_at"`
	Stages    []StageResult `json:"stages"`

	// 포인터인 이유: omitempty 는 struct 를 비었다고 안 본다 — 값 타입이면 미거치 시에도 `"dns":{"addresses":null}` 이 나간다.
	DNS  *DNSInfo  `json:"dns,omitempty"`
	TCP  *TCPInfo  `json:"tcp,omitempty"`
	HTTP *HTTPInfo `json:"http,omitempty"`

	// 각 단계의 합이 아니다 — 요청 write·리다이렉트·정리 시간이 함께 들어간다.
	Total time.Duration `json:"total_ns"`

	FailureStage Stage     `json:"failure_stage,omitempty"`
	ErrorKind    ErrorKind `json:"error_kind,omitempty"`
}
