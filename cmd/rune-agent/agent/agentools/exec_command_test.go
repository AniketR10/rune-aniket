// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExecCommand(t *testing.T) (*execCommandTool, *SessionManager) {
	t.Helper()
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	t.Cleanup(func() { _ = mgr.Close() })
	dir := t.TempDir()
	tool := NewExecCommand(mgr, dirURI(dir)).(*execCommandTool)
	return tool, mgr
}

func TestExecCommand_success(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":"echo hello"}`)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Contains(t, out.Output, "hello")
	assert.NotNil(t, out.ExitCode)
	assert.Equal(t, 0, *out.ExitCode)
	assert.Equal(t, 0, out.SessionID) // 0 means exited
	assert.Greater(t, out.WallTimeSeconds, 0.0)
}

func TestExecCommand_nonzero_exit(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":"exit 7"}`)
	assert.True(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	require.NotNil(t, out.ExitCode)
	assert.Equal(t, 7, *out.ExitCode)
}

func TestExecCommand_long_running_returns_session_id(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":"sleep 60","yield_time_ms":250}`)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Greater(t, out.SessionID, 0, "should return session_id for running process")
	assert.Nil(t, out.ExitCode, "should not have exit_code for running process")
}

func TestExecCommand_workdir(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":"pwd","workdir":"/tmp"}`)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Contains(t, out.Output, "/tmp")
}

func TestExecCommand_custom_shell(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":"echo sh_test","shell":"sh"}`)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Contains(t, out.Output, "sh_test")
}

func TestExecCommand_empty_cmd(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `{"cmd":""}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "cmd must not be empty")
}

func TestExecCommand_invalid_json(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	result := tool.Execute(context.Background(), `bad`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestExecCommand_yield_time_clamped(t *testing.T) {
	tool, _ := newTestExecCommand(t)

	// Very small yield — should be clamped to min (250ms).
	result := tool.Execute(context.Background(), `{"cmd":"sleep 60","yield_time_ms":1}`)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Greater(t, out.SessionID, 0)
	// Wall time should be at least ~250ms.
	assert.GreaterOrEqual(t, out.WallTimeSeconds, 0.2)
}

func TestExecCommand_Definition(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	def := tool.Definition()
	assert.Equal(t, "exec_command", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
	assert.NotNil(t, def.Function.Parameters)
}

func TestExecCommand_Summary(t *testing.T) {
	tool, _ := newTestExecCommand(t)
	assert.Equal(t, "echo hello", tool.Summary(`{"cmd":"echo hello"}`))
	assert.Equal(t, "", tool.Summary(`bad`))
}

func TestClamp(t *testing.T) {
	assert.Equal(t, 5, clamp(5, 1, 10))
	assert.Equal(t, 1, clamp(0, 1, 10))
	assert.Equal(t, 10, clamp(99, 1, 10))
}
