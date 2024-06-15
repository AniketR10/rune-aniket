// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package ssh

import (
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	bluenet "github.com/unstablebuild/blue/net"
)

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
