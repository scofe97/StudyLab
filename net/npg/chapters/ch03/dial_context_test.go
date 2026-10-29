package ch03

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// 실험 1 — 연결 시도는 context 로 끊는다 (원문 Listing 3-6·3-7).
//
// 진짜로 응답 없는 서버를 찾는 대신 원문처럼 Dialer.Control 에서 잠들게 해
// "핸드셰이크가 오래 걸리는 상황"을 만든다. Control 은 소켓을 만든 뒤
// connect 를 부르기 직전에 실행되는 훅이다.

// classify 는 Q4 에서 예측한 판별 방법 세 가지를 같은 에러에 모두 적용해 본다.
func classify(t *testing.T, err error) {
	t.Helper()
	t.Logf("err            = %v", err)
	t.Logf("err 의 타입    = %T", err)
	t.Logf("errors.Is(err, context.DeadlineExceeded) = %v", errors.Is(err, context.DeadlineExceeded))
	t.Logf("errors.Is(err, context.Canceled)         = %v", errors.Is(err, context.Canceled))
	t.Logf("errors.Is(err, os.ErrDeadlineExceeded)   = %v", errors.Is(err, os.ErrDeadlineExceeded))

	var nErr net.Error
	if errors.As(err, &nErr) {
		t.Logf("errors.As(err, &net.Error) = true, Timeout() = %v", nErr.Timeout())
	} else {
		t.Logf("errors.As(err, &net.Error) = false")
	}
}

func TestDialContextDeadline(t *testing.T) {
	// Given — 5초 뒤가 마감인 context, connect 직전에 5초+1ms 잠드는 Dialer
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cancel()

	var d net.Dialer
	d.Control = func(_, _ string, _ syscall.RawConn) error {
		time.Sleep(5*time.Second + time.Millisecond)
		return nil
	}

	// When — 마감이 걸린 context 로 연결을 시도한다
	begin := time.Now()
	conn, err := d.DialContext(ctx, "tcp", "10.0.0.0:80")
	elapsed := time.Since(begin)

	// Then — 연결은 실패해야 하고, 무엇이 어떻게 실패했는지 남긴다
	if err == nil {
		conn.Close()
		t.Fatal("연결이 시간 초과로 끝나지 않았다")
	}
	t.Logf("걸린 시간      = %s", elapsed.Truncate(time.Millisecond))
	classify(t, err)
	t.Logf("ctx.Err()      = %v", ctx.Err())
}

func TestDialContextCancel(t *testing.T) {
	// Given — 시간 조건 없이 cancel 만 붙은 context, connect 직전에 1초 잠드는 Dialer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		var d net.Dialer
		d.Control = func(_, _ string, _ syscall.RawConn) error {
			time.Sleep(time.Second)
			return nil
		}
		conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")
		if err == nil {
			conn.Close()
		}
		done <- err
	}()

	// When — 연결 시도가 진행 중일 때 바깥에서 취소한다
	begin := time.Now()
	cancel()
	err := <-done
	elapsed := time.Since(begin)

	// Then — 연결은 실패해야 하고, 무엇이 어떻게 실패했는지 남긴다
	if err == nil {
		t.Fatal("연결이 취소되지 않았다")
	}
	t.Logf("걸린 시간      = %s", elapsed.Truncate(time.Millisecond))
	classify(t, err)
	t.Logf("ctx.Err()      = %v", ctx.Err())
}

// 실험 1-b — 원문 Listing 3-7 은 cancel() 을 고루틴 시작 직후 바로 불러,
// 다이얼이 "진행 중"일 때 취소된다는 보장이 없다. 여기서는 100ms 기다려
// 고루틴이 Control 안에서 잠든 뒤에 취소한다.
func TestDialContextCancelMidway(t *testing.T) {
	// Given — 시간 조건 없이 cancel 만 붙은 context, connect 직전에 1초 잠드는 Dialer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	controlRan := make(chan struct{}, 1)

	go func() {
		var d net.Dialer
		d.Control = func(_, _ string, _ syscall.RawConn) error {
			controlRan <- struct{}{}
			time.Sleep(time.Second)
			return nil
		}
		conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")
		if err == nil {
			conn.Close()
		}
		done <- err
	}()

	// When — 100ms 뒤, 고루틴이 Control 안에서 자는 중에 취소한다
	begin := time.Now()
	time.Sleep(100 * time.Millisecond)
	cancel()
	t.Logf("cancel() 호출  = %s 시점", time.Since(begin).Truncate(time.Millisecond))
	err := <-done
	elapsed := time.Since(begin)

	// Then — 연결은 실패해야 하고, Control 이 실제로 돌았는지와 걸린 시간을 남긴다
	if err == nil {
		t.Fatal("연결이 취소되지 않았다")
	}
	select {
	case <-controlRan:
		t.Logf("Control 실행   = 예")
	default:
		t.Logf("Control 실행   = 아니오")
	}
	t.Logf("걸린 시간      = %s", elapsed.Truncate(time.Millisecond))
	classify(t, err)
}
