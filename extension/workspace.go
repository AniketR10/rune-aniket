package extension

import (
	"context"
	"io"
	"sync"
	"time"

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
	extensionID string, grantor Grantor, registrar proto.ServiceRegistrar,
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
	var cancelFn func()
	ctx, cancelFn = context.WithCancel(ctx)
	cmd.Watcher = newWrapWatcher(cmd.Watcher, cancelFn)
	ctx = bluectx.First(w.ctx, ctx)
	return w.Workspace.StartCommand(ctx, cmd)
}

func (w *trackingWorkspace) NewPty(ctx context.Context) (
	workspaceapi.Pty, error,
) {
	// FIXME this temporarily leaks a goroutine, once session is closed
	// but ctx or s.ctx have not been canceled yet (workspace is still active,
	// or extension is still active).
	// Since pty capability might be removed from a scheme, once sysprocattr
	// is enabled or if we decide to just remove it, it's ok to leave it
	// like this for now.
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

// wrap watcher to ensure that one of bluectx.First ctxs gets canceled
type wrapWatcher struct {
	watcher workspaceapi.Watcher
	ch      chan error
}

func newWrapWatcher(watcher workspaceapi.Watcher, cancelFn func()) wrapWatcher {
	ret := wrapWatcher{
		watcher: watcher,
		ch:      make(chan error),
	}
	go func() {
		err := <-ret.ch
		cancelFn()
		if ret.watcher != nil && ret.watcher.Watch() != nil {
			t := time.After(2 * time.Minute) // in case watcher is unresponsive
			select {
			case ret.watcher.Watch() <- err:
			case <-t:
			}
		}
	}()
	return ret
}

func (w wrapWatcher) Watch() chan error {
	return w.ch
}
