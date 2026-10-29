package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/user/netpath-lab/types"
	"github.com/user/netpath-lab/utils"
)

type Config struct {
	// 프로브 하나의 전체 예산. 0 이면 호출자의 ctx 를 그대로 따른다.
	Timeout time.Duration
}

// 반환 경계는 "재기 시작했는가" — 재다 만난 실패는 Result 에 담고 error 는 nil.
// 근거: docs/concepts/error-as-value.md
func Do(ctx context.Context, target string, cfg Config) (*types.Result, error) {
	u, err := validate(target)
	if err != nil {
		return nil, err
	}

	if cfg.Timeout > 0 {
		// 예산이 이미 없으면 안 보낸다 — 보내면 대상 문제로 오분류된다.
		if left := utils.RemainingTimeout(ctx, cfg.Timeout); left <= 0 {
			return nil, fmt.Errorf("데드라인이 이미 지났다: %v 남음", left)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	tr := &tracer{}
	req, err := http.NewRequestWithContext(
		httptrace.WithClientTrace(ctx, tr.clientTrace()), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("요청을 만들 수 없다 %q: %w", target, err)
	}

	client := &http.Client{Transport: utils.NewTransport()}
	startedAt := time.Now()
	resp, doErr := client.Do(req)
	total := time.Since(startedAt)

	if resp != nil {
		// 안 읽고 닫으면 연결이 정리되지 않는다.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return assemble(u, startedAt, total, tr.snapshot(), resp, doErr), nil
}

// Go 가 값으로 안 갈라주는 것만 막는다 — unsupported scheme·no Host 는 문자열 에러뿐.
func validate(target string) (*url.URL, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("대상을 해석할 수 없다 %q: %w", target, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("지원하지 않는 scheme %q: http 와 https 만 잰다", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("호스트가 없다: %q", target)
	}
	return u, nil
}

type stageInput struct {
	stage types.Stage
	span  stageSpan
	err   error
}

func assemble(u *url.URL, startedAt time.Time, total time.Duration,
	snap traceData, resp *http.Response, doErr error) *types.Result {

	stages := []stageInput{
		{types.StageDNS, snap.dns, snap.dns.err},
		{types.StageTCP, snap.conn, snap.conn.err},
		{types.StageHTTP, snap.http, nil}, // HTTP 훅은 에러를 안 주므로 blame 이 채운다
	}
	blame(stages, doErr)

	r := &types.Result{Target: u.String(), StartedAt: startedAt, Total: total}

	failed := false
	for _, s := range stages {
		sr := types.StageResult{Stage: s.stage, Duration: s.span.duration()}

		switch {
		case failed:
			sr.Status = types.StatusSkipped
		case s.err != nil:
			sr.Status = types.StatusFailed
			sr.ErrorKind = classify(s.stage, s.err)
			sr.Error = s.err.Error()
		case !s.span.started():
			// 애초에 해당 없음 — IP 직접 지정 시 DNS 가 여기다.
			sr.Status = types.StatusSkipped
		case s.stage == types.StageHTTP && resp != nil && resp.StatusCode >= 400:
			// 경로는 살아 있고 내용만 실패 — failure_stage=http 가 곧 "앞은 다 성공".
			sr.Status = types.StatusFailed
			sr.ErrorKind = types.ErrHTTPStatus
			sr.Error = resp.Status
		default:
			sr.Status = types.StatusOK
		}

		if sr.Status == types.StatusFailed && !failed {
			r.FailureStage, r.ErrorKind = sr.Stage, sr.ErrorKind
			failed = true
		}
		r.Stages = append(r.Stages, sr)
	}

	if len(snap.addresses) > 0 {
		r.DNS = &types.DNSInfo{Addresses: snap.addresses}
	}
	if snap.connAttempts > 0 || snap.remoteAddr != "" {
		r.TCP = &types.TCPInfo{
			RemoteAddr: snap.remoteAddr,
			LocalAddr:  snap.localAddr,
			Reused:     snap.reused,
			Attempts:   snap.connAttempts,
		}
	}
	if resp != nil {
		r.HTTP = &types.HTTPInfo{StatusCode: resp.StatusCode, Proto: resp.Proto}
	}
	return r
}

// dial 타임아웃 시 ConnectDone 이 안 불려 span.err 이 비므로, 귀속 안 하면 tcp_timeout 이 http_timeout 으로 보고된다.
func blame(stages []stageInput, doErr error) {
	if doErr == nil {
		return
	}
	for _, s := range stages {
		if s.err != nil {
			return // 훅이 이미 말한 단계가 있으면 그쪽이 주인
		}
	}
	for i, s := range stages {
		if s.span.hung() {
			stages[i].err = doErr
			return
		}
	}
	stages[len(stages)-1].err = doErr // 아무도 자기 것이라 안 하면 마지막 단계가 진다
}
