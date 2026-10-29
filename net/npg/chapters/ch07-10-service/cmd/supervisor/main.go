// supervisor 는 systemd 처럼 자식 프로세스를 띄우고 준비 완료 알림을 기다리는 작은 감독 프로세스다.
//
//	supervisor -socket /tmp/npgsvc/notify.sock -health http://127.0.0.1:8081/healthz -- ./bin/linux/api -addr 127.0.0.1:8081 …
//
// 자식에게는 NOTIFY_SOCKET=<socket> 을 넘긴다. 받은 데이터그램과 시각을 찍는다.
// -health 를 주면 자식을 띄운 뒤부터 그 URL 을 50ms 마다 불러 처음 200 이 온 시각도 찍는다.
// SIGINT·SIGTERM 은 자식에게 넘긴다.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"npg-practice/ch07-10-service/internal/notify"
)

func main() {
	socket := flag.String("socket", "/tmp/npgsvc/notify.sock", "만들 unixgram 소켓 경로")
	ack := flag.Bool("ack", false, "데이터그램을 받으면 보낸 주소로 답장한다")
	health := flag.String("health", "", "자식을 띄운 뒤 200 이 올 때까지 부를 URL")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: supervisor [flags] -- command args…")
		os.Exit(2)
	}

	rcv, err := notify.Listen(*socket, *ack)
	if err != nil {
		fmt.Fprintln(os.Stderr, "supervisor:", err)
		os.Exit(1)
	}
	defer rcv.Close()

	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	cmd.Env = append(os.Environ(), "NOTIFY_SOCKET="+*socket)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "supervisor:", err)
		os.Exit(1)
	}
	elapsed := func() string { return time.Since(start).Truncate(time.Millisecond).String() }
	fmt.Printf("supervisor +%s started pid=%d NOTIFY_SOCKET=%s\n", elapsed(), cmd.Process.Pid, *socket)

	go func() {
		for {
			m, err := rcv.Next()
			if err != nil {
				return
			}
			from := m.From
			if from == "" {
				from = "(주소 없음)"
			}
			line := fmt.Sprintf("supervisor +%s notify from=%s msg=%q", elapsed(), from, strings.ReplaceAll(m.Body, "\n", " "))
			if m.Replied {
				if m.ReplyErr != nil {
					line += fmt.Sprintf(" reply_err=%q", m.ReplyErr.Error())
				} else {
					line += " reply=ok"
				}
			}
			fmt.Println(line)
		}
	}()

	if *health != "" {
		go func() {
			hc := &http.Client{Timeout: 200 * time.Millisecond}
			var lastErr string
			for {
				resp, err := hc.Get(*health)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						fmt.Printf("supervisor +%s health %s → %s\n", elapsed(), *health, resp.Status)
						return
					}
					err = fmt.Errorf("status %s", resp.Status)
				}
				if e := err.Error(); e != lastErr {
					fmt.Printf("supervisor +%s health %s → %s\n", elapsed(), *health, e)
					lastErr = e
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()
	}

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for s := range sig {
			fmt.Printf("supervisor +%s forward %s\n", elapsed(), s)
			cmd.Process.Signal(s)
		}
	}()

	err = cmd.Wait()
	fmt.Printf("supervisor +%s child exited: %v\n", elapsed(), err)
}
