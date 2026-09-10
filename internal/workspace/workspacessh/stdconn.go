// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package workspacessh

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/bluenet"
	"github.com/unstablebuild/blue/logging"
)

var _ net.Conn = (*stdConn)(nil)

type stdConn struct {
	logger    *log.Logger
	closeHook func()
	conn      net.Conn
	closeOnce sync.Once
	hookOnce  sync.Once
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

func (s *stdConn) log(level log.Level, msg string, args ...any) {
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

// Close is safe to call concurrently and is idempotent
func (s *stdConn) Close() error {
	s.log(log.TraceLevel, "close called, invoking hooks now")
	s.hookOnce.Do(func() {
		if s.closeHook != nil {
			s.closeHook()
		}
	})
	var err error
	s.closeOnce.Do(func() {
		err = s.conn.Close()
	})
	return err
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
