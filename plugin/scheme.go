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
	// PermissionSchemeManager requests access to the workspace's URI scheme manager.
	PermissionSchemeManager Permission = "_PermSchemeManager"
)

type schemeManagerResourceServer struct {
	b workspace.SchemeManager
}

func newSchemeManagerResourceServer(b workspace.SchemeManager) *schemeManagerResourceServer {
	ret := new(schemeManagerResourceServer)
	ret.b = b
	return ret
}

func (s *schemeManagerResourceServer) Register(
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := workspacepb.NewSchemeManagerServer(broker, s.b, lock)
	workspacepb.RegisterManagerServer(registrar, server)
	return server, nil
}

// SchemeManagerResources returns a map of Permission to a ResourceServer
// capable of serving requests to PermissionSchemeManager.
func SchemeManagerResources(b workspace.SchemeManager) map[Permission]ResourceRegistrar {
	return map[Permission]ResourceRegistrar{
		PermissionSchemeManager: newSchemeManagerResourceServer(b),
	}
}

func dialSchemeManager(token uint32, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewSchemeManager(broker, conn)
	runtime.SetFinalizer(c, func(c *workspacepb.SchemeManagerClient) { c.Close() })
	return c, nil
}

// SchemeManager acquires the workspace's URI scheme manager with the given token.
func SchemeManager(token uint32, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	return dialSchemeManager(token, broker)
}
