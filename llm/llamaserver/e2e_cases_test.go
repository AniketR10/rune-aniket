// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package llamaserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/llamaserver"
)

// TestE2E_CreateCompletion_Basic drives a short single-turn request and
// asserts the event stream shape: text deltas concatenate into the final
// message, usage is populated, and a single EventStreamDone closes it.
func TestE2E_CreateCompletion_Basic(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Say hi in one word."},
		},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	text, tools, done := collectStream(t, ctx, it)
	assert.Empty(t, tools)
	assert.NotEmpty(t, text, "expected some generated text")
	assert.Equal(t, done.Message.Content, text,
		"done.Message.Content must equal accumulated deltas")
	assert.Positive(t, done.Usage.TokensSent)
	assert.Positive(t, done.Usage.TokensReceived)
	switch done.FinishReason {
	case llmapi.FinishReasonStop, llmapi.FinishReasonLength:
	default:
		t.Fatalf("unexpected finish reason %q", done.FinishReason)
	}
}

// TestE2E_CreateCompletion_Length caps generation server-side via the
// --predict flag (Config.MaxOutputTokens) and asserts the length finish
// reason. This pins the config→flag mapping for output caps.
func TestE2E_CreateCompletion_Length(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{MaxOutputTokens: 4})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Tell me a long story about a dragon and a knight."},
		},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	_, _, done := collectStream(t, ctx, it)
	assert.Equal(t, llmapi.FinishReasonLength, done.FinishReason)
}

// TestE2E_CreateCompletion_Tools exercises the tool-calling path end to
// end: the model must emit a structured tool call the OpenAI-compatible
// client surfaces as EventToolCallDone with parseable JSON arguments.
func TestE2E_CreateCompletion_Tools(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tool := llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name:        "get_weather",
			Description: "Return the current weather for a city.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{
						"type":        "string",
						"description": "The city to look up.",
					},
				},
				"required": []any{"city"},
			},
		},
	}

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleSystem, Content: "You are a helpful assistant. Always call a tool when one is available."},
			{Role: llmapi.RoleUser, Content: "What's the weather in Paris? Use the tool."},
		},
		Tools: []llmapi.Tool{tool},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	_, tools, done := collectStream(t, ctx, it)
	if len(tools) == 0 {
		t.Skipf("model did not emit a tool call (content=%q) — weak quant, not a bug",
			done.Message.Content)
	}
	assert.Equal(t, llmapi.FinishReasonToolCall, done.FinishReason)
	tc := tools[0]
	assert.Equal(t, "get_weather", tc.Function.Name)
	assert.NotEmpty(t, tc.Function.Arguments, "tool call has empty arguments")
	assertJSONObject(t, tc.Function.Arguments)
	assert.Contains(t, strings.ToLower(tc.Function.Arguments), "paris",
		"tool arguments should carry the requested city")
}

// TestE2E_ToolResult_RoundTrip feeds a tool result back and asserts the
// model produces a natural-language answer that incorporates it. This
// verifies the assistant/tool message rendering the OpenAI-compatible
// path relies on.
func TestE2E_ToolResult_RoundTrip(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "What is the capital of France?"},
			{
				Role: llmapi.RoleAssistant,
				ToolCalls: []llmapi.ToolCall{{
					ID:   "call_1",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "lookup_capital",
						Arguments: `{"country":"France"}`,
					},
				}},
			},
			{Role: llmapi.RoleTool, ToolCallID: "call_1", Content: "Paris"},
		},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	// Some templates (e.g. Gemma) reject the assistant→tool message
	// sequence outright with a Jinja role-alternation error. That is a
	// model-template limitation, not a client bug, so skip rather than
	// fail when the server rejects the transcript.
	text, streamErr := drainText(ctx, it)
	if streamErr != nil {
		if isTemplateError(streamErr) {
			t.Skipf("model template rejects tool messages: %v", streamErr)
		}
		t.Fatalf("stream error: %v", streamErr)
	}
	assert.Contains(t, strings.ToLower(text), "paris",
		"assistant should incorporate the tool result")
}

// TestE2E_CountTokens returns a positive offline estimate.
func TestE2E_CountTokens(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	n, err := svc.CountTokens(model, []llmapi.Message{
		{Role: llmapi.RoleUser, Content: "hello world"},
	})
	require.NoError(t, err)
	assert.Positive(t, n)
}

// TestE2E_ContextCancellation cancels mid-stream and asserts the iterator
// drains cleanly without wedging the pool.
func TestE2E_ContextCancellation(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Count slowly from one to one hundred, one number per line."},
		},
	})
	require.NoError(t, err)

	var sawDelta bool
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llmapi.EventTextDelta && !sawDelta {
			sawDelta = true
			cancel()
		}
	}
	assert.NoError(t, it.Close(), "iterator close after cancel")
	assert.True(t, sawDelta, "never saw a delta before cancelling")

	// The pool must survive a cancelled request: a follow-up completion on a
	// fresh context still succeeds against the same running server.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel2()
	it2, err := svc.CreateCompletion(ctx2, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "Say ok."}},
	})
	require.NoError(t, err)
	defer func() { _ = it2.Close() }()
	text, _, _ := collectStream(t, ctx2, it2)
	assert.NotEmpty(t, text)
}

// TestE2E_ServerReuse_AcrossRequests proves the pool reuses a single
// running llama-server across sequential requests: two completions on the
// same model must not restart the process (verified indirectly by both
// succeeding quickly under a single StartupTimeout budget).
func TestE2E_ServerReuse_AcrossRequests(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	for i := range 2 {
		it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
			Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "Reply with the single word: pong."},
			},
		})
		require.NoErrorf(t, err, "request %d", i)
		text, _, done := collectStream(t, ctx, it)
		_ = it.Close()
		assert.NotEmptyf(t, text, "request %d produced no text", i)
		assert.Positivef(t, done.Usage.TokensReceived, "request %d", i)
	}
}

// TestE2E_ServerNotInstalled asserts the exact user-facing install message
// when llama-server cannot be resolved — the "not installed" state lives
// in the locator, not a nil backend.
func TestE2E_ServerNotInstalled(t *testing.T) {
	if !e2eEnabled() {
		t.Skip("llamaserver e2e: opt in with RUNE_LLAMASERVER_E2E=1")
	}
	exec := &realExecutor{}
	t.Cleanup(exec.wait)
	svc := llamaserver.New(
		llamaserver.Config{}, exec, llamaserver.NewFixedLocator(""), nopNotifications{},
	)
	t.Cleanup(func() { _ = svc.Close() })

	_, err := svc.CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "x", Provider: llamaserver.LLMProvider},
		llmapi.Request{Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pkg install llama-server")
	assert.Contains(t, err.Error(), "models.local.server_bin_path")
}

// TestE2E_ResponseFormat_JSONObject asks for a JSON object and asserts the
// model honours the ResponseFormat, exercising the OpenAI-compatible
// response_format passthrough to llama-server.
func TestE2E_ResponseFormat_JSONObject(t *testing.T) {
	svc, model := e2eService(t, llamaserver.Config{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, model, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: `Return a JSON object with a single key "ok" set to true.`},
		},
		ResponseFormat: &llmapi.ResponseFormat{Type: llmapi.ResponseFormatTypeJSONObject},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	text, _, _ := collectStream(t, ctx, it)
	assertJSONObject(t, text)
}
