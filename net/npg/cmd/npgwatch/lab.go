package main

// lab.go 는 -lab 모드에서 백엔드(fakeinv)·프록시(npgproxy)·부하(npgload)를 npgwatch 가 직접 띄우고 멈추게 합니다.
// 웹 화면의 버튼은 여기 미리 정한 명령만 실행합니다. 화면에서 명령 문자열을 받지 않습니다.
// 장애 손잡이(fakeinv /_faults)는 버튼으로 내놓지 않습니다. 리허설에서는 원인을 숨겨야 하기 때문입니다.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// lineLog 는 최근 n 줄을 들고 있습니다.
type lineLog struct {
	mu    sync.Mutex
	n     int
	lines []string
}

func (l *lineLog) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, s)
	if len(l.lines) > l.n*2 { // 매번 자르지 않고 두 배가 되면 한 번에 자른다
		l.lines = append([]string(nil), l.lines[len(l.lines)-l.n:]...)
	}
}

func (l *lineLog) tail(k int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.lines) < k {
		k = len(l.lines)
	}
	return append([]string(nil), l.lines[len(l.lines)-k:]...)
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI 는 zerolog 사람용 출력의 색 코드를 지웁니다.
func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

// loadStat 은 npgload 1초 줄 하나입니다. 지연은 밀리초, 그 1초에 끝난 요청이 없으면 p50·p99 는 NaN.
type loadStat struct {
	at                    time.Time
	inflight, n           float64
	ok, c4xx, c5xx, other float64
	p50, p99              float64
}

// parseLoadLine 은 "15:04:05 [label] inflight=50 n=919 200:158 502:761 p50=32ms p99=227ms max=284ms" 를 읽습니다.
// 요약 줄("요약 sent=…")처럼 inflight 가 없는 줄은 ok=false 입니다.
func parseLoadLine(line string) (label string, st loadStat, ok bool) {
	f := strings.Fields(line)
	if len(f) < 3 {
		return "", st, false
	}
	label = "load"
	rest := f[1:] // f[0] 은 시각
	if !strings.ContainsAny(rest[0], "=:") {
		label, rest = rest[0], rest[1:]
	}
	st.p50, st.p99 = math.NaN(), math.NaN()
	seen := false
	for _, tok := range rest {
		if k, v, found := strings.Cut(tok, "="); found {
			switch k {
			case "inflight":
				st.inflight, _ = strconv.ParseFloat(v, 64)
				seen = true
			case "n":
				st.n, _ = strconv.ParseFloat(v, 64)
			case "p50", "p99":
				d, err := time.ParseDuration(v)
				if err != nil {
					continue
				}
				ms := float64(d) / float64(time.Millisecond)
				if k == "p50" {
					st.p50 = ms
				} else {
					st.p99 = ms
				}
			}
			continue
		}
		k, v, found := strings.Cut(tok, ":")
		if !found {
			continue
		}
		c, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		code, err := strconv.Atoi(k)
		switch {
		case err != nil:
			st.other += c // timeout·reset 처럼 상태 코드를 못 받은 실패
		case code < 400:
			st.ok += c
		case code < 500:
			st.c4xx += c
		default:
			st.c5xx += c
		}
	}
	return label, st, seen
}

// proc 은 화면에서 띄우고 멈추는 프로그램 하나입니다.
type proc struct {
	name   string
	args   []string // args[0] 은 실행 파일 경로
	log    *lineLog
	warn   *lineLog // WRN·ERR 줄만 (프록시 접근 로그가 초당 수천 줄이라 따로 둔다)
	onLine func(string)

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	pid     int
	since   time.Time
	exit    string
}

type procView struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Since   int64  `json:"since,omitempty"`
	Exit    string `json:"exit,omitempty"`
	Cmd     string `json:"cmd"`
}

func (p *proc) view() procView {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := procView{Name: p.name, Running: p.running, PID: p.pid, Exit: p.exit,
		Cmd: strings.Join(append([]string{"./" + filepath.Base(p.args[0])}, p.args[1:]...), " ")}
	if p.running {
		v.Since = p.since.UnixMilli()
	}
	return v
}

var errRunning = errors.New("이미 돌고 있습니다")

// scene 은 부하 묶음 하나입니다. 부하마다 label 이 npgload -label 로 들어가 1초 줄 앞에 붙습니다.
type scene struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Loads []sceneLoad `json:"-"`
}

type sceneLoad struct {
	label string
	args  []string // -url 의 {P} 는 프록시 주소로 바뀐다
}

var scenes = []scene{
	{ID: "base", Title: "평소 부하: 단건 조회 초당 50", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "50", "-poisson", "-d", "10m"}},
	}},
	{ID: "i1", Title: "I1: 단건 조회 초당 50 + 검색 초당 5", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "50", "-poisson", "-d", "10m"}},
		{"search", []string{"-url", "http://{P}/v1/devices?q=edge", "-rate", "5", "-poisson", "-d", "10m"}},
	}},
	{ID: "i3", Title: "I3: 단건 조회 동시 50, 60초", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-c", "50", "-d", "60s"}},
	}},
	{ID: "i2", Title: "I2: 단건 조회 초당 1 + 장비 등록 초당 1 (한산한 시간대)", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "1", "-poisson", "-d", "10m"}},
		{"post", []string{"-url", "http://{P}/v1/devices", "-method", "POST", "-H", "Content-Type: application/json",
			"-body", `{"serial":"SN-%d"}`, "-rate", "1", "-poisson", "-d", "10m"}},
	}},
}

// lab 은 프로그램들과 부하 1초 줄의 최신 값을 들고 있습니다.
type lab struct {
	ctx         context.Context
	bin, suffix string // 실행 파일 = bin/<이름><suffix>
	backendAddr string
	proxyAddr   string
	adminAddr   string
	logDir      string

	mu    sync.Mutex
	procs map[string]*proc
	loads map[string]loadStat
}

func newLab(ctx context.Context, bin, suffix, backendAddr, proxyAddr, adminAddr, logDir string) *lab {
	lb := &lab{ctx: ctx, bin: bin, suffix: suffix, backendAddr: backendAddr, proxyAddr: proxyAddr,
		adminAddr: adminAddr, logDir: logDir, procs: map[string]*proc{}, loads: map[string]loadStat{}}
	lb.procs["backend"] = lb.newProc("backend", "fakeinv", "-addr", backendAddr)
	lb.procs["proxy"] = lb.newProc("proxy", "npgproxy", "-addr", proxyAddr, "-admin", adminAddr, "-upstream", "http://"+backendAddr)
	return lb
}

func (lb *lab) newProc(name, binName string, args ...string) *proc {
	return &proc{name: name, args: append([]string{filepath.Join(lb.bin, binName+lb.suffix)}, args...),
		log: &lineLog{n: 400}, warn: &lineLog{n: 200}}
}

// start 는 프로그램을 띄우고 출력 줄을 log·파일·onLine 으로 흘립니다.
// 멈출 때는 SIGINT 를 보내 스스로 정리하게 하고, 3초 안에 안 끝나면 죽입니다.
func (lb *lab) start(p *proc) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return errRunning
	}
	ctx, cancel := context.WithCancel(lb.ctx)
	cmd := exec.CommandContext(ctx, p.args[0], p.args[1:]...)
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 3 * time.Second
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	var file *os.File
	if lb.logDir != "" {
		file, _ = os.OpenFile(filepath.Join(lb.logDir, "npglab-"+strings.ReplaceAll(p.name, ":", "-")+".log"),
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	}
	p.cancel, p.running, p.pid, p.since, p.exit = cancel, true, cmd.Process.Pid, time.Now(), ""
	p.log.add(fmt.Sprintf("── %s 시작: %s", p.since.Format("15:04:05"), strings.Join(p.args, " ")))

	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := stripANSI(sc.Text())
			p.log.add(line)
			if strings.Contains(line, " WRN ") || strings.Contains(line, " ERR ") {
				p.warn.add(line)
			}
			if file != nil {
				fmt.Fprintln(file, line)
			}
			if p.onLine != nil {
				p.onLine(line)
			}
		}
		if file != nil {
			file.Close()
		}
	}()
	go func() {
		err := cmd.Wait()
		pw.Close()
		cancel()
		p.mu.Lock()
		p.running = false
		p.exit = "정상 종료"
		if err != nil {
			p.exit = err.Error()
		}
		p.mu.Unlock()
		p.log.add(fmt.Sprintf("── %s 끝: %s", time.Now().Format("15:04:05"), p.exit))
	}()
	return nil
}

func (lb *lab) stop(p *proc) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running && p.cancel != nil {
		p.cancel()
	}
}

// startScene 은 장면의 부하를 모두 띄웁니다. 돌고 있는 부하가 있으면 거절합니다(먼저 멈춘다).
func (lb *lab) startScene(id string) error {
	i := slices.IndexFunc(scenes, func(s scene) bool { return s.ID == id })
	if i < 0 {
		return fmt.Errorf("없는 장면 %q", id)
	}
	lb.mu.Lock()
	for name, p := range lb.procs {
		if strings.HasPrefix(name, "load:") {
			if p.view().Running {
				lb.mu.Unlock()
				return fmt.Errorf("부하 %s 가 %w", name, errRunning)
			}
			delete(lb.procs, name)
		}
	}
	lb.loads = map[string]loadStat{}
	var started []*proc
	for _, l := range scenes[i].Loads {
		args := append([]string(nil), l.args...)
		for j := range args {
			args[j] = strings.ReplaceAll(args[j], "{P}", lb.proxyAddr)
		}
		args = append(args, "-label", l.label)
		p := lb.newProc("load:"+l.label, "npgload", args...)
		label := l.label
		p.onLine = func(line string) {
			if lbl, st, ok := parseLoadLine(line); ok && lbl == label {
				st.at = time.Now()
				lb.mu.Lock()
				lb.loads[label] = st
				lb.mu.Unlock()
			}
		}
		lb.procs[p.name] = p
		started = append(started, p)
	}
	lb.mu.Unlock()
	for _, p := range started {
		if err := lb.start(p); err != nil {
			return err
		}
	}
	return nil
}

func (lb *lab) stopLoads() {
	for _, p := range lb.list() {
		if strings.HasPrefix(p.name, "load:") {
			lb.stop(p)
		}
	}
}

// loadRaw 는 최근 2.5초 안에 들어온 부하 1초 줄을 "load.<label>.<값>" 과 합계 "load.all.<값>" 으로 펼칩니다.
func (lb *lab) loadRaw(now time.Time, out map[string]float64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	var all loadStat
	fresh := false
	for label, st := range lb.loads {
		if now.Sub(st.at) > 2500*time.Millisecond {
			continue
		}
		fresh = true
		k := "load." + label + "."
		out[k+"inflight"] = st.inflight
		out[k+"n"] = st.n
		if !math.IsNaN(st.p50) {
			out[k+"p50"] = st.p50
			out[k+"p99"] = st.p99
		}
		all.inflight += st.inflight
		all.n += st.n
		all.ok += st.ok
		all.c4xx += st.c4xx
		all.c5xx += st.c5xx
		all.other += st.other
	}
	if fresh {
		out["load.all.inflight"] = all.inflight
		out["load.all.n"] = all.n
		out["load.all.ok"] = all.ok
		out["load.all.4xx"] = all.c4xx
		out["load.all.5xx"] = all.c5xx
		out["load.all.other"] = all.other
	}
}

// portRange 는 리눅스 출발 포트 범위입니다. 없으면 0,0.
func portRange() [2]int {
	b, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
	if err != nil {
		return [2]int{}
	}
	f := strings.Fields(string(b))
	if len(f) != 2 {
		return [2]int{}
	}
	lo, _ := strconv.Atoi(f[0])
	hi, _ := strconv.Atoi(f[1])
	return [2]int{lo, hi}
}

type labState struct {
	Lab       bool                `json:"lab"`
	Procs     []procView          `json:"procs"`
	Scenes    []scene             `json:"scenes"`
	Logs      map[string][]string `json:"logs"`
	PortRange [2]int              `json:"port_range"`
}

func (lb *lab) state() labState {
	st := labState{Lab: true, Scenes: scenes, Logs: map[string][]string{}, PortRange: portRange()}
	for _, p := range lb.list() {
		st.Procs = append(st.Procs, p.view())
		st.Logs[p.name] = p.log.tail(40)
		if p.name == "proxy" {
			st.Logs["proxy:warn"] = p.warn.tail(40)
		}
	}
	return st
}

// list 는 프로그램을 이름순으로 돌려줍니다. 맵은 startScene 이 바꾸므로 잠근 채 복사한다.
func (lb *lab) list() []*proc {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	ps := make([]*proc, 0, len(lb.procs))
	for _, p := range lb.procs {
		ps = append(ps, p)
	}
	slices.SortFunc(ps, func(a, b *proc) int { return strings.Compare(a.name, b.name) })
	return ps
}

// wait 는 npgwatch 가 끝날 때 자식들이 SIGINT 로 정리할 시간을 d 까지 줍니다.
func (lb *lab) wait(d time.Duration) {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if !slices.ContainsFunc(lb.list(), func(p *proc) bool { return p.view().Running }) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// routes 는 버튼용 API 를 붙입니다. POST 는 X-NPG 헤더가 있어야 받습니다.
// 다른 사이트의 폼 전송은 이 헤더를 못 붙이고, fetch 로 붙이려면 CORS 사전 요청에서 막힙니다.
func (lb *lab) routes(mux *http.ServeMux) {
	guard := func(h func(r *http.Request) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-NPG") != "1" {
				http.Error(w, "X-NPG 헤더가 필요합니다", http.StatusForbidden)
				return
			}
			if err := h(r); err != nil {
				code := http.StatusBadRequest
				if errors.Is(err, errRunning) {
					code = http.StatusConflict
				}
				http.Error(w, err.Error(), code)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}
	proc := func(r *http.Request) (*proc, error) {
		name := r.PathValue("name")
		if name != "backend" && name != "proxy" {
			return nil, fmt.Errorf("없는 프로그램 %q", name)
		}
		lb.mu.Lock()
		defer lb.mu.Unlock()
		return lb.procs[name], nil
	}
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(lb.state())
	})
	mux.HandleFunc("POST /api/proc/{name}/start", guard(func(r *http.Request) error {
		p, err := proc(r)
		if err != nil {
			return err
		}
		return lb.start(p)
	}))
	mux.HandleFunc("POST /api/proc/{name}/stop", guard(func(r *http.Request) error {
		p, err := proc(r)
		if err != nil {
			return err
		}
		lb.stop(p)
		return nil
	}))
	mux.HandleFunc("POST /api/scene/{id}/start", guard(func(r *http.Request) error {
		return lb.startScene(r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/load/stop", guard(func(*http.Request) error {
		lb.stopLoads()
		return nil
	}))
}
