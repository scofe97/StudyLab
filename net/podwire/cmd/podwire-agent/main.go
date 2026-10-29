// podwire-agent 는 노드마다 하나씩 도는 데몬이다.
//
// CNI 플러그인이 Pod 하나를 배선하는 단발 프로그램이라면,
// 이쪽은 클러스터 전체를 계속 지켜보며 노드의 커널 상태를 맞춘다.
//
// 지금 맡는 일은 F-07 하나다 — 노드를 watch 해 다른 노드 podCIDR 로 가는 경로를 유지한다.
// F-09 이후로 Service·정책·암호화·프록시가 이 골격 위에 얹힌다.
// 그때 필요한 추상은 그때 만든다. 미리 잡아 둔 경계는 F-04~F-06 을 겪기 전이라 대개 틀린다.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/scofe97/podwire/internal/controller"
)

func main() {
	var (
		nodeName = flag.String("node", "", "이 에이전트가 도는 노드 이름 (downward API 로 주입)")
		podCIDR  = flag.String("pod-cidr", "", "이 노드에 할당된 Pod 대역")
	)
	flag.Parse()

	if *nodeName == "" {
		fmt.Fprintln(os.Stderr, "podwire-agent: --node 가 필요하다")
		os.Exit(1)
	}
	fmt.Printf("podwire-agent: 노드 %s (podCIDR %s) 에서 시작\n", *nodeName, *podCIDR)

	// SIGTERM 을 받으면 ctx 가 끝나고, 컨트롤러가 정리한 뒤 반환한다.
	// kubelet 이 Pod 을 지울 때 보내는 신호라 여기서 정리하지 않으면
	// 에이전트가 사라져도 커널에 경로만 남는다.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// TODO(F-07): client-go informer 로 Source 를, internal/link 로 Router 를 채운다.
	// 지금은 둘 다 nil 이라 Run 이 바로 오류를 낸다.
	var src controller.Source
	var rt controller.Router
	c := controller.New(src, rt, controller.DefaultOptions())

	if err := c.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "podwire-agent: %v\n", err)
		os.Exit(1)
	}
}
