package plugin

import (
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	textpb "unstable.build/go-tui/text/rpc"
)

func dialEditor(grant plugin.Grant, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	conn, err := broker.DialChannel(grant.Token)
	if err != nil {
		return nil, err
	}
	c := textpb.NewClient(broker, conn)
	wg := plugin.WaitGroupFromContext(grant.Context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-grant.Context.Done()
		c.Close()
	}()
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(grant plugin.Grant, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	return dialEditor(grant, broker)
}
