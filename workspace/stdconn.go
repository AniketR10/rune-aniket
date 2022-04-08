package workspace

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

var errStreamClosed = errors.New("stream closed")

var _ net.Conn = (*stdConn)(nil)

// stdConn is used to satisfy net.Conn by combining a io.Reader and a io.Writer.
type stdConn struct {
	mu        sync.Mutex
	closeHook func()
	in        io.Reader
	out       io.Writer
	close     bool
	local     *stdinAddr
	remote    *stdinAddr
}

func newStdConn(in io.Reader, out io.Writer, closeHook func()) *stdConn {
	return &stdConn{
		local:     newStdinAddr("local"),
		remote:    newStdinAddr("remote"),
		in:        in,
		out:       out,
		closeHook: closeHook,
	}
}

type stdinAddr struct {
	s string
}

func newStdinAddr(s string) *stdinAddr {
	return &stdinAddr{s}
}
func (a *stdinAddr) Network() string {
	return "stdio"
}

func (a *stdinAddr) String() string {
	return a.s
}

func (s *stdConn) LocalAddr() net.Addr {
	return s.local
}

func (s *stdConn) RemoteAddr() net.Addr {
	return s.remote
}

func (s *stdConn) closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close
}

func (s *stdConn) Read(b []byte) (n int, err error) {
	if s.closed() {
		return 0, errStreamClosed
	}
	return s.in.Read(b)
}

func (s *stdConn) Write(b []byte) (n int, err error) {
	if s.closed() {
		return 0, errStreamClosed
	}
	return s.out.Write(b)
}

func (s *stdConn) Close() error {
	if s.closed() {
		return nil
	}
	s.mu.Lock()
	s.close = true
	s.mu.Unlock()
	s.closeHook()
	s.closeHook = nil
	return nil
}

func (s *stdConn) SetDeadline(t time.Time) error {
	return nil
}

func (s *stdConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (s *stdConn) SetWriteDeadline(t time.Time) error {
	return nil
}
