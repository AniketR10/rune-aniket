package extension

import (
	"context"
	"os"

	browserapi "unstable.build/go-tui/api/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
)

func dialBrowser(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	browserapi.Browser, error,
) {
	conn, err := broker.DialChannel(ctx, grant.Token,
		os.Args[0], "browser", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := browserpb.NewClient(grant.Context, broker, conn)
	return c, nil
}

// WindowManager acquires the browser's WindowManager
// resource with the given token.
func WindowManager(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	browserapi.WindowManager, error,
) {
	return dialBrowser(ctx, grant, broker)
}

// ResourceOpener acquires the browser's ResourceOpener
// resource with the given token.
func ResourceOpener(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	browserapi.ResourceOpener, error,
) {
	return dialBrowser(ctx, grant, broker)
}

// Notifications acquires the browser's Notifications
// resource with the given token.
func Notifications(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	browserapi.Notifications, error,
) {
	return dialBrowser(ctx, grant, broker)
}

// EventPublisher acquires the browser's EventPublisher
// resource with the given token.
func EventPublisher(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	browserapi.EventPublisher, error,
) {
	return dialBrowser(ctx, grant, broker)
}
