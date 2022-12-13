package plugin

import (
	"io"
	"sync"

	"google.golang.org/grpc"

	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

type workspaceResourceServer struct {
	b workspace.Workspace
}

func newWorkspaceResourceServer(b workspace.Workspace) *workspaceResourceServer {
	ret := new(workspaceResourceServer)
	ret.b = b
	return ret
}

func (s *workspaceResourceServer) Register(
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := workspacepb.NewServer(s.b, lock)
	workspacepb.RegisterWorkspaceServer(registrar, server)
	return nopCloser{}, nil
}

// WorkspaceResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Workspace's resources.
func WorkspaceResources(b workspace.Workspace) map[Permission]ResourceRegistrar {
	s := newWorkspaceResourceServer(b)
	return map[Permission]ResourceRegistrar{
		Permission(workspaceplugin.PermissionWorkspace): s,
	}
}
