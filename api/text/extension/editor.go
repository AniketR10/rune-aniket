package extension

import (
	"os"

	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
	textpb "unstable.build/go-tui/text/rpc"
)

func dialEditor(grant extension.Grant, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "editor", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := textpb.NewClient(grant.Context, broker, conn)
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(grant extension.Grant, broker proto.MuxBroker) (
	textapi.Editor, error,
) {
	return dialEditor(grant, broker)
}
