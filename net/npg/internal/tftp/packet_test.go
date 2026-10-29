package tftp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAckEncodeBytes(t *testing.T) {
	// Given — 5번 블록의 ACK
	ack := Ack{Block: 5}

	// When — 와이어 형식으로 만든다
	got := ack.Encode()

	// Then — opcode 4 와 블록 번호 5 가 빅엔디언 2바이트씩
	require.Equal(t, []byte{0, 4, 0, 5}, got)
}

func TestDataEncodeBytes(t *testing.T) {
	// Given — 3번 블록과 페이로드 두 바이트
	data := Data{Block: 3, Payload: []byte{0xAA, 0xBB}}

	// When
	got := data.Encode()

	// Then — opcode 3, 블록 번호 3, 페이로드가 그대로 이어진다
	require.Equal(t, []byte{0, 3, 0, 3, 0xAA, 0xBB}, got)
}

func TestErrorEncodeBytes(t *testing.T) {
	// Given — 파일 없음 오류와 메시지
	e := ErrorPacket{Code: ErrFileNotFound, Message: "no"}

	// When
	got := e.Encode()

	// Then — opcode 5, 코드 1, 메시지, 종결자 0 하나
	require.Equal(t, []byte{0, 5, 0, 1, 'n', 'o', 0}, got)
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		pkt  Packet
	}{
		{"ack", Ack{Block: 65535}},
		{"data", Data{Block: 65535, Payload: []byte("abc")}},
		{"data 빈 페이로드", Data{Block: 1, Payload: []byte{}}},
		{"rrq 옵션 없음", ReadReq{Filename: "fw.bin", Mode: "octet"}},
		{"rrq 옵션 있음", ReadReq{
			Filename: "fw.bin", Mode: "octet",
			Options: map[string]string{"blksize": "1428", "timeout": "2"},
		}},
		{"error", ErrorPacket{Code: ErrAccessDenied, Message: "denied"}},
		{"oack", OAck{Options: map[string]string{"blksize": "1428"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 패킷 하나와 그것을 만든 바이트
			wire := tt.pkt.Encode()

			// When — 바이트를 다시 해석한다
			got, err := Decode(wire)

			// Then — 같은 패킷이 돌아온다
			require.NoError(t, err)
			require.Equal(t, tt.pkt, got)
		})
	}
}

func TestDecodeRRQLowercasesModeAndOptions(t *testing.T) {
	// Given — 모드와 옵션 이름이 대문자인 RRQ
	wire := []byte{0, 1}
	wire = append(wire, "fw.bin\x00OCTET\x00BlkSize\x001428\x00"...)

	// When
	got, err := Decode(wire)

	// Then — 모드와 옵션 이름은 소문자, 파일 이름과 옵션 값은 그대로
	require.NoError(t, err)
	require.Equal(t, ReadReq{
		Filename: "fw.bin", Mode: "octet",
		Options: map[string]string{"blksize": "1428"},
	}, got)
}

func TestDecodeRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		wire []byte
		want error
	}{
		{"비어 있음", nil, ErrShortPacket},
		{"opcode 한 바이트", []byte{0}, ErrShortPacket},
		{"알 수 없는 opcode", []byte{0, 9, 0, 0}, ErrBadOpcode},
		{"DATA 헤더 부족", []byte{0, 3, 0}, ErrShortPacket},
		{"ACK 가 너무 김", []byte{0, 4, 0, 1, 0}, ErrShortPacket},
		{"RRQ 종결자 없음", append([]byte{0, 1}, "fw.bin\x00octet"...), ErrBadRequest},
		{"RRQ 모드 없음", append([]byte{0, 1}, "fw.bin\x00"...), ErrBadRequest},
		{"RRQ 옵션 값 없음", append([]byte{0, 1}, "fw.bin\x00octet\x00blksize\x00"...), ErrBadRequest},
		{"ERROR 종결자 없음", []byte{0, 5, 0, 1, 'n', 'o'}, ErrBadRequest},
		{"ERROR 메시지 자리 없음", []byte{0, 5, 0, 1}, ErrBadRequest},
		{"OACK 옵션 없음", []byte{0, 6}, ErrBadRequest},
		{"OACK 값 없음", append([]byte{0, 6}, "blksize\x00"...), ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 형식이 틀린 바이트
			// When
			got, err := Decode(tt.wire)

			// Then — 패킷 없이 정해진 오류가 난다
			require.ErrorIs(t, err, tt.want)
			require.Nil(t, got)
		})
	}
}
