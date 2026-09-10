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

package extension

import (
	"errors"
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	tsyntaxrpc "unstable.build/rune/internal/ide/syntax/syntaxrpc"
	"unstable.build/rune/internal/rpc"
)

// SyntaxResources returns a map of Permission to a ResourceServer
// capable of serving each of the b SyntaxTree's resources.
func SyntaxResources(b syntaxapi.Parser) map[extensionapi.Permission]ResourceRegistrar {
	s := newSyntaxTreeResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionSyntaxTree: s.forPermission(
			extensionapi.PermissionSyntaxTree),
	}
}

type syntaxResourceServer struct {
	b syntaxapi.Parser
}

type syntaxResourcePermissionServer struct {
	p extensionapi.Permission
	*syntaxResourceServer
}

func newSyntaxTreeResourceServer(b syntaxapi.Parser) *syntaxResourceServer {
	ret := new(syntaxResourceServer)
	ret.b = b
	return ret
}

func (s *syntaxResourceServer) forPermission(p extensionapi.Permission) ResourceRegistrar {
	return syntaxResourcePermissionServer{p: p, syntaxResourceServer: s}
}

func (s syntaxResourcePermissionServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := tsyntaxrpc.NewServer(s.b)
	switch s.p {
	case extensionapi.PermissionSyntaxTree:
		syntaxrpc.RegisterSyntaxServer(registrar, server)
	default:
		return nil, errors.New("unknown permission for syntax server")
	}
	return server, nil
}
