package plugin

import (
	"io"
	"sync"

	schemeapi "unstable.build/go-tui/api/scheme"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

const (
	// PermissionSchemeManager requests access to the workspace's URI scheme manager.
	PermissionSchemeManager Permission = "_PermSchemeManager"
)

type schemeManagerResourceServer struct {
	b schemeapi.SchemeManager
}

func newSchemeManagerResourceServer(b schemeapi.SchemeManager) *schemeManagerResourceServer {
	ret := new(schemeManagerResourceServer)
	ret.b = b
	return ret
}

func (s *schemeManagerResourceServer) Register(
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := workspacepb.NewSchemeManagerServer(broker, s.b, lock)
	workspacepb.RegisterManagerServer(registrar, server)
	return server, nil
}

// SchemeManagerResources returns a map of Permission to a ResourceServer
// capable of serving requests to PermissionSchemeManager.
func SchemeManagerResources(b schemeapi.SchemeManager) map[Permission]ResourceRegistrar {
	return map[Permission]ResourceRegistrar{
		PermissionSchemeManager: newSchemeManagerResourceServer(b),
	}
}
