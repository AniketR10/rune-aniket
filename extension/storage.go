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
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	doclog "github.com/unstablebuild/blue/document/logging"
	docrpc "github.com/unstablebuild/blue/document/rpc"
	bproto "github.com/unstablebuild/blue/document/rpc/proto"
	"github.com/unstablebuild/blue/encoding/toml"

	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/storage"
	"unstable.build/go-tui/workspace"
)

type storageResourceServer struct {
	storageDir string
}

func newStorageResourceServer(storageDir string) *storageResourceServer {
	ret := new(storageResourceServer)
	ret.storageDir = storageDir
	return ret
}

func (s *storageResourceServer) setupStorage(lock sync.Locker, extensionID string) document.Service {
	path := filepath.Join(s.storageDir, ".dbextension", filepath.Clean(extensionID))
	// pass locker to underlying file scheme, so we can synchronize
	// network storage requests against event loop access.
	ctx := workspace.ContextWithLocker(context.Background(), lock)
	svc, err := storage.New(ctx, path, toml.Marshaler())
	if err != nil {
		log.Warnf("Failed to setup storage for extension %q: %v."+
			"Fallback to in-memory", extensionID, err)
		return document.NewInMemoryService()
	}
	return svc
}

func (s *storageResourceServer) Register(
	extensionID string, grantor Grantor, registrar rpc.ServiceRegistrar,
	broker rpc.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	svc := s.setupStorage(lock, extensionID)
	svc = doclog.WithLogging(svc, fmt.Sprintf("/ExtensionStorage/%s", extensionID))
	server := new(docrpc.Server)
	server.Init(svc, toml.Marshaler())
	bproto.RegisterDocumentStoreServer(registrar, server)
	// doc server stops grpc.Server, which is not something storageResourceserver
	// should be concerned about. Close storage resources created
	// within this call to register.
	return svc, nil
}

// StorageResource returns a map of Permission to a ResourceServer
// capable of serving a document.Service.
func StorageResources(storageDir string) map[Permission]ResourceRegistrar {
	s := newStorageResourceServer(storageDir)
	return map[Permission]ResourceRegistrar{
		PermissionStorage: s,
	}
}
