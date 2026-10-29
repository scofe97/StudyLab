// Package dns 는 net.LookupHost 없이 DNS 질의를 바이트로 짓고 응답을 푼다.
// 형식은 RFC 1035 §4.1 을 따른다. 여러 바이트 숫자는 모두 빅엔디언(네트워크 바이트 순서)이다.
package dns

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// 자원 레코드 타입과 클래스 (RFC 1035 §3.2.2, AAAA 는 RFC 3596).
const (
	TypeA     uint16 = 1
	TypeCNAME uint16 = 5
	TypeTXT   uint16 = 16
	TypeAAAA  uint16 = 28
	ClassIN   uint16 = 1
)

const (
	headerLen = 12
	flagRD    = 0x0100 // "모르면 끝까지 알아 와 달라"
	maxLabel  = 63     // 길이 1바이트의 위 두 비트는 압축 포인터 표시라 라벨은 63 까지만 쓴다
	maxName   = 255    // 길이 바이트와 끝 0 을 포함한 이름 전체
)

// ErrBadName 은 라벨이나 이름 전체 길이가 규격을 벗어났다는 뜻이다.
var ErrBadName = errors.New("dns: 이름이 규격을 벗어남")

// Flip 은 실험에서 일부러 바이트 순서를 뒤집을 칸이다.
type Flip int

const (
	FlipNone    Flip = iota
	FlipQDCount      // 질문 개수 1 을 00 01 대신 01 00 으로 보낸다
	FlipQType        // 타입을 리틀엔디언으로 보낸다 (A=1 이 01 00)
)

// Query 는 질의 한 장의 재료다.
type Query struct {
	ID   uint16
	Name string
	Type uint16
	Flip Flip
}

// Encode 는 헤더 12바이트와 질문(이름·타입·클래스)을 짓는다.
func (q Query) Encode() ([]byte, error) {
	name, err := encodeName(q.Name)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 0, headerLen+len(name)+4)
	b = binary.BigEndian.AppendUint16(b, q.ID)
	b = binary.BigEndian.AppendUint16(b, flagRD)
	b = put16(b, 1, q.Flip == FlipQDCount) // QDCOUNT
	b = append(b, 0, 0, 0, 0, 0, 0)        // ANCOUNT·NSCOUNT·ARCOUNT
	b = append(b, name...)
	b = put16(b, q.Type, q.Flip == FlipQType)
	b = binary.BigEndian.AppendUint16(b, ClassIN)
	return b, nil
}

// put16 은 v 를 2바이트로 붙인다. flip 이면 작은 자리부터 놓아 네트워크 바이트 순서를 일부러 어긴다.
func put16(b []byte, v uint16, flip bool) []byte {
	if flip {
		return binary.LittleEndian.AppendUint16(b, v)
	}
	return binary.BigEndian.AppendUint16(b, v)
}

// encodeName 은 "www.example.com" 을 라벨마다 길이 1바이트를 앞에 붙인 3www7example3com0 으로 바꾼다.
func encodeName(name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	var b []byte
	if name != "" {
		for _, label := range strings.Split(name, ".") {
			if len(label) < 1 || len(label) > maxLabel {
				return nil, fmt.Errorf("%w: 라벨 %q 길이 %d (1–%d)", ErrBadName, label, len(label), maxLabel)
			}
			b = append(b, byte(len(label)))
			b = append(b, label...)
		}
	}
	b = append(b, 0) // 끝 표시: 길이 0 인 라벨
	if len(b) > maxName {
		return nil, fmt.Errorf("%w: 이름 전체 %d 바이트 (최대 %d)", ErrBadName, len(b), maxName)
	}
	return b, nil
}
