package admin

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 소켓 경로는 sun_path 길이 제한(macOS 104바이트) 때문에 /tmp 아래 짧은 폴더에 만든다.
func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "npgadm-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func client(path string) *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}},
	}
}

func startAdmin(t *testing.T, opt Options) *Server {
	t.Helper()
	s, err := Listen(opt, Commands{Status: func() any { return map[string]int{"ok": 1} }}, nil)
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(func() { s.Shutdown(context.Background()) })
	return s
}

func TestSocketPermApplied(t *testing.T) {
	// Given — 권한 0600 을 건 관리 소켓
	path := filepath.Join(shortDir(t), "a.sock")
	startAdmin(t, Options{Path: path, Perm: 0o600})

	// When — 소켓 파일을 stat 한다
	fi, err := os.Stat(path)

	// Then — 소켓 파일이고 권한이 0600 이다
	if err != nil || fi.Mode()&os.ModeSocket == 0 || fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", fi.Mode(), err)
	}
}

func TestStatusOverUnixSocket(t *testing.T) {
	// Given — 인가를 끈 관리 소켓
	path := filepath.Join(shortDir(t), "a.sock")
	startAdmin(t, Options{Path: path})

	// When — unix 소켓으로 GET /status 와 GET /peer 를 부른다
	resp, err := client(path).Get("http://admin/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	peer, err := client(path).Get("http://admin/peer")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(peer.Body)
	peer.Body.Close()

	// Then — 200, 자격 증명 결과는 남긴다
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	t.Logf("/peer → %s", b)
}

// leaveStale 은 경로에 듣는 프로세스가 없는 소켓 파일을 남긴다.
func leaveStale(t *testing.T, path string) {
	t.Helper()
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	l.SetUnlinkOnClose(false)
	l.Close()
}

func TestStaleProbeRemovesDeadSocket(t *testing.T) {
	// Given — 듣는 프로세스가 없는 남은 소켓 파일
	path := filepath.Join(shortDir(t), "a.sock")
	leaveStale(t, path)

	// When — probe 방식으로 듣기 시작한다
	s, err := Listen(Options{Path: path, Stale: "probe"}, Commands{}, nil)

	// Then — 시작한다
	if err != nil {
		t.Fatal(err)
	}
	s.Shutdown(context.Background())
}

func TestStaleProbeKeepsLiveSocket(t *testing.T) {
	// Given — 이미 다른 서버가 듣고 있는 소켓
	path := filepath.Join(shortDir(t), "a.sock")
	startAdmin(t, Options{Path: path})

	// When — probe 방식으로 같은 경로에 듣기 시작한다
	_, err := Listen(Options{Path: path, Stale: "probe"}, Commands{}, nil)

	// Then — 시작을 거절하고 원래 서버는 계속 답한다
	if err == nil {
		t.Fatal("살아 있는 소켓 경로에서 시작했다")
	}
	resp, gerr := client(path).Get("http://admin/status")
	if gerr != nil {
		t.Fatal(gerr)
	}
	resp.Body.Close()
}

func TestStaleFailAndRemoveLog(t *testing.T) {
	for _, mode := range []string{"fail", "remove"} {
		// Given — 남은 소켓 파일
		path := filepath.Join(shortDir(t), "a.sock")
		leaveStale(t, path)

		// When — 그 방식으로 듣기 시작한다
		s, err := Listen(Options{Path: path, Stale: mode}, Commands{}, nil)

		// Then — 결과를 남긴다
		t.Logf("stale=%s → err=%v", mode, err)
		if s != nil {
			s.Shutdown(context.Background())
		}
	}
}

func TestRefuseNonSocketFile(t *testing.T) {
	// Given — 경로에 보통 파일이 있다
	path := filepath.Join(shortDir(t), "a.sock")
	os.WriteFile(path, []byte("keep"), 0o600)

	// When — remove 방식으로 듣기 시작한다
	_, err := Listen(Options{Path: path, Stale: "remove"}, Commands{}, nil)

	// Then — 거절하고 파일은 그대로다
	b, _ := os.ReadFile(path)
	if err == nil || string(b) != "keep" {
		t.Fatalf("err=%v content=%q", err, b)
	}
}
