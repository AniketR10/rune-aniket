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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/cmd/rune-agent/agent"
)

func TestRequestSkill_Done(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"done"}},
		},
	}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "web_search",
		"description": "Search the web for documentation"
	}`)

	require.False(t, result.IsError, result.Content)
	assert.Contains(t, result.Content, "installed")
	assert.Contains(t, result.Content, "web_search")

	require.Len(t, mp.calls, 1)
	assert.Contains(t, mp.calls[0].Title, "web_search")
	assert.Equal(t, "Skill", mp.calls[0].Header)
	assert.Equal(t, "Search the web for documentation", mp.calls[0].Body)
	require.Len(t, mp.calls[0].Options, 2)
	assert.Equal(t, "Done", mp.calls[0].Options[0].Label)
	assert.Equal(t, "Won't do", mp.calls[0].Options[1].Label)
}

func TestRequestSkill_WontDo(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"wont_do"}},
		},
	}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "deploy",
		"description": "Deploy to production"
	}`)

	require.False(t, result.IsError, result.Content)
	assert.Contains(t, result.Content, "declined")
	assert.Contains(t, result.Content, "deploy")
}

func TestRequestSkill_PrompterError(t *testing.T) {
	mp := &mockPrompter{err: errors.New("dismissed")}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "web_search",
		"description": "Search the web"
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "dismissed")
}

func TestRequestSkill_InvalidJSON(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `bad json`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestRequestSkill_EmptySkillName(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestSkill(mp)
	result := tool.Execute(context.Background(), `{
		"skill": "",
		"description": "Something"
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "skill name is required")
}

func TestRequestSkill_Summary(t *testing.T) {
	tool := NewRequestSkill(&mockPrompter{})
	assert.Equal(t, "web_search", tool.Summary(`{"skill":"web_search","description":"search"}`))
	assert.Equal(t, "", tool.Summary(`bad`))
}

func TestRequestSkill_Definition(t *testing.T) {
	tool := NewRequestSkill(&mockPrompter{})
	def := tool.Definition()
	assert.Equal(t, "request_skill", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
}
