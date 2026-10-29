package scan

import (
	"context"
	"net"
	"testing"
	"time"
)

// closedPort 는 잠깐 열었다 닫아 지금은 아무도 듣지 않는 loopback 포트 번호를 돌려준다.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestProbeConnectOpenAndClosed(t *testing.T) {
	// Given — 듣고 있는 포트 하나와 아무도 듣지 않는 포트 하나
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	open := ln.Addr().(*net.TCPAddr).Port
	closed := closedPort(t)

	// When — 두 포트를 CONNECT 로 묻는다
	got := Run(context.Background(), "127.0.0.1", []int{open, closed}, ProbeConnect, Options{Workers: 2, Timeout: time.Second})

	// Then — 열린 포트는 open, 닫힌 포트는 RST 를 받아 ECONNREFUSED 로 closed
	if got[0].Status != StatusOpen {
		t.Errorf("open 포트 = %+v", got[0])
	}
	if got[1].Status != StatusClosed || got[1].Errno != "ECONNREFUSED" {
		t.Errorf("closed 포트 = %+v", got[1])
	}
}

func TestRunRetriesOnlyOnNoReply(t *testing.T) {
	// Given — 포트 1 은 늘 무응답, 포트 2 는 늘 거절하는 탐침 (판정 로직만 보려고 네트워크 대신 정해진 결과를 준다)
	probe := func(_ context.Context, _ string, port int, _ time.Duration) Result {
		if port == 1 {
			return Result{Port: port, Status: StatusFiltered, Errno: errTimeout}
		}
		return Result{Port: port, Status: StatusClosed, Errno: "ECONNREFUSED"}
	}

	// When — 재시도 2 로 돌린다
	got := Run(context.Background(), "h", []int{1, 2}, probe, Options{Workers: 1, Retries: 2})

	// Then — 무응답 포트만 1+2 번, 거절된 포트는 1 번 묻는다
	if got[0].Attempts != 3 || got[1].Attempts != 1 {
		t.Fatalf("attempts = %d, %d, want 3, 1", got[0].Attempts, got[1].Attempts)
	}
}

func TestRunOverlapsWaits(t *testing.T) {
	// Given — 탐침 하나가 100ms 를 기다리는 포트 10 개
	probe := func(_ context.Context, _ string, port int, _ time.Duration) Result {
		time.Sleep(100 * time.Millisecond)
		return Result{Port: port, Status: StatusFiltered, Errno: errTimeout}
	}
	ports := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// When — worker 10 개로 돌린다
	start := time.Now()
	Run(context.Background(), "h", ports, probe, Options{Workers: 10})
	elapsed := time.Since(start)

	// Then — 순서대로면 1초가 걸릴 일이 기다림이 겹쳐 한 번 기다린 만큼에 끝난다
	if elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed = %s, want < 500ms", elapsed)
	}
}

func TestRunRateLimitsProbes(t *testing.T) {
	// Given — 곧바로 끝나는 탐침과 포트 10 개
	probe := func(_ context.Context, _ string, port int, _ time.Duration) Result {
		return Result{Port: port, Status: StatusClosed, Errno: "ECONNREFUSED"}
	}
	ports := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// When — worker 는 10 개지만 rate 는 초당 20 개로 묶는다
	start := time.Now()
	Run(context.Background(), "h", ports, probe, Options{Workers: 10, Rate: 20})
	elapsed := time.Since(start)

	// Then — worker 수와 관계없이 50ms 간격으로 하나씩 나가 약 0.5초가 걸린다
	if elapsed < 400*time.Millisecond {
		t.Fatalf("elapsed = %s, want >= 400ms", elapsed)
	}
}
