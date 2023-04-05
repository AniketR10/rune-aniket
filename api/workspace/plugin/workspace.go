package plugin

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dial(token string, broker proto.MuxBroker) (
	*workspacepb.Client, error,
) {
	conn, err := broker.DialChannel(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	return c, nil
}

// FileSystem acquires the workspace's file-system with the given token.
func FileSystem(token string, broker proto.MuxBroker) (
	workspaceapi.FileSystem, error,
) {
	return dial(token, broker)
}

// Executor acquires the workspace's processes executor with the given token.
func Executor(token string, broker proto.MuxBroker) (
	workspaceapi.Executor, error,
) {
	return dial(token, broker)
}

// Terminal acquires the workspace's pseudo-terminal with the given token.
func Terminal(token string, broker proto.MuxBroker) (
	workspaceapi.Terminal, error,
) {
	return dial(token, broker)
}
