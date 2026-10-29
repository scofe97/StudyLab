//go:build linux

package admin

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestAuthzSettingApplied(t *testing.T) {
	// Given — 이 프로세스 사용자의 기본 그룹과, 이 사용자가 속하지 않은 그룹
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	mine, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Fatal(err)
	}
	other := "nogroup"
	if _, err := user.LookupGroup(other); err != nil {
		t.Skip("nogroup 그룹이 없다")
	}

	cases := []struct {
		allow []string
		want  int
	}{
		{nil, 200},
		{[]string{mine.Name}, 200},
		{[]string{other}, 403},
	}
	for _, tc := range cases {
		path := filepath.Join(shortDir(t), "a.sock")
		startAdmin(t, Options{Path: path, AllowGroups: tc.allow})

		// When — 같은 사용자로 GET /status 를 부른다
		resp, err := client(path).Get("http://admin/status")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// Then — 허용 그룹 설정대로 열리고 닫힌다
		if resp.StatusCode != tc.want {
			t.Fatalf("allow=%v status=%d", tc.allow, resp.StatusCode)
		}
	}
}

func countFDs() int {
	ents, _ := os.ReadDir("/proc/self/fd")
	return len(ents)
}

func TestPeerCredMethodsFDLog(t *testing.T) {
	for _, m := range []string{"rawconn", "file", "file-noclose"} {
		// Given — 자격 증명 읽기 방법이 다른 관리 소켓, 연결을 매번 새로 맺는 클라이언트
		path := filepath.Join(shortDir(t), "a.sock")
		startAdmin(t, Options{Path: path, PeerCred: m})
		c := client(path)
		c.Transport.(*http.Transport).DisableKeepAlives = true
		before := countFDs()

		// When — 20 번 연결해 GET /status 를 부른다
		for i := 0; i < 20; i++ {
			resp, err := c.Get("http://admin/status")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		}
		after := countFDs()

		// Then — 앞뒤 fd 개수를 남긴다
		t.Logf("%-9s fd before=%d after=%d (%s)", m, before, after, fmt.Sprintf("Δ%+d", after-before))
	}
}
