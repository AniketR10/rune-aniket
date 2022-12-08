package plugin

import (
	browserapi "unstable.build/go-tui/api/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
)

func dialBrowser(token uint32, broker proto.MuxBroker) (
	browserapi.Browser, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := browserpb.NewClient(broker, conn)
	return c, nil
}

// WindowManager acquires the browser's WindowManager
// resource with the given token.
func WindowManager(token uint32, broker proto.MuxBroker) (
	browserapi.WindowManager, error,
) {
	return dialBrowser(token, broker)
}

// ResourceOpener acquires the browser's ResourceOpener
// resource with the given token.
func ResourceOpener(token uint32, broker proto.MuxBroker) (
	browserapi.ResourceOpener, error,
) {
	return dialBrowser(token, broker)
}

// Messenger acquires the browser's Messenger
// resource with the given token.
func Messenger(token uint32, broker proto.MuxBroker) (
	browserapi.Messenger, error,
) {
	return dialBrowser(token, broker)
}

// EventPublisher acquires the browser's EventPublisher
// resource with the given token.
func EventPublisher(token uint32, broker proto.MuxBroker) (
	browserapi.EventPublisher, error,
) {
	return dialBrowser(token, broker)
}
