// Package notify 는 systemd 의 sd_notify 방식으로 상태를 알린다.
//
// 감독 프로세스는 unixgram 소켓 파일을 하나 만들고 그 경로를 NOTIFY_SOCKET 환경 변수로 넘긴다.
// 서비스는 준비가 끝나면 그 소켓으로 "READY=1" 데이터그램 하나를 보낸다.
// 이 패키지는 답장(ACK)을 기다리는 확장을 하나 더 둔다. systemd 는 답장을 보내지 않는다.
package notify

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"time"
)

// Sender 는 상태 데이터그램을 보내는 쪽이다.
type Sender struct {
	// 보낼 곳. 비면 NOTIFY_SOCKET 을 쓴다. 둘 다 비면 Send 는 아무것도 하지 않는다.
	Socket string
	// 보내는 쪽 소켓을 묶을 경로. 비면 경로 없이 보낸다.
	LocalPath string
	// 0 보다 크면 보낸 뒤 이만큼 답장을 기다린다.
	AckWait time.Duration
}

// Send 는 state 를 보내고, AckWait 가 있으면 답장을 읽어 돌려준다.
func (s Sender) Send(state string) (ack string, err error) {
	target := s.Socket
	if target == "" {
		target = os.Getenv("NOTIFY_SOCKET")
	}
	if target == "" {
		return "", nil
	}
	var laddr *net.UnixAddr
	if s.LocalPath != "" {
		if err := os.Remove(s.LocalPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		laddr = &net.UnixAddr{Name: s.LocalPath, Net: "unixgram"}
		defer os.Remove(s.LocalPath)
	}
	conn, err := net.DialUnix("unixgram", laddr, &net.UnixAddr{Name: target, Net: "unixgram"})
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", target, err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(state)); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	if s.AckWait <= 0 {
		return "", nil
	}
	conn.SetReadDeadline(time.Now().Add(s.AckWait))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("ack: %w", err)
	}
	return string(buf[:n]), nil
}

// Message 는 감독 프로세스가 받은 데이터그램 하나다.
type Message struct {
	At time.Time
	// ReadFromUnix 가 준 보낸 쪽 주소
	From string
	Body string
	// 답장을 보냈다면 그 결과. 답장을 보내지 않았으면 nil 이고 Replied 는 false 다.
	Replied  bool
	ReplyErr error
}

// Receiver 는 감독 프로세스 쪽 소켓이다.
type Receiver struct {
	conn *net.UnixConn
	path string
	Ack  bool
}

// Listen 은 path 에 unixgram 소켓을 만든다. 경로에 파일이 있으면 먼저 지운다.
func Listen(path string, ack bool) (*Receiver, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return nil, err
	}
	return &Receiver{conn: c, path: path, Ack: ack}, nil
}

// Path 는 소켓 파일 경로다.
func (r *Receiver) Path() string { return r.path }

// Next 는 데이터그램 하나를 읽고, Ack 가 켜져 있으면 보낸 주소로 "ACK <본문>" 을 돌려보낸다.
func (r *Receiver) Next() (Message, error) {
	buf := make([]byte, 4096)
	n, from, err := r.conn.ReadFromUnix(buf)
	if err != nil {
		return Message{}, err
	}
	m := Message{At: time.Now(), Body: string(buf[:n])}
	if from != nil {
		m.From = from.Name
	}
	if r.Ack {
		m.Replied = true
		_, m.ReplyErr = r.conn.WriteToUnix([]byte("ACK "+m.Body), from)
	}
	return m, nil
}

// Close 는 소켓을 닫고 소켓 파일을 지운다.
func (r *Receiver) Close() error {
	err := r.conn.Close()
	os.Remove(r.path)
	return err
}
