package tftp

import (
	"errors"
	"os"
	"strings"
)

// ErrNotRegular는 요청한 이름이 일반 파일이 아닐 때의 오류입니다.
var ErrNotRegular = errors.New("일반 파일이 아닙니다")

// openFile은 루트 안의 일반 파일을 읽기 전용으로 엽니다 (ADR 0010).
func openFile(root *os.Root, name string) (*os.File, error) {
	// os.Root 는 절대 경로를 거절하므로 앞의 / 는 떼고 루트 안 이름으로 읽는다
	name = strings.TrimLeft(name, "/")
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, ErrNotRegular
	}
	return f, nil
}
