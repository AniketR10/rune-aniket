package plugin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ernestrc/blue/document"
	docrpc "github.com/ernestrc/blue/document/rpc"
	bproto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding/toml"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"unstable.build/go-tui/proto"
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

func (s *storageResourceServer) setupStorage(lock sync.Locker, pluginID string) document.Service {
	path := filepath.Join(s.storageDir, ".dbplugin", filepath.Clean(pluginID))
	// pass locker to underlying file scheme, so we can synchronize
	// network storage requests against event loop access.
	ctx := workspace.ContextWithLocker(context.Background(), lock)
	svc, err := storage.New(ctx, path, toml.Marshaler())
	if err != nil {
		log.Warnf("Failed to setup storage for plugin %q: %v."+
			"Fallback to in-memory", pluginID, err)
		return document.NewInMemoryService()
	}
	return svc
}

func (s *storageResourceServer) Register(
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	svc := s.setupStorage(lock, pluginID)
	server := new(docrpc.Server)
	// NOTE: if registrar is not a grpc.Server this will panic
	// but it's a small price to pay rather than exposing grpc.Server
	// across all ResourceRegistrar impls.
	server.Init(svc, toml.Marshaler(), registrar.(*grpc.Server))
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

// TODO move to api package
func dialStorage(grant Grant, broker proto.MuxBroker) (
	document.Service, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "storage", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := new(docrpc.Client)
	c.Init(conn, toml.Marshaler())
	return c, nil
}

// Storage acquires a client to persistent storage with
// the given token.
func Storage(grant Grant, broker proto.MuxBroker) (
	document.Service, error,
) {
	return dialStorage(grant, broker)
}
