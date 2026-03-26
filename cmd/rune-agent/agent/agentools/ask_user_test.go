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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/agent"
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
