package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{name: "빈 문자열이면 아무도 믿지 않는다", in: "", want: 0},
		{name: "CIDR 하나", in: "127.0.0.1/32", want: 1},
		{name: "쉼표로 나누고 앞뒤 빈칸은 무시한다", in: "127.0.0.1/32, 10.0.0.0/8", want: 2},
		{name: "IPv6 대역", in: "::1/128", want: 1},
		{name: "Caddy 의 private_ranges 같은 별명은 받지 않는다", in: "private_ranges", wantErr: true},
		{name: "길이 없는 맨 IP 는 받지 않는다", in: "127.0.0.1", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			got, err := ParseTrustedProxies(tc.in)

			// Then
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, tc.want)
		})
	}
}

func TestTrustedProxiesTrusts(t *testing.T) {
	// Given — 루프백 두 주소만 믿는 목록
	loopback, err := ParseTrustedProxies("127.0.0.1/32, ::1/128")
	require.NoError(t, err)

	cases := []struct {
		name       string
		list       TrustedProxies
		remoteAddr string
		want       bool
	}{
		{name: "대역 안의 IPv4", list: loopback, remoteAddr: "127.0.0.1:5000", want: true},
		{name: "대역 밖의 IPv4", list: loopback, remoteAddr: "127.0.0.2:5000", want: false},
		{name: "대역 안의 IPv6", list: loopback, remoteAddr: "[::1]:5000", want: true},
		{name: "ip:port 꼴이 아니면 믿지 않는다", list: loopback, remoteAddr: "garbage", want: false},
		{name: "빈 목록은 아무도 믿지 않는다", list: nil, remoteAddr: "127.0.0.1:5000", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			got := tc.list.Trusts(tc.remoteAddr)

			// Then
			require.Equal(t, tc.want, got)
		})
	}
}

// startTrustingProxy 는 trusted 대역을 믿는 프록시를 진짜 TCP 서버로 띄운다.
// 시험 손님은 127.0.0.1 에서 붙으므로, 대역에 127.0.0.1 이 들어 있으면 손님이 곧 "믿는 앞 홉"이다.
func startTrustingProxy(t *testing.T, upstream string, trusted string) *httptest.Server {
	t.Helper()
	u, err := url.Parse(upstream)
	require.NoError(t, err)
	list, err := ParseTrustedProxies(trusted)
	require.NoError(t, err)
	h, _ := newProxy(u, zerolog.Nop(), defaultClientLimits, list)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestProxyForwardedForTrust(t *testing.T) {
	cases := []struct {
		name    string
		trusted string
		sent    []string // 앞 홉이 보낸 X-Forwarded-For 줄들
		want    []string // 업스트림이 받은 X-Forwarded-For
	}{
		{
			name:    "믿는 앞 홉이면 받은 XFF 뒤에 앞 홉 주소를 덧붙인다",
			trusted: "127.0.0.1/32",
			sent:    []string{"203.0.113.195"},
			want:    []string{"203.0.113.195, 127.0.0.1"},
		},
		{
			name:    "믿는 앞 홉이 여러 줄로 보내도 한 줄로 이어 붙인다",
			trusted: "127.0.0.1/32",
			sent:    []string{"198.51.100.7", "203.0.113.195"},
			want:    []string{"198.51.100.7, 203.0.113.195, 127.0.0.1"},
		},
		{
			name:    "믿지 않는 앞 홉이면 받은 XFF 를 버리고 앞 홉 주소만 적는다",
			trusted: "10.0.0.0/8",
			sent:    []string{"203.0.113.195"},
			want:    []string{"127.0.0.1"},
		},
		{
			name:    "목록이 비면 지금처럼 버린다",
			trusted: "",
			sent:    []string{"203.0.113.195"},
			want:    []string{"127.0.0.1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given — 받은 XFF 를 기록하는 업스트림과, trusted 대역을 믿는 프록시
			var got []string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Values("X-Forwarded-For")
			}))
			defer backend.Close()
			front := startTrustingProxy(t, backend.URL, tc.trusted)

			// When — 앞 홉(시험 손님, 127.0.0.1)이 XFF 를 실어 보낸다
			req, err := http.NewRequest(http.MethodGet, front.URL+"/v1/devices", nil)
			require.NoError(t, err)
			for _, v := range tc.sent {
				req.Header.Add("X-Forwarded-For", v)
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			// Then
			require.Equal(t, tc.want, got)
		})
	}
}
