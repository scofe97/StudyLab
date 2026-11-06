package main

// edge.go 는 -caddy-dir 를 주었을 때만 켜지는 10장 엣지(Caddy) 칸입니다.
// 버튼은 deploy/caddy/run.sh 의 정해진 하위 명령(up·down·load)만 실행합니다. 화면에서 명령 문자열을 받지 않습니다.
// 각 동작의 시작·끝 시각을 남겨 두면 화면이 그래프 위에 띠로 그려, 손님 쪽 실패 구간과 맞춰 볼 수 있습니다.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// edgeRunner 는 run.sh 하위 명령 하나를 돌립니다. 시험에서는 이 자리에 가짜를 끼운다.
type edgeRunner func(ctx context.Context, args ...string) (string, error)

// edgeLabels 는 버튼이 부를 수 있는 동작 전부입니다. 이 밖의 이름은 거절한다.
var edgeLabels = map[string]string{
	"up":      "▶ Caddy 시작",
	"down":    "■ Caddy 멈춤",
	"restart": "↻ 재시작(문 닫고 공사)",
	"load":    "⇄ 설정만 바꾸기(관리 API /load)",
	// E4 (-proxy2-addr 가 있을 때만 쓴다): 판 넷과 프록시 1 재시작
	"board-base":     "판: 기본",
	"board-active":   "판: 능동 검사",
	"board-retry":    "판: 재시도",
	"board-both":     "판: 둘 다",
	"proxy1-restart": "↻ 프록시 1 재시작(드레인)",
}

// e4Config 는 E4 판의 바탕 설정이다. reverse_proxy(@id npg_proxy) 가 npgproxy 두 대로 round_robin 하고, 검사·재시도는 없다.
const e4Config = "scenarios/e4-two-proxies.json"

// e4Pieces 는 판마다 바탕 위에 관리 API 로 올리는 조각이다. 경로는 reverse_proxy 의 @id(npg_proxy) 아래다.
// 조각 파일은 참조 구현(chapters/ch07-10-service/edge/scenarios/e4-*.json)에서 복사했다. 참조는 @id 가 api_proxy 였다.
var e4Pieces = map[string][][2]string{
	"board-base":   nil,
	"board-active": {{"/id/npg_proxy/health_checks", "scenarios/e4-active-health.json"}},
	"board-retry":  {{"/id/npg_proxy/load_balancing", "scenarios/e4-try-duration.json"}},
	"board-both": {
		{"/id/npg_proxy/health_checks", "scenarios/e4-active-health.json"},
		{"/id/npg_proxy/load_balancing", "scenarios/e4-try-duration.json"},
	},
}

// e4Actions 는 -proxy2-addr 없이는 거절할 동작들이다.
var e4Actions = map[string]bool{
	"board-base": true, "board-active": true, "board-retry": true, "board-both": true, "proxy1-restart": true,
}

const edgeKeep = 20 // 화면에 남길 동작 기록 수

type edgeEvent struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Start int64  `json:"start"`         // 유닉스 밀리초
	End   int64  `json:"end,omitempty"` // 0 이면 아직 도는 중
	Err   string `json:"err,omitempty"`
}

type edgeView struct {
	Addr   string      `json:"addr"`
	Up     bool        `json:"up"`
	Busy   string      `json:"busy,omitempty"`
	Board  string      `json:"board,omitempty"` // 지금 올라가 있는 E4 판(board-*), 모르면 빈 값
	Events []edgeEvent `json:"events"`
}

type edge struct {
	dir, addr, config string
	run               edgeRunner
	pause             time.Duration                        // 재시작에서 down 과 up 사이에 쉬는 시간
	reachable         func(addr string) bool               // 엣지 포트에 접속되는지
	now               func() time.Time                     // 시험에서 시각을 고정한다
	sleep             func(context.Context, time.Duration) // 시험에서 건너뛴다
	restartProxy1     func(context.Context) error          // lab 이 건다: npgproxy 1 에 SIGTERM → 끝날 때까지 기다림 → 다시 띄움

	mu     sync.Mutex
	busy   string
	board  string
	events []*edgeEvent
}

func newEdge(dir, addr, config string) *edge {
	e := &edge{dir: dir, addr: addr, config: config, pause: 2 * time.Second, now: time.Now}
	e.run = func(ctx context.Context, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, filepath.Join(dir, "run.sh"), args...).CombinedOutput()
		return string(out), err
	}
	e.reachable = func(addr string) bool {
		c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err != nil {
			return false
		}
		c.Close()
		return true
	}
	e.sleep = func(ctx context.Context, d time.Duration) {
		select {
		case <-ctx.Done():
		case <-time.After(d):
		}
	}
	return e
}

// lastLine 은 run.sh 출력의 마지막 비지 않은 줄입니다. 실패 이유가 보통 거기 있다.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// do 는 동작 kind 를 백그라운드로 시작합니다. 다른 동작이 도는 중이면 errRunning 입니다.
func (e *edge) do(ctx context.Context, kind string) error {
	label, ok := edgeLabels[kind]
	if !ok {
		return fmt.Errorf("없는 엣지 동작 %q", kind)
	}
	e.mu.Lock()
	if e.busy != "" {
		defer e.mu.Unlock()
		return fmt.Errorf("엣지 동작 %s 가 %w", edgeLabels[e.busy], errRunning)
	}
	ev := &edgeEvent{Kind: kind, Label: label, Start: e.now().UnixMilli()}
	e.busy = kind
	e.events = append(e.events, ev)
	if len(e.events) > edgeKeep {
		e.events = e.events[len(e.events)-edgeKeep:]
	}
	e.mu.Unlock()

	go func() {
		err := e.exec(ctx, kind)
		e.mu.Lock()
		defer e.mu.Unlock()
		ev.End = e.now().UnixMilli()
		if err != nil {
			ev.Err = err.Error()
		}
		e.busy = ""
	}()
	return nil
}

// httpMark 는 run.sh 의 관리 API 호출이 stderr 로 남기는 "[HTTP 200]" 표지다. curl 은 4xx·5xx 여도 exit 0 이라 이 표지로 본다.
var httpMark = regexp.MustCompile(`\[HTTP (\d{3})\]`)

func (e *edge) step(ctx context.Context, args ...string) error {
	out, err := e.run(ctx, args...)
	if err == nil && (args[0] == "load" || args[0] == "admin") {
		if m := httpMark.FindStringSubmatch(out); m != nil && m[1][0] != '2' {
			body := strings.TrimSpace(httpMark.ReplaceAllString(out, ""))
			err = fmt.Errorf("관리 API 가 HTTP %s 로 답했습니다", m[1])
			if body != "" {
				err = fmt.Errorf("%w: %s", err, lastLine(body))
			}
			return fmt.Errorf("run.sh %s: %w", args[0], err)
		}
	}
	if err != nil {
		if l := lastLine(out); l != "" {
			return fmt.Errorf("run.sh %s: %s (%w)", args[0], l, err)
		}
		return fmt.Errorf("run.sh %s: %w", args[0], err)
	}
	return nil
}

func (e *edge) setBoard(kind string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.board = kind
}

func (e *edge) exec(ctx context.Context, kind string) error {
	if kind != "proxy1-restart" {
		e.setBoard("") // 설정이 바뀌는 중이거나 바뀐 뒤라 어느 판인지 모른다. 판 버튼이 성공하면 아래에서 다시 적는다
	}
	if pieces, ok := e4Pieces[kind]; ok {
		if err := e.step(ctx, "load", e4Config); err != nil {
			return err
		}
		for _, pc := range pieces {
			if err := e.step(ctx, "admin", "POST", pc[0], pc[1]); err != nil {
				return err
			}
		}
		e.setBoard(kind)
		return nil
	}
	switch kind {
	case "proxy1-restart":
		if e.restartProxy1 == nil {
			return errors.New("프록시 재시작이 연결되지 않았습니다 (-proxy2-addr)")
		}
		return e.restartProxy1(ctx)
	case "up":
		return e.step(ctx, "up", e.config)
	case "down":
		return e.step(ctx, "down")
	case "load":
		return e.step(ctx, "load", e.config)
	case "restart":
		if err := e.step(ctx, "down"); err != nil {
			return err
		}
		e.sleep(ctx, e.pause)
		if ctx.Err() != nil {
			return errors.New("중단됨")
		}
		return e.step(ctx, "up", e.config)
	}
	return fmt.Errorf("없는 엣지 동작 %q", kind)
}

func (e *edge) view() edgeView {
	v := edgeView{Addr: e.addr, Up: e.reachable(e.addr), Events: []edgeEvent{}}
	e.mu.Lock()
	defer e.mu.Unlock()
	v.Busy = e.busy
	v.Board = e.board
	for _, ev := range e.events {
		v.Events = append(v.Events, *ev)
	}
	return v
}
