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

package extension

import (
	"errors"
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi/debugrpc"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	tdebugrpc "unstable.build/rune/internal/ide/idedebug/debugrpc"
	"unstable.build/rune/internal/rpc"
)

// DebugResources returns a map of Permission to a ResourceServer
// capable of serving each of the b DebugTree's resources.
func DebugResources(b debugapi.Debugger) map[extensionapi.Permission]ResourceRegistrar {
	s := newDebugTreeResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionDebugger: s.forPermission(extensionapi.PermissionDebugger),
	}
}

type debugResourceServer struct {
	b debugapi.Debugger
}

type debugResourcePermissionServer struct {
	p extensionapi.Permission
	*debugResourceServer
}

func newDebugTreeResourceServer(b debugapi.Debugger) *debugResourceServer {
	ret := new(debugResourceServer)
	ret.b = b
	return ret
}

func (s *debugResourceServer) forPermission(p extensionapi.Permission) ResourceRegistrar {
	return debugResourcePermissionServer{p: p, debugResourceServer: s}
}

func (s debugResourcePermissionServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := tdebugrpc.NewServer(s.b)
	switch s.p {
	case extensionapi.PermissionDebugger:
		debugrpc.RegisterDebuggerServer(registrar, server)
	default:
		return nil, errors.New("unknown permission for debug server")
	}
	return server, nil
}
