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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWriteStdin(t *testing.T) (*writeStdinTool, *SessionManager) {
	t.Helper()
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	t.Cleanup(func() { _ = mgr.Close() })
	return NewWriteStdin(mgr).(*writeStdinTool), mgr
}

func TestWriteStdin_send_input(t *testing.T) {
	tool, mgr := newTestWriteStdin(t)

	// Start a cat process that echoes stdin.
	sess, err := mgr.Create("cat", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	// Write some data.
	args := fmt.Sprintf(`{"session_id":%d,"chars":"hello\n","yield_time_ms":500}`, sess.ID)
	result := tool.Execute(context.Background(), args)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Contains(t, out.Output, "hello")
}

func TestWriteStdin_session_not_found(t *testing.T) {
	tool, _ := newTestWriteStdin(t)
	result := tool.Execute(context.Background(), `{"session_id":99999,"chars":"x"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "session 99999 not found")
}

func TestWriteStdin_invalid_json(t *testing.T) {
	tool, _ := newTestWriteStdin(t)
	result := tool.Execute(context.Background(), `bad`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestWriteStdin_poll_exited_process(t *testing.T) {
	tool, mgr := newTestWriteStdin(t)

	sess, err := mgr.Create("echo done", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	// Wait for process to finish.
	sess.Wait(5 * time.Second)
	require.True(t, sess.Exited())

	// Poll with empty input.
	args := fmt.Sprintf(`{"session_id":%d,"chars":"","yield_time_ms":250}`, sess.ID)
	result := tool.Execute(context.Background(), args)
	assert.False(t, result.IsError)

	var out execCommandOutput
	require.NoError(t, json.Unmarshal([]byte(result.Content), &out))
	assert.Contains(t, out.Output, "done")
	require.NotNil(t, out.ExitCode)
	assert.Equal(t, 0, *out.ExitCode)
}

func TestWriteStdin_Definition(t *testing.T) {
	tool, _ := newTestWriteStdin(t)
	def := tool.Definition()
	assert.Equal(t, "write_stdin", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
	assert.NotNil(t, def.Function.Parameters)
}

func TestWriteStdin_Summary(t *testing.T) {
	tool, _ := newTestWriteStdin(t)
	assert.Equal(t, "session 1000: hello\n", tool.Summary(`{"session_id":1000,"chars":"hello\n"}`))
	assert.Equal(t, "poll session 1000", tool.Summary(`{"session_id":1000,"chars":""}`))
	assert.Equal(t, "", tool.Summary(`bad`))
}
