package plugin

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

const (
	// PermissionSchemeManager requests access to the workspace's URI scheme manager.
	PermissionSchemeManager Permission = "_PermSchemeManager"
)

type schemeManagerResourceServer struct {
	mu  sync.Mutex
	b   workspace.SchemeManager
	srv proto.MuxServer
}

func newSchemeManagerResourceServer(b workspace.SchemeManager) *schemeManagerResourceServer {
	ret := new(schemeManagerResourceServer)
	ret.b = b
	return ret
}

func (s *schemeManagerResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.srv == nil {
				var srv proto.MuxServer
				l := debug.StandardLogger()
				if l != nil && l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				server := workspacepb.NewSchemeManagerServer(broker, s.b)
				workspacepb.RegisterManagerServer(grpc, server)
			}
			return s.srv
		})
}

// SchemeManagerResources returns a map of Permission to a ResourceServer
// capable of serving requests to PermissionSchemeManager.
func SchemeManagerResources(b workspace.SchemeManager) map[Permission]ResourceServer {
	return map[Permission]ResourceServer{
		PermissionSchemeManager: newSchemeManagerResourceServer(b),
	}
}

func dialSchemeManager(token uint32, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(workspace.SchemeManager), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewSchemeManager(broker, conn)
	clients.Store(token, c)
	return c, nil
}

// SchemeManager acquires the workspace's URI scheme manager with the given token.
func SchemeManager(token uint32, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	return dialSchemeManager(token, broker)
}
