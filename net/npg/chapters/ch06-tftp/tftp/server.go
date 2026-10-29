package tftp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// NetasciiPolicy 는 클라이언트가 netascii 모드로 요청했을 때 서버가 할 일이다.
type NetasciiPolicy string

const (
	// NetasciiConvert 는 RFC 1350 대로 LF·CR 을 바꿔 보낸다.
	NetasciiConvert NetasciiPolicy = "convert"
	// NetasciiRaw 는 모드를 무시하고 파일 바이트를 그대로 보낸다.
	NetasciiRaw NetasciiPolicy = "raw"
	// NetasciiReject 는 ERROR 로 거절한다.
	NetasciiReject NetasciiPolicy = "reject"
)

func ParseNetasciiPolicy(s string) (NetasciiPolicy, error) {
	switch NetasciiPolicy(s) {
	case NetasciiConvert, NetasciiRaw, NetasciiReject:
		return NetasciiPolicy(s), nil
	}
	return "", fmt.Errorf("netascii must be convert, raw or reject: %q", s)
}

// 책 Listing 의 Server 기본값과 같은 값이다.
const (
	DefaultTimeout = 6 * time.Second
	DefaultRetries = 10
)

// Server 는 루트 폴더의 파일을 이름으로 내주는 읽기 전용 TFTP 서버다.
// 0 값 필드는 Serve 가 기본값으로 채운다.
type Server struct {
	// Root 는 내줄 파일이 있는 폴더다.
	Root string
	// Timeout 은 DATA(또는 창)를 보낸 뒤 ACK 를 기다리는 시간이다. 기본 6초 (책과 같다).
	Timeout time.Duration
	// Retries 는 같은 블록(창)을 보내는 최대 횟수다. 기본 10 (책과 같다).
	Retries int
	// Options 가 true 면 RFC 2347 옵션 협상(OACK)을 한다. false 면 옵션을 무시한다.
	Options bool
	// MaxBlockSize 는 blksize 협상에서 받아들일 상한이다. 기본 65464 (RFC 2348 상한).
	MaxBlockSize int
	// MaxWindowSize 는 windowsize 협상에서 받아들일 상한이다. 기본 64.
	MaxWindowSize int
	// Rollover 는 블록 번호가 65535 를 넘을 때의 정책이다. 기본 zero.
	Rollover Rollover
	// Netascii 는 netascii 모드 요청을 다루는 방법이다. 기본 convert.
	Netascii NetasciiPolicy
	// LegacyDupAck 가 true 면 책 Listing 의 재전송 루프를 쓴다.
	// 지난 블록의 ACK 를 받으면 현재 블록(창)을 곧바로 다시 보내고 재시도 횟수를 하나 쓴다.
	// false 면 지난 ACK 는 세기만 하고 버리며, 타임아웃에서만 다시 보낸다 (RFC 1123 §4.2.3.1).
	LegacyDupAck bool
	// UnsafeOpen 이 true 면 os.Root 대신 filepath.Join(Root, 이름) 을 os.Open 으로 연다.
	UnsafeOpen bool
	// Log 는 전송마다 한 줄씩 남길 곳이다. nil 이면 버린다.
	Log *log.Logger

	root *os.Root
}

func (s *Server) setDefaults() {
	if s.Timeout <= 0 {
		s.Timeout = DefaultTimeout
	}
	if s.Retries <= 0 {
		s.Retries = DefaultRetries
	}
	if s.MaxBlockSize <= 0 || s.MaxBlockSize > MaxBlockSize {
		s.MaxBlockSize = MaxBlockSize
	}
	if s.MaxWindowSize <= 0 {
		s.MaxWindowSize = 64
	}
	if s.MaxWindowSize > MaxWindowSize {
		s.MaxWindowSize = MaxWindowSize
	}
	if s.Rollover == "" {
		s.Rollover = RolloverZero
	}
	if s.Netascii == "" {
		s.Netascii = NetasciiConvert
	}
	if s.Log == nil {
		s.Log = log.New(io.Discard, "", 0)
	}
}

// ListenAndServe 는 addr 에 UDP 리스너를 열고 Serve 한다.
func (s *Server) ListenAndServe(addr string) error {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	return s.Serve(pc)
}

// Serve 는 pc 로 들어오는 요청마다 고루틴을 띄운다. pc 가 닫히면 nil 로 돌아온다.
// 진행 중인 전송은 각자의 소켓으로 계속 간다.
func (s *Server) Serve(pc net.PacketConn) error {
	s.setDefaults()
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	s.root = root

	buf := make([]byte, 2048)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		pkt := append([]byte(nil), buf[:n]...)
		go s.handle(pc.LocalAddr(), addr, pkt)
	}
}

// record 는 전송 한 건의 한 줄 로그에 들어갈 값이다.
type record struct {
	client, file, mode string
	blksize, window    int
	bytes              int64
	blocks             uint64
	sent               int
	timeouts           int
	resendTimeout      int
	resendDupAck       int
	resendSeqErr       int
	dupAcks            int
	badPackets         int
	result             string
}

func (s *Server) logRecord(r *record, elapsed time.Duration) {
	rate := "-"
	if elapsed > 0 {
		rate = fmt.Sprintf("%.1fKiB/s", float64(r.bytes)/1024/elapsed.Seconds())
	}
	s.Log.Printf("client=%s file=%q mode=%s blksize=%d window=%d bytes=%d blocks=%d sent=%d "+
		"timeouts=%d resend_timeout=%d resend_dupack=%d resend_seqerr=%d dupack=%d bad=%d "+
		"elapsed=%s rate=%s result=%s",
		r.client, r.file, r.mode, r.blksize, r.window, r.bytes, r.blocks, r.sent,
		r.timeouts, r.resendTimeout, r.resendDupAck, r.resendSeqErr, r.dupAcks, r.badPackets,
		elapsed.Round(time.Millisecond), rate, r.result)
}

func (s *Server) handle(local, client net.Addr, pkt []byte) {
	begin := time.Now()
	rec := &record{client: client.String(), blksize: DefaultBlockSize, window: 1, result: "ok"}
	defer func() { s.logRecord(rec, time.Since(begin)) }()

	raddr, ok := client.(*net.UDPAddr)
	if !ok {
		rec.result = "error: not a UDP address"
		return
	}
	// 리스너가 특정 IP 에 묶여 있으면 응답도 그 IP 에서 나가게 한다.
	// 0.0.0.0 이면 커널이 경로를 보고 출발 IP 를 고른다.
	var laddr *net.UDPAddr
	if la, ok := local.(*net.UDPAddr); ok && !la.IP.IsUnspecified() {
		laddr = &net.UDPAddr{IP: la.IP}
	}
	// 요청마다 새 소켓(새 포트 = 서버 쪽 TID)을 연다. 연결된 UDP 소켓이라
	// 커널이 이 클라이언트 주소에서 온 데이터그램만 넘겨준다.
	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		rec.result = "error: dial: " + err.Error()
		return
	}
	defer conn.Close()

	fail := func(code ErrCode, msg, why string) {
		sendErr(conn, code, msg)
		rec.result = "refused: " + why
	}

	op, err := PeekOpCode(pkt)
	if err != nil {
		fail(ErrIllegalOp, "malformed request", err.Error())
		return
	}
	var req Request
	switch op {
	case OpRRQ:
		if err := req.UnmarshalBinary(pkt); err != nil {
			fail(ErrIllegalOp, "malformed request", err.Error())
			return
		}
	case OpWRQ:
		if err := req.UnmarshalBinary(pkt); err == nil {
			rec.file = req.Filename
		}
		fail(ErrAccess, "write requests are not accepted", "WRQ")
		return
	default:
		fail(ErrIllegalOp, "expected RRQ", "opcode "+op.String())
		return
	}
	rec.file, rec.mode = req.Filename, req.Mode

	convert := false
	switch req.Mode {
	case "octet":
	case "netascii":
		switch s.Netascii {
		case NetasciiReject:
			fail(ErrIllegalOp, "netascii mode is not supported", "netascii rejected")
			return
		case NetasciiConvert:
			convert = true
		}
	default:
		fail(ErrIllegalOp, "unsupported mode", "mode "+req.Mode)
		return
	}

	f, size, err := s.open(req.Filename)
	if err != nil {
		code, msg := ErrAccess, "access violation"
		if errors.Is(err, fs.ErrNotExist) {
			code, msg = ErrNotFound, "file not found"
		}
		// 클라이언트에게는 짧은 문구만, 자세한 이유는 서버 로그에만 남긴다
		fail(code, msg, err.Error())
		return
	}
	defer f.Close()

	var src io.Reader = f
	if convert {
		src = NewNetasciiEncoder(f)
	}

	snd := &sender{
		conn:     conn,
		src:      src,
		blksize:  DefaultBlockSize,
		window:   1,
		timeout:  s.Timeout,
		retries:  s.Retries,
		rollover: s.Rollover,
		legacy:   s.LegacyDupAck,
		rec:      rec,
		buf:      make([]byte, 65536),
	}

	var acked []Option
	if s.Options {
		acked = s.negotiate(req.Options, snd, size, convert)
	}
	rec.blksize, rec.window = snd.blksize, snd.window

	// 변환 없이 보내는 경우(octet, raw) 블록 번호가 넘칠 전송은 보내기 전에 거절한다.
	// 변환해 보내는 경우는 run 이 블록을 만들다가 거절한다.
	if s.Rollover == RolloverRefuse && !convert {
		need := uint64(size)/uint64(snd.blksize) + 1
		if need > 65535 {
			fail(ErrUndefined,
				fmt.Sprintf("file needs %d blocks at blksize %d; limit is 65535", need, snd.blksize),
				"rollover refuse")
			return
		}
	}

	if len(acked) > 0 {
		oack, _ := OAck{Options: acked}.MarshalBinary()
		if err := snd.handshake(oack); err != nil {
			rec.result = "error: " + err.Error()
			return
		}
	}

	if err := snd.run(); err != nil {
		rec.result = "error: " + err.Error()
	}
}

// open 은 루트 기준으로 파일을 연다. PXE 펌웨어처럼 앞에 / 를 붙여 요청하는
// 클라이언트가 있어 앞의 / 는 떼고 루트 기준 상대 경로로 본다.
func (s *Server) open(name string) (*os.File, int64, error) {
	name = strings.TrimLeft(name, "/")
	var (
		f   *os.File
		err error
	)
	if s.UnsafeOpen {
		f, err = os.Open(filepath.Join(s.Root, name))
	} else {
		f, err = s.root.Open(name)
	}
	if err != nil {
		return nil, 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, 0, fmt.Errorf("%s: not a regular file", name)
	}
	return f, fi.Size(), nil
}

// negotiate 는 요청의 옵션 중 받아들일 것을 골라 snd 에 반영하고 OACK 에 실을 목록을 돌려준다.
// 알 수 없거나 값이 틀린 옵션은 OACK 에서 뺀다 (RFC 2347: ERROR 를 만들지 않는다).
func (s *Server) negotiate(opts []Option, snd *sender, size int64, converted bool) []Option {
	var acked []Option
	seen := map[string]bool{}
	for _, o := range opts {
		if seen[o.Name] {
			continue
		}
		seen[o.Name] = true
		v, err := strconv.Atoi(o.Value)
		if err != nil {
			continue
		}
		switch o.Name {
		case "blksize":
			// RFC 2348: 8~65464, 서버는 요청 값 이하로만 답할 수 있다
			if v < MinBlockSize || v > MaxBlockSize {
				continue
			}
			v = min(v, s.MaxBlockSize)
			snd.blksize = v
			acked = append(acked, Option{"blksize", strconv.Itoa(v)})
		case "windowsize":
			// RFC 7440: 1~65535, 서버는 요청 값 이하로만 답할 수 있다
			if v < 1 || v > MaxWindowSize {
				continue
			}
			v = min(v, s.MaxWindowSize)
			snd.window = v
			acked = append(acked, Option{"windowsize", strconv.Itoa(v)})
		case "timeout":
			// RFC 2349: 1~255초, 서버는 요청 값 그대로 답해야 한다
			if v < 1 || v > 255 {
				continue
			}
			snd.timeout = time.Duration(v) * time.Second
			acked = append(acked, Option{"timeout", o.Value})
		case "tsize":
			// RFC 2349: RRQ 에서는 0 을 보내고 서버가 파일 크기를 채운다.
			// netascii 로 변환해 보낼 때는 싣지 않는다.
			if converted {
				continue
			}
			acked = append(acked, Option{"tsize", strconv.FormatInt(size, 10)})
		}
	}
	return acked
}

func sendErr(conn net.Conn, code ErrCode, msg string) {
	b, _ := Err{Code: code, Message: msg}.MarshalBinary()
	_, _ = conn.Write(b)
}

var errRetries = errors.New("exhausted retries")

// sender 는 전송 한 건의 보내는 쪽 상태다.
type sender struct {
	conn     *net.UDPConn
	src      io.Reader
	blksize  int
	window   int
	timeout  time.Duration
	retries  int
	rollover Rollover
	legacy   bool
	rec      *record
	buf      []byte
}

func isTimeout(err error) bool {
	var nErr net.Error
	return errors.As(err, &nErr) && nErr.Timeout()
}

// handshake 는 OACK 를 보내고 ACK 0 을 기다린다 (RFC 2347).
func (s *sender) handshake(oack []byte) error {
	for attempt := 0; attempt < s.retries; attempt++ {
		if _, err := s.conn.Write(oack); err != nil {
			return err
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(s.timeout))
		for {
			n, err := s.conn.Read(s.buf)
			if err != nil {
				if isTimeout(err) {
					s.rec.timeouts++
					break
				}
				return err
			}
			op, _ := PeekOpCode(s.buf[:n])
			switch op {
			case OpAck:
				var a Ack
				if a.UnmarshalBinary(s.buf[:n]) == nil && a == 0 {
					return nil
				}
			case OpErr:
				var e Err
				_ = e.UnmarshalBinary(s.buf[:n])
				return fmt.Errorf("client refused options: %v", e)
			}
			s.rec.badPackets++
		}
	}
	return errRetries
}

// run 은 창 단위로 DATA 를 보내고 ACK 를 기다린다. 창이 1 이면 정지-대기다.
func (s *sender) run() error {
	var (
		base    uint64   = 1 // 아직 ACK 를 못 받은 첫 블록
		next    uint64   = 1 // 다음에 만들 블록
		pending [][]byte     // base, base+1, ... 블록의 DATA 패킷
		sizes   []int        // 각 블록의 페이로드 길이
		final   bool         // 마지막 블록(블록 크기 미만)을 만들었는가
	)
	defer func() { s.rec.blocks = base - 1 }()

	for {
		// 지난 창에서 ACK 받지 못하고 남은 블록은 다시 보내는 것이다
		carried := len(pending)
		for len(pending) < s.window && !final {
			if s.rollover == RolloverRefuse && next > 65535 {
				sendErr(s.conn, ErrUndefined, "block number would exceed 65535")
				return errors.New("block number would exceed 65535")
			}
			payload := make([]byte, s.blksize)
			n, err := io.ReadFull(s.src, payload)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				sendErr(s.conn, ErrUndefined, "read error")
				return err
			}
			if n < s.blksize {
				final = true
			}
			pkt, _ := Data{Block: s.rollover.WireBlock(next), Payload: payload[:n]}.MarshalBinary()
			pending = append(pending, pkt)
			sizes = append(sizes, n)
			next++
		}
		if len(pending) == 0 {
			return nil
		}

		if err := s.sendAll(pending); err != nil {
			return err
		}
		s.rec.resendSeqErr += carried
		attempts := 1
		deadline := time.Now().Add(s.timeout)

	WAIT:
		for {
			_ = s.conn.SetReadDeadline(deadline)
			n, err := s.conn.Read(s.buf)
			if err != nil {
				if !isTimeout(err) {
					return err
				}
				s.rec.timeouts++
				if attempts >= s.retries {
					return errRetries
				}
				attempts++
				if err := s.sendAll(pending); err != nil {
					return err
				}
				s.rec.resendTimeout += len(pending)
				deadline = time.Now().Add(s.timeout)
				continue
			}

			pkt := s.buf[:n]
			op, _ := PeekOpCode(pkt)
			if op == OpErr {
				var e Err
				_ = e.UnmarshalBinary(pkt)
				return fmt.Errorf("client sent %v", e)
			}
			var a Ack
			if op == OpAck && a.UnmarshalBinary(pkt) == nil {
				// 이 창 안의 어느 블록에 대한 ACK 인가
				for i := range pending {
					if s.rollover.WireBlock(base+uint64(i)) == uint16(a) {
						k := i + 1
						for _, sz := range sizes[:k] {
							s.rec.bytes += int64(sz)
						}
						pending, sizes = pending[k:], sizes[k:]
						base += uint64(k)
						break WAIT
					}
				}
				s.rec.dupAcks++
			} else {
				s.rec.badPackets++
			}

			if !s.legacy {
				// 지난 ACK·이상한 패킷은 버리고 같은 데드라인 안에서 다시 읽는다
				continue
			}
			// 책 Listing 의 동작: 곧바로 다시 보내고 재시도 횟수를 하나 쓴다
			if attempts >= s.retries {
				return errRetries
			}
			attempts++
			if err := s.sendAll(pending); err != nil {
				return err
			}
			s.rec.resendDupAck += len(pending)
			deadline = time.Now().Add(s.timeout)
		}
	}
}

func (s *sender) sendAll(pkts [][]byte) error {
	for _, p := range pkts {
		if _, err := s.conn.Write(p); err != nil {
			return err
		}
		s.rec.sent++
	}
	return nil
}
