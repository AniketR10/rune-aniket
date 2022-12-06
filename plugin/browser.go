package plugin

import (
	"io"
	"runtime"
	"sync"

	"google.golang.org/grpc"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"

	"unstable.build/go-tui/proto"
)

const (
	// PermissionBrowserWindowManager requests access to a browser's window manager.
	PermissionBrowserWindowManager Permission = "_PermBrowserWindowManager"
	// PermissionBrowserResourceOpener requests access to open new files.
	PermissionBrowserResourceOpener = "_PermBrowserResourceOpener"
	// PermissionBrowserMessenger requests access to send messages to the UI.
	PermissionBrowserMessenger = "_PermBrowserMessenger"
	// PermissionBrowserEventPublisher requests access to publish term events.
	// This is useful if your plugin handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	// TODO rename to Interrupt
	PermissionBrowserEventPublisher = "_PermBrowserEventPublisher"
)

type browserResourceServer struct {
	mu     sync.Mutex
	b      browser.Browser
	server *browser.Server
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
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// all permissions within a grantee must share the same browser.Server
	// it's stateful and it's internal cannot be shared.
	if s.server == nil {
		s.server = new(browser.Server)
		s.server.Init(broker, s.b, lock, interruptWindowServer)
	}
	rpcServer := interruptBrowserServer(s.server, interrupt)
	switch s.p {
	case PermissionBrowserWindowManager:
		browserpb.RegisterWindowManagerServer(registrar, rpcServer)
	case PermissionBrowserResourceOpener:
		browserpb.RegisterResourceOpenerServer(registrar, rpcServer)
	case PermissionBrowserMessenger:
		browserpb.RegisterMessengerServer(registrar, rpcServer)
	case PermissionBrowserEventPublisher:
		browserpb.RegisterEventPublisherServer(registrar, rpcServer)
	}
	// browser.Server handles multiple call to Close gracefully
	return s.server, nil
}

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(b browser.Browser) map[Permission]ResourceRegistrar {
	s := newBrowserResourceServer(b)
	return map[Permission]ResourceRegistrar{
		PermissionBrowserWindowManager:  s.forPermission(PermissionBrowserWindowManager),
		PermissionBrowserResourceOpener: s.forPermission(PermissionBrowserResourceOpener),
		PermissionBrowserMessenger:      s.forPermission(PermissionBrowserMessenger),
		PermissionBrowserEventPublisher: s.forPermission(PermissionBrowserEventPublisher),
	}
}

func dialBrowser(token uint32, broker proto.MuxBroker) (
	browser.Browser, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := browser.NewClient(broker, conn)
	runtime.SetFinalizer(c, func(c *browser.Client) { c.Close() })
	return c, nil
}

// WindowManager acquires the browser's WindowManager
// resource with the given token.
func WindowManager(token uint32, broker proto.MuxBroker) (
	browser.WindowManager, error,
) {
	return dialBrowser(token, broker)
}

// ResourceOpener acquires the browser's ResourceOpener
// resource with the given token.
func ResourceOpener(token uint32, broker proto.MuxBroker) (
	browser.ResourceOpener, error,
) {
	return dialBrowser(token, broker)
}

// Messenger acquires the browser's Messenger
// resource with the given token.
func Messenger(token uint32, broker proto.MuxBroker) (
	browser.Messenger, error,
) {
	return dialBrowser(token, broker)
}

// EventPublisher acquires the browser's EventPublisher
// resource with the given token.
func EventPublisher(token uint32, broker proto.MuxBroker) (
	browser.EventPublisher, error,
) {
	return dialBrowser(token, broker)
}
