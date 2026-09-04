// Copyright (C) 2017-2026 Unstable Build, LLC
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

package extensionv2

import (
	"net"

	"unstable.build/rune/extension/extensionv2/peerprocess"
	"unstable.build/rune/ide/ideauthorizer"
)

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
		remoteAddr = ideauthorizer.PeerProcessAddr{
			Addr:    remoteAddr,
			Process: process,
			Err:     perr,
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
