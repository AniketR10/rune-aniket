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

package workspace

import (
	"context"
	"fmt"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/cell"
)

var _ Workspace = (multi)(multi{})

// Multi wraps a Workspace to provide oob Recover and Load requests to other workspaces/schemes
// whether initialized or not.
func Multi(
	ctx context.Context, m WorkspaceManager, def Workspace, uri workspaceapi.URI,
) Workspace {
	return newMulti(ctx, m, uri, def)
}

type multi struct {
	parentCtx context.Context
	defURI    workspaceapi.URI
	Workspace
	manager WorkspaceManager
}

func newMulti(
	ctx context.Context, manager WorkspaceManager,
	defURI workspaceapi.URI, def Workspace,
) *multi {
	return &multi{parentCtx: ctx, Workspace: def, defURI: defURI, manager: manager}
}

func (m multi) Load(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.Workspace, file)
	if err != nil {
		return nil, fmt.Errorf("workspaceapi.URI: %s", err)
	}
	if !is {
		return m.loadExtraneous(file, buf, swapDir, readOnly)
	}
	return m.Workspace.Load(file, buf, swapDir, readOnly)
}

func (m multi) Recover(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	is, err := IsWorkspaceURI(m.Workspace, file)
	if err != nil {
		return nil, fmt.Errorf("workspaceapi.URI: %s", err)
	}
	if !is {
		return m.recoverExtraneous(file, swapFilePath, buf, force)
	}
	return m.Workspace.Recover(file, swapFilePath, buf, force)
}

func (m multi) loadExtraneous(
	file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool,
) (FlusherCloser, error) {
	_, ok, err := m.manager.Workspace(file)
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, ErrOpenInOtherWorkspace
	}
	dir := workspaceapi.Dir(file)
	w, err := m.manager.AddWorkspace(m.parentCtx, dir)
	if err != nil {
		return nil, err
	}
	fc, err := w.Load(file, buf, swapDir, readOnly)
	if err != nil {
		return nil, err
	}
	return m.wrapExtraneous(dir, fc), nil
}

func (m multi) recoverExtraneous(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	dir := workspaceapi.Dir(file)
	workspace, err := m.manager.AddWorkspace(m.parentCtx, dir)
	if err != nil {
		return nil, err
	}
	fc, err := workspace.Recover(file, swapFilePath, buf, force)
	if err != nil {
		return nil, err
	}
	return m.wrapExtraneous(dir, fc), nil
}

// wrapExtraneous increments the workspace refcount for dir and
// returns a FlusherCloser whose Close decrements it exactly once.
func (m multi) wrapExtraneous(dir workspaceapi.URI, fc FlusherCloser) FlusherCloser {
	m.manager.IncrementReference(dir)
	return &extraneousCloser{FlusherCloser: fc, manager: m.manager, dir: dir}
}

// extraneousCloser wraps a per-file FlusherCloser so its Close
// also decrements the workspace refcount the file represents.
// Close is idempotent: a repeated call returns the cached error
// without decrementing twice, so a tab that runs Close from both
// its normal teardown and a shutdown path cannot under-count the
// workspace.
type extraneousCloser struct {
	FlusherCloser
	manager  WorkspaceManager
	dir      workspaceapi.URI
	closeOne sync.Once
	closeErr error
}

func (e *extraneousCloser) Close() error {
	e.closeOne.Do(func() {
		e.closeErr = e.FlusherCloser.Close()
		if relErr := e.manager.DecrementReference(e.dir); relErr != nil && e.closeErr == nil {
			e.closeErr = relErr
		}
	})
	return e.closeErr
}

// OnDisconnect forwards to the wrapped workspace's RemoteScheme
// when present. The embedded Workspace's method set does not
// include OnDisconnect (RemoteScheme is an optional interface), so
// without this explicit forwarder a multi value would not satisfy
// RemoteScheme even when the underlying workspace does — and
// callers like vtereservoir.New that type-assert against
// RemoteScheme to install transport-drop watchers would silently
// skip installation, leaving dead VTEs in the pool after SSH
// reconnects.
func (m multi) OnDisconnect() <-chan struct{} {
	if rs, ok := m.Workspace.(RemoteScheme); ok {
		return rs.OnDisconnect()
	}
	return nil
}

// WaitConnected forwards [RemoteScheme.WaitConnected] when the
// wrapped workspace is remote. Local workspaces are always
// "connected".
func (m multi) WaitConnected(ctx context.Context) error {
	if rs, ok := m.Workspace.(RemoteScheme); ok {
		return rs.WaitConnected(ctx)
	}
	return nil
}
