package announce

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeWritesVersionTypeNonceThenBody(t *testing.T) {
	// Given — nonce 0x0102030405060708 인 HELLO, 본문 "dev1"
	m := Message{Type: TypeHello, Nonce: 0x0102030405060708, Body: []byte("dev1")}

	// When — 와이어 형식으로 만든다
	got, err := m.Encode()

	// Then — 버전 1, 타입 1, 빅엔디언 nonce 8바이트, 본문이 그대로 붙는다
	require.NoError(t, err)
	require.Equal(t, []byte{1, 1, 1, 2, 3, 4, 5, 6, 7, 8, 'd', 'e', 'v', '1'}, got)
}

func TestMessageRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
	}{
		{"HELLO", Message{Type: TypeHello, Nonce: 42, Body: []byte("dev1")}},
		{"본문 없는 ACK", Message{Type: TypeAck, Nonce: 42, Body: []byte{}}},
		{"상한 크기 본문", Message{Type: TypeHello, Nonce: 1, Body: bytes.Repeat([]byte{'x'}, MaxAnnounceSize-HeaderSize)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 인코딩된 바이트
			data, err := tt.msg.Encode()
			require.NoError(t, err)

			// When — 다시 읽는다
			got, err := Decode(data)

			// Then — 원래 메시지와 같다
			require.NoError(t, err)
			require.Equal(t, tt.msg, got)
		})
	}
}

func TestEncodeRejectsBodyOverLimit(t *testing.T) {
	// Given — 전체 크기가 상한보다 1바이트 큰 메시지
	m := Message{Type: TypeHello, Body: bytes.Repeat([]byte{'x'}, MaxAnnounceSize-HeaderSize+1)}

	// When — 인코딩한다
	_, err := m.Encode()

	// Then — 상한 초과 오류
	require.ErrorIs(t, err, ErrTooLarge)
}

func TestDecodeRejectsMalformedData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"헤더보다 1바이트 짧음", make([]byte, HeaderSize-1), ErrShortMessage},
		{"알 수 없는 버전", []byte{9, 1, 0, 0, 0, 0, 0, 0, 0, 0}, ErrBadVersion},
		{"알 수 없는 타입", []byte{1, 7, 0, 0, 0, 0, 0, 0, 0, 0}, ErrUnknownType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 형식이 어긋난 바이트 (tt.data)

			// When — 읽는다
			_, err := Decode(tt.data)

			// Then — 어긋난 곳에 맞는 오류
			require.ErrorIs(t, err, tt.want)
		})
	}
}
