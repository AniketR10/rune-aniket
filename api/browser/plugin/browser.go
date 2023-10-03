package plugin

import (
	"os"

	browserapi "unstable.build/go-tui/api/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

func dialBrowser(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.Browser, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "browser", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := browserpb.NewClient(grant.Context, broker, conn)
	return c, nil
}

// WindowManager acquires the browser's WindowManager
// resource with the given token.
func WindowManager(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.WindowManager, error,
) {
	return dialBrowser(grant, broker)
}

// ResourceOpener acquires the browser's ResourceOpener
// resource with the given token.
func ResourceOpener(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.ResourceOpener, error,
) {
	return dialBrowser(grant, broker)
}

// Notifications acquires the browser's Notifications
// resource with the given token.
func Notifications(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.Notifications, error,
) {
	return dialBrowser(grant, broker)
}

// EventPublisher acquires the browser's EventPublisher
// resource with the given token.
func EventPublisher(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.EventPublisher, error,
) {
	return dialBrowser(grant, broker)
}
