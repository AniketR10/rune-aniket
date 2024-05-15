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
