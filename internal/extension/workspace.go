// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package extension

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi/workspacerpc"
	"unstable.build/rune/internal/debug"
	"unstable.build/rune/internal/rpc"
	"unstable.build/rune/internal/workspace"
	tworkspacerpc "unstable.build/rune/internal/workspace/workspacerpc"
)

type workspaceResourceServer struct {
	b                 workspace.Workspace
	p                 extensionapi.Permission
	commandAuthorizer tworkspacerpc.CommandAuthorizer
}

func newWorkspaceResourceServer(
	b workspace.Workspace, p extensionapi.Permission,
	commandAuthorizer tworkspacerpc.CommandAuthorizer,
) *workspaceResourceServer {
	ret := new(workspaceResourceServer)
	ret.p = p
	ret.b = b
	ret.commandAuthorizer = commandAuthorizer
	return ret
}

func (s *workspaceResourceServer) Register(
	registrar rpc.ServiceRegistrar, _ sync.Locker,
) (io.Closer, error) {
	// create a closer able to close all processes created by grantee
	// without closing workspace.Workspace, which we cannot assume about
	// its lifecycle
	w := &trackingWorkspace{
		Workspace: s.b,
	}
	w.ctx, w.cancelCtx = context.WithCancel(context.Background())
	server := tworkspacerpc.NewServer(w, s.commandAuthorizer)
	switch s.p {
	case extensionapi.PermissionFileSystem:
		if !rpc.IsRegistered(registrar, workspacerpc.Files_ServiceDesc) {
			workspacerpc.RegisterFilesServer(registrar, server)
		}
		workspacerpc.RegisterSchemeServer(registrar, server)
	case extensionapi.PermissionTerminal:
		if !rpc.IsRegistered(registrar, workspacerpc.Files_ServiceDesc) {
			workspacerpc.RegisterFilesServer(registrar, server)
		}
		workspacerpc.RegisterTerminalServer(registrar, server)
	case extensionapi.PermissionExecute:
		workspacerpc.RegisterExecutorServer(registrar, server)
	}
	w.server = server
	return w, nil
}

// WorkspaceResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Workspace's resources.
func WorkspaceResources(
	b workspace.Workspace, commandAuthorizer tworkspacerpc.CommandAuthorizer,
) map[extensionapi.Permission]ResourceRegistrar {
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionFileSystem: newWorkspaceResourceServer(
			b, extensionapi.PermissionFileSystem, commandAuthorizer),
		extensionapi.PermissionTerminal: newWorkspaceResourceServer(
			b, extensionapi.PermissionTerminal, commandAuthorizer),
		extensionapi.PermissionExecute: newWorkspaceResourceServer(
			b, extensionapi.PermissionExecute, commandAuthorizer),
	}
}

type trackingWorkspace struct {
	workspace.Workspace
	ctx       context.Context
	cancelCtx func()
	server    *tworkspacerpc.Server
}

// OnDisconnect forwards [workspace.RemoteScheme.OnDisconnect] when the
// embedded Workspace's underlying scheme is remote. Returns nil
// for local workspaces.
func (w *trackingWorkspace) OnDisconnect() <-chan struct{} {
	if rs, ok := w.Workspace.(workspace.RemoteScheme); ok {
		return rs.OnDisconnect()
	}
	return nil
}

// WaitConnected forwards [workspace.RemoteScheme.WaitConnected] when
// the embedded Workspace is remote. Local workspaces are always
// "connected".
func (w *trackingWorkspace) WaitConnected(ctx context.Context) error {
	if rs, ok := w.Workspace.(workspace.RemoteScheme); ok {
		return rs.WaitConnected(ctx)
	}
	return nil
}

func (w *trackingWorkspace) Command(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	ctx, cancelFn := bluectx.First(w.ctx, ctx)
	cmd.Watcher = newWrapWatcher(cmd.Watcher, cancelFn)
	return w.Workspace.StartCommand(ctx, cmd)
}

func (w *trackingWorkspace) NewPty(ctx context.Context) (
	workspaceapi.Pty, error,
) {
	return w.Workspace.NewPty(w.ctx)
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
	watcher workspaceapi.ProcessWatcher
	ch      chan error
}

func newWrapWatcher(watcher workspaceapi.ProcessWatcher, cancelFn func()) wrapWatcher {
	ret := wrapWatcher{
		watcher: watcher,
		ch:      make(chan error),
	}
	go debug.CapturePanicReport(func() {
		err := <-ret.ch
		cancelFn()
		if ret.watcher != nil && ret.watcher.WatchProcess() != nil {
			t := time.After(2 * time.Minute) // in case watcher is unresponsive
			select {
			case ret.watcher.WatchProcess() <- err:
			case <-t:
			}
		}
	})
	return ret
}

func (w wrapWatcher) WatchProcess() chan error {
	return w.ch
}
