// Package tftp 는 『Network Programming with Go』 6장의 TFTP 를 실무판으로 다시 짠 것이다.
//
// 책의 서버는 메모리의 페이로드 하나를 내주지만, 여기 서버는 루트 폴더의 파일을
// 이름으로 내준다. 패킷은 RFC 1350 의 RRQ·WRQ·DATA·ACK·ERROR 와
// RFC 2347 의 OACK 여섯 종을 다룬다. 서버는 WRQ 를 받지 않고 ERROR 로 거절한다.
package tftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// OpCode 는 패킷 맨 앞 2바이트다.
type OpCode uint16

const (
	OpRRQ  OpCode = 1
	OpWRQ  OpCode = 2
	OpData OpCode = 3
	OpAck  OpCode = 4
	OpErr  OpCode = 5
	OpOAck OpCode = 6
)

func (o OpCode) String() string {
	switch o {
	case OpRRQ:
		return "RRQ"
	case OpWRQ:
		return "WRQ"
	case OpData:
		return "DATA"
	case OpAck:
		return "ACK"
	case OpErr:
		return "ERROR"
	case OpOAck:
		return "OACK"
	}
	return fmt.Sprintf("op(%d)", uint16(o))
}

const (
	// HeaderSize 는 DATA 패킷의 opcode 2바이트와 블록 번호 2바이트다.
	HeaderSize = 4
	// DefaultBlockSize 는 옵션 협상이 없을 때의 블록 크기다 (RFC 1350).
	DefaultBlockSize = 512
	// MinBlockSize·MaxBlockSize 는 RFC 2348 이 허용하는 blksize 범위다.
	MinBlockSize = 8
	MaxBlockSize = 65464
	// MaxWindowSize 는 RFC 7440 이 허용하는 windowsize 상한이다.
	MaxWindowSize = 65535
)

// ErrCode 는 ERROR 패킷의 오류 코드다. 0~7 은 RFC 1350, 8 은 RFC 2347.
type ErrCode uint16

const (
	ErrUndefined     ErrCode = 0
	ErrNotFound      ErrCode = 1
	ErrAccess        ErrCode = 2
	ErrDiskFull      ErrCode = 3
	ErrIllegalOp     ErrCode = 4
	ErrUnknownTID    ErrCode = 5
	ErrFileExists    ErrCode = 6
	ErrNoUser        ErrCode = 7
	ErrOptionRefused ErrCode = 8
)

var (
	errShort   = errors.New("packet too short")
	errOpcode  = errors.New("unexpected opcode")
	errNoNul   = errors.New("field is not NUL-terminated")
	errOddOpts = errors.New("option without value")
)

// PeekOpCode 는 패킷을 끝까지 해석하기 전에 종류만 본다.
func PeekOpCode(b []byte) (OpCode, error) {
	if len(b) < 2 {
		return 0, errShort
	}
	return OpCode(binary.BigEndian.Uint16(b)), nil
}

// Option 은 RFC 2347 의 이름·값 쌍이다. 순서를 지키려고 map 대신 슬라이스로 둔다.
type Option struct {
	Name  string
	Value string
}

// Request 는 RRQ 또는 WRQ 다.
//
//	| opcode 2 | filename | 0 | mode | 0 | opt1 | 0 | value1 | 0 | ...
type Request struct {
	Op       OpCode
	Filename string
	Mode     string
	Options  []Option
}

func (r Request) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, r.Op)
	fields := []string{r.Filename, r.Mode}
	for _, o := range r.Options {
		fields = append(fields, o.Name, o.Value)
	}
	for _, f := range fields {
		b.WriteString(f)
		b.WriteByte(0)
	}
	return b.Bytes(), nil
}

// UnmarshalBinary 는 RRQ·WRQ 를 해석한다. 모드와 옵션 이름은 대소문자를
// 가리지 않으므로 (RFC 1350·2347) 소문자로 바꿔 둔다.
func (r *Request) UnmarshalBinary(p []byte) error {
	op, err := PeekOpCode(p)
	if err != nil {
		return err
	}
	if op != OpRRQ && op != OpWRQ {
		return fmt.Errorf("%w: %s", errOpcode, op)
	}
	fields, err := splitNul(p[2:])
	if err != nil {
		return err
	}
	if len(fields) < 2 {
		return errShort
	}
	if fields[0] == "" {
		return errors.New("empty filename")
	}
	if (len(fields)-2)%2 != 0 {
		return errOddOpts
	}
	r.Op = op
	r.Filename = fields[0]
	r.Mode = strings.ToLower(fields[1])
	r.Options = nil
	for i := 2; i < len(fields); i += 2 {
		r.Options = append(r.Options, Option{Name: strings.ToLower(fields[i]), Value: fields[i+1]})
	}
	return nil
}

// Data 는 블록 하나다. Payload 는 0 ~ 협상한 블록 크기 바이트다.
//
//	| opcode 2 | block 2 | payload |
type Data struct {
	Block   uint16
	Payload []byte
}

func (d Data) MarshalBinary() ([]byte, error) {
	b := make([]byte, HeaderSize+len(d.Payload))
	binary.BigEndian.PutUint16(b, uint16(OpData))
	binary.BigEndian.PutUint16(b[2:], d.Block)
	copy(b[HeaderSize:], d.Payload)
	return b, nil
}

// UnmarshalBinary 의 Payload 는 p 를 복사하지 않고 가리킨다.
// p 를 다시 쓰기 전에 Payload 를 다 써야 한다.
func (d *Data) UnmarshalBinary(p []byte) error {
	op, err := PeekOpCode(p)
	if err != nil {
		return err
	}
	if op != OpData {
		return fmt.Errorf("%w: %s", errOpcode, op)
	}
	if len(p) < HeaderSize {
		return errShort
	}
	d.Block = binary.BigEndian.Uint16(p[2:])
	d.Payload = p[HeaderSize:]
	return nil
}

// Ack 는 받은 블록 번호 하나다. 고정 4바이트.
//
//	| opcode 2 | block 2 |
type Ack uint16

func (a Ack) MarshalBinary() ([]byte, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b, uint16(OpAck))
	binary.BigEndian.PutUint16(b[2:], uint16(a))
	return b, nil
}

func (a *Ack) UnmarshalBinary(p []byte) error {
	op, err := PeekOpCode(p)
	if err != nil {
		return err
	}
	if op != OpAck {
		return fmt.Errorf("%w: %s", errOpcode, op)
	}
	if len(p) != 4 {
		return fmt.Errorf("ACK must be 4 bytes, got %d", len(p))
	}
	*a = Ack(binary.BigEndian.Uint16(p[2:]))
	return nil
}

// Err 는 전송을 끝내는 ERROR 패킷이다.
//
//	| opcode 2 | code 2 | message | 0 |
type Err struct {
	Code    ErrCode
	Message string
}

func (e Err) Error() string { return fmt.Sprintf("tftp error %d: %s", e.Code, e.Message) }

func (e Err) MarshalBinary() ([]byte, error) {
	b := make([]byte, 4, 5+len(e.Message))
	binary.BigEndian.PutUint16(b, uint16(OpErr))
	binary.BigEndian.PutUint16(b[2:], uint16(e.Code))
	b = append(b, e.Message...)
	return append(b, 0), nil
}

func (e *Err) UnmarshalBinary(p []byte) error {
	op, err := PeekOpCode(p)
	if err != nil {
		return err
	}
	if op != OpErr {
		return fmt.Errorf("%w: %s", errOpcode, op)
	}
	if len(p) < 5 {
		return errShort
	}
	if p[len(p)-1] != 0 {
		return errNoNul
	}
	e.Code = ErrCode(binary.BigEndian.Uint16(p[2:]))
	e.Message = string(p[4 : len(p)-1])
	return nil
}

// OAck 는 서버가 받아들인 옵션을 돌려주는 패킷이다 (RFC 2347).
//
//	| opcode 2 | opt1 | 0 | value1 | 0 | ...
type OAck struct {
	Options []Option
}

func (o OAck) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, OpOAck)
	for _, opt := range o.Options {
		b.WriteString(opt.Name)
		b.WriteByte(0)
		b.WriteString(opt.Value)
		b.WriteByte(0)
	}
	return b.Bytes(), nil
}

func (o *OAck) UnmarshalBinary(p []byte) error {
	op, err := PeekOpCode(p)
	if err != nil {
		return err
	}
	if op != OpOAck {
		return fmt.Errorf("%w: %s", errOpcode, op)
	}
	fields, err := splitNul(p[2:])
	if err != nil {
		return err
	}
	if len(fields)%2 != 0 {
		return errOddOpts
	}
	o.Options = nil
	for i := 0; i < len(fields); i += 2 {
		o.Options = append(o.Options, Option{Name: strings.ToLower(fields[i]), Value: fields[i+1]})
	}
	return nil
}

// splitNul 은 0 바이트로 끝나는 문자열들을 차례로 자른다.
// 마지막 필드 뒤에 0 바이트가 없으면 에러다.
func splitNul(p []byte) ([]string, error) {
	var out []string
	for len(p) > 0 {
		i := bytes.IndexByte(p, 0)
		if i < 0 {
			return nil, errNoNul
		}
		out = append(out, string(p[:i]))
		p = p[i+1:]
	}
	return out, nil
}

// Rollover 는 블록 번호가 65535 를 넘을 때의 정책이다. RFC 1350 은 이 경우를 정하지 않았다.
type Rollover string

const (
	// RolloverZero 는 65535 다음을 0 으로 센다 (uint16 덧셈이 넘친 그대로).
	RolloverZero Rollover = "zero"
	// RolloverOne 은 65535 다음을 1 로 센다.
	RolloverOne Rollover = "one"
	// RolloverRefuse 는 65535 블록 안에 끝나지 않는 전송을 ERROR 로 거절한다.
	RolloverRefuse Rollover = "refuse"
)

// ParseRollover 는 플래그 문자열을 정책으로 바꾼다.
func ParseRollover(s string) (Rollover, error) {
	switch Rollover(s) {
	case RolloverZero, RolloverOne, RolloverRefuse:
		return Rollover(s), nil
	}
	return "", fmt.Errorf("rollover must be zero, one or refuse: %q", s)
}

// WireBlock 은 1부터 세는 절대 블록 번호 n 을 패킷에 실을 uint16 번호로 바꾼다.
// n 이 0 이면 0 이다 (OACK 에 대한 ACK 0).
func (r Rollover) WireBlock(n uint64) uint16 {
	if n == 0 {
		return 0
	}
	if r == RolloverOne {
		return uint16((n-1)%65535 + 1)
	}
	return uint16(n)
}
