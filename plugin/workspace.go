package plugin

import (
	"context"
	"io"
	"sync"

	bluectx "github.com/ernestrc/blue/context"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

type workspaceResourceServer struct {
	b workspace.Workspace
	p Permission
}

func newWorkspaceResourceServer(b workspace.Workspace, p Permission) *workspaceResourceServer {
	ret := new(workspaceResourceServer)
	ret.p = p
	ret.b = b
	return ret
}

func (s *workspaceResourceServer) Register(
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	// create a closer able to close all processes created by grantee
	// without closing workspace.Workspace, which we cannot assume about
	// its lifecycle
	w := &trackingWorkspace{
		Workspace: s.b,
	}
	w.ctx, w.cancelCtx = context.WithCancel(context.Background())
	server := workspacepb.NewServer(w, lock)
	switch s.p {
	case PermissionFileSystem:
		if !proto.IsRegistered(registrar, workspacepb.Files_ServiceDesc) {
			workspacepb.RegisterFilesServer(registrar, server)
		}
		workspacepb.RegisterSchemeServer(registrar, server)
	case PermissionTerminal:
		if !proto.IsRegistered(registrar, workspacepb.Files_ServiceDesc) {
			workspacepb.RegisterFilesServer(registrar, server)
		}
		workspacepb.RegisterTerminalServer(registrar, server)
	case PermissionExecute:
		workspacepb.RegisterExecutorServer(registrar, server)
	}
	w.server = server
	return w, nil
}

// WorkspaceResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Workspace's resources.
func WorkspaceResources(b workspace.Workspace) map[Permission]ResourceRegistrar {
	return map[Permission]ResourceRegistrar{
		PermissionFileSystem: newWorkspaceResourceServer(
			b, PermissionFileSystem),
		PermissionTerminal: newWorkspaceResourceServer(
			b, PermissionTerminal),
		PermissionExecute: newWorkspaceResourceServer(
			b, PermissionExecute),
	}
}

type trackingWorkspace struct {
	workspace.Workspace
	ctx       context.Context
	cancelCtx func()
	server    *workspacepb.Server
}

func (w *trackingWorkspace) Command(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	ctx = bluectx.First(w.ctx, ctx)
	return w.Workspace.StartCommand(ctx, cmd)
}

func (w *trackingWorkspace) NewPty(ctx context.Context) (
	workspaceapi.Pty, error,
) {
	ctx = bluectx.First(w.ctx, ctx)
	return w.Workspace.NewPty(ctx)
}

func (w *trackingWorkspace) Close() error {
	if w.cancelCtx != nil {
		cancelCtx := w.cancelCtx
		w.cancelCtx = nil
		cancelCtx()
		return w.server.Stop()
	}
	return nil
}
