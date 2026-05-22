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
	"errors"
	"sync/atomic"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

type currentExecutor struct {
	inner atomic.Pointer[schemeapi.Executor]
}

func (d *currentExecutor) set(exe schemeapi.Executor) {
	d.inner.Store(&exe)
}

func (d *currentExecutor) get() schemeapi.Executor {
	p := d.inner.Load()
	if p == nil {
		return nil
	}
	return *p
}

// StartCommand satisfies schemeapi.Executor.
func (d *currentExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	exe := d.get()
	if exe == nil {
		return 0, errors.New("currentExecutor: no underlying executor installed")
	}
	return exe.StartCommand(ctx, cmd)
}

// Signal satisfies schemeapi.Executor.
func (d *currentExecutor) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	exe := d.get()
	if exe == nil {
		return errors.New("currentExecutor: no underlying executor installed")
	}
	return exe.Signal(pid, sig)
}

// Close satisfies io.Closer. Close is a no-op on the proxy itself;
// ownership of the underlying executor lifecycle stays with the
// component that produced it (workspace_handler, doInit).
func (d *currentExecutor) Close() error { return nil }
