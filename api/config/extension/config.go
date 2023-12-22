package extension

import (
	"context"
	"os"

	"unstable.build/go-tui/api/config"
	configpb "unstable.build/go-tui/api/config/rpc"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
)

func dialConfig(ctx context.Context, grant extension.Grant, broker proto.MuxBroker) (
	config.Config, error,
) {
	conn, err := broker.DialChannel(ctx, grant.Token,
		os.Args[0], "config", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c, err := configpb.FetchConfig(conn)
	if err != nil {
		return nil, err
	}
	return c, conn.Close()
}

// FetchConfig acquires the loaded config with the given permission token.
func FetchConfig(ctx context.Context, grant extension.Grant, broker proto.MuxBroker) (
	config.Config, error,
) {
	return dialConfig(ctx, grant, broker)
}
