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
	// PermissionBrowserResourceOpener requests access to open new files.
	PermissionBrowserResourceOpener = "_PermBrowserResourceOpener"
	// PermissionBrowserMessenger requests access to send messages to the UI.
	PermissionBrowserMessenger = "_PermBrowserMessenger"
	// PermissionBrowserEventSubscriber requests access subscribe to term events.
	PermissionBrowserEventSubscriber = "_PermBrowserEventSubscriber"
	// PermissionBrowserEventPublisher requests access to publish term events.
	// This is useful if your plugin handler does async updates to its state, as
	// it enables interrupting the main event loop to redraw components.
	PermissionBrowserEventPublisher = "_PermBrowserEventPublisher"
)

type browserResourceServer struct {
	b browser.Browser
}

func newBrowserResourceServer(b browser.Browser) *browserResourceServer {
	ret := new(browserResourceServer)
	ret.b = b
	return ret
}

func (s *browserResourceServer) serve(perm Permission) resourceServerFn {
	return func(pluginID string, grantID uint32, broker proto.MuxBroker,
		lock sync.Locker, interruptDraw, interruptHandle func()) {

		broker.AcceptAndServe(grantID, func(opts []grpc.ServerOption) *grpc.Server {
			grpcServer := grpc.NewServer(opts...)
			server := browser.NewServer(broker, s.b, lock,
				interruptDraw, interruptHandle)
			switch perm {
			case PermissionBrowserWindowManager:
				proto.RegisterWindowManagerServer(grpcServer, server)
			case PermissionBrowserKeyMapper:
				proto.RegisterKeyMapperServer(grpcServer, server)
			case PermissionBrowserResourceOpener:
				proto.RegisterResourceOpenerServer(grpcServer, server)
			case PermissionBrowserMessenger:
				proto.RegisterMessengerServer(grpcServer, server)
			case PermissionBrowserEventSubscriber:
				proto.RegisterEventSubscriberServer(grpcServer, server)
			case PermissionBrowserEventPublisher:
				proto.RegisterEventPublisherServer(grpcServer, server)
			}
			return grpcServer
		})
	}
}

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(b browser.Browser) map[Permission]ResourceServer {
	s := newBrowserResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionBrowserWindowManager:   s.serve(PermissionBrowserWindowManager),
		PermissionBrowserKeyMapper:       s.serve(PermissionBrowserKeyMapper),
		PermissionBrowserResourceOpener:  s.serve(PermissionBrowserResourceOpener),
		PermissionBrowserMessenger:       s.serve(PermissionBrowserMessenger),
		PermissionBrowserEventSubscriber: s.serve(PermissionBrowserEventSubscriber),
		PermissionBrowserEventPublisher:  s.serve(PermissionBrowserEventPublisher),
	}
}

func dialBrowser(token uint32, broker proto.MuxBroker) (
	browser.Browser, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	return browser.NewClient(broker, conn), nil
}

// WindowManager acquires the browser's WindowManager
// resource with the given token.
func WindowManager(token uint32, broker proto.MuxBroker) (
	browser.WindowManager, error,
) {
	return dialBrowser(token, broker)
}

// KeyMapper acquires the browser's KeyMapper
// resource with the given token.
func KeyMapper(token uint32, broker proto.MuxBroker) (
	browser.KeyMapper, error,
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

// EventSubscriber acquires the browser's EventSubscriber
// resource with the given token.
func EventSubscriber(token uint32, broker proto.MuxBroker) (
	browser.EventSubscriber, error,
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
