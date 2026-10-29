package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/scofe97/runners-lab/net/gonet/internal/dns"
)

var dnsTypes = map[string]uint16{"A": dns.TypeA, "CNAME": dns.TypeCNAME, "TXT": dns.TypeTXT, "AAAA": dns.TypeAAAA}

// runDNS 는 gonet dns 를 처리하고 종료 코드를 돌려준다.
func runDNS(args []string) int {
	fs := flag.NewFlagSet("dns", flag.ExitOnError)
	server := fs.String("server", "1.1.1.1:53", "물을 DNS 서버 주소")
	typ := fs.String("type", "A", "A | AAAA | CNAME | TXT")
	tcpOnly := fs.Bool("tcp", false, "UDP 를 건너뛰고 TCP 로만 묻는다")
	flip := fs.String("flip", "", "일부러 빅엔디언을 어길 칸: qdcount | qtype")
	timeout := fs.Duration("timeout", 2*time.Second, "시도 한 번에 기다리는 시간")
	attempts := fs.Int("attempts", 3, "UDP 로 같은 질의를 보내는 최대 횟수")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gonet dns [flags] <name>  (예: gonet dns -type TXT google.com)")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	qtype, ok := dnsTypes[strings.ToUpper(*typ)]
	flips := map[string]dns.Flip{"": dns.FlipNone, "qdcount": dns.FlipQDCount, "qtype": dns.FlipQType}
	f, fok := flips[*flip]
	if fs.NArg() != 1 || !ok || !fok {
		fs.Usage()
		return 2
	}

	// ID 는 무작위로 고른다. 경로 밖의 위조 응답이 ID 를 맞히기 어렵게 하려는 것이다 (RFC 5452).
	q := dns.Query{ID: uint16(rand.Uint32()), Name: fs.Arg(0), Type: qtype, Flip: f}
	sent, err := q.Encode()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ex, err := dns.Resolve(ctx, *server, sent, dns.Options{Timeout: *timeout, Attempts: *attempts, TCPOnly: *tcpOnly})
	printExchange(os.Stdout, *server, ex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// printExchange 는 보낸 질의와 받은 응답을 hex 로 찍고, 풀어낸 헤더·포인터·답을 덧붙인다.
func printExchange(w io.Writer, server string, ex *dns.Exchange) {
	fmt.Fprintf(w, "→ 질의 %d 바이트 → %s\n%s", len(ex.Sent), server, hex.Dump(ex.Sent))
	if ex.Truncated != nil {
		fmt.Fprintf(w, "← udp 응답 %d 바이트, TC=1 이라 TCP 로 다시 묻습니다 (길이 접두 % x)\n%s",
			len(ex.Truncated), binary.BigEndian.AppendUint16(nil, uint16(len(ex.Sent))), hex.Dump(ex.Truncated))
	}
	if ex.Raw == nil {
		fmt.Fprintf(w, "udp 시도 %d 번, 짝이 맞지 않아 버린 데이터그램 %d 개\n", ex.Attempts, ex.Skipped)
		return
	}
	note := ""
	if ex.Via == "tcp" {
		note = fmt.Sprintf(" (길이 접두 % x)", binary.BigEndian.AppendUint16(nil, uint16(len(ex.Raw))))
	}
	fmt.Fprintf(w, "← %s 응답 %d 바이트%s\n%s", ex.Via, len(ex.Raw), note, hex.Dump(ex.Raw))
	fmt.Fprintf(w, "헤더   %s\n", ex.Resp.Header)
	for _, j := range ex.Resp.Jumps {
		fmt.Fprintf(w, "포인터 @%d(0x%02x) → %d(0x%02x)\n", j.At, j.At, j.To, j.To)
	}
	for _, a := range ex.Resp.Answers {
		fmt.Fprintf(w, "답     %s %s ttl=%d %s\n", a.Name, typeName(a.Type), a.TTL, a.Value)
	}
	fmt.Fprintf(w, "udp 시도 %d 번, 짝이 맞지 않아 버린 데이터그램 %d 개\n", ex.Attempts, ex.Skipped)
}

func typeName(t uint16) string {
	for name, v := range dnsTypes {
		if v == t {
			return name
		}
	}
	return fmt.Sprintf("TYPE%d", t)
}
