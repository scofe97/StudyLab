package tftp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// 서버 설정의 기본값입니다 (ADR 0009).
const (
	DefaultTimeout    = time.Second
	DefaultRetries    = 6
	DefaultMaxBlksize = 1428
)

// ErrBadConfig는 서버 설정이 범위를 벗어났을 때의 오류입니다.
var ErrBadConfig = errors.New("서버 설정이 범위를 벗어났습니다")

// Server는 루트 폴더의 파일을 TFTP 로 내주는 읽기 전용 서버입니다 (ADR 0009, 0010).
type Server struct {
	// Root는 내줄 파일이 있는 폴더입니다.
	Root *os.Root
	// Timeout은 첫 재전송까지의 대기입니다. 재전송마다 2배가 됩니다.
	Timeout time.Duration
	// Retries는 같은 블록을 보내는 최대 횟수입니다.
	Retries int
	// MaxBlksize는 협상에서 받아들이는 블록 크기의 상한입니다.
	MaxBlksize int
	Log        zerolog.Logger
}

// Serve는 pc로 오는 요청마다 고루틴 하나를 띄워 처리합니다.
// ctx가 끝나면 pc를 닫고 nil을 돌려줍니다.
func (s *Server) Serve(ctx context.Context, pc net.PacketConn) error {
	if s.Root == nil || s.Timeout <= 0 || s.Retries < 1 || s.MaxBlksize < minBlksize || s.MaxBlksize > maxBlksize {
		return ErrBadConfig
	}

	go func() {
		<-ctx.Done()
		pc.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, from, err := pc.ReadFrom(buf)
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}

		// 쓰기 요청은 받지 않고 ERROR 4 로 거절한다 (ADR 0010)
		if n >= 2 && Opcode(binary.BigEndian.Uint16(buf[:2])) == OpWRQ {
			s.Log.Warn().Str("from", from.String()).Msg("쓰기 요청을 거절함")
			_, _ = pc.WriteTo(ErrorPacket{Code: ErrIllegalOp, Message: "write not supported"}.Encode(), from)
			continue
		}

		pkt, err := Decode(buf[:n])
		if err != nil {
			s.Log.Warn().Str("from", from.String()).Err(err).Msg("해석할 수 없는 요청을 버림")
			continue
		}
		req, ok := pkt.(ReadReq)
		if !ok {
			s.Log.Warn().Str("from", from.String()).Msg("읽기 요청이 아니라서 버림")
			continue
		}
		go s.handle(pc.LocalAddr(), from, req)
	}
}

// handle은 요청 하나를 전용 소켓으로 처리합니다.
func (s *Server) handle(local, from net.Addr, req ReadReq) {
	log := s.Log.With().Str("peer", from.String()).Str("file", req.Filename).Logger()

	localUDP, ok1 := local.(*net.UDPAddr)
	fromUDP, ok2 := from.(*net.UDPAddr)
	if !ok1 || !ok2 {
		log.Error().Msg("UDP 주소가 아님")
		return
	}

	// 응답은 리스너와 같은 IP 의 새 임시 포트(TID)에서 나가고, 이 클라이언트에만 연결합니다
	conn, err := net.DialUDP("udp", &net.UDPAddr{IP: localUDP.IP}, fromUDP)
	if err != nil {
		log.Error().Err(err).Msg("전용 소켓을 열지 못함")
		return
	}
	defer conn.Close()

	f, err := openFile(s.Root, req.Filename)
	if err != nil {
		log.Warn().Err(err).Msg("파일을 열지 못함")
		_, _ = conn.Write(ErrorPacket{Code: ErrFileNotFound, Message: "file not found"}.Encode())
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Warn().Err(err).Msg("파일 크기를 읽지 못함")
		_, _ = conn.Write(ErrorPacket{Code: ErrNotDefined, Message: "stat failed"}.Encode())
		return
	}

	s.transfer(conn, f, s.negotiate(req.Options, info.Size()), log)
}

// stats는 전송 하나의 숫자입니다. 서버 로그 한 줄과 시험이 읽습니다.
type stats struct {
	blocks    int
	sent      int
	timeouts  int
	staleAcks int
	bytes     int
}

// sendBlock은 패킷을 보내고 block 의 ACK 를 기다립니다. 타임아웃마다 2배로 늘려 다시 보냅니다.
func (s *Server) sendBlock(conn *net.UDPConn, pkt []byte, block uint16, timeout time.Duration, st *stats) error {
	buf := make([]byte, 65535)
	for attempt := 0; attempt < s.Retries; attempt++ {
		if _, err := conn.Write(pkt); err != nil {
			return fmt.Errorf("블록 %d 보내기: %w", block, err)
		}
		st.sent++

		// 데드라인은 보낼 때 한 번만 건다 (RFC 1123 §4.2.3.1)
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					st.timeouts++
					break
				}
				return fmt.Errorf("블록 %d ACK 읽기: %w", block, err)
			}

			p, err := Decode(buf[:n])
			if err != nil {
				continue
			}
			switch p := p.(type) {
			case Ack:
				if p.Block == block {
					return nil
				}
				// 지난 블록의 ACK 는 세기만 하고 다시 보내지 않는다
				st.staleAcks++
			case ErrorPacket:
				return fmt.Errorf("클라이언트 오류 %d: %s", p.Code, p.Message)
			}
		}
		timeout *= 2
	}
	return ErrRetriesExhausted
}

// sendFile은 파일 끝까지 블록을 하나씩 보냅니다. 마지막 블록은 blksize보다 작다.
func (s *Server) sendFile(conn *net.UDPConn, f *os.File, p params, st *stats) error {
	buf := make([]byte, p.blksize)
	var block uint16
	for {
		// 파일정보를 buf만큼 모두 읽는다.
		n, err := io.ReadFull(f, buf)

		// 에러발생시 EOF가 아니거나, 기대되지 않은 EOF가 아니라면 에러를 반환
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("파일 읽기: %w", err)
		}
		// 블록 번호가 uint16 한계에 닿으면 롤오버하지 않고 끝낸다
		if block == math.MaxUint16 {
			return ErrTooManyBlocks
		}
		block++

		// 읽은 데이터를 encode
		pkt := Data{Block: block, Payload: buf[:n]}.Encode()

		// 데이터 전송
		if err := s.sendBlock(conn, pkt, block, p.timeout, st); err != nil {
			return err
		}
		st.blocks++
		st.bytes += n

		// n이 blksize보다 작으면(마지막 데이터) 종료
		if n < p.blksize {
			return nil
		}
	}
}

// transfer는 파일 하나를 보내고 결과를 로그 한 줄로 남깁니다.
func (s *Server) transfer(conn *net.UDPConn, f *os.File, p params, log zerolog.Logger) {
	start := time.Now()
	var st stats

	var err error
	if len(p.oack) > 0 {
		// 옵션을 받아들였으면 OACK 를 보내고 ACK 0 을 기다린다 (RFC 2347)
		err = s.sendBlock(conn, OAck{Options: p.oack}.Encode(), 0, p.timeout, &st)
	}
	if err == nil {
		err = s.sendFile(conn, f, p, &st)
	}
	if err != nil {
		_, _ = conn.Write(ErrorPacket{Code: ErrNotDefined, Message: "transfer failed"}.Encode())
	}

	ev := log.Info()
	if err != nil {
		ev = log.Warn().Err(err)
	}
	ev.Int("blksize", p.blksize).
		Int("blocks", st.blocks).
		Int("sent", st.sent).
		Int("timeouts", st.timeouts).
		Int("stale_acks", st.staleAcks).
		Int("bytes", st.bytes).
		Dur("elapsed_ms", time.Since(start)).
		Msg("전송 끝")
}
