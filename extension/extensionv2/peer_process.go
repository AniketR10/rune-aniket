// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package extensionv2

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
	"unstable.build/go-tui/extension/extensionv2/peerprocess"
)

type peerProcessAddr struct {
	addr    net.Addr
	process peerprocess.Process
	err     error
}

func (a peerProcessAddr) Network() string {
	if a.addr != nil {
		return a.addr.Network()
	}
	return "unix"
}

func (a peerProcessAddr) String() string {
	if path := a.process.ProgramPath(); path != "" {
		return path
	}
	if a.addr != nil {
		return a.addr.String()
	}
	return ""
}

func peerProcessFromContext(ctx context.Context) (peerprocess.Process, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return peerprocess.Process{}, false
	}
	switch addr := p.Addr.(type) {
	case peerProcessAddr:
		return usablePeerProcess(addr.process, addr.err)
	case *peerProcessAddr:
		if addr == nil {
			return peerprocess.Process{}, false
		}
		return usablePeerProcess(addr.process, addr.err)
	default:
		return peerprocess.Process{}, false
	}
}

func usablePeerProcess(process peerprocess.Process, err error) (peerprocess.Process, bool) {
	if err != nil || process.ProgramPath() == "" {
		return peerprocess.Process{}, false
	}
	return process, true
}

type peerProcessListener struct {
	net.Listener
}

func newPeerProcessListener(listener net.Listener) net.Listener {
	return peerProcessListener{Listener: listener}
}

func (l peerProcessListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	remoteAddr := conn.RemoteAddr()
	unixConn, ok := conn.(*net.UnixConn)
	if ok {
		process, perr := peerprocess.Identify(unixConn)
		remoteAddr = peerProcessAddr{
			addr:    remoteAddr,
			process: process,
			err:     perr,
		}
	}
	return peerProcessConn{Conn: conn, remoteAddr: remoteAddr}, nil
}

type peerProcessConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (c peerProcessConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
