package notify

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "npgntf-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestReadyReceived(t *testing.T) {
	// Given — 답장하지 않는 감독 소켓
	dir := shortDir(t)
	r, err := Listen(filepath.Join(dir, "n.sock"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// When — READY=1 을 보낸다
	if _, err := (Sender{Socket: r.Path()}).Send("READY=1"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Next()

	// Then — 본문이 그대로 도착한다
	if err != nil || got.Body != "READY=1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestNoSocketIsNoop(t *testing.T) {
	// Given — NOTIFY_SOCKET 이 없는 환경
	t.Setenv("NOTIFY_SOCKET", "")

	// When — 보낸다
	ack, err := Sender{}.Send("READY=1")

	// Then — 아무것도 하지 않는다
	if ack != "" || err != nil {
		t.Fatalf("ack=%q err=%v", ack, err)
	}
}

func TestAckBoundVsUnboundLog(t *testing.T) {
	dir := shortDir(t)
	for _, local := range []string{"", filepath.Join(dir, "client.sock")} {
		// Given — 답장하는 감독 소켓
		r, err := Listen(filepath.Join(dir, "n.sock"), true)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan Message, 1)
		go func() {
			m, _ := r.Next()
			done <- m
		}()

		// When — 보내는 쪽 경로를 묶거나 묶지 않고 보낸 뒤 답장을 1초 기다린다
		ack, err := Sender{Socket: r.Path(), LocalPath: local, AckWait: time.Second}.Send("READY=1")
		m := <-done
		r.Close()

		// Then — 양쪽에서 본 결과를 남긴다
		t.Logf("local=%-28q | supervisor: from=%q reply_err=%v | sender: ack=%q err=%v", local, m.From, m.ReplyErr, ack, err)
	}
}
