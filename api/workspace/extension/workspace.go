package extension

import (
	"context"
	"os"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dial(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	*workspacepb.Client, error,
) {
	conn, err := broker.DialChannel(ctx, grant.Token,
		os.Args[0], "workspace", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	return c, nil
}

// FileSystem acquires the workspace's file-system with the given token.
func FileSystem(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	workspaceapi.FileSystem, error,
) {
	return dial(ctx, grant, broker)
}

// Executor acquires the workspace's processes executor with the given token.
func Executor(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	workspaceapi.Executor, error,
) {
	return dial(ctx, grant, broker)
}

// Terminal acquires the workspace's pseudo-terminal with the given token.
func Terminal(ctx context.Context, grant extension.Grant, broker rpc.MuxBroker) (
	workspaceapi.Terminal, error,
) {
	return dial(ctx, grant, broker)
}
