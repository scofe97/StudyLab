// upstream 은 외부 환율 제공자를 흉내 내는 가짜 API 를 띄운다.
//
//	go run ./ch07-10-service/cmd/upstream -addr 127.0.0.1:9090
//	curl -X PUT 'http://127.0.0.1:9090/_faults?latency=2s'
//	curl http://127.0.0.1:9090/_stats
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scofe97/runners-lab/net/npg/chapters/ch07-10-service/internal/upstream"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9090", "들을 주소")
	report := flag.Duration("report", time.Second, "보고 줄 간격 (0 이면 끔)")
	faults := flag.String("faults", "", "시작할 때 켤 장애, PUT /_faults 와 같은 형식 (예: 'latency=2s&error_rate=0.1')")
	idle := flag.Duration("idle-timeout", 0, "http.Server.IdleTimeout (0 이면 Go 기본 동작)")
	flag.Parse()

	up := upstream.New()
	if *faults != "" {
		q, err := url.ParseQuery(*faults)
		if err != nil {
			log.Fatalf("-faults: %v", err)
		}
		f, err := up.Faults().Apply(q)
		if err != nil {
			log.Fatalf("-faults: %v", err)
		}
		up.SetFaults(f)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{IdleTimeout: *idle}
	up.Configure(srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *report > 0 {
		go up.Report(ctx, os.Stdout, *report)
	}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	fmt.Printf("upstream listening on %s faults=%v\n", ln.Addr(), up.Faults().View())
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
