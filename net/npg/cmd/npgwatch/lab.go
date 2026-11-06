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
	"syscall"
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

func isWarnLine(line string) bool {
	return strings.Contains(line, " WRN ") || strings.Contains(line, " ERR ")
}

// finished 는 프록시 접근 로그 한 줄에서 읽은 끝난 요청 하나입니다. 산점도의 점 하나가 됩니다.
type finished struct {
	T      int64   `json:"t"` // 유닉스 밀리초. 줄에 찍힌 시각 대신 npgwatch 가 줄을 읽은 시각을 쓴다(자정 넘김을 신경 쓰지 않으려고)
	Method string  `json:"m"`
	Status int     `json:"s"`
	Flags  string  `json:"f"`
	Dur    float64 `json:"d"` // 밀리초
}

// notable 은 느리거나 실패한 요청인지 봅니다. 평소 요청 속에 묻히기 쉬워 따로 모아 둔다.
func (d finished) notable() bool { return d.Status >= 400 || d.Dur >= 1000 }

// parseAccessLine 은 "09:15:01.530 INF attempts=1 dur=30000.59 flags=UT method=POST path=/v1/devices status=504" 꼴의
// 접근 줄을 읽습니다. method 와 status 가 모두 있어야 접근 줄이고, WRN 같은 다른 줄은 ok=false 입니다.
func parseAccessLine(line string, now time.Time) (d finished, ok bool) {
	var hasMethod, hasStatus bool
	for _, tok := range strings.Fields(line) {
		k, v, found := strings.Cut(tok, "=")
		if !found {
			continue
		}
		switch k {
		case "method":
			d.Method, hasMethod = v, v != ""
		case "status":
			n, err := strconv.Atoi(v)
			if err != nil {
				return finished{}, false
			}
			d.Status, hasStatus = n, true
		case "flags":
			d.Flags = v
		case "dur":
			d.Dur, _ = strconv.ParseFloat(v, 64)
		}
	}
	if !hasMethod || !hasStatus {
		return finished{}, false
	}
	d.T = now.UnixMilli()
	return d, true
}

// 끝난 요청 보관량. GET 이 초당 50 개면 3000 개는 1분이면 밀려나 5분 창에 드문 POST 가 남지 않는다.
// 그래서 드문 것(GET 이 아니거나 느리거나 실패)은 따로 3000 개까지 모두 들고, 평범한 GET 은 10 개에 1 개만 1500 개까지 들고 있다.
const (
	finRare   = 3000
	finCommon = 1500
	finSample = 10
	finKeep   = 5 * time.Minute
)

type finishedLog struct {
	mu           sync.Mutex
	rare, common []finished
	seen         int
}

func appendCap(s []finished, d finished, n int) []finished {
	if len(s) >= n {
		s = s[1:]
	}
	return append(s, d)
}

func (l *finishedLog) add(d finished) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d.Method != http.MethodGet || d.notable() {
		l.rare = appendCap(l.rare, d, finRare)
		return
	}
	l.seen++
	if l.seen%finSample == 0 {
		l.common = appendCap(l.common, d, finCommon)
	}
}

// snapshot 은 최근 finKeep 안의 요청을 시각순으로 돌려줍니다.
func (l *finishedLog) snapshot(now time.Time) []finished {
	l.mu.Lock()
	defer l.mu.Unlock()
	from := now.Add(-finKeep).UnixMilli()
	out := make([]finished, 0, len(l.rare)+len(l.common))
	for _, d := range slices.Concat(l.rare, l.common) {
		if d.T >= from {
			out = append(out, d)
		}
	}
	slices.SortStableFunc(out, func(a, b finished) int { return int(a.T - b.T) })
	return out
}

// proxyLine 은 프록시 출력 한 줄을 읽어 끝난 요청 기록(access=true 일 때)과 「실패·느린 줄」 창에 넣을지를 정합니다.
func proxyLine(line string, now time.Time) (d finished, access, keep bool) {
	if d, access = parseAccessLine(line, now); access {
		return d, true, d.notable()
	}
	return d, false, isWarnLine(line)
}

// loadStat 은 npgload 1초 줄 하나입니다. 지연은 밀리초, 그 1초에 끝난 요청이 없으면 p50·p99 는 NaN.
type loadStat struct {
	at                    time.Time
	inflight, n           float64
	ok, c4xx, c5xx, other float64
	p50, p99              float64
	kinds                 map[string]float64 // 상태 코드 없는 실패의 종류별 수: refused·reset·eof·timeout·body_* 등
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
			if st.kinds == nil {
				st.kinds = map[string]float64{}
			}
			st.kinds[k] += c
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
	slow   *lineLog // 프록시만: 실패(4xx 이상)·1초 넘게 걸린 접근 줄 + WRN·ERR 줄
	onLine func(string)
	// waitDelay 는 멈춤 신호 뒤 강제로 죽이기까지 기다리는 시간이다. 0 이면 3초. 드레인이 있는 프록시는 그만큼 늘린다.
	waitDelay time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	cmd     *exec.Cmd
	done    chan struct{} // 프로그램이 끝나고 running 이 false 가 된 뒤 닫힌다
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
	Edge  bool        `json:"-"` // true 면 -caddy-dir 를 준 때만 보이고 돌릴 수 있다
	// Proxy2 가 true 면 -proxy2-addr 까지 준 때만 보이고 돌릴 수 있다(E4: npgproxy 두 대가 있어야 한다)
	Proxy2 bool `json:"-"`
}

// errGroup 는 에러 종류를 그래프의 갈래 넷으로 묶습니다. body_ 는 헤더는 받고 본문을 읽다 난 같은 실패입니다.
func errGroup(kind string) string {
	switch strings.TrimPrefix(kind, "body_") {
	case "refused":
		return "refused"
	case "reset":
		return "reset"
	case "eof":
		return "eof"
	}
	return "etc"
}

type sceneLoad struct {
	label string
	args  []string // -url 의 {P} 는 프록시 주소, {E} 는 엣지 주소로 바뀐다
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
	{ID: "j1", Title: "J1: 단건 조회 초당 50 + 장비 등록 초당 2 (등록 몰림 시간대)", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "50", "-poisson", "-d", "10m"}},
		{"reg", []string{"-url", "http://{P}/v1/devices", "-method", "POST", "-H", "Content-Type: application/json",
			"-body", `{"serial":"SN-%d"}`, "-body-every", "2s", "-rate", "2", "-poisson", "-d", "10m"}},
	}},
	// 검색을 초당 0.5 로 둔다. 검색 칸(8+대기 7)을 넘지 않아야 UO(503)가 섞이지 않는다.
	{ID: "j2", Title: "J2: 단건 조회 초당 50 + 검색 초당 0.5", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "50", "-poisson", "-d", "10m"}},
		{"search", []string{"-url", "http://{P}/v1/devices?q=edge", "-rate", "0.5", "-poisson", "-d", "10m"}},
	}},
	// 10장: 손님 → Caddy → npgproxy. 엣지 칸의 버튼으로 재시작·설정 교체를 하면서 본다.
	{ID: "e1", Title: "E1: 엣지 경유 단건 조회 동시 10, 60초", Edge: true, Loads: []sceneLoad{
		{"get", []string{"-url", "http://{E}/v1/devices/7", "-c", "10", "-d", "60s"}},
	}},
	// E4: 판 버튼 → 부하 시작 → 몇 초 뒤 [프록시 1 재시작(드레인)] 순서로 쓴다. 20초 안에 재시작이 한 번 끝난다(드레인 5초 + 다시 뜨기).
	{ID: "e4", Title: "E4: 엣지 경유 단건 조회 동시 10, 20초", Edge: true, Proxy2: true, Loads: []sceneLoad{
		{"get", []string{"-url", "http://{E}/v1/devices/7", "-c", "10", "-d", "20s"}},
	}},
	// 배포는 화면의 프록시 [정지] → [시작] 으로 한다. 정지 버튼은 SIGINT 를 보낸다(SIGTERM 과 같은 길로 처리된다).
	{ID: "j4", Title: "J4: 단건 조회 초당 50 + 장비 등록 초당 2 (배포 시간대)", Loads: []sceneLoad{
		{"get", []string{"-url", "http://{P}/v1/devices/7", "-rate", "50", "-poisson", "-d", "10m"}},
		{"reg", []string{"-url", "http://{P}/v1/devices", "-method", "POST", "-H", "Content-Type: application/json",
			"-body", `{"serial":"SN-%d"}`, "-rate", "2", "-poisson", "-d", "10m"}},
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
	edge        *edge // nil 이면 엣지 칸이 꺼진 것
	proxy2      bool  // true 면 두 번째 npgproxy 와 E4 버튼이 켜진 것
	drainDelay  time.Duration

	mu    sync.Mutex
	procs map[string]*proc
	loads map[string]loadStat
	done  finishedLog // 프록시 접근 줄에서 읽은 끝난 요청
}

func newLab(ctx context.Context, bin, suffix, backendAddr, proxyAddr, adminAddr, logDir string) *lab {
	lb := &lab{ctx: ctx, bin: bin, suffix: suffix, backendAddr: backendAddr, proxyAddr: proxyAddr,
		adminAddr: adminAddr, logDir: logDir, procs: map[string]*proc{}, loads: map[string]loadStat{}}
	lb.procs["backend"] = lb.newProc("backend", "fakeinv", "-addr", backendAddr)
	proxy := lb.newProc("proxy", "npgproxy", "-addr", proxyAddr, "-admin", adminAddr, "-upstream", "http://"+backendAddr)
	proxy.onLine = lb.accessHook(proxy)
	lb.procs["proxy"] = proxy
	return lb
}

// accessHook 은 프록시 출력 줄을 끝난 요청 기록과 「실패·느린 줄」 창으로 흘리는 함수를 만든다.
func (lb *lab) accessHook(p *proc) func(string) {
	return func(line string) {
		d, access, keep := proxyLine(line, time.Now())
		if access {
			lb.done.add(d)
		}
		if keep {
			p.slow.add(line)
		}
	}
}

// enableProxy2 는 두 번째 npgproxy(E4)를 켭니다. addr 이 비면 켜지 않습니다.
// drain 이 0 보다 크면 두 npgproxy 에 -drain-delay 로 넘기고, 멈춤 때 강제로 죽이기까지의 시간도 그만큼 늘립니다.
// proxy2 를 끈 채로는 -drain-delay 를 넘기지 않아 9장의 즉시 종료 동작이 그대로입니다.
func (lb *lab) enableProxy2(addr, admin string, drain time.Duration) {
	if addr == "" {
		return
	}
	lb.proxy2, lb.drainDelay = true, drain
	p1 := lb.procs["proxy"]
	p2 := lb.newProc("proxy2", "npgproxy", "-addr", addr, "-admin", admin, "-upstream", "http://"+lb.backendAddr)
	p2.onLine = lb.accessHook(p2)
	if drain > 0 {
		for _, p := range []*proc{p1, p2} {
			p.args = append(p.args, "-drain-delay", drain.String())
			p.waitDelay = drain + 4*time.Second
		}
	}
	lb.procs["proxy2"] = p2
	lb.wireEdge()
}

// wireEdge 는 엣지 칸의 「프록시 1 재시작」 버튼이 부를 함수를 건다. 엣지와 proxy2 가 모두 켜진 뒤에만 의미가 있다.
func (lb *lab) wireEdge() {
	if lb.edge == nil || !lb.proxy2 {
		return
	}
	lb.edge.restartProxy1 = func(ctx context.Context) error {
		return lb.restart(ctx, lb.procs["proxy"], proxyRestartLimit)
	}
}

// proxyRestartLimit 은 SIGTERM 뒤 프로그램이 끝나기를 기다리는 최대 시간이다. 드레인 5초 + 종료 대기 20초보다 길다.
const proxyRestartLimit = 40 * time.Second

// enableEdge 는 엣지(Caddy) 칸을 켭니다. dir 이 비면 켜지 않습니다.
func (lb *lab) enableEdge(dir, addr, config string) {
	if dir != "" {
		lb.edge = newEdge(dir, addr, config)
		lb.wireEdge()
	}
}

func (lb *lab) newProc(name, binName string, args ...string) *proc {
	return &proc{name: name, args: append([]string{filepath.Join(lb.bin, binName+lb.suffix)}, args...),
		log: &lineLog{n: 400}, warn: &lineLog{n: 200}, slow: &lineLog{n: 200}}
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
	if p.waitDelay > 0 {
		cmd.WaitDelay = p.waitDelay
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	// 이어 쓴다. 다시 띄울 때 지우면 재시작(배포) 직전 줄이 사라진다.
	var file *os.File
	if lb.logDir != "" {
		file, _ = os.OpenFile(filepath.Join(lb.logDir, "npglab-"+strings.ReplaceAll(p.name, ":", "-")+".log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	}
	done := make(chan struct{})
	p.cancel, p.cmd, p.done, p.running, p.pid, p.since, p.exit = cancel, cmd, done, true, cmd.Process.Pid, time.Now(), ""
	p.log.add(fmt.Sprintf("── %s 시작: %s", p.since.Format("15:04:05"), strings.Join(p.args, " ")))
	if file != nil {
		fmt.Fprintf(file, "── %s 시작 pid=%d\n", p.since.Format("15:04:05"), p.pid)
	}

	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := stripANSI(sc.Text())
			p.log.add(line)
			if isWarnLine(line) {
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
		// 정지 버튼으로 끈 프로그램은 exit 0 이어도 Wait 가 context canceled 를 돌려준다(os/exec Cmd.Cancel 설명).
		// 그 글자 대신 프로그램이 실제로 남긴 종료 코드를 보인다. 배포 도구가 보는 값이 이것이다.
		p.exit = "정상 종료"
		if st := cmd.ProcessState; st != nil {
			p.exit = st.String() // "exit status 0", "signal: killed" 꼴
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			p.exit += " (" + err.Error() + ")"
		}
		exit := p.exit // 잠근 채 복사한다. 재시작이 곧바로 start 로 p.exit 를 다시 쓴다
		p.mu.Unlock()
		p.log.add(fmt.Sprintf("── %s 끝: %s", time.Now().Format("15:04:05"), exit))
		close(done) // 끝 줄을 남긴 뒤에 알려, 재시작이 새로 쓰는 시작 줄이 끝 줄보다 앞서지 않게 한다
	}()
	return nil
}

// restart 는 프로그램에 SIGTERM 을 보내 스스로 내려가게 하고, 끝날 때까지 기다린 뒤 다시 띄웁니다(배포 때 한 대씩 재시작하는 모양).
// 멈춤 버튼의 SIGINT 와 달리 SIGTERM 은 쿠버네티스·systemd 가 배포 때 보내는 신호입니다. limit 안에 안 끝나면 오류입니다.
func (lb *lab) restart(ctx context.Context, p *proc, limit time.Duration) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("%s 가 떠 있지 않습니다", p.name)
	}
	cmd, done := p.cmd, p.done
	p.mu.Unlock()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case <-done:
	case <-ctx.Done():
		return errors.New("중단됨")
	case <-time.After(limit):
		return fmt.Errorf("%s 가 %s 안에 끝나지 않았습니다", p.name, limit)
	}
	return lb.start(p)
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
	if scenes[i].Edge && lb.edge == nil {
		return fmt.Errorf("장면 %q 는 -caddy-dir 가 있어야 합니다", id)
	}
	if scenes[i].Proxy2 && !lb.proxy2 {
		return fmt.Errorf("장면 %q 는 -proxy2-addr 가 있어야 합니다", id)
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
			if lb.edge != nil {
				args[j] = strings.ReplaceAll(args[j], "{E}", lb.edge.addr)
			}
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
	errs := map[string]float64{}
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
		for kind, c := range st.kinds {
			errs[errGroup(kind)] += c
		}
	}
	if fresh {
		out["load.all.inflight"] = all.inflight
		out["load.all.n"] = all.n
		out["load.all.ok"] = all.ok
		out["load.all.4xx"] = all.c4xx
		out["load.all.5xx"] = all.c5xx
		out["load.all.other"] = all.other
		// 에러 갈래 그래프용: 상태 코드 없는 실패는 종류별로, HTTP 5xx 는 따로(502·503 이 E4 의 주인공), HTTP 4xx 와 나머지 종류는 etc 로 묶는다
		out["load.all.err.5xx"] = all.c5xx
		out["load.all.err.refused"] = errs["refused"]
		out["load.all.err.reset"] = errs["reset"]
		out["load.all.err.eof"] = errs["eof"]
		out["load.all.err.etc"] = errs["etc"] + all.c4xx
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
	Edge      *edgeView           `json:"edge,omitempty"`
	Proxy2    bool                `json:"proxy2,omitempty"`
}

func (lb *lab) state() labState {
	st := labState{Lab: true, Logs: map[string][]string{}, PortRange: portRange(), Proxy2: lb.proxy2}
	for _, s := range scenes {
		if (!s.Edge || lb.edge != nil) && (!s.Proxy2 || lb.proxy2) {
			st.Scenes = append(st.Scenes, s)
		}
	}
	if lb.edge != nil {
		v := lb.edge.view()
		st.Edge = &v
	}
	for _, p := range lb.list() {
		st.Procs = append(st.Procs, p.view())
		st.Logs[p.name] = p.log.tail(40)
		if p.name == "proxy" {
			st.Logs["proxy:warn"] = p.warn.tail(40)
			st.Logs["proxy:slow"] = p.slow.tail(40)
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
		if name != "backend" && name != "proxy" && name != "proxy2" {
			return nil, fmt.Errorf("없는 프로그램 %q", name)
		}
		lb.mu.Lock()
		defer lb.mu.Unlock()
		p := lb.procs[name]
		if p == nil { // proxy2 를 켜지 않았다
			return nil, fmt.Errorf("꺼져 있는 프로그램 %q", name)
		}
		return p, nil
	}
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(lb.state())
	})
	mux.HandleFunc("GET /api/finished", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(lb.done.snapshot(time.Now()))
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
	mux.HandleFunc("POST /api/edge/{action}", guard(func(r *http.Request) error {
		if lb.edge == nil {
			return errors.New("엣지 칸이 꺼져 있습니다 (-caddy-dir)")
		}
		action := r.PathValue("action")
		if e4Actions[action] && !lb.proxy2 {
			return errors.New("E4 버튼은 -proxy2-addr 가 있어야 합니다")
		}
		return lb.edge.do(lb.ctx, action)
	}))
	mux.HandleFunc("POST /api/load/stop", guard(func(*http.Request) error {
		lb.stopLoads()
		return nil
	}))
}
