package scan

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestMarshalSYNHeader(t *testing.T) {
	// Given — 출발지 40000, 목적지 80, seq 0x11223344 의 SYN 헤더
	h := tcpHeader{srcPort: 40000, dstPort: 80, seq: 0x11223344, flags: flagSYN, window: 65535}

	// When — 20바이트로 짓는다
	b := h.marshal()

	// Then — 칸마다 빅엔디언, 데이터 오프셋 5(=0x50), 플래그 0x02
	want := []byte{
		0x9c, 0x40, 0x00, 0x50, 0x11, 0x22, 0x33, 0x44,
		0x00, 0x00, 0x00, 0x00, 0x50, 0x02, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00,
	}
	if len(b) != tcpHeaderLen {
		t.Fatalf("길이 = %d, want %d", len(b), tcpHeaderLen)
	}
	for i := range want {
		if b[i] != want[i] {
			t.Fatalf("바이트 %d = 0x%02x, want 0x%02x\n  got  % x\n  want % x", i, b[i], want[i], b, want)
		}
	}
}

func TestChecksumIsVerifiable(t *testing.T) {
	// Given — 체크섬을 채운 SYN 세그먼트
	src, dst := net.IP{10, 0, 0, 1}, net.IP{10, 0, 0, 2}
	seg := synSegment(src, dst, 40000, 80, 0)

	// When — 받는 쪽이 하듯 체크섬 칸까지 포함해 다시 계산한다
	got := checksum(src, dst, seg)

	// Then — 올바른 세그먼트는 0 이 된다 (인터넷 체크섬의 성질)
	if got != 0 {
		t.Fatalf("검산 = 0x%04x, want 0 (체크섬이 틀림)", got)
	}
}

func TestChecksumCatchesCorruption(t *testing.T) {
	// Given — 체크섬을 채운 뒤 seq 한 바이트를 뒤집은 세그먼트
	src, dst := net.IP{127, 0, 0, 1}, net.IP{127, 0, 0, 1}
	seg := synSegment(src, dst, 40000, 8080, 0)
	seg[4] ^= 0xff

	// When — 다시 검산한다
	got := checksum(src, dst, seg)

	// Then — 0 이 아니다 (한 비트만 바뀌어도 걸린다)
	if got == 0 {
		t.Fatal("손상된 세그먼트가 검산을 통과함")
	}
}

func TestSynReplyMatchesByPortAndAck(t *testing.T) {
	// Given — baseSeq 1000, 우리 출발지 포트 40000. 포트 8080 의 SYN-ACK 응답을 짓는다
	const mySrc, base = uint16(40000), uint32(1000)
	reply := tcpHeader{srcPort: 8080, dstPort: mySrc, ack: base + 8080 + 1, flags: flagSYN | flagACK}.marshal()

	// When — 응답을 푼다
	port, flags, ok := synReply(reply, mySrc, base)

	// Then — 포트 8080, SYN·ACK 플래그로 짝이 맞는다
	if !ok || port != 8080 || flags&(flagSYN|flagACK) != (flagSYN|flagACK) {
		t.Fatalf("port=%d flags=0x%02x ok=%t", port, flags, ok)
	}
}

func TestSynReplyRejectsWrongAck(t *testing.T) {
	// Given — 포트는 맞지만 ack 가 어긋난 응답 (다른 질의의 답, 또는 위조)
	const mySrc, base = uint16(40000), uint32(1000)
	reply := tcpHeader{srcPort: 8080, dstPort: mySrc, ack: 999, flags: flagSYN | flagACK}.marshal()

	// When — 응답을 푼다
	_, _, ok := synReply(reply, mySrc, base)

	// Then — 짝이 맞지 않아 버린다
	if ok {
		t.Fatal("ack 가 어긋난 응답을 받아들임")
	}
}

func TestSynReplySkipsIPv4Header(t *testing.T) {
	// Given — Linux raw 수신처럼 앞에 IPv4 헤더 20바이트가 붙은 RST 응답
	const mySrc, base = uint16(40000), uint32(1000)
	tcp := tcpHeader{srcPort: 81, dstPort: mySrc, ack: base + 81 + 1, flags: flagRST | flagACK}.marshal()
	pkt := make([]byte, 20+len(tcp))
	pkt[0] = 0x45 // IPv4, IHL 5 (20바이트)
	binary.BigEndian.PutUint16(pkt[2:], uint16(len(pkt)))
	copy(pkt[20:], tcp)

	// When — 응답을 푼다
	port, flags, ok := synReply(pkt, mySrc, base)

	// Then — IP 헤더를 건너뛰고 포트 81, RST 를 읽는다
	if !ok || port != 81 || flags&flagRST == 0 {
		t.Fatalf("port=%d flags=0x%02x ok=%t", port, flags, ok)
	}
}
