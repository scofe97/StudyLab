package ctrl

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteFrameWritesBigEndianLengthThenBody(t *testing.T) {
	// Given — 4바이트 본문 "ping"
	var buf bytes.Buffer

	// When — 프레임으로 쓴다
	err := WriteFrame(&buf, []byte("ping"))

	// Then — 헤더는 빅엔디언 길이 4, 뒤에 본문이 그대로 붙는다
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 4, 'p', 'i', 'n', 'g'}, buf.Bytes())
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"빈 본문", []byte{}},
		{"ping", []byte("ping")},
		{"줄바꿈이 든 본문", []byte("a\nb\n")},
		{"상한 크기 본문", bytes.Repeat([]byte{'x'}, MaxFrameSize)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 쓰기로 채운 버퍼
			var buf bytes.Buffer
			require.NoError(t, WriteFrame(&buf, tt.body))

			// When — 읽는다
			got, err := ReadFrame(&buf)

			// Then — 같은 본문이 돌아오고 남는 바이트가 없다
			require.NoError(t, err)
			require.Equal(t, tt.body, got)
			require.Zero(t, buf.Len())
		})
	}
}

func TestWriteFrameRejectsBodyOverLimit(t *testing.T) {
	// Given — 상한보다 1바이트 큰 본문
	var buf bytes.Buffer
	body := make([]byte, MaxFrameSize+1)

	// When
	err := WriteFrame(&buf, body)

	// Then — 오류이고 아무것도 쓰지 않는다
	require.ErrorIs(t, err, ErrFrameTooLarge)
	require.Zero(t, buf.Len())
}

func TestReadFrameRejectsForgedLengthBeforeReadingBody(t *testing.T) {
	tests := []struct {
		name string
		size uint32
	}{
		{"상한보다 1 큼", MaxFrameSize + 1},
		{"uint32 최댓값", 1<<32 - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 헤더만 있고 본문은 없는 입력
			var header [HeaderSize]byte
			binary.BigEndian.PutUint32(header[:], tt.size)
			r := bytes.NewReader(header[:])

			// When
			got, err := ReadFrame(r)

			// Then — 본문을 읽으려 하기 전에 상한 오류(본문이 없는데 UnexpectedEOF 가 아님)
			require.ErrorIs(t, err, ErrFrameTooLarge)
			require.Nil(t, got)
		})
	}
}

func TestReadFrameReportsTruncatedInput(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{"입력이 비어 있음", []byte{}, io.EOF},
		{"헤더가 반만 옴", []byte{0, 0}, io.ErrUnexpectedEOF},
		{"본문이 길이보다 짧음", []byte{0, 0, 0, 4, 'p', 'i'}, io.ErrUnexpectedEOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 중간에 끊긴 입력
			r := bytes.NewReader(tt.input)

			// When
			_, err := ReadFrame(r)

			// Then
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
