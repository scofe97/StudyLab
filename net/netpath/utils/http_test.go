package utils

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// 전역 http.DefaultTransport 를 오염시키지 않는 것이 이 함수의 존재 이유 절반이다.
// 오염은 컴파일도 테스트도 통과하면서 같은 프로세스의 다른 요청을 조용히 바꾼다.
func TestNewTransportLeavesDefaultAlone(t *testing.T) {
	// Given — 전역 Transport 의 현재 설정
	def := http.DefaultTransport.(*http.Transport)
	before := def.DisableKeepAlives

	// When — 우리 Transport 를 만든다
	got := NewTransport()

	// Then — 우리 것은 커넥션 재사용이 꺼져 있다
	if !got.DisableKeepAlives {
		t.Error("DisableKeepAlives 가 꺼져 있다. 재사용되면 dial·handshake 훅이 안 불린다")
	}

	// Then — 전역은 건드리지 않았다
	if def.DisableKeepAlives != before {
		t.Error("http.DefaultTransport 가 바뀌었다. 같은 프로세스의 다른 요청에 영향을 준다")
	}
	if got == def {
		t.Error("전역 Transport 를 그대로 돌려줬다. Clone 이 빠졌다")
	}
}

func TestRemainingTimeout(t *testing.T) {
	const want = 5 * time.Second

	// Given — 데드라인이 없는 컨텍스트
	// When · Then — 원하는 값이 그대로 나온다
	if got := RemainingTimeout(t.Context(), want); got != want {
		t.Errorf("데드라인 없음: got %v, want %v", got, want)
	}

	// Given — 남은 시간이 원하는 값보다 짧은 컨텍스트
	short, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	// When
	got := RemainingTimeout(short, want)

	// Then — 남은 시간 쪽으로 깎인다
	if got > 1*time.Second || got <= 0 {
		t.Errorf("남은 시간이 짧을 때: got %v, want (0, 1s]", got)
	}

	// Given — 이미 지난 데드라인
	expired, cancel2 := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel2()

	// When · Then — 0 이하가 나와야 한다. 호출자가 이걸 보고 멈춘다
	if got := RemainingTimeout(expired, want); got > 0 {
		t.Errorf("지난 데드라인: got %v, want <= 0", got)
	}
}
