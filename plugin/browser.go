package plugin

import (
	"io"
	"sync"

	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
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

func (s *browserResourceServer) forPermission(p Permission) ResourceRegistrar {
	return browserResourcePermissionServer{p: p, browserResourceServer: s}
}

func (s browserResourcePermissionServer) Register(
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := browserpb.NewServer(broker, s.b, lock)
	rpcServer := interruptBrowserServer(server, func() {
		tui.PublishEvent(term.Event{Type: term.EventInterrupt})
	})
	switch s.p {
	case PermissionBrowserWindowManager:
		browserpb.RegisterWindowManagerServer(registrar, rpcServer)
	case PermissionBrowserResourceOpener:
		browserpb.RegisterResourceOpenerServer(registrar, rpcServer)
	case PermissionBrowserNotifications:
		browserpb.RegisterNotificationsServer(registrar, rpcServer)
	case PermissionBrowserEventPublisher:
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
		PermissionBrowserWindowManager: s.forPermission(
			PermissionBrowserWindowManager),
		PermissionBrowserResourceOpener: s.forPermission(
			PermissionBrowserResourceOpener),
		PermissionBrowserNotifications: s.forPermission(
			PermissionBrowserNotifications),
		PermissionBrowserEventPublisher: s.forPermission(
			PermissionBrowserEventPublisher),
	}
}
