package plugin

import (
	browserapi "unstable.build/go-tui/api/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

func dialBrowser(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.Browser, error,
) {
	conn, err := broker.DialChannel(grant.Token)
	if err != nil {
		return nil, err
	}
	c := browserpb.NewClient(broker, conn)
	wg := plugin.WaitGroupFromContext(grant.Context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-grant.Context.Done()
		_ = c.Close()
	}()
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

// Messenger acquires the browser's Messenger
// resource with the given token.
func Messenger(grant plugin.Grant, broker proto.MuxBroker) (
	browserapi.Messenger, error,
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
