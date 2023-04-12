package plugin

import (
	"os"

	schemeapi "unstable.build/go-tui/api/scheme"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dialSchemeManager(grant plugin.Grant, broker proto.MuxBroker) (
	schemeapi.SchemeManager, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "scheme", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewSchemeManager(grant.Context, broker, conn)
	return c, nil
}

// SchemeManager acquires the workspace's URI scheme manager with the given token.
func SchemeManager(grant plugin.Grant, broker proto.MuxBroker) (
	schemeapi.SchemeManager, error,
) {
	return dialSchemeManager(grant, broker)
}
