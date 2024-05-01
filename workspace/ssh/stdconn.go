package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/unstablebuild/blue/logging"
	bluenet "github.com/unstablebuild/blue/net"
	log "github.com/sirupsen/logrus"
)

var errStreamClosed = errors.New("stream closed")

var _ net.Conn = (*stdConn)(nil)

type stdConn struct {
	logger    *log.Logger
	closeHook func()
	conn      net.Conn
}

func newStdConn(
	logger *log.Logger,
	in *os.File, out *os.File, stdio bool,
	closeHook func(),
) (*stdConn, error) {
	var conn net.Conn
	var err error
	// PipeConn returns error if in, out are not pipes
	// but os.Stdin and os.Stdout are not, so we need to call
	// special constructor StdioConn to avoid assertion.
	if stdio {
		conn = bluenet.StdioConn()
	} else {
		conn, err = bluenet.PipeConn(in, out)
	}
	if err != nil {
		return nil, fmt.Errorf("pipe conn: %v", err)
	}
	return &stdConn{
		logger:    logger,
		conn:      conn,
		closeHook: closeHook,
	}, nil
}

func (s *stdConn) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *stdConn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *stdConn) log(level log.Level, msg string, args ...interface{}) {
	s.logger.
		WithFields(log.Fields{logging.KeyClass: "stdConn"}).
		Logf(level, msg, args...)
}

func (s *stdConn) Read(b []byte) (n int, err error) {
	n, err = s.conn.Read(b)
	return
}

func (s *stdConn) Write(b []byte) (n int, err error) {
	n, err = s.conn.Write(b)
	return
}

func (s *stdConn) callHook(hook *func()) {
	hookFn := *hook
	*hook = nil
	if hookFn == nil {
		return
	}
	hookFn()
}

func (s *stdConn) Close() error {
	s.log(log.TraceLevel, "close called, invoking hooks now")
	s.callHook(&s.closeHook)
	return s.conn.Close()
}

func (s *stdConn) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *stdConn) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *stdConn) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}
