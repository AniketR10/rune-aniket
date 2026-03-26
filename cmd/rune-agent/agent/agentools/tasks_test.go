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
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"
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
