package plugin

import (
	"sync"

	"github.com/ernestrc/blue/document"
	docrpc "github.com/ernestrc/blue/document/rpc"
	bproto "github.com/ernestrc/blue/document/rpc/proto"
	"github.com/ernestrc/blue/encoding/toml"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/proto"
)

const (
	// PermissionStorage requests access to persistent storage.
	PermissionStorage = "_PermStorage"
)

type storageResourceServer struct {
	mu     sync.Mutex
	svc    document.Service
	srv    proto.MuxServer
	server *docrpc.Server
}

func newStorageResourceServer(b document.Service) *storageResourceServer {
	ret := new(storageResourceServer)
	ret.svc = b
	return ret
}

func (s *storageResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			// FIXME do not share, create a partition!
			if s.srv == nil {
				var srv proto.MuxServer
				l := debug.StandardLogger()
				if l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				s.server = new(docrpc.Server)
				s.server.Init(s.svc, toml.Marshaler(), grpc)
				bproto.RegisterDocumentStoreServer(grpc, s.server)
			}
			return s.srv
		})
}

func (s *storageResourceServer) Close() error {
	if s.srv != nil {
		// docrpc.Server closes grpc.Server
		return s.server.Close()
	}
	return nil
}

// StorageResource returns a map of Permission to a ResourceServer
// capable of serving a document.Service.
func StorageResource(b document.Service) map[Permission]ResourceServer {
	s := newStorageResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionStorage: s,
	}
}

func dialStorage(token uint32, broker proto.MuxBroker) (
	document.Service, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(document.Service), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := new(docrpc.Client)
	c.Init(conn, toml.Marshaler())
	clients.Store(token, c)
	return c, nil
}

// Storage acquires a client to persistent storage with
// the given token.
func Storage(token uint32, broker proto.MuxBroker) (
	document.Service, error,
) {
	return dialStorage(token, broker)
}
