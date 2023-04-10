package plugin

import (
	"os"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dial(grant plugin.Grant, broker proto.MuxBroker) (
	*workspacepb.Client, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "workspace", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	return c, nil
}

// FileSystem acquires the workspace's file-system with the given token.
func FileSystem(grant plugin.Grant, broker proto.MuxBroker) (
	workspaceapi.FileSystem, error,
) {
	return dial(grant, broker)
}

// Executor acquires the workspace's processes executor with the given token.
func Executor(grant plugin.Grant, broker proto.MuxBroker) (
	workspaceapi.Executor, error,
) {
	return dial(grant, broker)
}

// Terminal acquires the workspace's pseudo-terminal with the given token.
func Terminal(grant plugin.Grant, broker proto.MuxBroker) (
	workspaceapi.Terminal, error,
) {
	return dial(grant, broker)
}
