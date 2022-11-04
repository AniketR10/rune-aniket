package plugin

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/debug"
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
	srv    proto.MuxServer
}

func newBrowserResourceServer(b browser.Browser) *browserResourceServer {
	ret := new(browserResourceServer)
	ret.b = b
	return ret
}

func (s *browserResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			// NOTE: unfortunately all plugins must share the same browser.Server,
			// because it's internal state is not shared.
			if s.srv == nil {
				var srv proto.MuxServer
				l := debug.StandardLogger()
				if l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				s.server = new(browser.Server)
				s.server.Init(broker, s.b, lock, interruptWindowServer)
				rpcServer := interruptBrowserServer(s.server, interrupt)
				browserpb.RegisterWindowManagerServer(grpc, rpcServer)
				browserpb.RegisterResourceOpenerServer(grpc, rpcServer)
				browserpb.RegisterMessengerServer(grpc, rpcServer)
				browserpb.RegisterEventPublisherServer(grpc, rpcServer)
			}
			return s.srv
		})
}

func (s *browserResourceServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		s.srv.Stop()
		s.srv = nil
		return s.server.Close()
	}
	return nil
}

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(b browser.Browser) map[Permission]ResourceServer {
	s := newBrowserResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionBrowserWindowManager:  s,
		PermissionBrowserResourceOpener: s,
		PermissionBrowserMessenger:      s,
		PermissionBrowserEventPublisher: s,
	}
}

func dialBrowser(token uint32, broker proto.MuxBroker) (
	browser.Browser, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(browser.Browser), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := browser.NewClient(broker, conn)
	clients.Store(token, c)
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
