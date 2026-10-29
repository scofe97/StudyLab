package tftp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// ClientOptions 는 Get 이 보낼 요청과 받는 쪽 동작을 정한다. 0 값은 "요청하지 않음" 또는 기본값이다.
type ClientOptions struct {
	// Mode 는 "octet"(기본) 또는 "netascii" 다.
	Mode string
	// BlockSize 가 0 이 아니면 blksize 옵션을 요청한다 (RFC 2348).
	BlockSize int
	// WindowSize 가 0 이 아니면 windowsize 옵션을 요청한다 (RFC 7440).
	WindowSize int
	// TSize 가 true 면 tsize 옵션을 0 으로 요청해 파일 크기를 묻는다 (RFC 2349).
	TSize bool
	// TimeoutOption 이 0 이 아니면 timeout 옵션(초)을 요청한다 (RFC 2349).
	TimeoutOption int
	// Timeout 은 다음 패킷을 기다리는 시간이다. 지나면 마지막 ACK(또는 RRQ)를 다시 보낸다. 기본 5초.
	Timeout time.Duration
	// Retries 는 연달아 타임아웃이 나도 버티는 횟수다. 기본 5.
	Retries int
	// Rollover 는 블록 번호 65535 다음을 몇으로 기대할지다. 기본 zero.
	Rollover Rollover
	// AckDelayBlock 이 0 이 아니면 그 블록(1부터 센 절대 번호)의 첫 ACK 를
	// AckDelay 만큼 늦게 보낸다. 같은 블록을 다시 받아 보내는 ACK 는 늦추지 않는다.
	AckDelayBlock uint64
	AckDelay      time.Duration
}

// ClientStats 는 받는 쪽에서 센 값이다.
type ClientStats struct {
	Bytes      int64
	Blocks     uint64
	BlockSize  int
	WindowSize int
	// TSize 는 OACK 로 받은 파일 크기다. 받지 못했으면 -1.
	TSize int64
	OAck  []Option
	// DupData 는 마지막으로 받은 블록과 같은 번호의 DATA 를 다시 받은 횟수다.
	DupData int
	// Unexpected 는 기다리던 번호도, 마지막 블록 번호도 아닌 DATA 를 받은 횟수다.
	Unexpected int
	AcksSent   int
	Timeouts   int
	// Strangers 는 전송 상대(TID)가 아닌 주소에서 온 패킷 수다.
	Strangers int
	Elapsed   time.Duration
}

// Get 은 server(host:port)에서 filename 을 받아 w 에 쓴다.
func Get(server, filename string, opt ClientOptions, w io.Writer) (st ClientStats, err error) {
	begin := time.Now()
	defer func() { st.Elapsed = time.Since(begin) }()

	if opt.Mode == "" {
		opt.Mode = "octet"
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 5 * time.Second
	}
	if opt.Retries <= 0 {
		opt.Retries = 5
	}
	if opt.Rollover == "" {
		opt.Rollover = RolloverZero
	}
	st.BlockSize, st.WindowSize, st.TSize = DefaultBlockSize, 1, -1

	raddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return st, err
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return st, err
	}
	defer conn.Close()

	var decoder *NetasciiDecoder
	if opt.Mode == "netascii" {
		decoder = NewNetasciiDecoder(w)
		w = decoder
	}

	req := Request{Op: OpRRQ, Filename: filename, Mode: opt.Mode}
	if opt.BlockSize > 0 {
		req.Options = append(req.Options, Option{"blksize", strconv.Itoa(opt.BlockSize)})
	}
	if opt.WindowSize > 0 {
		req.Options = append(req.Options, Option{"windowsize", strconv.Itoa(opt.WindowSize)})
	}
	if opt.TimeoutOption > 0 {
		req.Options = append(req.Options, Option{"timeout", strconv.Itoa(opt.TimeoutOption)})
	}
	if opt.TSize {
		req.Options = append(req.Options, Option{"tsize", "0"})
	}
	reqPkt, _ := req.MarshalBinary()
	if _, err := conn.WriteToUDP(reqPkt, raddr); err != nil {
		return st, err
	}

	var (
		tid      *net.UDPAddr     // 서버가 이 전송에 쓰는 주소:포트, 첫 응답에서 정해진다
		expect   uint64       = 1 // 기다리는 블록
		lastGood uint64           // 마지막으로 차례대로 받은 블록
		inWindow int              // 이번 창에서 받은 블록 수
		attempts int
		delayed  bool
		buf      = make([]byte, HeaderSize+MaxBlockSize+64)
	)

	sendAck := func(block uint64) {
		pkt, _ := Ack(opt.Rollover.WireBlock(block)).MarshalBinary()
		st.AcksSent++
		if !delayed && opt.AckDelayBlock != 0 && block == opt.AckDelayBlock {
			delayed = true
			to := tid
			time.AfterFunc(opt.AckDelay, func() { _, _ = conn.WriteToUDP(pkt, to) })
			return
		}
		_, _ = conn.WriteToUDP(pkt, tid)
	}
	sendErrTo := func(to *net.UDPAddr, code ErrCode, msg string) {
		b, _ := Err{Code: code, Message: msg}.MarshalBinary()
		_, _ = conn.WriteToUDP(b, to)
	}

	for {
		_ = conn.SetReadDeadline(time.Now().Add(opt.Timeout))
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			if !isTimeout(err) {
				return st, err
			}
			st.Timeouts++
			attempts++
			if attempts > opt.Retries {
				return st, errors.New("timed out waiting for server")
			}
			if tid == nil {
				_, _ = conn.WriteToUDP(reqPkt, raddr)
			} else {
				sendAck(lastGood)
			}
			continue
		}
		if tid == nil {
			tid = from
		} else if !from.IP.Equal(tid.IP) || from.Port != tid.Port {
			// RFC 1350: 다른 TID 에서 온 패킷은 버리고 그쪽에 ERROR 5 를 보낸다
			st.Strangers++
			sendErrTo(from, ErrUnknownTID, "unknown transfer ID")
			continue
		}

		pkt := buf[:n]
		op, _ := PeekOpCode(pkt)
		switch op {
		case OpOAck:
			if lastGood > 0 {
				continue
			}
			if st.OAck == nil {
				if err := applyOAck(pkt, req.Options, &st); err != nil {
					sendErrTo(tid, ErrOptionRefused, err.Error())
					return st, err
				}
			}
			attempts = 0
			// OACK 에는 블록 0 의 ACK 로 답한다 (RFC 2347)
			sendAck(0)

		case OpData:
			var d Data
			_ = d.UnmarshalBinary(pkt)
			if len(d.Payload) > st.BlockSize {
				sendErrTo(tid, ErrIllegalOp, "block larger than negotiated blksize")
				return st, fmt.Errorf("block %d has %d bytes, blksize is %d", d.Block, len(d.Payload), st.BlockSize)
			}
			if d.Block != opt.Rollover.WireBlock(expect) {
				if lastGood > 0 && d.Block == opt.Rollover.WireBlock(lastGood) {
					st.DupData++
				} else {
					st.Unexpected++
				}
				// 차례대로 받은 마지막 블록을 다시 알려 준다
				sendAck(lastGood)
				inWindow = 0
				continue
			}
			attempts = 0
			if _, err := w.Write(d.Payload); err != nil {
				sendErrTo(tid, ErrDiskFull, "write failed")
				return st, err
			}
			st.Bytes += int64(len(d.Payload))
			lastGood = expect
			expect++
			inWindow++
			final := len(d.Payload) < st.BlockSize
			if final || inWindow >= st.WindowSize {
				sendAck(lastGood)
				inWindow = 0
			}
			if final {
				st.Blocks = lastGood
				if decoder != nil {
					if err := decoder.Close(); err != nil {
						return st, err
					}
				}
				return st, nil
			}

		case OpErr:
			var e Err
			_ = e.UnmarshalBinary(pkt)
			return st, e
		}
	}
}

// applyOAck 는 서버가 돌려준 옵션을 확인하고 st 에 반영한다.
// 요청하지 않은 옵션이나 요청보다 큰 값이 오면 에러다 (RFC 2347·2348·7440).
func applyOAck(pkt []byte, requested []Option, st *ClientStats) error {
	var o OAck
	if err := o.UnmarshalBinary(pkt); err != nil {
		return err
	}
	asked := map[string]int{}
	for _, r := range requested {
		v, _ := strconv.Atoi(r.Value)
		asked[r.Name] = v
	}
	for _, opt := range o.Options {
		want, ok := asked[opt.Name]
		if !ok {
			return fmt.Errorf("unrequested option %q", opt.Name)
		}
		v, err := strconv.ParseInt(opt.Value, 10, 64)
		if err != nil {
			return fmt.Errorf("option %s: %w", opt.Name, err)
		}
		switch opt.Name {
		case "blksize":
			if v < MinBlockSize || int(v) > want {
				return fmt.Errorf("blksize %d not in [%d, %d]", v, MinBlockSize, want)
			}
			st.BlockSize = int(v)
		case "windowsize":
			if v < 1 || int(v) > want {
				return fmt.Errorf("windowsize %d not in [1, %d]", v, want)
			}
			st.WindowSize = int(v)
		case "tsize":
			st.TSize = v
		}
	}
	st.OAck = o.Options
	if st.OAck == nil {
		st.OAck = []Option{}
	}
	return nil
}
