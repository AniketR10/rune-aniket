package plugin

import (
	"io"
	"runtime"
	"sync"

	"google.golang.org/grpc"

	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

const (
	// PermissionWorkspace requests access to manage a workspace.
	PermissionWorkspace Permission = "_PermWorkspace"
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
		PermissionWorkspace: s,
	}
}

func dialWorkspace(token uint32, broker proto.MuxBroker) (
	workspace.API, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	runtime.SetFinalizer(c, func(c *workspacepb.Client) { c.Close() })
	return c, nil
}

// Workspace acquires the workspace's API server with the given token.
func Workspace(token uint32, broker proto.MuxBroker) (
	workspace.API, error,
) {
	return dialWorkspace(token, broker)
}
