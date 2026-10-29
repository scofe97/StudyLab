package tftp

import "io"

// netascii 는 RFC 1350 이 Telnet 규약(RFC 854)에서 빌려 온 전송 모드다.
// 보내는 쪽은 LF 를 CR LF 로, CR 을 CR NUL 로 바꿔 보내고
// 받는 쪽은 그 반대로 되돌린다.

// NetasciiEncoder 는 원본 바이트를 netascii 로 바꿔 읽어 주는 io.Reader 다.
type NetasciiEncoder struct {
	src   io.Reader
	in    []byte
	stage []byte
	err   error
}

func NewNetasciiEncoder(src io.Reader) *NetasciiEncoder {
	return &NetasciiEncoder{src: src, in: make([]byte, 4096)}
}

func (e *NetasciiEncoder) Read(p []byte) (int, error) {
	for len(e.stage) == 0 {
		if e.err != nil {
			return 0, e.err
		}
		n, err := e.src.Read(e.in)
		e.stage = e.stage[:0]
		for _, c := range e.in[:n] {
			switch c {
			case '\n':
				e.stage = append(e.stage, '\r', '\n')
			case '\r':
				e.stage = append(e.stage, '\r', 0)
			default:
				e.stage = append(e.stage, c)
			}
		}
		e.err = err
	}
	n := copy(p, e.stage)
	e.stage = e.stage[n:]
	return n, nil
}

// NetasciiDecoder 는 netascii 바이트를 받아 원래 바이트로 되돌려 w 에 쓰는 io.Writer 다.
// CR 이 블록 경계에서 끊길 수 있으므로 CR 하나를 다음 Write 까지 들고 있는다.
// 마지막에 Close 로 들고 있던 CR 을 내보낸다.
type NetasciiDecoder struct {
	w   io.Writer
	cr  bool
	out []byte
}

func NewNetasciiDecoder(w io.Writer) *NetasciiDecoder {
	return &NetasciiDecoder{w: w}
}

func (d *NetasciiDecoder) Write(p []byte) (int, error) {
	d.out = d.out[:0]
	for _, c := range p {
		if d.cr {
			d.cr = false
			switch c {
			case '\n':
				d.out = append(d.out, '\n')
				continue
			case 0:
				d.out = append(d.out, '\r')
				continue
			default:
				// 규약에 없는 CR 뒤 바이트는 CR 을 그대로 두고 그 바이트를 이어서 처리한다
				d.out = append(d.out, '\r')
			}
		}
		if c == '\r' {
			d.cr = true
			continue
		}
		d.out = append(d.out, c)
	}
	if _, err := d.w.Write(d.out); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (d *NetasciiDecoder) Close() error {
	if !d.cr {
		return nil
	}
	d.cr = false
	_, err := d.w.Write([]byte{'\r'})
	return err
}
