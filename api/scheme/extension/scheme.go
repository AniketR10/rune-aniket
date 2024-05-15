package extension

import (
	"context"
	"os"

	schemeapi "unstable.build/go-tui/api/scheme"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dialSchemeManager(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	schemeapi.SchemeManager, error,
) {
	conn, err := broker.DialChannel(ctx, grant.Token,
		os.Args[0], "scheme", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewSchemeManager(grant.Context, broker, conn)
	return c, nil
}

// SchemeManager acquires the workspace's URI scheme manager with the given token.
func SchemeManager(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	schemeapi.SchemeManager, error,
) {
	return dialSchemeManager(ctx, grant, broker)
}
