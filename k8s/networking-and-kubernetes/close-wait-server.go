//go:build ignore

// 관찰용으로만 쓰는 서버입니다. 연결을 받고 닫지 않아 CLOSE-WAIT 를 남깁니다
package main

import (
	"fmt"
	"net"
)

func main() {
	ln, _ := net.Listen("tcp", ":9090")
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// 일부러 conn.Close() 를 호출하지 않습니다.
		// 상대가 끊어도 우리 쪽 소켓이 CLOSE-WAIT 에 머뭅니다.
		fmt.Println("accepted:", conn.RemoteAddr())
	}
}
