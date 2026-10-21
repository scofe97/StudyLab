package ctrl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// 길이 필드는 4바이트이고 본문 상항은 64Kib다.
	HeaderSize   = 4
	MaxFrameSize = 64 * 1024
)

var ErrFrameTooLarge = errors.New("프레임 본문이 상한을 넘었습니다.")

// WriteFrame은 길이 접두를 붙여 body를 한 프레임으로 씁니다.
func WriteFrame(w io.Writer, body []byte) error {
	if len(body) > MaxFrameSize {
		return ErrFrameTooLarge
	}

	buf := make([]byte, HeaderSize+len(body))
	binary.BigEndian.PutUint32(buf, uint32(len(body)))
	copy(buf[HeaderSize:], body)

	_, err := w.Write(buf)
	return err
}

func ReadFrame(r io.Reader) ([]byte, error) {
	// 헤더크기 만큼 읽는다.
	var header [HeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	// 헤더 전체를 읽고 크기 확인? (함수 동작이 어떻게 되는지 모르겠음 )
	size := binary.BigEndian.Uint32(header[:])
	if size > MaxFrameSize {
		return nil, fmt.Errorf("%w: %d", ErrFrameTooLarge, size)
	}

	// 헤더에서 확인한 크기로 배열 만듬
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}
