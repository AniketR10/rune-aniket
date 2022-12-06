package plugin

import (
	"io"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ernestrc/blue/document"
	docrpc "github.com/ernestrc/blue/document/rpc"
	bproto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding/toml"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/storage"
)

const (
	// PermissionStorage requests access to persistent storage.
	PermissionStorage = "_PermStorage"
)

type storageResourceServer struct {
	storageDir string
}

func newStorageResourceServer(storageDir string) *storageResourceServer {
	ret := new(storageResourceServer)
	ret.storageDir = storageDir
	return ret
}

func (s *storageResourceServer) setupStorage(pluginID string) document.Service {
	path := filepath.Join(s.storageDir, ".dbplugin", filepath.Clean(pluginID))
	svc, err := storage.New(path, toml.Marshaler())
	if err != nil {
		log.Warnf("Failed to setup storage for plugin %q: %v."+
			"Fallback to in-memory", pluginID, err)
		return document.NewInMemoryService()
	}
	return svc
}

func (s *storageResourceServer) Register(
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	svc := s.setupStorage(pluginID)
	server := new(docrpc.Server)
	// NOTE: if registrar is not a grpc.Server this will panic
	// but it's a small price to pay rather than exposing grpc.Server
	// across all ResourceRegistrar impls.
	server.Init(svc, toml.Marshaler(), registrar.(*grpc.Server))
	bproto.RegisterDocumentStoreServer(registrar, server)
	return server, nil
}

// StorageResource returns a map of Permission to a ResourceServer
// capable of serving a document.Service.
func StorageResources(storageDir string) map[Permission]ResourceRegistrar {
	s := newStorageResourceServer(storageDir)
	return map[Permission]ResourceRegistrar{
		PermissionStorage: s,
	}
}

func dialStorage(token uint32, broker proto.MuxBroker) (
	document.Service, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := new(docrpc.Client)
	c.Init(conn, toml.Marshaler())
	runtime.SetFinalizer(c, func(c *docrpc.Client) { c.Close() })
	return c, nil
}

// Storage acquires a client to persistent storage with
// the given token.
func Storage(token uint32, broker proto.MuxBroker) (
	document.Service, error,
) {
	return dialStorage(token, broker)
}
