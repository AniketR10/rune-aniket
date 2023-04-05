package plugin

import (
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/proto"
	textpb "unstable.build/go-tui/text/rpc"
)

func dialEditor(token string, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	conn, err := broker.DialChannel(token)
	if err != nil {
		return nil, err
	}
	c := textpb.NewClient(broker, conn)
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(token string, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	return dialEditor(token, broker)
}
