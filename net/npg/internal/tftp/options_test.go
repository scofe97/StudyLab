package tftp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNegotiate(t *testing.T) {
	srv := &Server{Timeout: time.Second, MaxBlksize: 1428}
	tests := []struct {
		name        string
		opts        map[string]string
		wantBlksize int
		wantTimeout time.Duration
		wantOAck    map[string]string
	}{
		{"옵션 없음", nil, 512, time.Second, map[string]string{}},
		{"blksize 가 상한보다 작음", map[string]string{"blksize": "1024"}, 1024, time.Second,
			map[string]string{"blksize": "1024"}},
		{"blksize 가 상한보다 큼은 상한으로 줄임", map[string]string{"blksize": "8192"}, 1428, time.Second,
			map[string]string{"blksize": "1428"}},
		{"blksize 가 범위 밖이면 무시", map[string]string{"blksize": "7"}, 512, time.Second, map[string]string{}},
		{"blksize 가 65464 초과면 무시", map[string]string{"blksize": "65465"}, 512, time.Second, map[string]string{}},
		{"blksize 가 숫자가 아니면 무시", map[string]string{"blksize": "abc"}, 512, time.Second, map[string]string{}},
		{"timeout 받아들임", map[string]string{"timeout": "5"}, 512, 5 * time.Second,
			map[string]string{"timeout": "5"}},
		{"timeout 범위 밖이면 무시", map[string]string{"timeout": "0"}, 512, time.Second, map[string]string{}},
		{"timeout 256 이면 무시", map[string]string{"timeout": "256"}, 512, time.Second, map[string]string{}},
		{"tsize 0 이면 파일 크기로 답함", map[string]string{"tsize": "0"}, 512, time.Second,
			map[string]string{"tsize": "4096"}},
		{"tsize 가 0 이 아니면 무시", map[string]string{"tsize": "100"}, 512, time.Second, map[string]string{}},
		{"모르는 옵션은 뺌", map[string]string{"windowsize": "8", "foo": "1"}, 512, time.Second, map[string]string{}},
		{"여럿", map[string]string{"blksize": "1428", "timeout": "2", "tsize": "0", "windowsize": "8"}, 1428, 2 * time.Second,
			map[string]string{"blksize": "1428", "timeout": "2", "tsize": "4096"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 서버 설정(타임아웃 1초, 상한 1428)과 크기 4096 의 파일
			// When — 클라이언트 옵션을 협상한다
			got := srv.negotiate(tt.opts, 4096)

			// Then — 받아들인 것만 OACK 에 들어가고 전송 값이 정해진다
			require.Equal(t, tt.wantBlksize, got.blksize)
			require.Equal(t, tt.wantTimeout, got.timeout)
			require.Equal(t, tt.wantOAck, got.oack)
		})
	}
}
