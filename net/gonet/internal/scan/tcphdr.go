package scan

import (
	"encoding/binary"
	"net"
)

// TCP 플래그 (04-01 §1 의 플래그 표). 단 2 는 SYN 을 보내고 돌아온 플래그를 직접 읽는다.
const (
	flagFIN uint8 = 1 << 0
	flagSYN uint8 = 1 << 1
	flagRST uint8 = 1 << 2
	flagACK uint8 = 1 << 4
)

const tcpHeaderLen = 20 // 옵션 없는 TCP 헤더

// tcpHeader 는 옵션 없는 TCP 헤더 20바이트다. 03 DNS 에서 익힌 빅엔디언 필드 인코딩을 TCP 에 잇는다.
type tcpHeader struct {
	srcPort, dstPort uint16
	seq, ack         uint32
	flags            uint8
	window           uint16
}

// marshal 은 헤더를 20바이트로 짓는다. 체크섬 칸은 0 으로 두고 checksum() 으로 따로 채운다.
func (h tcpHeader) marshal() []byte {
	b := make([]byte, tcpHeaderLen)
	binary.BigEndian.PutUint16(b[0:], h.srcPort)
	binary.BigEndian.PutUint16(b[2:], h.dstPort)
	binary.BigEndian.PutUint32(b[4:], h.seq)
	binary.BigEndian.PutUint32(b[8:], h.ack)
	b[12] = 5 << 4 // 데이터 오프셋 5 (= 20바이트), 옵션 없음
	b[13] = h.flags
	binary.BigEndian.PutUint16(b[14:], h.window)
	// b[16:18] 체크섬은 0, b[18:20] urgent 포인터도 0
	return b
}

// checksum 은 pseudo-header 위에서 인터넷 체크섬을 계산한다.
// TCP 체크섬에 출발지·목적지 IP 가 들어가는 이유가 pseudo-header 다. 세그먼트가 엉뚱한 호스트로 샜는지까지 걸러낸다.
func checksum(src, dst net.IP, seg []byte) uint16 {
	pseudo := make([]byte, 12)
	copy(pseudo[0:4], src.To4())
	copy(pseudo[4:8], dst.To4())
	pseudo[9] = 6 // 프로토콜 번호 TCP
	binary.BigEndian.PutUint16(pseudo[10:], uint16(len(seg)))
	return internetChecksum(pseudo, seg)
}

// internetChecksum 은 16비트 워드를 1의 보수로 더한 뒤 뒤집는다 (RFC 1071).
func internetChecksum(parts ...[]byte) uint16 {
	var sum uint32
	var carry byte
	var haveCarry bool
	for _, p := range parts {
		for i := 0; i < len(p); i++ {
			if haveCarry {
				sum += uint32(carry)<<8 | uint32(p[i])
				haveCarry = false
			} else if i == len(p)-1 {
				carry, haveCarry = p[i], true
			} else {
				sum += uint32(p[i])<<8 | uint32(p[i+1])
				i++
			}
		}
	}
	if haveCarry {
		sum += uint32(carry) << 8
	}
	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// synSegment 는 src:srcPort → dst:dstPort 로 보낼 SYN 세그먼트를 체크섬까지 채워 돌려준다.
// seq 에 포트를 섞어, 돌아온 ACK(= seq+1)로 어느 포트의 답인지 짝을 맞춘다 (03 DNS 의 ID 대조와 같은 문제).
func synSegment(src, dst net.IP, srcPort, dstPort uint16, baseSeq uint32) []byte {
	h := tcpHeader{
		srcPort: srcPort,
		dstPort: dstPort,
		seq:     baseSeq + uint32(dstPort),
		flags:   flagSYN,
		window:  65535,
	}
	seg := h.marshal()
	binary.BigEndian.PutUint16(seg[16:], checksum(src, dst, seg))
	return seg
}

// synReply 는 돌아온 세그먼트를 읽어 (보낸 쪽이 스캔한 포트, 플래그)를 돌려준다.
// Linux raw 소켓 수신은 IP 헤더를 포함하므로, 첫 바이트가 IPv4 면 IHL 만큼 건너뛴다.
func synReply(pkt []byte, mySrcPort uint16, baseSeq uint32) (port int, flags uint8, ok bool) {
	if len(pkt) > 0 && pkt[0]>>4 == 4 { // IPv4 헤더가 앞에 붙어 있으면 건너뛴다
		ihl := int(pkt[0]&0x0f) * 4
		if len(pkt) < ihl {
			return 0, 0, false
		}
		pkt = pkt[ihl:]
	}
	if len(pkt) < tcpHeaderLen {
		return 0, 0, false
	}
	srcPort := binary.BigEndian.Uint16(pkt[0:]) // 응답의 출발지 = 우리가 스캔한 포트
	dstPort := binary.BigEndian.Uint16(pkt[2:]) // 응답의 목적지 = 우리 출발지 포트
	ack := binary.BigEndian.Uint32(pkt[8:])
	flags = pkt[13]
	// 우리 소켓으로 온 답이고, ack 가 우리가 보낸 seq+1 이어야 짝이 맞는다.
	if dstPort != mySrcPort || ack != baseSeq+uint32(srcPort)+1 {
		return 0, 0, false
	}
	return int(srcPort), flags, true
}
