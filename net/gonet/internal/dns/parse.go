package dns

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	ErrShort       = errors.New("dns: 메시지가 짧음")
	ErrNotResponse = errors.New("dns: 응답이 아님 (QR=0)")
	ErrMismatch    = errors.New("dns: 내 질의의 응답이 아님")
	ErrPointer     = errors.New("dns: 압축 포인터가 앞쪽을 가리키지 않음")
)

// Header 는 메시지 맨 앞 12바이트다. 칸 여섯이 모두 2바이트 고정 길이다.
type Header struct {
	ID, Flags                          uint16
	QDCount, ANCount, NSCount, ARCount uint16
}

func (h Header) QR() bool   { return h.Flags&0x8000 != 0 }
func (h Header) TC() bool   { return h.Flags&0x0200 != 0 }
func (h Header) RD() bool   { return h.Flags&0x0100 != 0 }
func (h Header) RA() bool   { return h.Flags&0x0080 != 0 }
func (h Header) RCode() int { return int(h.Flags & 0x000f) }

func (h Header) String() string {
	return fmt.Sprintf("id=0x%04x qr=%t tc=%t rd=%t ra=%t rcode=%d qd=%d an=%d ns=%d ar=%d",
		h.ID, h.QR(), h.TC(), h.RD(), h.RA(), h.RCode(),
		h.QDCount, h.ANCount, h.NSCount, h.ARCount)
}

// Jump 는 압축 포인터 하나다. At 자리의 2바이트가 To 오프셋을 가리킨다.
type Jump struct{ At, To int }

// Record 는 답 절의 자원 레코드 하나다.
type Record struct {
	Name  string
	Type  uint16
	TTL   uint32
	Data  []byte
	Value string // 사람이 읽는 값: A·AAAA 는 주소, CNAME 은 이름, TXT 는 문자열
}

// Response 는 풀어낸 응답이다. Jumps 는 이름을 읽으며 따라간 압축 포인터를 순서대로 담는다.
type Response struct {
	Header  Header
	Answers []Record
	Jumps   []Jump
}

// Parse 는 응답 msg 를 푼다. sent 는 내가 보낸 질의 바이트이고, 응답이 그 질의의 짝인지 ID 와 질문으로 가린다.
func Parse(msg, sent []byte) (*Response, error) {
	if len(msg) < headerLen || len(sent) < headerLen {
		return nil, ErrShort
	}
	h := readHeader(msg)
	if !h.QR() {
		return nil, ErrNotResponse
	}
	// 짝 맞추기 1: ID 는 응답에 그대로 복사되어 돌아온다.
	if sentID := binary.BigEndian.Uint16(sent); h.ID != sentID {
		return nil, fmt.Errorf("%w: id 0x%04x, 보낸 id 0x%04x", ErrMismatch, h.ID, sentID)
	}
	r := &Response{Header: h}
	off := headerLen
	// 짝 맞추기 2: 질문 칸도 그대로 복사된다. 메시지의 첫 이름이라 압축되지 않으므로 바이트로 비교한다.
	// 형식 오류 응답은 질문을 비워(qd=0) 보내기도 하니 그때는 ID 대조만 남는다.
	if h.QDCount > 0 {
		q := sent[headerLen:]
		end := off + len(q)
		if end > len(msg) || !bytes.EqualFold(msg[off:end], q) {
			return nil, fmt.Errorf("%w: 질문 칸이 다름", ErrMismatch)
		}
		off = end
		for i := 1; i < int(h.QDCount); i++ { // 질문이 더 있으면 이름·타입·클래스를 건너뛴다
			_, next, err := r.readName(msg, off)
			if err != nil {
				return nil, err
			}
			off = next + 4
		}
	}
	for i := 0; i < int(h.ANCount); i++ {
		rec, next, err := r.readRecord(msg, off)
		if err != nil {
			return nil, fmt.Errorf("답 %d: %w", i+1, err)
		}
		r.Answers = append(r.Answers, rec)
		off = next
	}
	return r, nil
}

func readHeader(b []byte) Header {
	u := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	return Header{ID: u(0), Flags: u(2), QDCount: u(4), ANCount: u(6), NSCount: u(8), ARCount: u(10)}
}

// readName 은 off 에서 이름을 읽어 점으로 이은 문자열과, 이 이름 다음 자리를 돌려준다.
// 길이 바이트의 위 두 비트가 11 이면 압축 포인터이고 나머지 14비트가 건너뛸 오프셋이다.
// 포인터를 한 번이라도 만나면 다음 자리는 그 포인터 2바이트 바로 뒤다.
func (r *Response) readName(msg []byte, off int) (string, int, error) {
	var labels []string
	next := -1
	// 포인터는 지금 읽는 구간의 시작보다 앞쪽만 가리킬 수 있게 한다.
	// 그래야 건너뛸 때마다 시작이 줄어들어 포인터 고리가 생기지 않는다.
	limit := off
	for {
		if off >= len(msg) {
			return "", 0, ErrShort
		}
		n := int(msg[off])
		switch {
		case n == 0:
			if next < 0 {
				next = off + 1
			}
			return strings.Join(labels, "."), next, nil
		case n&0xc0 == 0xc0:
			if off+1 >= len(msg) {
				return "", 0, ErrShort
			}
			to := int(binary.BigEndian.Uint16(msg[off:]) & 0x3fff)
			if to >= limit {
				return "", 0, fmt.Errorf("%w: @%d → %d", ErrPointer, off, to)
			}
			r.Jumps = append(r.Jumps, Jump{At: off, To: to})
			if next < 0 {
				next = off + 2
			}
			off, limit = to, to
		case n&0xc0 != 0:
			return "", 0, fmt.Errorf("dns: 예약된 라벨 길이 0x%02x @%d", n, off)
		default:
			if off+1+n > len(msg) {
				return "", 0, ErrShort
			}
			labels = append(labels, string(msg[off+1:off+1+n]))
			off += 1 + n
		}
	}
}

// readRecord 는 이름·타입·클래스·TTL·데이터 길이·데이터 순서의 자원 레코드 하나를 읽는다.
func (r *Response) readRecord(msg []byte, off int) (Record, int, error) {
	name, off, err := r.readName(msg, off)
	if err != nil {
		return Record{}, 0, err
	}
	if off+10 > len(msg) {
		return Record{}, 0, ErrShort
	}
	rec := Record{
		Name: name,
		Type: binary.BigEndian.Uint16(msg[off:]),
		TTL:  binary.BigEndian.Uint32(msg[off+4:]),
	}
	rdlen := int(binary.BigEndian.Uint16(msg[off+8:]))
	off += 10
	if off+rdlen > len(msg) {
		return Record{}, 0, ErrShort
	}
	rec.Data = msg[off : off+rdlen]
	rec.Value, err = r.value(msg, rec.Type, off, rec.Data)
	return rec, off + rdlen, err
}

func (r *Response) value(msg []byte, typ uint16, off int, data []byte) (string, error) {
	switch {
	case typ == TypeA && len(data) == 4, typ == TypeAAAA && len(data) == 16:
		return net.IP(data).String(), nil
	case typ == TypeCNAME:
		// CNAME 의 값도 이름이라 압축될 수 있다. 포인터를 따라가려면 메시지 전체가 필요하다.
		name, _, err := r.readName(msg, off)
		return name, err
	case typ == TypeTXT:
		// TXT 는 길이 1바이트 + 문자열 조각의 반복이다.
		var parts []string
		for i := 0; i < len(data); {
			n := int(data[i])
			if i+1+n > len(data) {
				return "", ErrShort
			}
			parts = append(parts, strconv.Quote(string(data[i+1:i+1+n])))
			i += 1 + n
		}
		return strings.Join(parts, " "), nil
	default:
		return hex.EncodeToString(data), nil
	}
}
