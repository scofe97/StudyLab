// Command connchurn 은 장애 리허설용으로 연결을 n 개 열고 곧바로 닫습니다.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/scofe97/runners-lab/net/npg/internal/ctrl"
)

func main() {
	addr := flag.String("addr", ctrl.DefaultAddr, "접속할 주소")
	n := flag.Int("n", 20, "열었다 닫을 연결 수")
	flag.Parse()

	for i := 0; i < *n; i++ {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dial 실패:", err)
			os.Exit(1)
		}
		conn.Close()
	}
	fmt.Printf("%s 연결 %d개를 열고 닫음\n", time.Now().Format("15:04:05"), *n)
}
