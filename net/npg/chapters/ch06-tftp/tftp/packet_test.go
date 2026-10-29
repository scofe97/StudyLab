package tftp

import (
	"bytes"
	"reflect"
	"testing"
)

// 패킷 인코딩·디코딩 불변식: 왕복하면 같은 값이 나오고, 형식이 틀린 바이트는 거절한다.

func TestRequestRoundTrip(t *testing.T) {
	// Given — 옵션 넷이 붙은 RRQ (이름·모드는 대소문자를 섞었다)
	in := Request{Op: OpRRQ, Filename: "pxelinux.0", Mode: "OCTET", Options: []Option{
		{"BLKSIZE", "1428"}, {"windowsize", "8"}, {"tsize", "0"}, {"timeout", "1"},
	}}

	// When — 바이트로 바꿨다가 되돌린다
	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var out Request
	err = out.UnmarshalBinary(b)

	// Then — 이름·값은 그대로, 모드·옵션 이름은 소문자로 돌아온다
	if err != nil {
		t.Fatal(err)
	}
	want := Request{Op: OpRRQ, Filename: "pxelinux.0", Mode: "octet", Options: []Option{
		{"blksize", "1428"}, {"windowsize", "8"}, {"tsize", "0"}, {"timeout", "1"},
	}}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %+v, want %+v", out, want)
	}
	t.Logf("RRQ %d바이트: % x", len(b), b)
}

func TestDataAckErrOAckRoundTrip(t *testing.T) {
	// Given — 네 종류의 패킷
	data := Data{Block: 65535, Payload: bytes.Repeat([]byte{0xAB}, 512)}
	ack := Ack(65535)
	errPkt := Err{Code: ErrNotFound, Message: "file not found"}
	oack := OAck{Options: []Option{{"blksize", "1428"}, {"tsize", "1048576"}}}

	// When — 각각 바이트로 바꿨다가 되돌린다
	db, _ := data.MarshalBinary()
	ab, _ := ack.MarshalBinary()
	eb, _ := errPkt.MarshalBinary()
	ob, _ := oack.MarshalBinary()
	var (
		d2 Data
		a2 Ack
		e2 Err
		o2 OAck
	)

	// Then — 모두 같은 값으로 돌아온다
	if err := d2.UnmarshalBinary(db); err != nil || d2.Block != data.Block || !bytes.Equal(d2.Payload, data.Payload) {
		t.Fatalf("DATA: %v %+v", err, d2.Block)
	}
	if err := a2.UnmarshalBinary(ab); err != nil || a2 != ack {
		t.Fatalf("ACK: %v %v", err, a2)
	}
	if err := e2.UnmarshalBinary(eb); err != nil || e2 != errPkt {
		t.Fatalf("ERROR: %v %+v", err, e2)
	}
	if err := o2.UnmarshalBinary(ob); err != nil || !reflect.DeepEqual(o2, oack) {
		t.Fatalf("OACK: %v %+v", err, o2)
	}
	t.Logf("DATA %d바이트, ACK % x, ERROR % x", len(db), ab, eb)
}

func TestMalformedRejected(t *testing.T) {
	// Given — 형식이 틀린 바이트들과, 그것을 해석할 타입
	cases := []struct {
		name string
		into interface{ UnmarshalBinary([]byte) error }
		b    []byte
	}{
		{"빈 패킷", new(Request), nil},
		{"opcode 1바이트", new(Request), []byte{0}},
		{"RRQ 모드 뒤 NUL 없음", new(Request), []byte("\x00\x01file\x00octet")},
		{"RRQ 빈 파일명", new(Request), []byte("\x00\x01\x00octet\x00")},
		{"RRQ 옵션 값 빠짐", new(Request), []byte("\x00\x01f\x00octet\x00blksize\x00")},
		{"DATA 를 Request 로", new(Request), []byte{0, 3, 0, 1}},
		{"DATA 3바이트", new(Data), []byte{0, 3, 0}},
		{"ACK 3바이트", new(Ack), []byte{0, 4, 0}},
		{"ACK 5바이트", new(Ack), []byte{0, 4, 0, 1, 0}},
		{"RRQ 를 ACK 로", new(Ack), []byte{0, 1, 0, 1}},
		{"ERROR 끝 NUL 없음", new(Err), []byte("\x00\x05\x00\x01oops")},
		{"OACK 값 빠짐", new(OAck), []byte("\x00\x06blksize\x00")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// When — 해석한다
			err := c.into.UnmarshalBinary(c.b)

			// Then — 에러여야 한다
			if err == nil {
				t.Fatalf("% x 를 받아들였다", c.b)
			}
			t.Logf("err = %v", err)
		})
	}
}
