// pw 는 Stage 0 의 실습 CLI 다.
//
// ip 명령으로 손수 치던 순서를 netlink 호출로 옮긴 것이고,
// 이 프로젝트에서 가장 먼저 돌아가는 프로그램이다.
// CNI 플러그인은 kubelet 이 부르므로 디버깅 루프가 길다 —
// 같은 netlink 코드를 여기서 먼저 손에 익힌 뒤 Stage 1 로 간다.
//
//	sudo pw lab up
//	sudo pw lab verify
//	sudo pw lab down
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/scofe97/runners-lab/net/podwire/internal/lab"
)

const usage = `pw — podwire 실습 도구

사용법:
  pw lab up        실습 토폴로지를 세운다 (netns 2 + veth + br0 + MASQUERADE)
  pw lab down      세운 것을 허문다
  pw lab verify    현재 상태를 읽어 출력한다

리눅스에서 root 로 실행한다. macOS 에서는 동작하지 않는다 —
netns 도 veth 도 리눅스 커널 기능이라 OrbStack VM 안에서 돌린다.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 || args[0] != "lab" {
		flag.Usage()
		os.Exit(2)
	}

	cfg := lab.Default()

	var err error
	switch args[1] {
	case "up":
		err = lab.Up(cfg)
	case "down":
		err = lab.Down(cfg)
	case "verify":
		err = lab.Verify(cfg, os.Stdout)
	default:
		flag.Usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "pw: %v\n", err)
		os.Exit(1)
	}
}
