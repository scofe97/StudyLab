package scan

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

// RunSYN 은 raw 소켓으로 SYN 하나씩만 보내고 돌아온 플래그로 판정한다 (04-01 §1·§2).
// 연결을 끝까지 맺지 않으므로 대상 서버의 앱은 연결을 받지 못한다. 대상은 자기 소유 호스트로만 한정한다.
//
// net.Dial 과 달리 커널에 연결 기록이 남지 않는다. 그래서 대상이 SYN-ACK 로 답하면
// 스캐너 쪽 커널이 "모르는 연결"로 보고 RST 를 보낸다. 그 RST 는 판정과 무관하다 (04-01 §2).
func RunSYN(ctx context.Context, host string, ports []int, opt Options) ([]Result, error) {
	dstIP, src, err := resolve(host)
	if err != nil {
		return nil, err
	}

	// ip4:tcp raw 소켓. 커널이 IP 헤더를 붙여 주고, 우리는 TCP 헤더 20바이트를 직접 짠다.
	// 열려면 root 이거나 CAP_NET_RAW 가 필요하다. Linux 전용(BSD 는 raw 소켓에 TCP 를 안 넘긴다, 04-01 §2).
	conn, err := net.ListenPacket("ip4:tcp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("raw 소켓 열기 실패(sudo 또는 setcap cap_net_raw+ep 필요): %w", err)
	}
	defer conn.Close()

	srcPort := uint16(30000 + rand.IntN(20000)) // 응답의 목적지가 이 포트여야 우리 것으로 받는다
	baseSeq := rand.Uint32()

	// 아직 판정되지 않은 포트를 추적한다. 응답은 reader 가 여기에 채운다.
	var mu sync.Mutex
	status := make(map[int]Status, len(ports))
	for _, p := range ports {
		status[p] = StatusUnknown
	}
	pending := func() []int {
		mu.Lock()
		defer mu.Unlock()
		var left []int
		for p, s := range status {
			if s == StatusUnknown {
				left = append(left, p)
			}
		}
		return left
	}

	// reader: 같은 소켓으로 돌아온 세그먼트를 읽어 짝이 맞으면 판정한다. ctx 가 끝나면 Read 를 깨운다.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 1500)
		for {
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			port, flags, ok := synReply(buf[:n], srcPort, baseSeq)
			if !ok {
				continue
			}
			mu.Lock()
			if status[port] == StatusUnknown {
				switch {
				case flags&flagSYN != 0 && flags&flagACK != 0:
					status[port] = StatusOpen
				case flags&flagRST != 0:
					status[port] = StatusClosed
				}
			}
			mu.Unlock()
		}
	}()
	stop := context.AfterFunc(ctx, func() { conn.SetReadDeadline(time.Now()) })
	defer stop()

	// 무응답인 포트는 round 마다 다시 SYN 을 보낸다. 무언가 돌아온 포트는 이미 판정됐다 (04-01 §4).
	var tick <-chan time.Time
	if opt.Rate > 0 {
		t := time.NewTicker(time.Second / time.Duration(opt.Rate))
		defer t.Stop()
		tick = t.C
	}
	for round := 0; round <= opt.Retries; round++ {
		for _, port := range pending() {
			if tick != nil {
				select {
				case <-tick:
				case <-ctx.Done():
					goto collect
				}
			}
			seg := synSegment(src, dstIP, srcPort, uint16(port), baseSeq)
			if _, err := conn.WriteTo(seg, &net.IPAddr{IP: dstIP}); err != nil {
				return nil, err
			}
		}
		select {
		case <-time.After(opt.Timeout):
		case <-ctx.Done():
			goto collect
		}
	}

collect:
	conn.SetReadDeadline(time.Now())
	<-readerDone
	mu.Lock()
	defer mu.Unlock()
	results := make([]Result, len(ports))
	for i, p := range ports {
		r := Result{Port: p, Status: status[p]}
		if r.Status == StatusUnknown { // 끝내 아무것도 안 돌아옴 → filtered (04-01 §1)
			r.Status, r.Errno = StatusFiltered, errTimeout
		}
		results[i] = r
	}
	return results, nil
}

// resolve 는 host 의 목적지 IP 와, 커널이 그 목적지로 쓸 출발지 IP 를 돌려준다.
// 출발지 IP 는 체크섬의 pseudo-header 에 필요하다. UDP 소켓을 connect 만 해 커널이 고른 주소를 읽는다.
func resolve(host string) (dst, src net.IP, err error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, nil, err
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			dst = v4
			break
		}
	}
	if dst == nil {
		return nil, nil, fmt.Errorf("IPv4 주소를 찾지 못함: %s", host)
	}
	c, err := net.Dial("udp", net.JoinHostPort(dst.String(), "9")) // 보내지 않는다, 출발지 IP 만 얻는다
	if err != nil {
		return nil, nil, err
	}
	defer c.Close()
	return dst, c.LocalAddr().(*net.UDPAddr).IP.To4(), nil
}
