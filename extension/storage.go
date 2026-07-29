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

package extension

import (
	"errors"
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"unstable.build/go-tui/localstorage/storagerpc"
	"unstable.build/go-tui/rpc"
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
