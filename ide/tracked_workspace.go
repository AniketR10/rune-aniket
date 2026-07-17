// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package ide

import (
	"context"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/ide/ideshell/workspaceshell"
	"unstable.build/go-tui/workspace"
)

// trackedWorkspace wraps a workspace.Workspace, overriding StartCommand
// and Signal to delegate to a workspaceshell.Executor so that all
// processes started through this workspace — including those accessed
// via gRPC by extensions — are tracked.
type trackedWorkspace struct {
	workspace.Workspace
	exec *workspaceshell.Executor
}

// OnDisconnect forwards [workspace.RemoteScheme.OnDisconnect] when the
// embedded Workspace's underlying scheme is remote. Returns nil
// for local workspaces, which callers interpret as "no transport
// invalidation events will ever fire".
func (w *trackedWorkspace) OnDisconnect() <-chan struct{} {
	if rs, ok := w.Workspace.(workspace.RemoteScheme); ok {
		return rs.OnDisconnect()
	}
	return nil
}

// WaitConnected forwards [workspace.RemoteScheme.WaitConnected] when
// the embedded Workspace is remote. Local workspaces are always
// "connected".
func (w *trackedWorkspace) WaitConnected(ctx context.Context) error {
	if rs, ok := w.Workspace.(workspace.RemoteScheme); ok {
		return rs.WaitConnected(ctx)
	}
	return nil
}

func (w *trackedWorkspace) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return w.exec.StartCommand(ctx, cmd)
}

func (w *trackedWorkspace) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	return w.exec.Signal(pid, sig)
}
