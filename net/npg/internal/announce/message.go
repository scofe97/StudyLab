package announce

import (
	"encoding/binary"
	"errors"
)

const (
	// Version은 와이어 형식의 현재 버전
	Version = 1
	// HeaderSize는 버전 1 + 타입 1 + nonce 8 바이트
	HeaderSize = 10
	// MaxAnnounceSize는 UDP 페이로드 상한
	MaxAnnounceSize = 1232
)

// Type은 공지 메시지의 종류입니다.
type Type uint8

const (
	TypeHello Type = 1
	TypeAck   Type = 2
)

var (
	ErrTooLarge = errors.New("공지 메시지가 상한을 넘었습니다")
)

// Message는 HELLO 또는 ACK 한 개입니다.
type Message struct {
	Type  Type
	Nonce uint64
	Body  []byte
}

// Encode는 메시지를 와이어 형식의 바이트로 만듭니다.
func (m Message) Encode() ([]byte, error) {
	if HeaderSize+len(m.Body) > MaxAnnounceSize {
		return nil, ErrTooLarge
	}

	buf := make([]byte, HeaderSize, HeaderSize+len(m.Body))
	buf[0] = Version
	buf[1] = byte(m.Type)
	binary.BigEndian.PutUint64(buf[2:], m.Nonce)

	return append(buf, m.Body...), nil
}

var (
	ErrShortMessage = errors.New("공지 메시지가 헤더보다 짧습니다")
	ErrBadVersion   = errors.New("알 수 없는 공지 버전입니다")
	ErrUnknownType  = errors.New("알 수 없는 공지 타입입니다")
)

// Decode는 와이어 형식의 바이트를 Message로 읽습니다.
func Decode(data []byte) (Message, error) {
	// 헤더 사이즈보다 작다
	if len(data) < HeaderSize {
		return Message{}, ErrShortMessage
	}
	// 버전이 다르다
	if data[0] != Version {
		return Message{}, ErrBadVersion
	}

	// 알 수 없는 타입이다.
	t := Type(data[1])
	if t != TypeHello && t != TypeAck {
		return Message{}, ErrUnknownType
	}

	return Message{
		Type:  t,
		Nonce: binary.BigEndian.Uint64(data[2:HeaderSize]),
		Body:  data[HeaderSize:],
	}, nil
}
