package tftp

import (
	"encoding/binary"
	"errors"
	"strings"
)

const (
	// HeaderSize는 opcode 2바이트와 블록 번호 2바이트
	HeaderSize = 4
	// DefaultBlksize는 협상이 없을 때의 블록 크기 (RFC 1350)
	DefaultBlksize = 512
)

// Opcode는 TFTP 패킷의 종류입니다.
type Opcode uint16

const (
	OpRRQ   Opcode = 1
	OpWRQ   Opcode = 2
	OpData  Opcode = 3
	OpAck   Opcode = 4
	OpError Opcode = 5
	OpOAck  Opcode = 6
)

// ErrorCode는 ERROR 패킷의 오류 번호입니다.
type ErrorCode uint16

const (
	ErrNotDefined   ErrorCode = 0
	ErrFileNotFound ErrorCode = 1
	ErrAccessDenied ErrorCode = 2
	ErrIllegalOp    ErrorCode = 4
	ErrUnknownTID   ErrorCode = 5
	ErrBadOption    ErrorCode = 8
)

var (
	ErrShortPacket = errors.New("패킷이 너무 짧습니다")
	ErrBadOpcode   = errors.New("알 수 없는 opcode입니다")
	ErrBadRequest  = errors.New("요청 형식이 올바르지 않습니다")
)

// Ack는 블록 하나를 받았다는 확인입니다.
type Ack struct {
	Block uint16
}

// Encode는 ACK를 와이어 형식의 바이트로 만듭니다.
func (a Ack) Encode() []byte {
	b := make([]byte, HeaderSize)
	binary.BigEndian.PutUint16(b[0:2], uint16(OpAck))
	binary.BigEndian.PutUint16(b[2:4], a.Block)
	return b
}

// Data는 블록 번호와 그 블록의 페이로드입니다.
type Data struct {
	Block uint16
	// Payload는 Decode 로 만들면 입력 버퍼를 가리킵니다. 다음 읽기가 덮으므로 바로 쓰거나 복사해야 합니다.
	Payload []byte
}

// Encode는 DATA를 와이어 형식의 바이트로 만듭니다.
func (d Data) Encode() []byte {
	b := make([]byte, HeaderSize+len(d.Payload))
	binary.BigEndian.PutUint16(b[0:2], uint16(OpData))
	binary.BigEndian.PutUint16(b[2:4], d.Block)
	copy(b[HeaderSize:], d.Payload)
	return b
}

// ReadReq는 파일 하나를 읽겠다는 요청(RRQ)입니다.
type ReadReq struct {
	Filename string
	Mode     string
	// Options는 이름을 소문자로 바꾼 옵션입니다 (RFC 2347)
	Options map[string]string
}

// Encode는 RRQ를 와이어 형식의 바이트로 만듭니다.
func (r ReadReq) Encode() []byte {
	// opcode 2바이트를 빅엔디언으로 먼저 붙입니다
	b := binary.BigEndian.AppendUint16(nil, uint16(OpRRQ))

	b = append(b, r.Filename...)
	b = append(b, 0)
	b = append(b, r.Mode...)
	b = append(b, 0)

	for name, value := range r.Options {
		b = append(b, name...)
		b = append(b, 0)
		b = append(b, value...)
		b = append(b, 0)
	}

	return b
}

// ErrorPacket은 더는 진행할 수 없다는 알림(ERROR)입니다.
type ErrorPacket struct {
	Code    ErrorCode
	Message string
}

// Encode는 ERROR를 와이어 형식의 바이트로 만듭니다.
func (e ErrorPacket) Encode() []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(OpError))
	b = binary.BigEndian.AppendUint16(b, uint16(e.Code))
	b = append(b, e.Message...)
	return append(b, 0)
}

// OAck는 서버가 받아들인 옵션의 응답(OACK)입니다.
type OAck struct {
	Options map[string]string
}

// Encode는 OACK를 와이어 형식의 바이트로 만듭니다.
func (o OAck) Encode() []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(OpOAck))
	for name, value := range o.Options {
		b = append(b, name...)
		b = append(b, 0)
		b = append(b, value...)
		b = append(b, 0)
	}
	return b
}

// Packet은 Decode가 돌려주는 패킷의 공통 모양입니다.
type Packet interface {
	Encode() []byte
}

// Decode는 받은 바이트를 패킷 하나로 해석합니다.
func Decode(b []byte) (Packet, error) {
	if len(b) < 2 {
		return nil, ErrShortPacket
	}

	switch Opcode(binary.BigEndian.Uint16(b[0:2])) {
	case OpData:
		if len(b) < HeaderSize {
			return nil, ErrShortPacket
		}
		return Data{
			Block:   binary.BigEndian.Uint16(b[2:4]),
			Payload: b[HeaderSize:],
		}, nil

	case OpAck:
		if len(b) != HeaderSize {
			return nil, ErrShortPacket
		}
		return Ack{
			Block: binary.BigEndian.Uint16(b[2:4]),
		}, nil

	case OpRRQ:
		// 파일 이름, 모드, 그 뒤로 (옵션 이름, 값) 쌍
		fields, err := splitFields(b[2:])
		if err != nil {
			return nil, err
		}
		if len(fields) < 2 || len(fields)%2 != 0 {
			return nil, ErrBadRequest
		}
		return ReadReq{
			Filename: fields[0],
			Mode:     strings.ToLower(fields[1]),
			Options:  optionMap(fields[2:]),
		}, nil

	case OpError:
		// opcode 2 + 오류 코드 2 + 메시지 + 종결자 0 하나
		if len(b) < HeaderSize+1 || b[len(b)-1] != 0 {
			return nil, ErrBadRequest
		}
		return ErrorPacket{
			Code:    ErrorCode(binary.BigEndian.Uint16(b[2:4])),
			Message: string(b[HeaderSize : len(b)-1]),
		}, nil

	case OpOAck:
		fields, err := splitFields(b[2:])
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 || len(fields)%2 != 0 {
			return nil, ErrBadRequest
		}
		return OAck{Options: optionMap(fields)}, nil

	default:
		return nil, ErrBadOpcode
	}
}

// splitFields는 0 바이트로 끝나는 문자열들을 순서대로 잘라 냅니다.
func splitFields(b []byte) ([]string, error) {
	if len(b) == 0 || b[len(b)-1] != 0 {
		return nil, ErrBadRequest
	}

	return strings.Split(string(b[:len(b)-1]), "\x00"), nil
}

// optionMap은 (이름, 값) 순서로 놓인 문자열들을 소문자 이름의 맵으로 만듭니다.
func optionMap(fields []string) map[string]string {
	if len(fields) == 0 {
		return nil
	}

	options := make(map[string]string, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		options[strings.ToLower(fields[i])] = fields[i+1]
	}
	return options
}
