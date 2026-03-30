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

type requestUserInputResult struct {
	Answers map[string]struct {
		Answers []string `json:"answers"`
	} `json:"answers"`
}

func TestRequestUserInput_SingleQuestion(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"PostgreSQL"}},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "database",
			"header": "Database",
			"question": "Which database?",
			"options": [
				{"label": "PostgreSQL", "description": "Mature RDBMS"},
				{"label": "SQLite", "description": "Lightweight"}
			]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"PostgreSQL"}, res.Answers["database"].Answers)

	// Verify prompt request.
	require.Len(t, mp.calls, 1)
	assert.Equal(t, "Which database?", mp.calls[0].Title)
	assert.Equal(t, "Database", mp.calls[0].Header)
	assert.False(t, mp.calls[0].MultiSelect)
	// "Other" is auto-appended.
	require.Len(t, mp.calls[0].Options, 3)
	assert.Equal(t, "Other", mp.calls[0].Options[2].Label)
	assert.True(t, mp.calls[0].Options[2].RequiresInput)
	assert.Equal(t, "Type a custom answer", mp.calls[0].Options[2].Description)
}

func TestRequestUserInput_MultipleQuestions(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Go"}},
			{Values: []string{"REST"}},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [
			{"id": "lang", "question": "Language?", "options": [{"label": "Go", "description": "Fast"}, {"label": "Rust", "description": "Safe"}]},
			{"id": "api", "question": "API style?", "options": [{"label": "REST", "description": "Simple"}, {"label": "gRPC", "description": "Fast"}]}
		]
	}`)

	require.False(t, result.IsError)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"Go"}, res.Answers["lang"].Answers)
	assert.Equal(t, []string{"REST"}, res.Answers["api"].Answers)
}

func TestRequestUserInput_OtherSelected(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Other"}, TextInput: "MongoDB"},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "db",
			"question": "Which database?",
			"options": [
				{"label": "PostgreSQL", "description": "Mature"},
				{"label": "SQLite", "description": "Lightweight"}
			]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"MongoDB"}, res.Answers["db"].Answers)
}

func TestRequestUserInput_OtherSelectedWithoutTextReprompts(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Other"}},
			{TextInput: "MongoDB"},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "db",
			"header": "Database",
			"question": "Which database?",
			"options": [
				{"label": "PostgreSQL", "description": "Mature"},
				{"label": "SQLite", "description": "Lightweight"}
			]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"MongoDB"}, res.Answers["db"].Answers)
	require.Len(t, mp.calls, 2)
	require.Len(t, mp.calls[0].Options, 3)
	assert.Equal(t, "Other", mp.calls[0].Options[2].Label)
	assert.True(t, mp.calls[0].Options[2].RequiresInput)
	assert.Equal(t, "Type a custom answer", mp.calls[0].Options[2].Description)
	assert.Equal(t, "Which database?", mp.calls[1].Title)
	assert.Equal(t, "Database", mp.calls[1].Header)
	assert.Empty(t, mp.calls[1].Options)
}

func TestRequestUserInput_PrompterError(t *testing.T) {
	mp := &mockPrompter{err: errors.New("dismissed")}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "pick",
			"question": "Pick?",
			"options": [{"label": "A", "description": "Option A"}, {"label": "B", "description": "Option B"}]
		}]
	}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "dismissed")
}

func TestRequestUserInput_InvalidJSON(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `bad json`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestRequestUserInput_EmptyOptions(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{TextInput: "I want MongoDB"},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "pick",
			"question": "Pick?",
			"options": []
		}]
	}`)

	// Empty options triggers free-form input mode.
	require.False(t, result.IsError, result.Content)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"I want MongoDB"}, res.Answers["pick"].Answers)

	// Verify prompt request: no options, no "Other" appended.
	require.Len(t, mp.calls, 1)
	assert.Empty(t, mp.calls[0].Options)
}

func TestRequestUserInput_SingleOption(t *testing.T) {
	mp := &mockPrompter{
		responses: []agent.PromptResponse{
			{Values: []string{"Only one"}},
		},
	}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{
		"questions": [{
			"id": "pick",
			"question": "Pick?",
			"options": [{"label": "Only one", "description": "Sole option"}]
		}]
	}`)

	require.False(t, result.IsError, result.Content)

	var res requestUserInputResult
	require.NoError(t, json.Unmarshal([]byte(result.Content), &res))
	assert.Equal(t, []string{"Only one"}, res.Answers["pick"].Answers)

	// "Other" is still appended, so prompter sees 2 options.
	require.Len(t, mp.calls[0].Options, 2)
}

func TestRequestUserInput_NoQuestions(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestUserInput(mp)
	result := tool.Execute(context.Background(), `{"questions": []}`)

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "at least one question")
}

func TestRequestUserInput_Summary(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestUserInput(mp)

	assert.Equal(t, "Which database?",
		tool.Summary(`{"questions": [{"question": "Which database?"}]}`))
	assert.Equal(t, "", tool.Summary(`bad`))
	assert.Equal(t, "", tool.Summary(`{"questions": []}`))
}

func TestRequestUserInput_Definition(t *testing.T) {
	mp := &mockPrompter{}
	tool := NewRequestUserInput(mp)
	def := tool.Definition()
	assert.Equal(t, "request_user_input", def.Function.Name)
	assert.Equal(t, "Request user input for one to three short questions and wait for the response. When options is empty, the user types a free-form text answer. If the user needs a custom answer, the client automatically adds an Other option and collects free-form text before returning.", def.Function.Description)
}
