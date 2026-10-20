package ctrl

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestDialConnectsToListener(t *testing.T) {
	// Given — 임시 포트에서 듣고 있는 서버
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// When — 그 주소로 Dial 한다
	conn, err := Dial(context.Background(), ln.Addr().String())

	// Then — 오류 없이 연결된다
	if err != nil {
		t.Fatalf("Dial 실패: %v", err)
	}
	conn.Close()
}

func TestDialFailsWhenNobodyListens(t *testing.T) {
	// Given — 열었다가 바로 닫아 아무도 듣지 않는 주소
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	// When — 그 주소로 Dial 한다
	_, err = Dial(context.Background(), addr)

	// Then — 오류를 돌려준다
	if err == nil {
		t.Fatal("듣는 쪽이 없는데 Dial 이 성공했다")
	}
}

func TestDialStopsWhenContextCanceled(t *testing.T) {
	// Given — 이미 취소된 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When — Dial 한다
	_, err := Dial(ctx, "127.0.0.1:7100")

	// Then — 취소 오류가 감싸져 있다
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
