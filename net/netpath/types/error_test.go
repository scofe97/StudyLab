package types

import "testing"

// 값 열세 개가 손으로 적은 문자열이라, 컴파일러가 봐 주지 않는 둘만 막는다.
// 빈 문자열은 `omitempty` 가 필드째로 삭제하고, 중복은 서로 다른 원인을
// 같은 이름으로 보고하게 만든다. 둘 다 컴파일과 실행을 통과하면서 조용히 틀린다.
//
// 아래 목록에 넣은 것만 본다. 새 상수를 선언하고 목록에 안 넣으면 이 테스트는 그대로 통과한다.
func TestErrorKindValues(t *testing.T) {
	// Given — 선언된 오류 분류 전부
	all := []ErrorKind{
		ErrDNSNotFound, ErrDNSTimeout, ErrDNSOther,
		ErrTCPRefused, ErrTCPTimeout, ErrTCPUnreachable, ErrTCPOther,
		ErrHTTPStatus, ErrHTTPTimeout, ErrHTTPProtocol,
		ErrCanceled, ErrInvalidTarget, ErrOther,
	}

	// When · Then — 하나씩 훑으며 빈 값과 중복을 본다
	seen := make(map[ErrorKind]bool, len(all))
	for i, k := range all {
		if k == "" {
			t.Errorf("all[%d]: 빈 문자열은 omitempty 가 삭제한다", i)
		}
		if seen[k] {
			t.Errorf("%q: 값이 중복됐다", k)
		}
		seen[k] = true
	}
}
