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

package rpc

import (
	context "context"
	"io"

	grpc "google.golang.org/grpc"
)

// MuxBroker allows a client or server to multiplex over connections.
// Deprecated: see extensionv2 package for more details.
type MuxBroker interface {
	DialChannel(context.Context, string, ...string) (grpc.ClientConnInterface, error)
	io.Closer
}

// ServiceRegistrar adds GetServiceInfo to grpc.ServiceRegistrar.
type ServiceRegistrar interface {
	GetServiceInfo() map[string]grpc.ServiceInfo
	grpc.ServiceRegistrar
}

// IsRegistered returns whether the given service is registered already
// with the given ServiceRegistrar. This is to avoid RegisterService panicking.
func IsRegistered(srv ServiceRegistrar, desc grpc.ServiceDesc) bool {
	_, ok := srv.GetServiceInfo()[desc.ServiceName]
	return ok
}
