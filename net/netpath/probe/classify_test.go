package probe

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/scofe97/runners-lab/net/netpath/types"
)

// 손으로 만든 에러가 아니라 실제 요청에서 나온 에러로 분류를 검사한다.
// 체인 모양을 잘못 가정하면 손으로 만든 에러는 통과하고 실전에서 틀린다.
func TestClassifyRealErrors(t *testing.T) {
	tests := []struct {
		name  string
		stage types.Stage
		do    func(context.Context) error
		want  types.ErrorKind
	}{
		{
			name:  "닫힌 포트는 refused",
			stage: types.StageTCP,
			do:    func(ctx context.Context) error { return get(ctx, "http://127.0.0.1:9") },
			want:  types.ErrTCPRefused,
		},
		{
			// url.Parse 는 통과하고 dial 에서 *net.AddrError 가 된다. 체인 두 겹 안에 있어서
			// urlErr.Err 를 타입 단언하는 구현은 이 케이스를 조용히 놓친다.
			name:  "포트 범위 초과는 invalid_target",
			stage: types.StageTCP,
			do:    func(ctx context.Context) error { return get(ctx, "http://127.0.0.1:99999") },
			want:  types.ErrInvalidTarget,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// When — 실제로 요청해 에러를 받아 분류한다
			err := tc.do(t.Context())
			if err == nil {
				t.Fatal("에러가 나야 하는데 성공했다")
			}

			// Then
			if got := classify(tc.stage, err); got != tc.want {
				t.Errorf("classify = %q, want %q\n  원본: %v", got, tc.want, err)
			}
		})
	}
}

// dial timeout 의 실제 체인은 *url.Error → context.deadlineExceededError 다(실측).
// *net.OpError 가 없어서 에러만으로는 단계를 알 수 없고, 단계는 호출자가 알려준다.
func TestClassifyTimeoutUsesStage(t *testing.T) {
	// Given — 실측한 체인을 그대로 재구성한 에러
	err := &url.Error{Op: "Get", URL: "http://10.255.255.1:8080", Err: context.DeadlineExceeded}

	want := map[types.Stage]types.ErrorKind{
		types.StageDNS:  types.ErrDNSTimeout,
		types.StageTCP:  types.ErrTCPTimeout,
		types.StageHTTP: types.ErrHTTPTimeout,
	}

	// When · Then — 같은 에러가 단계에 따라 다르게 분류된다
	for stage, kind := range want {
		if got := classify(stage, err); got != kind {
			t.Errorf("stage %q: classify = %q, want %q", stage, got, kind)
		}
	}
}

func TestClassifyCanceledIsNotTimeout(t *testing.T) {
	// Given — 호출자가 취소한 상황
	err := &url.Error{Op: "Get", URL: "http://127.0.0.1:9", Err: context.Canceled}

	// When · Then — 취소는 측정 실패가 아니므로 단계별 타임아웃으로 가지 않는다
	if got := classify(types.StageTCP, err); got != types.ErrCanceled {
		t.Errorf("classify = %q, want %q", got, types.ErrCanceled)
	}
}

func TestClassifyNilIsEmpty(t *testing.T) {
	// Given · When · Then — 성공한 단계는 분류값이 없어야 한다 (omitempty 가 필드를 뺀다)
	if got := classify(types.StageTCP, nil); got != "" {
		t.Errorf("classify(nil) = %q, want 빈 문자열", got)
	}
}

// t.Context() 는 테스트가 끝나면 자동으로 취소된다. 그 위에 프로브 예산을 얹는다.
func get(ctx context.Context, target string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
