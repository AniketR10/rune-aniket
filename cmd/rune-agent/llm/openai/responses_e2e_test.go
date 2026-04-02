// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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

package openai_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi/semanticrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"go.uber.org/mock/gomock"
	"unstable.build/go-tui/rpc/rpctest"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/openai"
)

func disconnectedTestLSP(t *testing.T) *semanticrpc.Client {
	t.Helper()
	ctrl := gomock.NewController(t)
	cc := rpctest.NewMockClientConnInterface(ctrl)
	return semanticrpc.NewClient(context.Background(), cc)
}

// TestResponsesE2E is a table-driven end-to-end test that sends real requests
// to the OpenAI Responses API using our actual agent tool schemas.
//
// It verifies that:
//  1. Our tool schemas are accepted by the Responses API (strict mode).
//  2. The model selects the correct tool for a given prompt.
//  3. Tool call arguments are well-formed JSON with expected fields.
//  4. Multi-turn conversations (tool call → tool result → text) work.
//
// Enable by setting OPENAI_TESTING_KEY.
func TestResponsesE2E(t *testing.T) {
	token := os.Getenv("OPENAI_TESTING_KEY")
	if token == "" {
		t.Skip("OPENAI_TESTING_KEY not set")
	}

	model := openai.GPT5Dot3Codex
	require.True(t, openai.IsResponsesOnlyModel(model), "test model must be responses-only")

	// Collect real tool definitions from agent/agentools.
	agentTools, _ := agentools.DefaultTools(nil, nil, workspaceapi.URI{}, disconnectedTestLSP(t), agentools.Config{})
	registry := agent.NewRegistry(agentTools...)
	tools := registry.AllTools()
	require.NotEmpty(t, tools, "expected at least one tool definition")

	systemPrompt := "You are a coding assistant. Always use the provided tools — never answer directly when a tool can help."

	tests := []struct {
		name     string
		messages []llm.Message
		wantTool string
		wantArgs []string // substrings expected in tool call arguments JSON
	}{
		{
			name: "read_file for a specific path",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: "Read the file cmd/main.go for me."},
			},
			wantTool: "read_file",
			wantArgs: []string{"main.go"},
		},
		{
			name: "search_content for a pattern",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: "Search the codebase for the function handleRequest."},
			},
			wantTool: "search_content",
			wantArgs: []string{"handleRequest"},
		},
		{
			name: "find_files by pattern",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: "Find all Go test files in the project."},
			},
			wantTool: "find_files",
			wantArgs: []string{"test"},
		},
		{
			name: "bash to run a command",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: systemPrompt},
				{Role: llm.RoleUser, Content: "Run 'go test ./...' to check if the tests pass."},
			},
			wantTool: "bash",
			wantArgs: []string{"go test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := openai.NewClient(token, openai.Config{
				Model: model,
				Tools: tools,
			}, openai.AvailableModels())
			ctx := context.Background()

			it, err := c.CreateCompletion(ctx, llm.Request{Messages: tt.messages})
			require.NoError(t, err)
			defer func() { _ = it.Close() }()

			var toolCalls []llm.ToolCall
			var doneData *llm.DoneData
			for {
				ev, ok := it.Next(ctx)
				if !ok {
					break
				}
				switch ev.Type {
				case llm.EventToolCallDone:
					toolCalls = append(toolCalls, *ev.ToolCall)
				case llm.EventStreamDone:
					doneData = ev.DoneData
				case llm.EventStreamError:
					t.Fatalf("stream error: %v", ev.Error)
				}
			}
			require.NoError(t, it.Err())
			require.NotNil(t, doneData)
			assert.Equal(t, llm.FinishReasonToolCall, doneData.FinishReason)

			// Find the expected tool call.
			var found *llm.ToolCall
			for i, tc := range toolCalls {
				if tc.Function.Name == tt.wantTool {
					found = &toolCalls[i]
					break
				}
			}
			require.NotNilf(t, found, "expected tool call %q, got: %v", tt.wantTool, toolCallNames(toolCalls))

			// Arguments must be valid JSON.
			assert.True(t, json.Valid([]byte(found.Function.Arguments)),
				"arguments should be valid JSON: %s", found.Function.Arguments)

			// Check expected substrings in arguments.
			for _, want := range tt.wantArgs {
				assert.Contains(t, found.Function.Arguments, want,
					"arguments should contain %q", want)
			}
		})
	}
}

// TestResponsesE2E_MultiTurn verifies a full tool-use round trip:
// user prompt → model calls read_file → we provide result → model responds with text.
func TestResponsesE2E_MultiTurn(t *testing.T) {
	token := os.Getenv("OPENAI_TESTING_KEY")
	if token == "" {
		t.Skip("OPENAI_TESTING_KEY not set")
	}

	model := openai.GPT5Dot3Codex

	agentTools, _ := agentools.DefaultTools(nil, nil, workspaceapi.URI{}, disconnectedTestLSP(t), agentools.Config{})
	registry := agent.NewRegistry(agentTools...)
	tools := registry.AllTools()

	c := openai.NewClient(token, openai.Config{
		Model: model,
		Tools: tools,
	}, openai.AvailableModels())
	ctx := context.Background()

	// Turn 1: user asks to read a file → model calls read_file.
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a coding assistant. Use the read_file tool to read files."},
		{Role: llm.RoleUser, Content: "Read the file main.go and tell me what package it declares."},
	}}
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)

	var turn1Done *llm.DoneData
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llm.EventStreamDone:
			turn1Done = ev.DoneData
		case llm.EventStreamError:
			t.Fatalf("turn 1 error: %v", ev.Error)
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()
	require.NotNil(t, turn1Done)
	require.NotEmpty(t, turn1Done.Message.ToolCalls, "model should call read_file")

	tc := turn1Done.Message.ToolCalls[0]
	assert.Equal(t, "read_file", tc.Function.Name)

	// Turn 2: provide the tool result → model responds with text.
	fileContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	req2 := llm.Request{Messages: []llm.Message{
		req.Messages[0],
		req.Messages[1],
		turn1Done.Message,
		{Role: llm.RoleTool, ToolCallID: tc.ID, Content: fileContent},
	}}
	it2, err := c.CreateCompletion(ctx, req2)
	require.NoError(t, err)
	defer func() { _ = it2.Close() }()

	var text strings.Builder
	var turn2Done *llm.DoneData
	for {
		ev, ok := it2.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llm.EventTextDelta:
			text.WriteString(ev.Text)
		case llm.EventStreamDone:
			turn2Done = ev.DoneData
		case llm.EventStreamError:
			t.Fatalf("turn 2 error: %v", ev.Error)
		}
	}
	require.NoError(t, it2.Err())
	require.NotNil(t, turn2Done)
	assert.Equal(t, llm.FinishReasonStop, turn2Done.FinishReason)

	// The model should mention "main" since that's the package.
	response := strings.ToLower(text.String())
	assert.Contains(t, response, "main", "expected response to mention the package name")
}

func toolCallNames(tcs []llm.ToolCall) []string {
	names := make([]string, len(tcs))
	for i, tc := range tcs {
		names[i] = tc.Function.Name
	}
	return names
}
