// podwire 는 CNI 플러그인 바이너리다.
//
// kubelet 이 Pod 을 띄울 때마다 이 프로그램을 새 프로세스로 실행한다.
// 데몬이 아니라 한 번 실행되고 끝나는 프로그램이라는 점이 설계를 결정한다 —
// 상태는 메모리가 아니라 디스크(IPAM 저장소)에 있어야 하고,
// 동시 실행을 파일 잠금으로 막아야 한다.
//
// 손으로 호출해 보려면:
//
//	CNI_COMMAND=ADD CNI_CONTAINERID=test CNI_NETNS=/var/run/netns/test \
//	CNI_IFNAME=eth0 CNI_PATH=/opt/cni/bin \
//	./podwire < deploy/10-podwire.conflist
package main

import (
	"errors"

	"github.com/scofe97/podwire/internal/cni"
)

var errTODO = errors.New("podwire: 구현 전")

func main() {
	cni.Main(cni.Funcs{Add: add, Del: del_, Check: check})
}

// add 는 Pod 하나를 네트워크에 붙인다.
//
// 순서:
//  1. IPAM 에서 주소를 하나 받는다 (containerID 로 멱등하게)
//  2. veth 쌍을 만든다. 호스트 쪽 이름은 충돌하지 않게 containerID 로 짓는다
//  3. 한쪽 끝을 args.Netns 로 옮기고 args.IfName(보통 eth0)으로 이름을 바꾼다
//  4. 호스트 쪽 끝을 노드 브리지에 꽂고 UP
//  5. 컨테이너 안에서 주소를 붙이고 UP, lo 도 UP, 기본 경로를 브리지 주소로
//  6. 만든 것을 Result 로 돌려준다
//
// 실패하면 그때까지 만든 것을 반드시 되돌려야 한다.
// 안 그러면 고아 veth 와 새어 나간 IP 가 쌓이고, 노드가 조용히 주소를 다 써 버린다.
//
// TODO(Stage 1)
func add(args *cni.Args, conf *cni.NetConf) (*cni.Result, error) {
	return nil, errTODO
}

// del_ 은 Pod 을 네트워크에서 뗀다.
//
// 멱등해야 한다. 이미 지워진 것을 지우라는 요청도 성공으로 답한다 —
// 여기서 오류를 내면 Pod 이 Terminating 에 갇힌다.
// 네임스페이스가 이미 사라진 경우(args.Netns 가 빈 문자열이거나 없는 경로)에도
// IPAM 반납은 해야 한다.
//
// TODO(Stage 1)
func del_(args *cni.Args, conf *cni.NetConf) error {
	return errTODO
}

// check 는 현재 배선이 설정과 맞는지 확인한다.
// TODO(Stage 1)
func check(args *cni.Args, conf *cni.NetConf) error {
	return errTODO
}
