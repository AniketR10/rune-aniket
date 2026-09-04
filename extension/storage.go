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
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"unstable.build/rune/localstorage/storagerpc"
	"unstable.build/rune/rpc"
)

type storageResourceServer struct {
	svc storageapi.Service
}

func newStorageResourceServer(svc storageapi.Service) *storageResourceServer {
	ret := new(storageResourceServer)
	ret.svc = svc
	return ret
}

func (s *storageResourceServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := new(storagerpc.Server)
	server.Init(s.svc, docbson.Marshaler())
	docpb.RegisterDocumentStoreServer(registrar, server)
	return storageServerCloser{server: server}, nil
}

// storageServerCloser releases the per-registration gRPC server. The
// storage service is owned by the caller of StorageResources and shared
// across every workspace, so it is deliberately not closed here.
type storageServerCloser struct {
	server *storagerpc.Server
}

func (c storageServerCloser) Close() (err error) {
	if c.server != nil {
		err = errors.Join(err, c.server.Close())
	}
	return err
}

// StorageResources returns a map of Permission to a ResourceServer
// capable of serving svc to extensions.
//
// svc is borrowed, not owned: callers must share a single service across
// all workspaces and close it themselves. Constructing one service per
// workspace makes every workspace but the first a firstmover follower of
// its own process, so each extension storage call round-trips over a
// loopback gRPC hop and is encoded and decoded twice.
func StorageResources(svc storageapi.Service) map[extensionapi.Permission]ResourceRegistrar {
	s := newStorageResourceServer(svc)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionStorage: s,
	}
}
