package tftp

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 서버와 클라이언트를 한 프로세스의 루프백으로 붙인다.
// 단언은 "받은 파일 = 원본" 과 "루트 밖은 열리지 않는다" 뿐이고,
// 재전송 수·걸린 시간 같은 관찰값은 t.Logf 로 남기기만 한다.

// lineSink 는 서버 로그를 줄 단위로 채널에 넘긴다.
type lineSink chan string

func (c lineSink) Write(p []byte) (int, error) {
	c <- strings.TrimRight(string(p), "\n")
	return len(p), nil
}

// waitLog 는 서버가 전송 한 건을 끝내고 남긴 줄을 기다린다.
func (c lineSink) waitLog(t *testing.T) string {
	t.Helper()
	select {
	case l := <-c:
		return l
	case <-time.After(10 * time.Second):
		t.Fatal("서버 로그가 오지 않았다")
		return ""
	}
}

// startServer 는 s 를 127.0.0.1 의 빈 포트에 띄우고 주소와 로그 채널을 돌려준다.
func startServer(t *testing.T, s *Server) (string, lineSink) {
	t.Helper()
	sink := make(lineSink, 64)
	s.Log = log.New(sink, "", 0)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = s.Serve(pc) }()
	t.Cleanup(func() { pc.Close() })
	return pc.LocalAddr().String(), sink
}

func randomFile(t *testing.T, dir, name string, size int, seed uint64) []byte {
	t.Helper()
	b := make([]byte, size)
	rng := rand.New(rand.NewPCG(seed, seed))
	for i := range b {
		b[i] = byte(rng.Uint32())
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func sum(b []byte) [32]byte { return sha512.Sum512_256(b) }

func TestGetFileIntegrity(t *testing.T) {
	// Given — 경계 크기를 포함한 파일들과 옵션 협상을 켠 서버
	root := t.TempDir()
	sizes := []int{0, 1, 511, 512, 513, 1024, 1428 * 8, 100_000}
	files := map[string][]byte{}
	for i, n := range sizes {
		name := "f" + string(rune('a'+i)) + ".bin"
		files[name] = randomFile(t, root, name, n, uint64(i+1))
	}
	addr, logs := startServer(t, &Server{Root: root, Options: true, Timeout: time.Second})

	clients := map[string]ClientOptions{
		"기본 octet":                  {},
		"blksize1428·window8·tsize": {BlockSize: 1428, WindowSize: 8, TSize: true},
		"netascii":                  {Mode: "netascii"},
	}
	for cname, opt := range clients {
		for name, want := range files {
			t.Run(cname+"/"+name, func(t *testing.T) {
				// When — 클라이언트로 받는다
				var got bytes.Buffer
				st, err := Get(addr, name, opt, &got)
				line := logs.waitLog(t)

				// Then — 에러 없이 받고, 해시가 원본과 같다
				if err != nil {
					t.Fatalf("Get: %v (서버: %s)", err, line)
				}
				if sum(got.Bytes()) != sum(want) {
					t.Fatalf("해시가 다르다: 받은 %d바이트, 원본 %d바이트", got.Len(), len(want))
				}
				t.Logf("클라이언트: %+v", st)
				t.Logf("서버: %s", line)
			})
		}
	}
}

func TestWRQRejected(t *testing.T) {
	// Given — 서버와, 쓰기 요청(WRQ) 패킷
	addr, logs := startServer(t, &Server{Root: t.TempDir()})
	// 응답은 서버의 새 포트(TID)에서 오므로 Dial 한 소켓이 아니라 ListenPacket 소켓으로 받는다
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	wrq, _ := Request{Op: OpWRQ, Filename: "config.txt", Mode: "octet"}.MarshalBinary()

	// When — WRQ 를 보내고 응답 하나를 읽는다
	if _, err := conn.WriteTo(wrq, raddr); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 600)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Then — 응답은 ERROR 패킷이다
	var e Err
	if err := e.UnmarshalBinary(buf[:n]); err != nil {
		t.Fatalf("ERROR 가 아닌 응답: % x", buf[:n])
	}
	t.Logf("응답: code=%d message=%q", e.Code, e.Message)
	t.Logf("서버: %s", logs.waitLog(t))
}

func TestRootConfinement(t *testing.T) {
	// Given — 루트 폴더와 그 옆(루트 밖)의 비밀 파일
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := []byte("root-outside-secret\n")
	if err := os.WriteFile(filepath.Join(base, "secret.txt"), secret, 0o644); err != nil {
		t.Fatal(err)
	}
	addr, logs := startServer(t, &Server{Root: root})

	// When — 상위 폴더로 올라가는 이름을 요청한다
	var got bytes.Buffer
	_, err := Get(addr, "../secret.txt", ClientOptions{Timeout: time.Second, Retries: 2}, &got)

	// Then — TFTP ERROR 로 끝나고, 비밀 파일 바이트는 한 바이트도 오지 않는다
	var e Err
	if !errors.As(err, &e) {
		t.Fatalf("TFTP ERROR 가 아니다: %v", err)
	}
	if got.Len() != 0 {
		t.Fatalf("루트 밖 파일 %d바이트를 받았다", got.Len())
	}
	t.Logf("클라이언트 에러: %v", err)
	t.Logf("서버: %s", logs.waitLog(t))
}

// TestDelayedAckObserve 는 T4 를 한 프로세스 안에서 돌린다.
// 블록 하나의 ACK 를 서버 타임아웃보다 늦게 보내고, 두 재전송 루프의 숫자를 남긴다.
func TestDelayedAckObserve(t *testing.T) {
	root := t.TempDir()
	want := randomFile(t, root, "fw.bin", 512*8000+100, 7)

	for _, legacy := range []bool{true, false} {
		name := map[bool]string{true: "legacy(책 루프)", false: "fixed(타임아웃에서만)"}[legacy]
		t.Run(name, func(t *testing.T) {
			// Given — 타임아웃 100ms 서버, 10번 블록 ACK 를 150ms 늦추는 클라이언트
			addr, logs := startServer(t, &Server{Root: root, Timeout: 100 * time.Millisecond, LegacyDupAck: legacy})
			opt := ClientOptions{AckDelayBlock: 10, AckDelay: 150 * time.Millisecond}

			// When — 받는다
			var got bytes.Buffer
			st, err := Get(addr, "fw.bin", opt, &got)
			line := logs.waitLog(t)

			// Then — 파일은 원본과 같아야 하고, 숫자는 남기기만 한다
			if err != nil {
				t.Fatalf("Get: %v (서버: %s)", err, line)
			}
			if sum(got.Bytes()) != sum(want) {
				t.Fatal("해시가 다르다")
			}
			t.Logf("클라이언트: blocks=%d dup_data=%d unexpected=%d acks=%d elapsed=%s",
				st.Blocks, st.DupData, st.Unexpected, st.AcksSent, st.Elapsed.Round(time.Millisecond))
			t.Logf("서버: %s", line)
		})
	}
}
