// Package controller 는 클러스터 상태를 지켜보다 노드의 커널 상태를 맞춘다.
//
// 여기가 goroutine·channel·context 가 처음으로 필연이 되는 자리다.
// watch 는 끝나지 않는 루프이고, 종료 신호를 받으면 정리하고 멈춰야 한다.
//
// F-07 의 책임은 하나다 — 노드 목록을 보고 다른 노드 podCIDR 로 가는 경로를 유지한다.
// Service·정책은 F-09 이후에 같은 골격 위로 얹는다. 지금 그 인터페이스를 미리 잡지 않는 이유는,
// F-04~F-06 을 만들기 전에 잡은 경계는 대개 틀리기 때문이다.
//
// client-go 를 아직 쓰지 않는다. Source 인터페이스로 경계를 세워 두면
// 테스트에서 가짜를 끼워 넣을 수 있고, F-07 에서 informer 로 채운다.
package controller

import (
	"context"
	"errors"
	"net"
	"time"
)

var ErrTODO = errors.New("podwire: 구현 전")

// Node 는 경로를 놓는 데 필요한 최소 정보다.
// client-go 의 corev1.Node 를 그대로 쓰지 않는 이유는, 이 패키지가
// 쿠버네티스 타입에 묶이면 가짜 Source 를 만들기 어려워지기 때문이다.
type Node struct {
	Name    string
	PodCIDR *net.IPNet // 이 노드가 Pod 에 나눠 주는 대역
	NodeIP  net.IP     // 그 대역으로 가려면 넘겨야 할 next-hop
	IsSelf  bool       // 내 노드면 경로를 놓지 않는다
}

// Source 는 클러스터에서 원하는 상태를 읽어 오는 곳이다.
// F-07 에서 client-go informer 로 채운다.
type Source interface {
	// Nodes 는 현재 노드 전체를 돌려준다.
	Nodes(ctx context.Context) ([]Node, error)
	// Changed 는 무언가 바뀌었을 때 신호를 보내는 채널이다.
	// 무엇이 바뀌었는지는 싣지 않는다 — 어차피 전체를 다시 읽어 통째로 맞추기 때문이다.
	Changed() <-chan struct{}
}

// Router 는 노드 경로를 실제로 놓고 지우는 쪽이다.
// internal/link 가 이걸 구현한다. 인터페이스로 끊어 두면
// 커널을 만지지 않고 reconcile 로직만 테스트할 수 있다.
type Router interface {
	// ListPodRoutes 는 podwire 가 놓은 경로만 돌려준다.
	// 남이 놓은 경로를 지우지 않으려면 내 것을 구분할 수 있어야 한다.
	ListPodRoutes() ([]Node, error)
	// AddPodRoute 는 dst 대역으로 가는 경로를 gw 로 놓는다.
	AddPodRoute(dst *net.IPNet, gw net.IP) error
	// DelPodRoute 는 그 경로를 지운다.
	DelPodRoute(dst *net.IPNet) error
}

// Options 는 컨트롤러 동작을 조절한다.
type Options struct {
	// ResyncPeriod 는 아무 이벤트가 없어도 강제로 다시 맞추는 주기다.
	// 이벤트를 놓쳤거나 누가 손으로 경로를 지웠을 때 스스로 회복하는 안전망이다.
	ResyncPeriod time.Duration
	// MinInterval 은 이벤트가 몰아칠 때 최소 간격이다.
	// 노드 여럿이 동시에 붙으면 이벤트가 여러 번 오는데 매번 전체 반영하면 낭비다.
	MinInterval time.Duration
}

// DefaultOptions 는 무난한 값이다. 실측 후 조정한다.
func DefaultOptions() Options {
	return Options{ResyncPeriod: 30 * time.Second, MinInterval: 100 * time.Millisecond}
}

// Controller 는 Source 를 보고 Router 에 반영한다.
type Controller struct {
	src  Source
	rt   Router
	opts Options
}

// New 는 컨트롤러를 만든다.
func New(src Source, rt Router, opts Options) *Controller {
	return &Controller{src: src, rt: rt, opts: opts}
}

// Run 은 ctx 가 끝날 때까지 조정 루프를 돈다.
//
// 구조:
//
//	for {
//	  select {
//	  case <-ctx.Done():        정리하고 반환
//	  case <-src.Changed():     디바운스 후 reconcile
//	  case <-ticker.C:          주기적 강제 reconcile
//	  }
//	}
//
// TODO(F-07)
func (c *Controller) Run(ctx context.Context) error { return ErrTODO }

// reconcile 은 한 번의 조정이다.
//
// 원하는 상태(Source 의 노드 목록)와 실제 상태(Router 가 아는 경로)를 대조해
// 없는 것은 놓고 남는 것은 지운다. 이벤트가 "무엇이 바뀌었나"를 싣지 않는 이유가 여기 있다 —
// 전체를 대조하면 이벤트를 놓쳐도 다음 주기에 회복된다.
//
// 실패해도 루프는 멈추지 않는다. 다음 주기에 다시 시도하면 되기 때문이다.
//
// TODO(F-07)
func (c *Controller) reconcile(ctx context.Context) error { return ErrTODO }
