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

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi/semanticrpc"
	tsemanticrpc "unstable.build/rune/ide/idelsp/semanticrpc"
	"unstable.build/rune/rpc"
)

// SemanticResources returns a map of Permission to a ResourceServer
// capable of serving each of the b SemanticTree's resources.
func SemanticResources(b semanticapi.LSP) map[extensionapi.Permission]ResourceRegistrar {
	s := newSemanticTreeResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionLSP: s.forPermission(
			extensionapi.PermissionLSP),
	}
}

type semanticResourceServer struct {
	b semanticapi.LSP
}

type semanticResourcePermissionServer struct {
	p extensionapi.Permission
	*semanticResourceServer
}

func newSemanticTreeResourceServer(b semanticapi.LSP) *semanticResourceServer {
	ret := new(semanticResourceServer)
	ret.b = b
	return ret
}

func (s *semanticResourceServer) forPermission(p extensionapi.Permission) ResourceRegistrar {
	return semanticResourcePermissionServer{p: p, semanticResourceServer: s}
}

func (s semanticResourcePermissionServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := tsemanticrpc.NewServer(s.b)
	switch s.p {
	case extensionapi.PermissionLSP:
		semanticrpc.RegisterLSPServer(registrar, server)
	default:
		return nil, errors.New("unknown permission for semantic server")
	}
	return server, nil
}
