// phase2-loss 는 번호 붙은 메시지 n 개를 UDP echo 와 TCP echo 로 각각 보내고,
// 돌아온 수·빠진 번호·순서 뒤바뀜·걸린 시간을 비교해 찍는다.
// netem 으로 lo 에 손실·재정렬을 건 상태에서 두 프로토콜이 어떻게 달리 무너지는지 보는 용도다.
//
//	gonet udp-listen :8080  와  gonet listen :9000  을 먼저 띄운다.
//	phase2-loss -udp 127.0.0.1:8080 -tcp 127.0.0.1:9000 -n 100
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	udpAddr := flag.String("udp", "127.0.0.1:8080", "UDP echo 서버 주소 (빈 값이면 건너뜀)")
	tcpAddr := flag.String("tcp", "127.0.0.1:9000", "TCP echo 서버 주소 (빈 값이면 건너뜀)")
	n := flag.Int("n", 100, "보낼 메시지 수")
	gap := flag.Duration("gap", time.Millisecond, "메시지 사이 간격 (수신 버퍼 넘침이 아니라 netem 손실만 보려고 둔다)")
	wait := flag.Duration("wait", 2*time.Second, "UDP: 마지막 전송 뒤 echo 를 기다리는 시간")
	flag.Parse()

	if *udpAddr != "" {
		if err := runUDP(*udpAddr, *n, *gap, *wait); err != nil {
			fmt.Fprintln(os.Stderr, "udp:", err)
			os.Exit(1)
		}
	}
	if *tcpAddr != "" {
		if err := runTCP(*tcpAddr, *n, *gap); err != nil {
			fmt.Fprintln(os.Stderr, "tcp:", err)
			os.Exit(1)
		}
	}
}

func runUDP(addr string, n int, gap, wait time.Duration) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	start := time.Now()
	// 받기는 따로 돌린다. 보내는 동안에도 echo 가 돌아오기 때문이다.
	type result struct {
		seqs []int
		last time.Time
	}
	got := make(chan result, 1)
	go func() {
		var res result
		buf := make([]byte, 64*1024)
		for {
			// 마지막 전송이 끝난 뒤 wait 가 지나면 멈춘다. 전송 중에는 deadline 을 계속 민다.
			conn.SetReadDeadline(time.Now().Add(wait + time.Duration(n)*gap))
			r, err := conn.Read(buf)
			if err != nil {
				got <- res
				return
			}
			if v, err := strconv.Atoi(strings.TrimPrefix(string(buf[:r]), "seq=")); err == nil {
				res.seqs = append(res.seqs, v)
				res.last = time.Now()
			}
		}
	}()
	for i := 1; i <= n; i++ {
		if _, err := fmt.Fprintf(conn, "seq=%d", i); err != nil {
			return err
		}
		time.Sleep(gap)
	}
	res := <-got
	// UDP 는 끝을 알리는 신호가 없어 wait 만큼 더 기다렸다. 걸린 시간은 마지막 echo 를 받은 시각까지로 잰다.
	elapsed := time.Duration(0)
	if !res.last.IsZero() {
		elapsed = res.last.Sub(start)
	}
	report("udp", n, res.seqs, elapsed)
	return nil
}

func runTCP(addr string, n int, gap time.Duration) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	start := time.Now()
	go func() {
		for i := 1; i <= n; i++ {
			// TCP 는 경계가 없으므로 줄바꿈으로 프레이밍한다.
			fmt.Fprintf(conn, "seq=%d\n", i)
			time.Sleep(gap)
		}
	}()
	var seqs []int
	sc := bufio.NewScanner(conn)
	// 손실이 심하면 재전송이 길어진다. 60 초 안에 다 못 받으면 받은 만큼만 보고한다.
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	for len(seqs) < n && sc.Scan() {
		if v, err := strconv.Atoi(strings.TrimPrefix(sc.Text(), "seq=")); err == nil {
			seqs = append(seqs, v)
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	report("tcp", n, seqs, time.Since(start))
	return nil
}

// report 는 받은 번호 목록에서 개수·빠진 번호·순서가 앞 번호보다 작게 온 횟수를 찍는다.
func report(proto string, n int, seqs []int, elapsed time.Duration) {
	seen := make(map[int]bool, len(seqs))
	reordered, max := 0, 0
	for _, v := range seqs {
		seen[v] = true
		if v < max {
			reordered++
		}
		if v > max {
			max = v
		}
	}
	var missing []int
	for i := 1; i <= n; i++ {
		if !seen[i] {
			missing = append(missing, i)
		}
	}
	sort.Ints(missing)
	fmt.Printf("%s: sent=%d received=%d missing=%d reordered=%d elapsed=%s\n",
		proto, n, len(seqs), len(missing), reordered, elapsed.Round(time.Millisecond))
	if len(missing) > 0 {
		fmt.Printf("%s: missing seq=%v\n", proto, missing)
	}
}
