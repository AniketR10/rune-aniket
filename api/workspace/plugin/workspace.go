package plugin

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func dial(grant plugin.Grant, broker proto.MuxBroker) (
	*workspacepb.Client, error,
) {
	conn, err := broker.DialChannel(grant.Token)
	if err != nil {
		return nil, err
	}
	c := workspacepb.NewClient(conn)
	wg := plugin.WaitGroupFromContext(grant.Context)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-grant.Context.Done()
		_ = c.Close()
	}()
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
