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

package idelsp

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace/processctx"
)

type recordingStartExecutor struct {
	ctx context.Context
	cmd workspaceapi.Cmd
	err error
}

func (e *recordingStartExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	e.ctx = ctx
	e.cmd = cmd
	return 0, e.err
}

func (e *recordingStartExecutor) Signal(
	workspaceapi.Pid, syscall.Signal,
) error {
	return nil
}

func (e *recordingStartExecutor) Close() error {
	return nil
}

func TestLangServerStartCarriesProcessContext(t *testing.T) {
	startErr := errors.New("start failed")
	exec := &recordingStartExecutor{err: startErr}
	srv := newLangServer(
		context.Background(),
		langConfig{id: "go", command: "gopls", args: []string{"serve"}},
		"gopls",
		exec,
		"file:///workspace",
		nil,
		semanticapi.InitializeParams{},
	)

	ctx := processctx.ContextWithExtensionID(context.Background(), "go")
	err := srv.start(ctx)
	require.ErrorIs(t, err, startErr)

	extensionID, ok := processctx.ExtensionIDFromContext(exec.ctx)
	require.True(t, ok)
	assert.Equal(t, "go", extensionID)
	assert.Equal(t, "gopls", exec.cmd.Path)
	assert.Equal(t, []string{"serve"}, exec.cmd.Args)
}
