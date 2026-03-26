// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package anthropic_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/anthropic"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

const (
	testModel = anthropic.ClaudeSonnet4Dot6
	apiKeyEnv = "ANTHROPIC_API_KEY"
)

func testClient(t *testing.T) (llm.Service, string) {
	t.Helper()
	token := os.Getenv(apiKeyEnv)
	if token == "" {
		t.Skipf("%s not set", apiKeyEnv)
	}
	c := anthropic.NewClient(token, anthropic.Config{
		Model:        testModel,
		MaxTokens:    1024,
		CacheControl: "5m",
	}, anthropic.AvailableModels())
	return c, token
}

// drainStream reads all events from an iterator and returns collected data.
func drainStream(t *testing.T, ctx context.Context, it interface {
	Next(context.Context) (llm.Event, bool)
	Err() error
	Close() error
}) (text string, toolCalls []llm.ToolCall, doneData *llm.DoneData) {
	t.Helper()
	var builder strings.Builder
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llm.EventTextDelta:
			builder.WriteString(ev.Text)
		case llm.EventToolCallDone:
			toolCalls = append(toolCalls, *ev.ToolCall)
		case llm.EventStreamDone:
			doneData = ev.DoneData
		case llm.EventStreamError:
			t.Fatalf("stream error: %v", ev.Error)
		}
	}
	require.NoError(t, it.Err())
	return builder.String(), toolCalls, doneData
}

// TestAnthropicE2E_SimpleCompletion verifies a simple streaming text completion.
func TestAnthropicE2E_SimpleCompletion(t *testing.T) {
	c, _ := testClient(t)
	ctx := context.Background()

	it, err := c.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Say 'hello world' and nothing else."},
		},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	text, _, doneData := drainStream(t, ctx, it)

	require.NotNil(t, doneData)
	assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	assert.Contains(t, strings.ToLower(text), "hello world")
	assert.Greater(t, doneData.Usage.TokensSent, 0)
	assert.Greater(t, doneData.Usage.TokensReceived, 0)
}

// TestAnthropicE2E_SystemPrompt verifies system prompt handling.
func TestAnthropicE2E_SystemPrompt(t *testing.T) {
	c, _ := testClient(t)
	ctx := context.Background()

	it, err := c.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a pirate. Always respond in pirate speak."},
			{Role: llm.RoleUser, Content: "Say hello."},
		},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	text, _, doneData := drainStream(t, ctx, it)

	require.NotNil(t, doneData)
	assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	assert.NotEmpty(t, text)
}

// TestAnthropicE2E_ToolUse verifies tool calling works correctly.
func TestAnthropicE2E_ToolUse(t *testing.T) {
	c, _ := testClient(t)
	ctx := context.Background()

	agentTools, _ := agentools.DefaultTools(nil, nil, workspaceapi.URI{}, agentools.Config{})
	registry := agent.NewRegistry(agentTools...)
	tools := registry.Tools("")
	require.NotEmpty(t, tools)

	it, err := c.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a coding assistant. Always use tools."},
			{Role: llm.RoleUser, Content: "Read the file main.go."},
		},
		Tools: tools,
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	_, toolCalls, doneData := drainStream(t, ctx, it)

	require.NotNil(t, doneData)
	assert.Equal(t, llm.FinishReasonToolCall, doneData.FinishReason)
	require.NotEmpty(t, toolCalls, "model should call a tool")

	// Find the read_file tool call.
	var found *llm.ToolCall
	for i, tc := range toolCalls {
		if tc.Function.Name == "read_file" {
			found = &toolCalls[i]
			break
		}
	}
	require.NotNilf(t, found, "expected read_file tool call, got: %v", toolCallNames(toolCalls))
	assert.True(t, json.Valid([]byte(found.Function.Arguments)),
		"arguments should be valid JSON: %s", found.Function.Arguments)
	assert.Contains(t, found.Function.Arguments, "main.go")
}

// TestAnthropicE2E_MultiTurn verifies a full tool-use round trip:
// user prompt -> model calls read_file -> we provide result -> model responds.
func TestAnthropicE2E_MultiTurn(t *testing.T) {
	c, _ := testClient(t)
	ctx := context.Background()

	agentTools, _ := agentools.DefaultTools(nil, nil, workspaceapi.URI{}, agentools.Config{})
	registry := agent.NewRegistry(agentTools...)
	tools := registry.Tools("")

	// Turn 1: user asks to read a file.
	req := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a coding assistant. Use the read_file tool."},
			{Role: llm.RoleUser, Content: "Read the file main.go and tell me what package it declares."},
		},
		Tools: tools,
	}
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)

	_, _, turn1Done := drainStream(t, ctx, it)
	_ = it.Close()

	require.NotNil(t, turn1Done)
	require.NotEmpty(t, turn1Done.Message.ToolCalls, "model should call read_file")

	tc := turn1Done.Message.ToolCalls[0]
	assert.Equal(t, "read_file", tc.Function.Name)

	// Turn 2: provide tool result -> model responds with text.
	fileContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	req2 := llm.Request{
		Messages: []llm.Message{
			req.Messages[0],
			req.Messages[1],
			turn1Done.Message,
			{Role: llm.RoleTool, ToolCallID: tc.ID, Content: fileContent},
		},
		Tools: tools,
	}
	it2, err := c.CreateCompletion(ctx, req2)
	require.NoError(t, err)
	defer func() { _ = it2.Close() }()

	text, _, turn2Done := drainStream(t, ctx, it2)

	require.NotNil(t, turn2Done)
	assert.Equal(t, llm.FinishReasonStop, turn2Done.FinishReason)
	assert.Contains(t, strings.ToLower(text), "main",
		"response should mention the package name")
}

// TestAnthropicE2E_PromptCaching verifies that prompt caching works by
// making two identical requests and checking that the second one reports
// cache read tokens.
//
// Per the Anthropic docs, "a cache entry only becomes available after the
// first response begins." We drain the full first response and then allow
// a brief propagation window before sending the second request.
func TestAnthropicE2E_PromptCaching(t *testing.T) {
	c, _ := testClient(t)
	ctx := context.Background()

	agentTools, _ := agentools.DefaultTools(nil, nil, workspaceapi.URI{}, agentools.Config{})
	registry := agent.NewRegistry(agentTools...)
	tools := registry.Tools("")

	// Build a large enough system prompt to exceed the caching threshold.
	// The 1h TTL used for system prompts requires a minimum of 4096 tokens
	// (~16K chars). We use 20K to have margin.
	var longPrompt strings.Builder
	longPrompt.WriteString("You are a coding assistant. Always use tools when available. ")
	for longPrompt.Len() < 20000 {
		longPrompt.WriteString("This is additional context to ensure the prompt exceeds the minimum caching threshold. ")
	}

	req := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: longPrompt.String()},
			{Role: llm.RoleUser, Content: "Say hello."},
		},
		Tools: tools,
	}

	// First request: should create a cache entry.
	it1, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)
	_, _, done1 := drainStream(t, ctx, it1)
	_ = it1.Close()
	require.NotNil(t, done1)

	t.Logf("Request 1 — sent: %d, received: %d, cached: %d, cache_created: %d",
		done1.Usage.TokensSent, done1.Usage.TokensReceived,
		done1.Usage.TokensCached, done1.Usage.TokensCacheCreated)

	require.Greater(t, done1.Usage.TokensCacheCreated, 0,
		"first request must create a cache entry (tokens above minimum, cache_control set)")

	// Allow the cache entry to propagate server-side. The Anthropic docs
	// state entries become available "after the first response begins," but
	// back-to-back requests can arrive before propagation completes.
	time.Sleep(2 * time.Second)

	// Second request: should hit the cache.
	it2, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)
	_, _, done2 := drainStream(t, ctx, it2)
	_ = it2.Close()
	require.NotNil(t, done2)

	t.Logf("Request 2 — sent: %d, received: %d, cached: %d, cache_created: %d",
		done2.Usage.TokensSent, done2.Usage.TokensReceived,
		done2.Usage.TokensCached, done2.Usage.TokensCacheCreated)

	assert.Greater(t, done2.Usage.TokensCached, 0,
		"second request should read tokens from cache")
}

// TestAnthropicE2E_CountTokens verifies the CountTokens API integration.
func TestAnthropicE2E_CountTokens(t *testing.T) {
	c, _ := testClient(t)

	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.RoleUser, Content: "Hello, how are you?"},
	}

	count, err := c.CountTokens(msgs)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "token count should be positive")
	t.Logf("Token count for test messages: %d", count)
}

// TestAnthropicE2E_ContextWindowExceeded verifies the context window check.
func TestAnthropicE2E_ContextWindowExceeded(t *testing.T) {
	token := os.Getenv(apiKeyEnv)
	if token == "" {
		t.Skipf("%s not set", apiKeyEnv)
	}

	// Create a client with a tiny context window to trigger the check.
	c := anthropic.NewClient(token, anthropic.Config{
		Model:     testModel,
		MaxTokens: 1024,
	}, map[string]int{testModel: 10}) // 10 token context window

	ctx := context.Background()
	_, err := c.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "This message exceeds the tiny context window."},
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, &llm.ErrContextWindowExceeded{})
}

func toolCallNames(tcs []llm.ToolCall) []string {
	names := make([]string, len(tcs))
	for i, tc := range tcs {
		names[i] = tc.Function.Name
	}
	return names
}
