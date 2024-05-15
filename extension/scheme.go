package extension

import (
	"io"
	"sync"

	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
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
	extensionID string, grantor Grantor, registrar rpc.ServiceRegistrar,
	broker rpc.MuxBroker, lock sync.Locker,
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
