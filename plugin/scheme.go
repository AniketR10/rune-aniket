package plugin

import (
	"io"
	"sync"

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
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
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

// TODO move to api package
func dialSchemeManager(grant Grant, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	conn, err := broker.DialChannel(grant.Token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewSchemeManager(broker, conn)
	wg := WaitGroupFromContext(grant.Context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-grant.Context.Done()
		_ = c.Close()
	}()
	return c, nil
}

// SchemeManager acquires the workspace's URI scheme manager with the given token.
func SchemeManager(grant Grant, broker proto.MuxBroker) (
	workspace.SchemeManager, error,
) {
	return dialSchemeManager(grant, broker)
}
