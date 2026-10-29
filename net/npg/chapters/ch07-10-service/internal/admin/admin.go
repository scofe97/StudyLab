// Package admin 은 Unix 도메인 소켓 위에서 도는 관리용 HTTP 서버다.
//
// Docker 의 docker.sock 처럼 파일 경로 하나가 관리 채널의 주소다.
// 접근 제어로 소켓 파일 권한·그룹과 SO_PEERCRED(Linux 전용) 인가를 둔다.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// Options 는 관리 소켓 설정이다.
type Options struct {
	// 소켓 파일 경로.
	Path string
	// Listen 뒤 소켓 파일에 거는 권한. 0 이면 손대지 않는다.
	Perm os.FileMode
	// Listen 뒤 소켓 파일의 그룹을 이 그룹으로 바꾼다. 비면 손대지 않는다.
	Group string
	// 경로에 소켓 파일이 이미 있을 때. fail(그대로 Listen) | remove(지우고 Listen) | probe(Dial 해 보고 판단)
	Stale string
	// 이 그룹 가운데 하나에 속한 호출자만 허용한다. 비면 자격 증명 인가를 하지 않는다.
	AllowGroups []string
	// 자격 증명을 읽는 방법. rawconn(SyscallConn().Control) | file(File() 로 복제 뒤 닫기) | file-noclose(File() 로 복제 뒤 닫지 않기)
	PeerCred string
	// 허용 판정 기준. uid-groups | gid
	AuthzBy string
}

// Commands 는 관리 명령이 부르는 함수들이다.
type Commands struct {
	Status   func() any
	Drain    func()
	Level    func() string
	SetLevel func(string) error
}

// Cred 는 커널이 알려 준 상대 프로세스의 자격 증명이다.
type Cred struct {
	PID int32  `json:"pid"`
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

type peerInfo struct {
	cred *Cred
	err  error
}

type peerKey struct{}

// Server 는 관리 소켓 서버다.
type Server struct {
	opt     Options
	cmds    Commands
	log     *slog.Logger
	ln      *net.UnixListener
	srv     *http.Server
	allowed map[uint32]string
}

var errUnsupported = errors.New("이 플랫폼에서는 SO_PEERCRED 를 읽지 않는다")

// Listen 은 소켓 파일을 준비하고 듣기 시작한다. Serve 는 따로 부른다.
func Listen(opt Options, cmds Commands, log *slog.Logger) (*Server, error) {
	if opt.Stale == "" {
		opt.Stale = "probe"
	}
	if opt.PeerCred == "" {
		opt.PeerCred = "rawconn"
	}
	if opt.AuthzBy == "" {
		opt.AuthzBy = "uid-groups"
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	switch opt.Stale {
	case "fail", "remove", "probe":
	default:
		return nil, fmt.Errorf("stale: %q", opt.Stale)
	}
	switch opt.PeerCred {
	case "rawconn", "file", "file-noclose":
	default:
		return nil, fmt.Errorf("peercred: %q", opt.PeerCred)
	}
	switch opt.AuthzBy {
	case "uid-groups", "gid":
	default:
		return nil, fmt.Errorf("authz-by: %q", opt.AuthzBy)
	}
	if len(opt.AllowGroups) > 0 && !peerCredSupported {
		return nil, fmt.Errorf("허용 그룹을 걸었지만 %s: %w", runtime.GOOS, errUnsupported)
	}

	allowed := map[uint32]string{}
	for _, name := range opt.AllowGroups {
		g, err := user.LookupGroup(name)
		if err != nil {
			return nil, fmt.Errorf("허용 그룹 %q: %w", name, err)
		}
		gid, err := strconv.ParseUint(g.Gid, 10, 32)
		if err != nil {
			return nil, err
		}
		allowed[uint32(gid)] = name
	}

	if err := prepareSocketPath(opt.Path, opt.Stale, log); err != nil {
		return nil, err
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: opt.Path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if opt.Perm != 0 {
		if err := os.Chmod(opt.Path, opt.Perm); err != nil {
			ln.Close()
			return nil, err
		}
	}
	if opt.Group != "" {
		g, err := user.LookupGroup(opt.Group)
		if err == nil {
			gid, _ := strconv.Atoi(g.Gid)
			err = os.Chown(opt.Path, -1, gid)
		}
		if err != nil {
			ln.Close()
			return nil, fmt.Errorf("소켓 그룹 %q: %w", opt.Group, err)
		}
	}

	s := &Server{opt: opt, cmds: cmds, log: log, ln: ln, allowed: allowed}
	s.srv = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ConnContext:       s.connContext,
	}
	if fi, err := os.Stat(opt.Path); err == nil {
		log.Info("admin listening", "path", opt.Path, "mode", fi.Mode().String(),
			"allow_groups", opt.AllowGroups, "peercred", opt.PeerCred, "authz_by", opt.AuthzBy)
	}
	return s, nil
}

// prepareSocketPath 는 경로에 이미 있는 파일을 stale 방식대로 처리한다.
func prepareSocketPath(path, stale string, log *slog.Logger) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&fs.ModeSocket == 0 {
		return fmt.Errorf("%s 는 소켓 파일이 아니라서 건드리지 않는다", path)
	}
	switch stale {
	case "remove":
		log.Info("admin socket exists, removing", "path", path)
		return os.Remove(path)
	case "probe":
		c, err := net.DialTimeout("unix", path, 300*time.Millisecond)
		if err == nil {
			c.Close()
			return fmt.Errorf("%s: Dial 이 성공했다 — 다른 프로세스가 듣는 중이라 지우지 않는다", path)
		}
		log.Info("admin socket exists, dial failed", "path", path, "err", err.Error())
		if errors.Is(err, syscall.ECONNREFUSED) {
			return os.Remove(path)
		}
		return fmt.Errorf("%s 확인 중 Dial 에러라 지우지 않는다: %w", path, err)
	}
	return nil
}

// Path 는 소켓 파일 경로다.
func (s *Server) Path() string { return s.opt.Path }

// Serve 는 Shutdown 까지 요청을 받는다.
func (s *Server) Serve() error {
	err := s.srv.Serve(s.ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown 은 관리 서버를 멈춘다.
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }

func (s *Server) connContext(ctx context.Context, c net.Conn) context.Context {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return context.WithValue(ctx, peerKey{}, peerInfo{err: fmt.Errorf("%T 는 UnixConn 이 아니다", c)})
	}
	cred, err := readPeerCred(uc, s.opt.PeerCred)
	return context.WithValue(ctx, peerKey{}, peerInfo{cred: cred, err: err})
}

// decision 은 한 요청의 인가 판정 기록이다.
type decision struct {
	Peer        *Cred    `json:"peer"`
	PeerErr     string   `json:"peer_err,omitempty"`
	User        string   `json:"user,omitempty"`
	UserGroups  []string `json:"user_groups,omitempty"`
	AllowGroups []string `json:"allow_groups"`
	AuthzBy     string   `json:"authz_by"`
	PeerCredBy  string   `json:"peercred_by"`
	Allowed     bool     `json:"allowed"`
	Reason      string   `json:"reason"`
}

func groupName(gid string) string {
	if g, err := user.LookupGroupId(gid); err == nil {
		return g.Name
	}
	return gid
}

func (s *Server) decide(p peerInfo) decision {
	d := decision{Peer: p.cred, AllowGroups: s.opt.AllowGroups, AuthzBy: s.opt.AuthzBy, PeerCredBy: s.opt.PeerCred}
	if d.AllowGroups == nil {
		d.AllowGroups = []string{}
	}
	if p.err != nil {
		d.PeerErr = p.err.Error()
	}
	var gids []string
	if p.cred != nil {
		if u, err := user.LookupId(strconv.FormatUint(uint64(p.cred.UID), 10)); err == nil {
			d.User = u.Username
			gids, _ = u.GroupIds()
			for _, g := range gids {
				d.UserGroups = append(d.UserGroups, groupName(g))
			}
		}
	}
	switch {
	case len(s.allowed) == 0:
		d.Allowed, d.Reason = true, "허용 그룹 미설정 — 자격 증명 인가를 하지 않음"
	case p.cred == nil:
		d.Allowed, d.Reason = false, "자격 증명을 읽지 못함"
	case s.opt.AuthzBy == "gid":
		name, ok := s.allowed[p.cred.GID]
		d.Allowed = ok
		d.Reason = fmt.Sprintf("자격 증명 GID %d 를 허용 목록과 비교 (일치: %q)", p.cred.GID, name)
	default:
		d.Reason = fmt.Sprintf("UID %d 의 사용자 DB 소속 그룹 %v 를 허용 목록과 비교", p.cred.UID, d.UserGroups)
		for _, g := range gids {
			n, err := strconv.ParseUint(g, 10, 32)
			if err != nil {
				continue
			}
			if _, ok := s.allowed[uint32(n)]; ok {
				d.Allowed = true
				break
			}
		}
	}
	return d
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		var v any
		if s.cmds.Status != nil {
			v = s.cmds.Status()
		}
		writeJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("POST /drain", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]string{"result": "drain started"})
		if s.cmds.Drain != nil {
			go s.cmds.Drain()
		}
	})
	mux.HandleFunc("GET /log-level", func(w http.ResponseWriter, _ *http.Request) {
		lv := ""
		if s.cmds.Level != nil {
			lv = s.cmds.Level()
		}
		writeJSON(w, http.StatusOK, map[string]string{"level": lv})
	})
	mux.HandleFunc("PUT /log-level", func(w http.ResponseWriter, r *http.Request) {
		if s.cmds.SetLevel == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "no log level"})
			return
		}
		if err := s.cmds.SetLevel(r.URL.Query().Get("level")); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"level": s.cmds.Level()})
	})
	mux.HandleFunc("GET /peer", func(w http.ResponseWriter, r *http.Request) {
		p, _ := r.Context().Value(peerKey{}).(peerInfo)
		writeJSON(w, http.StatusOK, s.decide(p))
	})
	mux.HandleFunc("POST /gc", func(w http.ResponseWriter, _ *http.Request) {
		runtime.GC()
		writeJSON(w, http.StatusOK, map[string]string{"result": "runtime.GC done"})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := r.Context().Value(peerKey{}).(peerInfo)
		d := s.decide(p)
		attrs := []any{"method", r.Method, "path", r.URL.Path, "allowed", d.Allowed, "reason", d.Reason}
		if d.Peer != nil {
			attrs = append(attrs, "pid", d.Peer.PID, "uid", d.Peer.UID, "gid", d.Peer.GID, "user", d.User)
		}
		if d.PeerErr != "" {
			attrs = append(attrs, "peer_err", d.PeerErr)
		}
		s.log.Info("admin request", attrs...)
		if !d.Allowed {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden", "decision": d})
			return
		}
		mux.ServeHTTP(w, r)
	})
}
