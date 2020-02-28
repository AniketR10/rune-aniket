package plugin

import (
	"sync"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

const (
	// PermissionBrowserWindowManager requests access to a browser's window manager.
	PermissionBrowserWindowManager Permission = "_PermBrowserWindowManager"
	// PermissionBrowserKeyMapper requests access to a browser's key mapper.
	PermissionBrowserKeyMapper = "_PermBrowserKeyMapper"
	// PermissionBrowserFileOpener requests access to open new files.
	PermissionBrowserFileOpener = "_PermBrowserFileOpener"
	// PermissionBrowserMessenger requests access to send messages to the UI.
	PermissionBrowserMessenger = "_PermBrowserMessenger"
)

type browserResource struct {
	b browser.Browser
}

func newBrowserResource(b browser.Browser) *browserResource {
	ret := new(browserResource)
	ret.b = b
	return ret
}

func (s *browserResource) serve(perm Permission) resourceServerFn {
	return func(pluginID string, grantID uint32, broker proto.MuxBroker, lock sync.Locker) {
		broker.AcceptAndServe(grantID, func(opts []grpc.ServerOption) *grpc.Server {
			grpcServer := grpc.NewServer(opts...)
			server := browser.NewServer(broker, s.b, lock)
			switch perm {
			case PermissionBrowserWindowManager:
				proto.RegisterWindowManagerServer(grpcServer, server)
			case PermissionBrowserKeyMapper:
				proto.RegisterKeyMapperServer(grpcServer, server)
			case PermissionBrowserFileOpener:
				proto.RegisterFileOpenerServer(grpcServer, server)
			case PermissionBrowserMessenger:
				proto.RegisterMessengerServer(grpcServer, server)
			}
			return grpcServer
		})
	}
}

func (s *browserResource) ServeWindowManager() ResourceServer {
	return s.serve(PermissionBrowserWindowManager)
}
func (s *browserResource) ServeFileOpener() ResourceServer {
	return s.serve(PermissionBrowserFileOpener)
}
func (s *browserResource) ServeKeyMapper() ResourceServer {
	return s.serve(PermissionBrowserKeyMapper)
}
func (s *browserResource) ServeMessenger() ResourceServer {
	return s.serve(PermissionBrowserMessenger)
}

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(b browser.Browser) map[Permission]ResourceServer {
	server := newBrowserResource(b)
	return map[Permission]ResourceServer{
		PermissionBrowserWindowManager: server.ServeWindowManager(),
		PermissionBrowserKeyMapper:     server.ServeKeyMapper(),
		PermissionBrowserFileOpener:    server.ServeFileOpener(),
		PermissionBrowserMessenger:     server.ServeMessenger(),
	}
}

func dialBrowser(token uint32, broker proto.MuxBroker) (browser.Browser, error) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	return browser.NewClient(broker, conn), nil
}

// WindowManager acquires the browser's WindowManager resources with the given token.
func WindowManager(token uint32, broker proto.MuxBroker) (browser.WindowManager, error) {
	b, err := dialBrowser(token, broker)
	if err != nil {
		return nil, err
	}
	return b.(browser.WindowManager), nil
}

// KeyMapper acquires the browser's WindowManager resources with the given token.
func KeyMapper(token uint32, broker proto.MuxBroker) (browser.KeyMapper, error) {
	b, err := dialBrowser(token, broker)
	if err != nil {
		return nil, err
	}
	return b.(browser.KeyMapper), nil
}

// FileOpener acquires the browser's WindowManager resources with the given token.
func FileOpener(token uint32, broker proto.MuxBroker) (browser.FileOpener, error) {
	b, err := dialBrowser(token, broker)
	if err != nil {
		return nil, err

	}
	return b.(browser.FileOpener), nil
}

// Messenger acquires the browser's WindowManager resources with the given token.
func Messenger(token uint32, broker proto.MuxBroker) (browser.Messenger, error) {
	b, err := dialBrowser(token, broker)
	if err != nil {
		return nil, err

	}
	return b.(browser.Messenger), nil
}
