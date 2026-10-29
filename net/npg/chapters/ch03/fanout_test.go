package ch03

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 실험 2 — 팬아웃 다이얼과 select (원문 Listing 3-8).
//
// 원문 그대로의 흐름에 "몇 명이 연결에 성공했나 / 몇 명이 취소로 끝났나"를
// 세는 계수기만 더했다. 리스너는 원문처럼 연결을 딱 하나만 Accept 한다.

type fanoutCount struct {
	connected atomic.Int32 // DialContext 가 성공한 다이얼러
	canceled  atomic.Int32 // DialContext 가 에러로 돌아온 다이얼러
	lost      atomic.Int32 // 연결은 됐지만 select 에서 ctx.Done() 쪽으로 빠진 다이얼러
}

func TestDialContextCancelFanOut(t *testing.T) { runFanOut(t, 0) }

// 실험 2-c — 첫 응답을 받은 뒤 100ms 기다렸다 취소한다.
// 나머지 다이얼러에게 핸드셰이크를 끝낼 시간을 준다.
func TestDialContextCancelFanOutDelayed(t *testing.T) { runFanOut(t, 100*time.Millisecond) }

func runFanOut(t *testing.T, cancelDelay time.Duration) {
	t.Helper()
	// Given — 10초 마감 context, 연결을 하나만 받는 리스너
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		// 원문: 연결을 하나만 받는다
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	var cnt fanoutCount
	dial := func(ctx context.Context, address string, response chan int, id int, wg *sync.WaitGroup) {
		defer wg.Done()

		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			cnt.canceled.Add(1)
			return
		}
		cnt.connected.Add(1)
		c.Close()

		select {
		case <-ctx.Done():
			cnt.lost.Add(1)
		case response <- id:
		}
	}

	// When — 다이얼러 10개를 동시에 띄우고, 첫 응답을 받자마자 취소한다
	res := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go dial(ctx, listener.Addr().String(), res, i+1, &wg)
	}

	response := <-res
	time.Sleep(cancelDelay)
	cancel()
	wg.Wait()
	close(res)

	// Then — 취소로 끝났는지 확인하고, 다이얼러들이 어떻게 끝났는지 센다
	if ctx.Err() != context.Canceled {
		t.Errorf("expected canceled context; actual: %s", ctx.Err())
	}
	t.Logf("승자            = dialer %d", response)
	t.Logf("연결 성공       = %d", cnt.connected.Load())
	t.Logf("  그중 select 에서 ctx.Done() 으로 빠짐 = %d", cnt.lost.Load())
	t.Logf("DialContext 에러 = %d", cnt.canceled.Load())
}

// 실험 2-b — select 를 빼고 response <- id 만 쓰면 어떻게 되나.
// wg.Wait() 가 영원히 안 풀릴 수 있으므로 3초 감시를 붙인다.
// 멈춘 고루틴은 테스트가 끝나도 회수되지 않는다 (누수 그 자체가 관찰 대상).
func TestDialFanOutWithoutSelect(t *testing.T) { runWithoutSelect(t, 0) }

// 실험 2-d — select 없는 판에서도 100ms 기다렸다 취소한다.
func TestDialFanOutWithoutSelectDelayed(t *testing.T) {
	runWithoutSelect(t, 100*time.Millisecond)
}

func runWithoutSelect(t *testing.T, cancelDelay time.Duration) {
	t.Helper()
	// Given — 실험 2 와 같은 리스너, select 없는 다이얼러
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	var finished atomic.Int32
	dial := func(ctx context.Context, address string, response chan int, id int, wg *sync.WaitGroup) {
		defer wg.Done()
		defer finished.Add(1)

		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return
		}
		c.Close()
		response <- id // select 없음
	}

	// When — 다이얼러 10개, 첫 응답 뒤 취소, 그리고 wg.Wait() 를 3초 감시
	res := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go dial(ctx, listener.Addr().String(), res, i+1, &wg)
	}

	response := <-res
	time.Sleep(cancelDelay)
	cancel()

	waited := make(chan struct{})
	go func() {
		wg.Wait()
		close(waited)
	}()

	// Then — wg.Wait() 가 풀렸는지, 끝난 다이얼러가 몇 개인지 남긴다
	select {
	case <-waited:
		t.Logf("wg.Wait() 풀림 — 끝난 다이얼러 %d/10", finished.Load())
	case <-time.After(3 * time.Second):
		t.Logf("wg.Wait() 3초 동안 안 풀림 — 끝난 다이얼러 %d/10 (승자 dialer %d)", finished.Load(), response)
	}
}
