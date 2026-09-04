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
