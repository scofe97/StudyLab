package tftp

import (
	"bytes"
	"io"
	"math/rand/v2"
	"testing"
)

func TestNetasciiRoundTrip(t *testing.T) {
	// Given — CR·LF·NUL 이 자주 섞인 바이트 64KiB
	rng := rand.New(rand.NewPCG(1, 2))
	src := make([]byte, 64*1024)
	alphabet := []byte{'\r', '\n', 0, 'a', 0xFF}
	for i := range src {
		src[i] = alphabet[rng.IntN(len(alphabet))]
	}

	// When — 인코더로 작은 조각씩 읽고, 디코더에 제각각 크기로 나눠 쓴다
	enc := NewNetasciiEncoder(bytes.NewReader(src))
	var wire bytes.Buffer
	chunk := make([]byte, 7)
	for {
		n, err := enc.Read(chunk)
		wire.Write(chunk[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	dec := NewNetasciiDecoder(&out)
	w := wire.Bytes()
	for len(w) > 0 {
		k := min(len(w), 1+rng.IntN(13))
		if _, err := dec.Write(w[:k]); err != nil {
			t.Fatal(err)
		}
		w = w[k:]
	}
	if err := dec.Close(); err != nil {
		t.Fatal(err)
	}

	// Then — 원래 바이트가 그대로 돌아온다
	if !bytes.Equal(out.Bytes(), src) {
		t.Fatal("netascii 왕복 뒤 바이트가 다르다")
	}
	t.Logf("원본 %d바이트 → 전송 %d바이트 → 복원 %d바이트", len(src), wire.Len(), out.Len())
}
