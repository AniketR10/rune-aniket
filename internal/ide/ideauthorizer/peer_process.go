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

package ideauthorizer

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
	"unstable.build/rune/internal/extension/extensionv2/peerprocess"
)

// PeerProcessAddr is a net.Addr that carries information about the process on
// the other end of a Unix-domain socket. The extension runner's listener wraps
// each accepted connection's remote address in a PeerProcessAddr so the
// authorizer can identify the calling plugin by its program path/args/PID.
type PeerProcessAddr struct {
	Addr    net.Addr
	Process peerprocess.Process
	Err     error
}

// Network returns the network of the underlying address, or "unix" if none.
func (a PeerProcessAddr) Network() string {
	if a.Addr != nil {
		return a.Addr.Network()
	}
	return "unix"
}

// String returns the peer process's program path when available, falling back
// to the underlying address.
func (a PeerProcessAddr) String() string {
	if path := a.Process.ProgramPath(); path != "" {
		return path
	}
	if a.Addr != nil {
		return a.Addr.String()
	}
	return ""
}

func peerProcessFromContext(ctx context.Context) (peerprocess.Process, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return peerprocess.Process{}, false
	}
	switch addr := p.Addr.(type) {
	case PeerProcessAddr:
		return usablePeerProcess(addr.Process, addr.Err)
	case *PeerProcessAddr:
		if addr == nil {
			return peerprocess.Process{}, false
		}
		return usablePeerProcess(addr.Process, addr.Err)
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
