package dns

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// reply 는 질의 q 에 대한 응답을 짓는다. 헤더와 질문을 그대로 복사하고 플래그·답 개수를 바꾼 뒤 답 레코드를 붙인다.
func reply(q []byte, flags uint16, answers ...[]byte) []byte {
	b := bytes.Clone(q)
	binary.BigEndian.PutUint16(b[2:], flags)
	binary.BigEndian.PutUint16(b[6:], uint16(len(answers)))
	for _, a := range answers {
		b = append(b, a...)
	}
	return b
}

// aRecord 는 질문 이름(오프셋 12)을 가리키는 압축 포인터 c0 0c 로 시작하는 A 레코드다.
func aRecord(ip [4]byte) []byte {
	b := []byte{0xc0, 0x0c}
	b = binary.BigEndian.AppendUint16(b, TypeA)
	b = binary.BigEndian.AppendUint16(b, ClassIN)
	b = binary.BigEndian.AppendUint32(b, 60)
	b = binary.BigEndian.AppendUint16(b, 4)
	return append(b, ip[:]...)
}

func mustEncode(t *testing.T, q Query) []byte {
	t.Helper()
	b, err := q.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// startUDP 는 loopback 의 빈 포트에 가짜 DNS 서버를 띄운다.
// 질의가 올 때마다 handle 이 돌려준 데이터그램을 차례로 보낸다. 소켓은 테스트가 끝날 때 닫는다.
func startUDP(t *testing.T, handle func(n int, q []byte) [][]byte) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 64*1024)
		for n := 1; ; n++ {
			m, from, err := pc.ReadFrom(buf)
			if err != nil {
				return // Cleanup 의 Close 가 이 goroutine 을 끝낸다
			}
			for _, d := range handle(n, bytes.Clone(buf[:m])) {
				pc.WriteTo(d, from)
			}
		}
	}()
	return pc.LocalAddr().String()
}

func TestEncodeExampleComA(t *testing.T) {
	// Given — ID 0x1234 로 example.com 의 A 를 묻는 질의
	q := Query{ID: 0x1234, Name: "example.com", Type: TypeA}

	// When — 바이트로 짓는다
	got := mustEncode(t, q)

	// Then — 헤더 12 + 이름 13 + 타입·클래스 4 = 29바이트, 칸마다 빅엔디언
	want := []byte{
		0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0,
		0x00, 0x01, 0x00, 0x01,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("질의 = % x\n  want % x", got, want)
	}
}

func TestEncodeRejectsBadNames(t *testing.T) {
	cases := map[string]string{
		"라벨 64자":    strings.Repeat("a", 64) + ".com",
		"빈 라벨":      "a..com",
		"이름 전체 257": strings.Repeat(strings.Repeat("a", 63)+".", 4),
	}
	for name, n := range cases {
		t.Run(name, func(t *testing.T) {
			// Given — 규격을 벗어난 이름
			q := Query{ID: 1, Name: n, Type: TypeA}

			// When — 바이트로 지으려 한다
			_, err := q.Encode()

			// Then — ErrBadName 으로 거부한다
			if !errors.Is(err, ErrBadName) {
				t.Fatalf("err = %v, want ErrBadName", err)
			}
		})
	}
}

func TestEncodeFlipPutsLittleEndian(t *testing.T) {
	// Given — 질문 개수와 타입을 각각 뒤집은 질의
	qd := mustEncode(t, Query{ID: 1, Name: "a", Type: TypeA, Flip: FlipQDCount})
	qt := mustEncode(t, Query{ID: 1, Name: "a", Type: TypeA, Flip: FlipQType})

	// When — 해당 칸을 꺼낸다
	gotQD, gotQT := qd[4:6], qt[len(qt)-4:len(qt)-2]

	// Then — 둘 다 01 00 으로 작은 자리가 앞에 온다
	if !bytes.Equal(gotQD, []byte{1, 0}) || !bytes.Equal(gotQT, []byte{1, 0}) {
		t.Fatalf("qdcount = % x, qtype = % x, want 01 00", gotQD, gotQT)
	}
}

func TestParseFollowsCompressionPointer(t *testing.T) {
	// Given — www.example.com 질의와, 답 이름을 c0 0c 포인터로 적은 응답
	sent := mustEncode(t, Query{ID: 7, Name: "www.example.com", Type: TypeA})
	msg := reply(sent, 0x8180, aRecord([4]byte{93, 184, 216, 34}))

	// When — 응답을 푼다
	r, err := Parse(msg, sent)

	// Then — 포인터를 따라 질문의 이름을 읽고, 포인터 자리는 질문 바로 뒤다
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Answers) != 1 || r.Answers[0].Name != "www.example.com" || r.Answers[0].Value != "93.184.216.34" {
		t.Fatalf("답 = %+v", r.Answers)
	}
	if want := (Jump{At: len(sent), To: 12}); len(r.Jumps) != 1 || r.Jumps[0] != want {
		t.Fatalf("포인터 = %v, want [%v]", r.Jumps, want)
	}
}

func TestParseRejectsPointerToItself(t *testing.T) {
	// Given — 답 이름 포인터가 자기 자리를 가리키는 응답
	sent := mustEncode(t, Query{ID: 7, Name: "a", Type: TypeA})
	rec := aRecord([4]byte{1, 2, 3, 4})
	binary.BigEndian.PutUint16(rec, 0xc000|uint16(len(sent)))
	msg := reply(sent, 0x8180, rec)

	// When — 응답을 푼다
	_, err := Parse(msg, sent)

	// Then — 끝없이 돌지 않고 ErrPointer 로 멈춘다
	if !errors.Is(err, ErrPointer) {
		t.Fatalf("err = %v, want ErrPointer", err)
	}
}

func TestResolveSkipsMismatchedID(t *testing.T) {
	// Given — ID 가 다른 응답을 먼저, 진짜 응답을 나중에 보내는 서버
	addr := startUDP(t, func(_ int, q []byte) [][]byte {
		wrong := reply(q, 0x8180, aRecord([4]byte{6, 6, 6, 6}))
		wrong[1] ^= 0xff
		return [][]byte{wrong, reply(q, 0x8180, aRecord([4]byte{10, 0, 0, 1}))}
	})
	sent := mustEncode(t, Query{ID: 0x0101, Name: "example.com", Type: TypeA})

	// When — UDP 로 묻는다
	ex, err := Resolve(context.Background(), addr, sent, Options{Timeout: time.Second})

	// Then — 가짜 하나를 버리고 짝이 맞는 응답의 주소를 받는다
	if err != nil {
		t.Fatal(err)
	}
	if ex.Skipped != 1 || ex.Resp.Answers[0].Value != "10.0.0.1" {
		t.Fatalf("skipped = %d, 답 = %+v", ex.Skipped, ex.Resp.Answers)
	}
}

func TestResolveResendsAfterTimeout(t *testing.T) {
	// Given — 첫 질의는 버리고 두 번째 질의에만 답하는 서버 (손실 흉내)
	addr := startUDP(t, func(n int, q []byte) [][]byte {
		if n == 1 {
			return nil
		}
		return [][]byte{reply(q, 0x8180, aRecord([4]byte{10, 0, 0, 2}))}
	})
	sent := mustEncode(t, Query{ID: 0x0202, Name: "example.com", Type: TypeA})

	// When — 시도당 200ms, 최대 3번으로 묻는다
	ex, err := Resolve(context.Background(), addr, sent, Options{Timeout: 200 * time.Millisecond, Attempts: 3})

	// Then — 두 번째 시도에서 답을 받는다
	if err != nil {
		t.Fatal(err)
	}
	if ex.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", ex.Attempts)
	}
}

func TestResolveRetriesOverTCPWhenTruncated(t *testing.T) {
	// Given — UDP 로는 TC=1 에 답 0개, 같은 포트의 TCP 로는 답을 주는 서버
	addr := startUDP(t, func(_ int, q []byte) [][]byte {
		return [][]byte{reply(q, 0x8380)} // QR·TC·RD·RA
	})
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("같은 포트에 TCP 를 열지 못함: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	prefix := make(chan uint16, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close() // 이 goroutine 이 연결의 owner 다
		var l [2]byte
		if _, err := io.ReadFull(conn, l[:]); err != nil {
			return
		}
		prefix <- binary.BigEndian.Uint16(l[:])
		q := make([]byte, binary.BigEndian.Uint16(l[:]))
		if _, err := io.ReadFull(conn, q); err != nil {
			return
		}
		resp := reply(q, 0x8180, aRecord([4]byte{10, 0, 0, 3}))
		conn.Write(append(binary.BigEndian.AppendUint16(nil, uint16(len(resp))), resp...))
	}()
	sent := mustEncode(t, Query{ID: 0x0303, Name: "example.com", Type: TypeTXT})

	// When — UDP 로 먼저 묻는다
	ex, err := Resolve(context.Background(), addr, sent, Options{Timeout: time.Second})

	// Then — TC 를 보고 TCP 로 다시 물었고, 길이 접두는 질의 길이와 같다
	if err != nil {
		t.Fatal(err)
	}
	if ex.Via != "tcp" || ex.Truncated == nil || len(ex.Resp.Answers) != 1 {
		t.Fatalf("via = %s, truncated = %v, 답 = %+v", ex.Via, ex.Truncated != nil, ex.Resp.Answers)
	}
	if got := <-prefix; int(got) != len(sent) {
		t.Fatalf("길이 접두 = %d, want %d", got, len(sent))
	}
}
