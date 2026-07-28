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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"
)

func TestUpdatePlanTool(t *testing.T) {
	t.Run("updates checklist and returns success", func(t *testing.T) {
		updater := &mockProgressUpdater{}
		tool := NewUpdatePlan(updater)
		result := tool.Execute(context.Background(), `{
			"explanation": "Add auth middleware",
			"plan": [
				{"step": "Create middleware", "status": "completed"},
				{"step": "Add tests", "status": "in_progress"},
				{"step": "Update docs", "status": "pending"}
			]
		}`)

		assert.False(t, result.IsError)
		assert.False(t, result.ClearContext)
		assert.Equal(t, "Plan updated", result.Content)
		assert.Len(t, updater.calls, 3)
		assert.Equal(t, taskstore.Task{ID: "1", Subject: "Create middleware", Status: "completed"}, updater.calls[0])
		assert.Equal(t, taskstore.Task{ID: "2", Subject: "Add tests", Status: "in_progress"}, updater.calls[1])
		assert.Equal(t, taskstore.Task{ID: "3", Subject: "Update docs", Status: "pending"}, updater.calls[2])
	})

	t.Run("reuses existing task ids and updates statuses across calls", func(t *testing.T) {
		updater := &mockProgressUpdater{}
		tool := NewUpdatePlan(updater)

		first := tool.Execute(context.Background(), `{
			"plan": [
				{"step": "Inspect handlers", "status": "completed"},
				{"step": "Patch update_plan", "status": "in_progress"},
				{"step": "Add tests", "status": "pending"}
			]
		}`)
		assert.False(t, first.IsError)
		assert.Len(t, updater.calls, 3)

		second := tool.Execute(context.Background(), `{
			"plan": [
				{"step": "Patch update_plan", "status": "completed"},
				{"step": "Add tests", "status": "in_progress"}
			]
		}`)
		assert.False(t, second.IsError)
		require.Len(t, updater.calls, 6)

		assert.Equal(t, taskstore.Task{ID: "2", Subject: "Patch update_plan", Status: "completed"}, updater.calls[3])
		assert.Equal(t, taskstore.Task{ID: "3", Subject: "Add tests", Status: "in_progress"}, updater.calls[4])
		assert.Equal(t, "1", updater.calls[5].ID)
		assert.Equal(t, "Inspect handlers", updater.calls[5].Subject)
		assert.Equal(t, "deleted", updater.calls[5].Status)
	})

	t.Run("recreates deleted step with new id when it returns later", func(t *testing.T) {
		updater := &mockProgressUpdater{}
		tool := NewUpdatePlan(updater)

		assert.False(t, tool.Execute(context.Background(), `{
			"plan": [
				{"step": "One", "status": "pending"},
				{"step": "Two", "status": "pending"}
			]
		}`).IsError)
		assert.False(t, tool.Execute(context.Background(), `{
			"plan": [
				{"step": "Two", "status": "completed"}
			]
		}`).IsError)
		assert.False(t, tool.Execute(context.Background(), `{
			"plan": [
				{"step": "One", "status": "in_progress"},
				{"step": "Two", "status": "completed"}
			]
		}`).IsError)

		require.Len(t, updater.calls, 6)
		assert.Equal(t, taskstore.Task{ID: "1", Subject: "One", Status: "pending"}, updater.calls[0])
		assert.Equal(t, taskstore.Task{ID: "2", Subject: "Two", Status: "pending"}, updater.calls[1])
		assert.Equal(t, taskstore.Task{ID: "2", Subject: "Two", Status: "completed"}, updater.calls[2])
		assert.Equal(t, taskstore.Task{ID: "1", Subject: "One", Status: "deleted"}, updater.calls[3])
		assert.Equal(t, taskstore.Task{ID: "3", Subject: "One", Status: "in_progress"}, updater.calls[4])
		assert.Equal(t, taskstore.Task{ID: "2", Subject: "Two", Status: "completed"}, updater.calls[5])
	})

	t.Run("rejects multiple in_progress steps", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		result := tool.Execute(context.Background(), `{
			"plan": [
				{"step": "One", "status": "in_progress"},
				{"step": "Two", "status": "in_progress"}
			]
		}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "at most one step can be in_progress")
	})

	t.Run("empty plan returns error", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		result := tool.Execute(context.Background(), `{"plan": []}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "at least one step")
	})

	t.Run("missing plan returns error", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		result := tool.Execute(context.Background(), `{"explanation": "no plan"}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "at least one step")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		result := tool.Execute(context.Background(), `bad json`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid arguments")
	})

	t.Run("rejects unknown status", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		result := tool.Execute(context.Background(), `{
			"plan": [{"step": "Step one", "status": "blocked"}]
		}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "unknown status")
	})

	t.Run("definition has correct name and schema", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		def := tool.Definition()

		assert.Equal(t, llmapi.ToolTypeFunction, def.Type)
		assert.Equal(t, "update_plan", def.Function.Name)
		assert.Contains(t, def.Function.Description, "Updates the task plan")
	})

	t.Run("summary uses explanation", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		assert.Equal(t, "Add auth", tool.Summary(
			`{"explanation":"Add auth","plan":[{"step":"x","status":"pending"}]}`))
	})

	t.Run("summary falls back to first step", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		assert.Equal(t, "Create middleware", tool.Summary(
			`{"plan":[{"step":"Create middleware","status":"pending"}]}`))
	})

	t.Run("summary returns empty for invalid json", func(t *testing.T) {
		tool := NewUpdatePlan(&mockProgressUpdater{})
		assert.Equal(t, "", tool.Summary(`bad`))
	})
}
