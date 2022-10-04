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
	// PermissionWorkspace requests access to manage a workspace.
	PermissionWorkspace Permission = "_PermWorkspace"
)

type workspaceResourceServer struct {
	mu  sync.Mutex
	b   workspace.Workspace
	srv proto.MuxServer
}

func newWorkspaceResourceServer(b workspace.Workspace) *workspaceResourceServer {
	ret := new(workspaceResourceServer)
	ret.b = b
	return ret
}

func (s *workspaceResourceServer) Serve(
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
				server := workspacepb.NewServer(s.b, lock)
				workspacepb.RegisterWorkspaceServer(grpc, server)
			}
			return s.srv
		})
}

// WorkspaceResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Workspace's resources.
func WorkspaceResources(b workspace.Workspace) map[Permission]ResourceServer {
	s := newWorkspaceResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionWorkspace: s,
	}
}

func dialWorkspace(token uint32, broker proto.MuxBroker) (
	workspace.API, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(workspace.API), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	clients.Store(token, c)
	return c, nil
}

// Workspace acquires the workspace's API server with the given token.
func Workspace(token uint32, broker proto.MuxBroker) (
	workspace.API, error,
) {
	return dialWorkspace(token, broker)
}
