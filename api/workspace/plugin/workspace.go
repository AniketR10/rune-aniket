package plugin

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dialWorkspace(token uint32, broker proto.MuxBroker) (
	workspaceapi.Workspace, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	return c, nil
}

// Workspace acquires the workspace's API server with the given token.
func Workspace(token uint32, broker proto.MuxBroker) (
	workspaceapi.Workspace, error,
) {
	return dialWorkspace(token, broker)
}
