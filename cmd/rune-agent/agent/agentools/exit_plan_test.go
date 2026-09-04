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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

func TestExitPlanTool(t *testing.T) {
	t.Run("approved plan writes file and returns success", func(t *testing.T) {
		dir := t.TempDir()
		mp := &mockPrompter{responses: []agent.PromptResponse{
			{Values: []string{approveValue}},
		}}
		tool := NewExitPlan(dir, mp)
		result := tool.Execute(context.Background(), `{
			"title": "Add auth middleware",
			"plan": "## Plan\n\n1. Create middleware\n2. Add tests"
		}`)

		assert.False(t, result.IsError)
		assert.True(t, result.ClearContext, "approved plan must clear context")
		assert.Contains(t, result.Content, "Plan approved")
		assert.Contains(t, result.Content, "add-auth-middleware.md")
		assert.Contains(t, result.Content, "## Plan", "plan content must be in result for context carry-over")

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
		require.NoError(t, err)
		assert.Equal(t, "## Plan\n\n1. Create middleware\n2. Add tests", string(data))
	})

	t.Run("feedback returns user feedback", func(t *testing.T) {
		dir := t.TempDir()
		mp := &mockPrompter{responses: []agent.PromptResponse{
			{Values: []string{feedbackValue}},
		}}
		tool := NewExitPlan(dir, mp)
		result := tool.Execute(context.Background(), `{
			"title": "Fix bug",
			"plan": "the plan"
		}`)

		assert.False(t, result.IsError)
		assert.False(t, result.ClearContext, "feedback must not clear context")
		assert.Contains(t, result.Content, "feedback")

		// Plan file is still written even when feedback is given.
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("custom feedback value is returned", func(t *testing.T) {
		dir := t.TempDir()
		mp := &mockPrompter{responses: []agent.PromptResponse{
			{Values: []string{feedbackValue}, TextInput: "Add error handling to step 3"},
		}}
		tool := NewExitPlan(dir, mp)
		result := tool.Execute(context.Background(), `{
			"title": "Fix bug",
			"plan": "the plan"
		}`)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Add error handling to step 3")
	})

	t.Run("creates plans directory if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "plans")
		mp := &mockPrompter{responses: []agent.PromptResponse{
			{Values: []string{approveValue}},
		}}
		tool := NewExitPlan(dir, mp)
		result := tool.Execute(context.Background(), `{
			"title": "Fix bug",
			"plan": "the plan"
		}`)

		assert.False(t, result.IsError)
		_, err := os.Stat(dir)
		assert.NoError(t, err)
	})

	t.Run("prompter error returns retry instruction", func(t *testing.T) {
		mp := &mockPrompter{err: errors.New("dismissed")}
		tool := NewExitPlan(t.TempDir(), mp)
		result := tool.Execute(context.Background(), `{
			"title": "Fix bug",
			"plan": "the plan"
		}`)

		assert.False(t, result.IsError, "prompt dismiss should not be an error")
		assert.Contains(t, result.Content, "dismissed the prompt")
		assert.Contains(t, result.Content, "call exit_plan_mode again")
	})

	t.Run("prompt shows correct options", func(t *testing.T) {
		mp := &mockPrompter{responses: []agent.PromptResponse{
			{Values: []string{approveValue}},
		}}
		tool := NewExitPlan(t.TempDir(), mp)
		tool.Execute(context.Background(), `{
			"title": "Test",
			"plan": "content"
		}`)

		require.Len(t, mp.calls, 1)
		assert.Equal(t, "Plan", mp.calls[0].Header)
		assert.Len(t, mp.calls[0].Options, 2)
		assert.Equal(t, "Approve", mp.calls[0].Options[0].Label)
		assert.Equal(t, "Give feedback", mp.calls[0].Options[1].Label)
	})

	t.Run("missing title returns error", func(t *testing.T) {
		mp := &mockPrompter{}
		tool := NewExitPlan(t.TempDir(), mp)
		result := tool.Execute(context.Background(), `{
			"title": "",
			"plan": "content"
		}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "title and plan are required")
	})

	t.Run("missing plan returns error", func(t *testing.T) {
		mp := &mockPrompter{}
		tool := NewExitPlan(t.TempDir(), mp)
		result := tool.Execute(context.Background(), `{
			"title": "something",
			"plan": ""
		}`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "title and plan are required")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		mp := &mockPrompter{}
		tool := NewExitPlan(t.TempDir(), mp)
		result := tool.Execute(context.Background(), `bad json`)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid arguments")
	})

	t.Run("definition has correct name and schema", func(t *testing.T) {
		mp := &mockPrompter{}
		tool := NewExitPlan(t.TempDir(), mp)
		def := tool.Definition()

		assert.Equal(t, llmapi.ToolTypeFunction, def.Type)
		assert.Equal(t, "exit_plan_mode", def.Function.Name)
		assert.NotEmpty(t, def.Function.Description)
	})

	t.Run("summary returns truncated title", func(t *testing.T) {
		mp := &mockPrompter{}
		tool := NewExitPlan(t.TempDir(), mp)
		assert.Equal(t, "Fix the bug", tool.Summary(`{"title":"Fix the bug","plan":"x"}`))
		assert.Equal(t, "", tool.Summary(`bad json`))
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Add auth middleware", "add-auth-middleware"},
		{"Fix Bug #123", "fix-bug-123"},
		{"  spaces  and--dashes  ", "spaces-and-dashes"},
		{"UPPER_CASE", "upper-case"},
		{"", "plan"},
		{"a" + string(make([]byte, 100)), "a"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
