// adminctl 은 api 의 관리 소켓에 HTTP 로 붙는다.
//
//	adminctl -socket /tmp/npgsvc/api-8081.sock status
//	adminctl -socket … drain | peer | gc | log-level [debug]
//	adminctl -socket … watch -every 1s     같은 클라이언트로 GET /status 를 되풀이한다
//
// http.Transport 의 DialContext 를 바꿔 TCP 대신 unix 소켓으로 연결한다.
// URL 의 호스트 이름(admin)은 Host 헤더에만 쓰이고 연결 대상과는 상관이 없다.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	socket := flag.String("socket", "/tmp/npgsvc/api-8081.sock", "관리 소켓 경로")
	every := flag.Duration("every", time.Second, "watch 간격")
	count := flag.Int("n", 0, "watch 횟수 (0 이면 Ctrl-C 까지)")
	timeout := flag.Duration("timeout", 3*time.Second, "요청 하나의 시한")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: adminctl [flags] status|drain|peer|gc|log-level [level]|watch\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", *socket)
		},
		MaxIdleConnsPerHost: 1,
	}
	hc := &http.Client{Transport: tr, Timeout: *timeout}

	cmd, args := flag.Arg(0), flag.Args()[1:]
	var method, path string
	switch cmd {
	case "status":
		method, path = http.MethodGet, "/status"
	case "drain":
		method, path = http.MethodPost, "/drain"
	case "peer":
		method, path = http.MethodGet, "/peer"
	case "gc":
		method, path = http.MethodPost, "/gc"
	case "log-level":
		method, path = http.MethodGet, "/log-level"
		if len(args) > 0 {
			method, path = http.MethodPut, "/log-level?level="+args[0]
		}
	case "watch":
		watch(hc, *every, *count)
		return
	default:
		flag.Usage()
		os.Exit(2)
	}

	req, _ := http.NewRequest(method, "http://admin"+path, nil)
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "adminctl:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
	io.Copy(os.Stdout, resp.Body)
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

// watch 는 같은 http.Client 로 GET /status 를 되풀이하고, 매번 연결을 새로 맺었는지 찍는다.
func watch(hc *http.Client, every time.Duration, n int) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	t := time.NewTicker(every)
	defer t.Stop()
	for i := 1; n == 0 || i <= n; i++ {
		var reused bool
		var local string
		ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				reused = info.Reused
				local = fmt.Sprintf("%p", info.Conn)
			},
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://admin/status", nil)
		start := time.Now()
		resp, err := hc.Do(req)
		line := fmt.Sprintf("%s #%d", time.Now().Format("15:04:05"), i)
		if err != nil {
			fmt.Printf("%s err=%v\n", line, err)
		} else {
			var v map[string]any
			dec := json.NewDecoder(resp.Body)
			dec.UseNumber()
			dec.Decode(&v)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			fmt.Printf("%s %s reused=%v conn=%s pid=%v fds=%v uptime=%v (%s)\n", line, strings.Fields(resp.Status)[0],
				reused, local, v["pid"], v["fds"], v["uptime"], time.Since(start).Truncate(time.Microsecond))
		}
		select {
		case <-t.C:
		case <-stop:
			return
		}
	}
}
