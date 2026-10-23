package tftp

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// newRoot 는 루트 안에 파일·디렉터리·링크를, 루트 밖에 비밀 파일을 만든 뒤 루트를 엽니다.
func newRoot(t *testing.T) *os.Root {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "root")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("ok"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "in.txt"), []byte("in"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(base, "secret.txt"), []byte("secret"), 0o644))
	require.NoError(t, os.Symlink("sub/in.txt", filepath.Join(dir, "link-in")))
	require.NoError(t, os.Symlink("../secret.txt", filepath.Join(dir, "link-out")))

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func TestOpenFileReadsRegularFile(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"a.txt", "ok"},
		{"sub/in.txt", "in"},
		{"sub/../a.txt", "ok"},
		{"link-in", "in"},
		{"/a.txt", "ok"},
		{"///a.txt", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given — 루트 안의 파일과 안쪽을 가리키는 링크
			root := newRoot(t)

			// When — 요청 이름으로 연다
			f, err := openFile(root, tt.name)

			// Then — 내용이 읽히고, 앞의 / 는 루트 안 이름으로 풀린다
			require.NoError(t, err)
			defer f.Close()
			got, err := io.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func TestOpenFileRejectsEscapes(t *testing.T) {
	tests := []string{
		"../secret.txt",
		"../../../../../../etc/passwd",
		"sub/../../secret.txt",
		"link-out",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			// Given — 루트 밖에 비밀 파일이 있다
			root := newRoot(t)

			// When — 밖으로 나가는 이름이나 밖을 가리키는 링크를 연다
			f, err := openFile(root, name)

			// Then — 열리지 않는다
			require.Error(t, err)
			require.Nil(t, f)
		})
	}
}

func TestOpenFileRejectsNonRegular(t *testing.T) {
	// Given — 루트 안의 디렉터리
	root := newRoot(t)

	// When
	f, err := openFile(root, "sub")

	// Then — os.Root 는 열어 주지만 일반 파일이 아니라서 거절한다
	require.ErrorIs(t, err, ErrNotRegular)
	require.Nil(t, f)
}

func TestOpenFileMissingAndMalformed(t *testing.T) {
	// Given
	root := newRoot(t)

	// When
	_, errMissing := openFile(root, "none.bin")
	_, errNul := openFile(root, "a.txt\x00x")
	_, errEmpty := openFile(root, "/")

	// Then — 없는 파일은 ErrNotExist, 이상한 이름은 오류
	require.True(t, errors.Is(errMissing, fs.ErrNotExist))
	require.Error(t, errNul)
	require.Error(t, errEmpty)
}
