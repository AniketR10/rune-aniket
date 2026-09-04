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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/cmd/rune-agent/agent"
)

type mockPrompter struct {
	responses []agent.PromptResponse
	err       error
	calls     []agent.PromptRequest
	callIndex int
}

func (m *mockPrompter) Prompt(_ context.Context, req agent.PromptRequest) (agent.PromptResponse, error) {
	m.calls = append(m.calls, req)
	if m.err != nil {
		return agent.PromptResponse{}, m.err
	}
	if m.callIndex >= len(m.responses) {
		return agent.PromptResponse{}, errors.New("no more responses")
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func TestAskUser_SingleQuestion(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"PostgreSQL"}},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Which database?",
			"header": "Database",
			"options": [
				{"label": "PostgreSQL", "description": "Mature RDBMS"},
				{"label": "SQLite", "description": "Lightweight"}
			]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res struct {
		Answers map[string]string `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, "PostgreSQL", res.Answers["Which database?"])

	// Verify prompt request
	require.Len(t, mp.calls, 1)
	assert.Equal(t, "Which database?", mp.calls[0].Title)
	assert.Equal(t, "Database", mp.calls[0].Header)
	assert.False(t, mp.calls[0].MultiSelect)
	// "Other" is auto-appended
	require.Len(t, mp.calls[0].Options, 3)
	assert.Equal(t, "Other", mp.calls[0].Options[2].Label)
}

func TestAskUser_MultipleQuestions(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Go"}},
			{Values: []string{"REST"}},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [
			{"question": "Language?", "options": [{"label": "Go"}, {"label": "Rust"}]},
			{"question": "API style?", "options": [{"label": "REST"}, {"label": "gRPC"}]}
		]
	}`)

	require.False(t, result.IsError)

	var res struct {
		Answers map[string]string `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, "Go", res.Answers["Language?"])
	assert.Equal(t, "REST", res.Answers["API style?"])
}

func TestAskUser_MultiSelect(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Logging", "Metrics"}},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Features?",
			"multiSelect": true,
			"options": [{"label": "Logging"}, {"label": "Metrics"}, {"label": "Tracing"}]
		}]
	}`)

	require.False(t, result.IsError)

	var res struct {
		Answers map[string]string `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, "Logging, Metrics", res.Answers["Features?"])
}

func TestAskUser_OtherSelected(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Other"}},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Which database?",
			"options": [
				{"label": "PostgreSQL"},
				{"label": "SQLite"}
			]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res struct {
		Answers map[string]string `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, "Other", res.Answers["Which database?"])
}

func TestAskUser_PrompterError(t *testing.T) {
	mp := &mockPrompter{err: errors.New("dismissed")}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Pick?",
			"options": [{"label": "A"}, {"label": "B"}]
		}]
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "dismissed")
}

func TestAskUser_InvalidJSON(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `bad json`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestAskUser_SingleOption(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Only one"}},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Pick?",
			"options": [{"label": "Only one"}]
		}]
	}`)

	// Single option is allowed — "Other" is appended, giving 2 options.
	require.False(t, result.IsError, result.Content)
	require.Len(t, mp.calls, 1)
	require.Len(t, mp.calls[0].Options, 2)
	assert.Equal(t, "Only one", mp.calls[0].Options[0].Label)
	assert.Equal(t, "Other", mp.calls[0].Options[1].Label)
}

func TestAskUser_FreeFormInput(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{TextInput: "I want MongoDB"},
		},
	}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"question": "Which database do you prefer?",
			"header": "Database",
			"options": []
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res struct {
		Answers map[string]string `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, "I want MongoDB", res.Answers["Which database do you prefer?"])

	// Verify prompt request: no options, no "Other" appended.
	require.Len(t, mp.calls, 1)
	assert.Equal(t, "Which database do you prefer?", mp.calls[0].Title)
	assert.Equal(t, "Database", mp.calls[0].Header)
	assert.Empty(t, mp.calls[0].Options)
}

func TestAskUser_NoQuestions(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewAskUser(mp)
	result := tool.Execute(context.Background(), `{"questions": []}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "at least one question")
}

func TestAskUser_Summary(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewAskUser(mp)

	assert.Equal(t, "Which database?",
		tool.Summary(`{"questions": [{"question": "Which database?"}]}`))
	assert.Equal(t, "", tool.Summary(`bad`))
	assert.Equal(t, "", tool.Summary(`{"questions": []}`))
}

func TestAskUser_Definition(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewAskUser(mp)
	def := tool.Definition()
	assert.Equal(t, "ask_user_question", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
}
