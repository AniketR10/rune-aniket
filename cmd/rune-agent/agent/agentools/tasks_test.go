// Copyright (C) 2017-2026 The Rune Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/cmd/rune-agent/agent/taskstore"
)

type mockProgressUpdater struct {
	calls []taskstore.Task
}

func (m *mockProgressUpdater) UpdateTaskProgress(_ context.Context, task taskstore.Task) {
	m.calls = append(m.calls, task)
}

func TestTaskCreateValid(t *testing.T) {
	store := taskstore.New()
	updater := &mockProgressUpdater{}
	tool := &taskCreateTool{store: store, updater: updater}

	result := tool.Execute(context.Background(), `{"subject":"Build store","description":"Implement the task store"}`)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, `"ID":"1"`)
	assert.Contains(t, result.Content, `"Subject":"Build store"`)
	assert.Contains(t, result.Content, `"Status":"pending"`)
	require.Len(t, updater.calls, 1)
	assert.Equal(t, "Build store", updater.calls[0].Subject)
}

func TestTaskCreateWithActiveForm(t *testing.T) {
	store := taskstore.New()
	updater := &mockProgressUpdater{}
	tool := &taskCreateTool{store: store, updater: updater}

	result := tool.Execute(context.Background(), `{"subject":"Test","description":"Run tests","activeForm":"Running tests"}`)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, `"ActiveForm":"Running tests"`)
}

func TestTaskCreateMissingSubject(t *testing.T) {
	store := taskstore.New()
	tool := &taskCreateTool{store: store, updater: &mockProgressUpdater{}}

	result := tool.Execute(context.Background(), `{"description":"desc"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "subject is required")
}

func TestTaskCreateMissingDescription(t *testing.T) {
	store := taskstore.New()
	tool := &taskCreateTool{store: store, updater: &mockProgressUpdater{}}

	result := tool.Execute(context.Background(), `{"subject":"test"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "description is required")
}

func TestTaskCreateInvalidJSON(t *testing.T) {
	store := taskstore.New()
	tool := &taskCreateTool{store: store, updater: &mockProgressUpdater{}}

	result := tool.Execute(context.Background(), `{invalid}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestTaskUpdateStatus(t *testing.T) {
	store := taskstore.New()
	updater := &mockProgressUpdater{}
	store.Create("task", "desc", "", nil)
	tool := &taskUpdateTool{store: store, updater: updater}

	result := tool.Execute(context.Background(), `{"taskId":"1","status":"in_progress"}`)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, `"Status":"in_progress"`)
	require.Len(t, updater.calls, 1)
	assert.Equal(t, "in_progress", updater.calls[0].Status)
}

func TestTaskUpdateNonExistentID(t *testing.T) {
	store := taskstore.New()
	tool := &taskUpdateTool{store: store, updater: &mockProgressUpdater{}}

	result := tool.Execute(context.Background(), `{"taskId":"999"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not found")
}

func TestTaskUpdateMissingTaskID(t *testing.T) {
	store := taskstore.New()
	tool := &taskUpdateTool{store: store, updater: &mockProgressUpdater{}}

	result := tool.Execute(context.Background(), `{}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "taskId is required")
}

func TestTaskUpdateDeleted(t *testing.T) {
	store := taskstore.New()
	updater := &mockProgressUpdater{}
	store.Create("task", "desc", "", nil)
	tool := &taskUpdateTool{store: store, updater: updater}

	result := tool.Execute(context.Background(), `{"taskId":"1","status":"deleted"}`)
	assert.False(t, result.IsError)

	tasks := store.List()
	assert.Empty(t, tasks)
}

func TestTaskGetValid(t *testing.T) {
	store := taskstore.New()
	store.Create("my task", "my description", "Working", nil)
	tool := &taskGetTool{store: store}

	result := tool.Execute(context.Background(), `{"taskId":"1"}`)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, `"Subject":"my task"`)
	assert.Contains(t, result.Content, `"Description":"my description"`)
}

func TestTaskGetNonExistent(t *testing.T) {
	store := taskstore.New()
	tool := &taskGetTool{store: store}

	result := tool.Execute(context.Background(), `{"taskId":"999"}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not found")
}

func TestTaskGetMissingTaskID(t *testing.T) {
	store := taskstore.New()
	tool := &taskGetTool{store: store}

	result := tool.Execute(context.Background(), `{}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "taskId is required")
}

func TestTaskListEmpty(t *testing.T) {
	store := taskstore.New()
	tool := &taskListTool{store: store}

	result := tool.Execute(context.Background(), `{}`)
	assert.False(t, result.IsError)
	assert.Equal(t, "[]", result.Content)
}

func TestTaskListMultiple(t *testing.T) {
	store := taskstore.New()
	store.Create("first", "desc1", "", nil)
	store.Create("second", "desc2", "", nil)
	tool := &taskListTool{store: store}

	result := tool.Execute(context.Background(), `{}`)
	assert.False(t, result.IsError)

	var summaries []struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}
	err := json.Unmarshal([]byte(result.Content), &summaries)
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.Equal(t, "1", summaries[0].ID)
	assert.Equal(t, "first", summaries[0].Subject)
	assert.Equal(t, "pending", summaries[0].Status)
}

func TestTaskToolDefinitions(t *testing.T) {
	store := taskstore.New()
	tools := NewTaskTools(store, &mockProgressUpdater{})

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Definition().Function.Name
	}
	assert.Equal(t, []string{"TaskCreate", "TaskUpdate", "TaskGet", "TaskList"}, names)
}

func TestTaskCreateSummary(t *testing.T) {
	tool := &taskCreateTool{}
	assert.Equal(t, "Build store", tool.Summary(`{"subject":"Build store","description":"desc"}`))
	assert.Equal(t, "", tool.Summary(`{invalid}`))
}

func TestTaskUpdateSummary(t *testing.T) {
	tool := &taskUpdateTool{}
	assert.Equal(t, "#1 → completed", tool.Summary(`{"taskId":"1","status":"completed"}`))
	assert.Equal(t, "#1", tool.Summary(`{"taskId":"1"}`))
	assert.Equal(t, "", tool.Summary(`{invalid}`))
}

func TestTaskGetSummary(t *testing.T) {
	tool := &taskGetTool{}
	assert.Equal(t, "#1", tool.Summary(`{"taskId":"1"}`))
	assert.Equal(t, "", tool.Summary(`{invalid}`))
}

func TestTaskListSummary(t *testing.T) {
	tool := &taskListTool{}
	assert.Equal(t, "", tool.Summary(`{}`))
}
