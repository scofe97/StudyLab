package proxy

import (
	"context"
	"time"
)

// reqInfo 는 요청 하나의 기록장이다. [1] 에서 비어 있게 달리고, [3]·[4b] 가 채우고, [5] 가 읽는다.
type reqInfo struct {
	reason   Reason        // 실패 원인코드, 성공이면 ReasonNone
	reused   bool          // 업스트림 연결을 풀에서 다시 쓰는가?
	idle     time.Duration // 다시 쓴 연결이 풀에서 쉬고 있던 시간
	attempts int           // 연결을 얻은 시도 수(GotConn 횟수). Transport 가 조용히 다시 보내면 2 이상, 연결을 못 맺은 시도는 안 셈 (ADR 0013)
}

// infoKey 는 ctx 에서 기록장을 찾는 열쇠다. 패키지 밖 타입과 겹치지 않도록 빈 struct 로 만든다.
type infoKey struct{}

// [1] ctx에 빈 기록장을 달고, 그 기록장을 함께 돌려준다.
func withInfo(ctx context.Context) (context.Context, *reqInfo) {
	info := &reqInfo{reason: ReasonNone}
	return context.WithValue(ctx, infoKey{}, info), info
}

// [3]·[4b] ctx에 달린 기록장을 꺼냅니다. 없으면 nil (accessLog 를 거치지 않은 요청)
func infoFrom(ctx context.Context) *reqInfo {
	info, _ := ctx.Value(infoKey{}).(*reqInfo)
	return info
}
