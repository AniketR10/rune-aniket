package plugin

import (
	"io"
	"sync"

	browserplugin "unstable.build/go-tui/api/browser/plugin"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
)

type browserResourceServer struct {
	b browser.Browser
}

type browserResourcePermissionServer struct {
	p Permission
	*browserResourceServer
}

func newBrowserResourceServer(b browser.Browser) *browserResourceServer {
	ret := new(browserResourceServer)
	ret.b = b
	return ret
}

func (s *browserResourceServer) forPermission(p string) ResourceRegistrar {
	return browserResourcePermissionServer{p: Permission(p), browserResourceServer: s}
}

func (s browserResourcePermissionServer) Register(
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := browserpb.NewServer(broker, s.b, lock)
	rpcServer := interruptBrowserServer(server, interrupt)
	switch string(s.p) {
	case browserplugin.PermissionBrowserWindowManager:
		browserpb.RegisterWindowManagerServer(registrar, rpcServer)
	case browserplugin.PermissionBrowserResourceOpener:
		browserpb.RegisterResourceOpenerServer(registrar, rpcServer)
	case browserplugin.PermissionBrowserMessenger:
		browserpb.RegisterMessengerServer(registrar, rpcServer)
	case browserplugin.PermissionBrowserEventPublisher:
		browserpb.RegisterEventPublisherServer(registrar, rpcServer)
	}
	return browserCloser{server}, nil
}

type browserCloser struct {
	server *browserpb.Server
}

func (b browserCloser) Close() error {
	return b.server.Stop()
}

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(b browser.Browser) map[Permission]ResourceRegistrar {
	s := newBrowserResourceServer(b)
	return map[Permission]ResourceRegistrar{
		Permission(browserplugin.PermissionBrowserWindowManager): s.forPermission(
			browserplugin.PermissionBrowserWindowManager),
		Permission(browserplugin.PermissionBrowserResourceOpener): s.forPermission(
			browserplugin.PermissionBrowserResourceOpener),
		Permission(browserplugin.PermissionBrowserMessenger): s.forPermission(
			browserplugin.PermissionBrowserMessenger),
		Permission(browserplugin.PermissionBrowserEventPublisher): s.forPermission(
			browserplugin.PermissionBrowserEventPublisher),
	}
}
