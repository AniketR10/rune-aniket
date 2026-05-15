// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package workspace

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
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
	w, err := m.manager.AddWorkspace(m.parentCtx, workspaceapi.Dir(file))
	if err != nil {
		return nil, err
	}
	return w.Load(file, buf, swapDir, readOnly)
}

func (m multi) recoverExtraneous(
	file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool,
) (FlusherCloser, error) {
	workspace, err := m.manager.AddWorkspace(m.parentCtx, workspaceapi.Dir(file))
	if err != nil {
		return nil, err
	}
	return workspace.Recover(file, swapFilePath, buf, force)
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
