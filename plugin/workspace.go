package plugin

import (
	"sync"

	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/workspace"
	workspacepb "github.com/ernestrc/go-tui/workspace/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	// PermissionWorkspace requests access to execute a process in a workspace.
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
	l *log.Logger, lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.srv == nil {
				var srv proto.MuxServer
				if l != nil && l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				server := workspace.NewServer(s.b)
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
	workspace.Workspace, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(workspace.Workspace), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspace.NewClient(conn)
	clients.Store(token, c)
	return c, nil
}

// Workspace acquires the workspace's process executor with the given token.
func Workspace(token uint32, broker proto.MuxBroker) (
	workspace.Workspace, error,
) {
	return dialWorkspace(token, broker)
}
