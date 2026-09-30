package tcp

import (
	"context"
	"io"
	"net"
)

// Connect 는 addr 에 접속해 in 을 보내고 받은 바이트를 out 에 쓴다.
// in 이 EOF 에 닿으면 쓰기 방향만 닫고(half-close, FIN 전송) 서버가 연결을 닫을 때까지 읽는다.
// 서버가 먼저 닫으면 in 에 입력이 없어도 곧바로 돌아온다.
func Connect(ctx context.Context, addr string, in io.Reader, out io.Writer) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// ctx 가 끝나면 conn 을 닫아 blocking Read/Write 를 깨운다.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(out, conn)
		readDone <- err
	}()

	// in → conn 도 별도 goroutine 에 둔다. 메인이 여기서 in 을 기다리면 서버의 종료(readDone)를
	// 보지 못해 conn 이 CLOSE-WAIT 로 남는다. in(stdin) 읽기는 취소할 방법이 없으므로
	// 이 goroutine 은 Connect 가 돌아온 뒤 다음 입력(닫힌 conn 에 쓰기 실패) 또는 in 의 EOF 에서 끝나고,
	// CLI 에서는 프로세스 종료가 회수한다. 쓰기 에러는 읽기 쪽 에러로 함께 드러나므로 따로 돌려주지 않는다.
	go func() {
		if _, err := io.Copy(conn, in); err != nil {
			return
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	err = <-readDone
	if ctx.Err() != nil {
		return nil
	}
	return err
}
