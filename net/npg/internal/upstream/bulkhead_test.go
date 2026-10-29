package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// recorder 는 받은 요청 수만 세고 빈 200 을 돌려주는 가짜 풀이다.
type recorder struct{ got int }

func (r *recorder) RoundTrip(*http.Request) (*http.Response, error) {
	r.got++
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name, method, target string
		search               bool
	}{
		{"검색은 검색 풀로", http.MethodGet, "/v1/devices?q=sw", true},
		{"q 가 비어 있어도 검색이다", http.MethodGet, "/v1/devices?q=", true},
		{"q 없는 목록 조회는 나머지 풀로", http.MethodGet, "/v1/devices", false},
		{"다른 질의 키는 검색이 아니다", http.MethodGet, "/v1/devices?page=2", false},
		{"단건 조회는 나머지 풀로", http.MethodGet, "/v1/devices/42", false},
		{"같은 경로라도 POST 는 나머지 풀로", http.MethodPost, "/v1/devices?q=sw", false},
		{"경로 끝 / 는 다른 경로다", http.MethodGet, "/v1/devices/?q=sw", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given: 두 풀 자리에 세기만 하는 가짜를 넣는다
			search, rest := &recorder{}, &recorder{}
			s := split{search: search, rest: rest}

			// When
			_, err := s.RoundTrip(httptest.NewRequest(c.method, "http://backend"+c.target, nil))

			// Then: 정확히 한쪽 풀만 요청을 받는다
			require.NoError(t, err)
			if c.search {
				require.Equal(t, [2]int{1, 0}, [2]int{search.got, rest.got})
			} else {
				require.Equal(t, [2]int{0, 1}, [2]int{search.got, rest.got})
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("검색 풀 8칸·줄 7, 나머지 풀 24칸·줄 1024 로 나뉜다", func(t *testing.T) {
		// When
		s, ok := New().(split)

		// Then: 풀마다 limit 이 Transport 를 감싸고, 입장권 수 = 칸 + 줄
		require.True(t, ok)
		search, rest := s.search.(limit), s.rest.(limit)
		require.Equal(t, searchConns, search.next.(*http.Transport).MaxConnsPerHost)
		require.Equal(t, restConns, rest.next.(*http.Transport).MaxConnsPerHost)
		require.Equal(t, 8+7, cap(search.slots))
		require.Equal(t, 24+1024, cap(rest.slots))
	})
}
