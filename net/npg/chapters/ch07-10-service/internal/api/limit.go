package api

import (
	"net"
	"sync"
	"sync/atomic"
)

// LimitListener 는 동시에 열려 있는 연결이 n 개가 되면 Accept 를 부르지 않고 기다린다.
// 연결 하나가 닫히면 다음 Accept 를 부른다.
func LimitListener(l net.Listener, n int) *Limited {
	return &Limited{Listener: l, sem: make(chan struct{}, n), done: make(chan struct{})}
}

// Limited 는 LimitListener 가 돌려주는 리스너다.
type Limited struct {
	net.Listener
	sem     chan struct{}
	done    chan struct{}
	once    sync.Once
	waiting atomic.Bool
}

// Accept 는 자리가 날 때까지 기다린 뒤 연결 하나를 받는다.
func (l *Limited) Accept() (net.Conn, error) {
	select {
	case l.sem <- struct{}{}:
	default:
		l.waiting.Store(true)
		select {
		case l.sem <- struct{}{}:
		case <-l.done:
			return nil, net.ErrClosed
		}
		l.waiting.Store(false)
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.sem
		return nil, err
	}
	return &limitedConn{Conn: c, release: sync.OnceFunc(func() { <-l.sem })}, nil
}

// Close 는 기다리던 Accept 도 깨운다.
func (l *Limited) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

// InUse 는 지금 차지한 자리 수, Cap 은 전체 자리 수, Waiting 은 Accept 가 자리를 기다리는 중인지다.
func (l *Limited) InUse() int    { return len(l.sem) }
func (l *Limited) Cap() int      { return cap(l.sem) }
func (l *Limited) Waiting() bool { return l.waiting.Load() }

type limitedConn struct {
	net.Conn
	release func()
}

func (c *limitedConn) Close() error {
	err := c.Conn.Close()
	c.release()
	return err
}
