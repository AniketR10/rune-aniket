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
	"bytes"
	"context"
	"fmt"
	"io"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

type mockExec struct {
	startFn func(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error)
}

func (m *mockExec) Start(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return m.startFn(ctx, cmd)
}

func (m *mockExec) Signal(workspaceapi.Pid, syscall.Signal) error { return nil }
func (m *mockExec) Close() error                                  { return nil }

func TestRecallConversation_ValidID(t *testing.T) {
	const transcript = "### user\n\nhow do I fix this?\n\n### assistant\n\nHere's the fix."

	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			_, _ = io.WriteString(cmd.Stdout, transcript+"\n")
			cmd.Watcher.WatchProcess() <- nil
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	result := tool.Execute(context.Background(), `{"memory_id":"handler-refactor"}`)
	assert.False(t, result.IsError)
	assert.Equal(t, transcript, result.Content)
}

func TestRecallConversation_NotFound(t *testing.T) {
	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			_, _ = fmt.Fprintln(cmd.Stderr.(*bytes.Buffer), "memory not found: bad-id")
			cmd.Watcher.WatchProcess() <- fmt.Errorf("exit status 1")
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	result := tool.Execute(context.Background(), `{"memory_id":"bad-id"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "exit status 1")
}

func TestRecallConversation_EmptyMemoryID(t *testing.T) {
	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			cmd.Watcher.WatchProcess() <- nil
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	result := tool.Execute(context.Background(), `{"memory_id":""}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "memory_id is required")
}

func TestRecallConversation_InvalidJSON(t *testing.T) {
	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			cmd.Watcher.WatchProcess() <- nil
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	result := tool.Execute(context.Background(), `{bad json}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid")
}

func TestRecallConversation_Summary(t *testing.T) {
	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			cmd.Watcher.WatchProcess() <- nil
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	assert.Equal(t, "handler-refactor", tool.Summary(`{"memory_id":"handler-refactor"}`))
	assert.Equal(t, "", tool.Summary(`{bad}`))
}

func TestRecallConversation_Definition(t *testing.T) {
	exec := &mockExec{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			cmd.Watcher.WatchProcess() <- nil
			return 0, nil
		},
	}

	tool := NewRecallConversation(exec, "/data/memories")
	def := tool.Definition()
	assert.Equal(t, "recall_conversation", def.Function.Name)
}
