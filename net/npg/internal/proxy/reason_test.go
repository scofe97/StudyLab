package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// upstreamErr 는 진짜 Transport 로 요청을 보내 실제 에러 모양을 얻습니다.
// 에러 값을 손으로 만들면 Go 가 실제로 감싸는 껍질(url.Error 등)을 놓치기 때문입니다.
func upstreamErr(t *testing.T, tr *http.Transport, url string) error {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err)
	return err
}

func TestClassify(t *testing.T) {
	t.Run("에러가 없으면 실패 없음", func(t *testing.T) {
		// Given
		ctx := context.Background()

		// When
		got := classify(ctx, nil)

		// Then
		require.Equal(t, ReasonNone, got)
	})

	t.Run("손님이 떠났으면 업스트림 에러 모양과 상관없이 DC", func(t *testing.T) {
		// Given — 들어온 요청의 context 가 취소됐다
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// When — 업스트림 쪽 에러는 연결 끊김처럼 보여도
		got := classify(ctx, errors.New("read: connection reset by peer"))

		// Then
		require.Equal(t, ReasonClientGone, got)
	})

	t.Run("닫힌 포트로 연결하면 UF", func(t *testing.T) {
		// Given — 열었다 바로 닫아 아무도 듣지 않는 주소
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := ln.Addr().String()
		ln.Close()

		// When
		err = upstreamErr(t, &http.Transport{}, "http://"+addr+"/")
		got := classify(context.Background(), err)

		// Then
		require.Equal(t, ReasonConnFail, got)
	})

	t.Run("응답 헤더가 기한 안에 안 오면 UT", func(t *testing.T) {
		// Given — 헤더를 늦게 주는 업스트림과 짧은 ResponseHeaderTimeout
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
		}))
		defer srv.Close()
		defer close(release)
		tr := &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond}

		// When
		got := classify(context.Background(), upstreamErr(t, tr, srv.URL))

		// Then
		require.Equal(t, ReasonTimeout, got)
	})

	t.Run("업스트림이 응답 없이 연결을 끊으면 UC", func(t *testing.T) {
		// Given — 요청을 받자마자 TCP 연결을 닫는 업스트림
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
		}))
		defer srv.Close()

		// When
		got := classify(context.Background(), upstreamErr(t, &http.Transport{}, srv.URL))

		// Then
		require.Equal(t, ReasonConnReset, got)
	})
}

func TestReasonStatus(t *testing.T) {
	tests := []struct {
		r    Reason
		want int
	}{
		{ReasonTimeout, http.StatusGatewayTimeout},
		{ReasonConnFail, http.StatusBadGateway},
		{ReasonConnReset, http.StatusBadGateway},
		{ReasonClientGone, 499},
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			// Given — 원인 코드 하나
			// When
			got := tt.r.Status()

			// Then — 시간 초과만 504, 손님 이탈은 499, 나머지 업스트림 실패는 502
			require.Equal(t, tt.want, got)
		})
	}
}
