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

package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

func TestAgentRun(t *testing.T) {
	tests := []struct {
		name      string
		responses []mockResponse
		tools     []Tool
		setup     func(store *mockStore, ag *Agent) // pre-test setup
		config    Config
		assertFn  func(t *testing.T, events []Event, store *mockStore, svc *mockService)
	}{
		{
			name:      "single turn no tools produces text and done",
			responses: []mockResponse{stopResponse("Hello ", "world!")},
			config:    Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				textEvents := eventsByType(events, EventText)
				require.Len(t, textEvents, 2)
				assert.Equal(t, "Hello ", textEvents[0].Text)
				assert.Equal(t, "world!", textEvents[1].Text)
				assert.True(t, hasEventType(events, EventDone))

				d, ok := store.getDialogue("d")
				require.True(t, ok)
				assert.Len(t, d.Messages, 3) // system + user + assistant
				assert.Equal(t, llm.RoleSystem, d.Messages[0].Role)
				assert.Equal(t, "test", d.Messages[0].Content)
				assert.Equal(t, llm.RoleUser, d.Messages[1].Role)
				assert.Equal(t, llm.RoleAssistant, d.Messages[2].Role)
				assert.Equal(t, "Hello world!", d.Messages[2].Content)
			},
		},
		{
			name: "single tool call → result → final response",
			responses: []mockResponse{
				toolCallResponse("my_tool", `{"a":1}`, "call_1"),
				stopResponse("Done!"),
			},
			tools:  []Tool{&mockTool{name: "my_tool", result: ToolResult{Content: "tool output"}}},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				toolCalls := eventsByType(events, EventToolCall)
				require.Len(t, toolCalls, 1)
				assert.Equal(t, "my_tool", toolCalls[0].ToolName)
				assert.Equal(t, `{"a":1}`, toolCalls[0].ToolArgs)

				toolResults := eventsByType(events, EventToolResult)
				require.Len(t, toolResults, 1)
				assert.Equal(t, "tool output", toolResults[0].ToolOutput)
				assert.False(t, toolResults[0].IsError)

				assert.True(t, hasEventType(events, EventDone))

				// Verify the second LLM call included the tool result message
				require.Equal(t, 2, svc.getCallCount())
				secondReq := svc.requests[1]
				var foundToolMsg bool
				for _, msg := range secondReq.Messages {
					if msg.Role == llm.RoleTool && msg.ToolCallID == "call_1" {
						foundToolMsg = true
						assert.Equal(t, "tool output", msg.Content)
					}
				}
				assert.True(t, foundToolMsg, "second request should contain tool result message")
			},
		},
		{
			name: "multiple parallel tool calls in single response",
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "tool_a", Arguments: "{}"}},
						{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "tool_b", Arguments: "{}"}},
					},
				},
				stopResponse("both done"),
			},
			tools: []Tool{
				&mockTool{name: "tool_a", result: ToolResult{Content: "a result"}},
				&mockTool{name: "tool_b", result: ToolResult{Content: "b result"}},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				toolCalls := eventsByType(events, EventToolCall)
				assert.Len(t, toolCalls, 2)
				toolResults := eventsByType(events, EventToolResult)
				assert.Len(t, toolResults, 2)
				assert.True(t, hasEventType(events, EventDone))

				// Both tool messages should be in the second request
				require.Equal(t, 2, svc.getCallCount())
				var toolMsgCount int
				for _, msg := range svc.requests[1].Messages {
					if msg.Role == llm.RoleTool {
						toolMsgCount++
					}
				}
				assert.Equal(t, 2, toolMsgCount)
			},
		},
		{
			name: "multi-step tool chain: read → edit → verify",
			responses: []mockResponse{
				toolCallResponse("read", "{}", "c1"),
				toolCallResponse("edit", "{}", "c2"),
				toolCallResponse("read", "{}", "c3"),
				stopResponse("All done"),
			},
			tools: []Tool{
				&mockTool{name: "read", result: ToolResult{Content: "file contents"}},
				&mockTool{name: "edit", result: ToolResult{Content: "edited ok"}},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.Equal(t, 4, svc.getCallCount())
				assert.Len(t, eventsByType(events, EventToolCall), 3)
				assert.Len(t, eventsByType(events, EventToolResult), 3)
				assert.True(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "unknown tool sends error result back to LLM",
			responses: []mockResponse{
				toolCallResponse("nonexistent", "{}", "c1"),
				stopResponse("sorry"),
			},
			tools:  nil, // no tools registered
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				results := eventsByType(events, EventToolResult)
				require.Len(t, results, 1)
				assert.True(t, results[0].IsError)
				assert.Contains(t, results[0].ToolOutput, "unknown tool")
				assert.True(t, hasEventType(events, EventDone))

				// The error message should be fed back as role=tool
				require.Equal(t, 2, svc.getCallCount())
				var foundToolMsg bool
				for _, msg := range svc.requests[1].Messages {
					if msg.Role == llm.RoleTool {
						foundToolMsg = true
						assert.Contains(t, msg.Content, "unknown tool")
					}
				}
				assert.True(t, foundToolMsg)
			},
		},
		{
			name: "tool execution error is fed back to LLM",
			responses: []mockResponse{
				toolCallResponse("fail_tool", "{}", "c1"),
				stopResponse("I see the error"),
			},
			tools:  []Tool{&mockTool{name: "fail_tool", result: ToolResult{Content: "boom!", IsError: true}}},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				results := eventsByType(events, EventToolResult)
				require.Len(t, results, 1)
				assert.True(t, results[0].IsError)
				assert.Equal(t, "boom!", results[0].ToolOutput)
				assert.True(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "max iterations reached",
			responses: func() []mockResponse {
				r := make([]mockResponse, 5)
				for i := range r {
					r[i] = toolCallResponse("t", "{}", "c")
				}
				return r
			}(),
			tools:  []Tool{&mockTool{name: "t", result: ToolResult{Content: "ok"}}},
			config: Config{SystemPrompt: "test", MaxIterations: 3},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				errs := eventsByType(events, EventError)
				require.NotEmpty(t, errs)
				var foundMaxIter bool
				for _, e := range errs {
					if e.Error != nil && assert.Contains(t, e.Error.Error(), "max iterations") {
						foundMaxIter = true
					}
				}
				assert.True(t, foundMaxIter)
				assert.Equal(t, 3, svc.getCallCount())
				assert.False(t, hasEventType(events, EventDone))

				// Messages should still be persisted
				_, ok := store.getDialogue("d")
				assert.True(t, ok, "dialogue should be persisted even on max iterations")
			},
		},
		{
			name: "CreateCompletion returns error",
			responses: []mockResponse{
				{err: errors.New("api error")},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				errs := eventsByType(events, EventError)
				require.Len(t, errs, 1)
				assert.Contains(t, errs[0].Error.Error(), "api error")
				assert.False(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "stream error after consuming chunks",
			responses: []mockResponse{
				{
					chunks:       []string{"partial "},
					finishReason: llm.FinishReasonStop,
					streamErr:    errors.New("connection reset"),
				},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				// Should get the text then an error
				assert.True(t, hasEventType(events, EventText))
				errs := eventsByType(events, EventError)
				require.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Error.Error(), "connection reset")
				assert.False(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "tool_calls finish reason with nil tool calls emits error",
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls:    nil, // no tool calls
				},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				errs := eventsByType(events, EventError)
				require.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Error.Error(), "no tool calls")
				assert.False(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "tool_calls finish reason with empty ToolCalls emits error",
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls:    []llm.ToolCall{}, // empty
				},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				errs := eventsByType(events, EventError)
				require.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Error.Error(), "no tool calls")
			},
		},
		{
			name: "finish reason length persists partial message",
			responses: []mockResponse{
				{chunks: []string{"truncated"}, finishReason: llm.FinishReasonLength},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				// FinishReasonLength is not an error — it indicates the
				// output was truncated by token limits. The partial
				// message should be persisted so the user can continue.
				assert.Empty(t, eventsByType(events, EventError),
					"FinishReasonLength must not emit an error")
				doneEvents := eventsByType(events, EventDone)
				require.Len(t, doneEvents, 1)
				assert.Equal(t, llm.FinishReasonLength, doneEvents[0].FinishReason)
				d, err := store.Get(context.Background(), "d")
				require.NoError(t, err)
				var found bool
				for _, m := range d.Messages {
					if m.Role == llm.RoleAssistant && m.Content == "truncated" {
						found = true
					}
				}
				assert.True(t, found, "partial assistant message must be persisted")
			},
		},
		{
			name: "unexpected finish reason (content_filter) emits error",
			responses: []mockResponse{
				{chunks: []string{""}, finishReason: llm.FinishReasonContentFilter},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				errs := eventsByType(events, EventError)
				require.NotEmpty(t, errs)
				assert.Contains(t, errs[0].Error.Error(), "unexpected finish reason")
			},
		},
		{
			name:      "resumes existing dialogue from store",
			responses: []mockResponse{stopResponse("continued")},
			config:    Config{SystemPrompt: "test"},
			setup: func(store *mockStore, ag *Agent) {
				store.mu.Lock()
				store.data["d"] = dialoguemanager.Dialogue{
					ID: "d",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "old system prompt"},
						{Role: llm.RoleUser, Content: "previous question"},
						{Role: llm.RoleAssistant, Content: "previous answer"},
					},
					Version: 1,
				}
				store.mu.Unlock()
			},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.True(t, hasEventType(events, EventDone))

				// The request should include the old messages + new user message
				// (skills system message is injected at index 1)
				require.Equal(t, 1, svc.getCallCount())
				msgs := svc.requests[0].Messages
				assert.Equal(t, llm.RoleSystem, msgs[0].Role)
				assert.Equal(t, "old system prompt", msgs[0].Content)
				assert.Equal(t, llm.RoleSystem, msgs[1].Role) // skills
				assert.Equal(t, "previous question", msgs[2].Content)
				assert.Equal(t, "previous answer", msgs[3].Content)
				assert.Equal(t, "test message", msgs[len(msgs)-1].Content)

				// Should use AppendMessages (existing dialogue)
				store.mu.Lock()
				assert.Contains(t, store.appended, "d")
				store.mu.Unlock()
			},
		},
		{
			name:      "orphaned tool calls are stripped from stored dialogue",
			responses: []mockResponse{stopResponse("continued")},
			config:    Config{SystemPrompt: "test"},
			setup: func(store *mockStore, ag *Agent) {
				store.mu.Lock()
				store.data["d"] = dialoguemanager.Dialogue{
					ID: "d",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "sys"},
						{Role: llm.RoleUser, Content: "do things"},
						// Assistant message with a tool call but no
						// matching tool result — simulates a truncated
						// turn that was persisted before tools ran.
						{
							Role:    llm.RoleAssistant,
							Content: "Let me help",
							ToolCalls: []llm.ToolCall{
								{ID: "call_orphan", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}},
							},
						},
					},
					Version: 1,
				}
				store.mu.Unlock()
			},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.True(t, hasEventType(events, EventDone))
				require.Equal(t, 1, svc.getCallCount())
				for _, msg := range svc.requests[0].Messages {
					if msg.Role == llm.RoleAssistant {
						assert.Empty(t, msg.ToolCalls,
							"orphaned tool calls must be stripped before sending to LLM")
						assert.Equal(t, "Let me help", msg.Content,
							"text content must be preserved")
					}
				}
			},
		},
		{
			name:      "orphaned tool results are stripped from stored dialogue",
			responses: []mockResponse{stopResponse("continued")},
			config:    Config{SystemPrompt: "test"},
			setup: func(store *mockStore, ag *Agent) {
				store.mu.Lock()
				store.data["d"] = dialoguemanager.Dialogue{
					ID: "d",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "sys"},
						{Role: llm.RoleUser, Content: "do things"},
						// Tool result with no matching assistant tool call.
						{Role: llm.RoleTool, Content: "file contents", ToolCallID: "call_ghost"},
					},
					Version: 1,
				}
				store.mu.Unlock()
			},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.True(t, hasEventType(events, EventDone))
				require.Equal(t, 1, svc.getCallCount())
				for _, msg := range svc.requests[0].Messages {
					assert.NotEqual(t, llm.RoleTool, msg.Role,
						"orphaned tool result must be removed before sending to LLM")
				}
			},
		},
		{
			name:      "context resources are included in messages",
			responses: []mockResponse{stopResponse("I see the file")},
			config:    Config{SystemPrompt: "test"},
			setup: func(store *mockStore, ag *Agent) {
				uri, _ := workspaceapi.ParseURI("file:///test.go")
				_ = ag.AddContextResource(context.Background(), uri, "package main")
			},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.True(t, hasEventType(events, EventDone))

				require.Equal(t, 1, svc.getCallCount())
				msgs := svc.requests[0].Messages
				var foundResource bool
				for _, msg := range msgs {
					if msg.Role == llm.RoleSystem && assert.Condition(t, func() bool {
						return len(msg.Content) > 0
					}) {
						if contains(msg.Content, "package main") {
							foundResource = true
						}
					}
				}
				assert.True(t, foundResource, "request should include context resource")
			},
		},
		{
			name: "default MaxIterations is 500",
			responses: func() []mockResponse {
				r := make([]mockResponse, 505)
				for i := range r {
					r[i] = toolCallResponse("t", "{}", "c")
				}
				return r
			}(),
			tools:  []Tool{&mockTool{name: "t", result: ToolResult{Content: "ok"}}},
			config: Config{SystemPrompt: "test", MaxIterations: 0}, // 0 → default 500
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				assert.Equal(t, 500, svc.getCallCount())
			},
		},
		{
			name: "reasoning model emits EventReasoning events",
			responses: []mockResponse{
				stopResponseWithReasoning(
					[]string{"Let me think", " about this"},
					"Hello ", "world!",
				),
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				reasoningEvents := eventsByType(events, EventReasoning)
				require.Len(t, reasoningEvents, 2)
				assert.Equal(t, "Let me think", reasoningEvents[0].Reasoning)
				assert.Equal(t, " about this", reasoningEvents[1].Reasoning)

				textEvents := eventsByType(events, EventText)
				require.Len(t, textEvents, 2)
				assert.Equal(t, "Hello ", textEvents[0].Text)
				assert.Equal(t, "world!", textEvents[1].Text)

				assert.True(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "reasoning content is persisted on assistant message",
			responses: []mockResponse{
				stopResponseWithReasoning(
					[]string{"step 1", " step 2"},
					"", "result",
				),
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				d, ok := store.getDialogue("d")
				require.True(t, ok)
				require.Len(t, d.Messages, 3) // system + user + assistant
				assert.Equal(t, "step 1 step 2", d.Messages[2].ReasoningContent)
				assert.Equal(t, "result", d.Messages[2].Content)
			},
		},
		{
			name: "ToolSummary is set on tool call and result events",
			responses: []mockResponse{
				toolCallResponse("my_tool", `{"a":1}`, "call_1"),
				stopResponse("Done!"),
			},
			tools: []Tool{&mockTool{
				name:    "my_tool",
				result:  ToolResult{Content: "ok"},
				summary: "custom summary",
			}},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				toolCalls := eventsByType(events, EventToolCall)
				require.Len(t, toolCalls, 1)
				assert.Equal(t, "custom summary", toolCalls[0].ToolSummary)

				toolResults := eventsByType(events, EventToolResult)
				require.Len(t, toolResults, 1)
				assert.Equal(t, "custom summary", toolResults[0].ToolSummary)
			},
		},
		{
			name:      "tools are passed in request",
			responses: []mockResponse{stopResponse("hi")},
			tools:     []Tool{&mockTool{name: "my_tool", result: ToolResult{Content: "ok"}}},
			config:    Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				require.Equal(t, 1, svc.getCallCount())
				require.Len(t, svc.requests[0].Tools, 1)
				assert.Equal(t, "my_tool", svc.requests[0].Tools[0].Function.Name)
			},
		},
		{
			name: "usage is accumulated and persisted across tool call iterations",
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "my_tool", Arguments: "{}"}},
					},
					usage: llm.Usage{TokensSent: 100, TokensReceived: 20, TokensReasoned: 5, TokensCached: 10},
				},
				{
					chunks:       []string{"Done!"},
					finishReason: llm.FinishReasonStop,
					usage:        llm.Usage{TokensSent: 150, TokensReceived: 30, TokensReasoned: 8, TokensCached: 15},
				},
			},
			tools:  []Tool{&mockTool{name: "my_tool", result: ToolResult{Content: "ok"}}},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				d, ok := store.getDialogue("d")
				require.True(t, ok)

				assert.Equal(t, 250, d.Usage.TokensSent)    // 100 + 150
				assert.Equal(t, 50, d.Usage.TokensReceived) // 20 + 30
				assert.Equal(t, 13, d.Usage.TokensReasoned) // 5 + 8
				assert.Equal(t, 25, d.Usage.TokensCached)   // 10 + 15
				assert.Equal(t, 2, d.Usage.Completions)
				assert.Equal(t, 1, d.Usage.ToolCalls)
				assert.True(t, d.Usage.TotalDuration > 0, "TotalDuration should be > 0")
				assert.True(t, d.Usage.InferenceDuration > 0, "InferenceDuration should be > 0")
				assert.True(t, d.Usage.ToolCallDuration > 0, "ToolCallDuration should be > 0")
			},
		},
		{
			name: "rate limit warnings pass through to agent events",
			responses: []mockResponse{
				{
					chunks:       []string{"Hello!"},
					finishReason: llm.FinishReasonStop,
					rateLimitWarnings: []*llm.RateLimitInfo{
						{Message: "Rate limited. Waiting 2s before retrying (attempt 1/3)."},
					},
				},
			},
			config: Config{SystemPrompt: "test"},
			assertFn: func(t *testing.T, events []Event, store *mockStore, svc *mockService) {
				warnings := eventsByType(events, EventRateLimitWarning)
				require.Len(t, warnings, 1)
				require.NotNil(t, warnings[0].RateLimit)
				assert.Contains(t, warnings[0].RateLimit.Message, "Rate limited")
				assert.Contains(t, warnings[0].RateLimit.Message, "attempt 1/3")

				// Text and done should still arrive.
				textEvents := eventsByType(events, EventText)
				require.NotEmpty(t, textEvents)
				assert.Equal(t, "Hello!", textEvents[0].Text)
				assert.True(t, hasEventType(events, EventDone))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockService{responses: tt.responses}
			store := newMockStore()
			registry := NewRegistry(tt.tools...)
			ag := NewAgent(svc, registry, noSkills(), store, NoMemory(), tt.config)

			if tt.setup != nil {
				tt.setup(store, ag)
			}

			it := ag.Run(context.Background(), "d", "test message")
			events := collectEvents(t, it)

			tt.assertFn(t, events, store, svc)
		})
	}
}

func TestCompactDialoguePreservesPlanContent(t *testing.T) {
	plan := &dialoguemanager.ApprovedPlan{Path: "/tmp/plan.md", Body: "Plan approved.\n\n## Step 1\nDo X"}

	svc := &mockService{
		responses: []mockResponse{
			stopResponse("Summary of work"),
		},
	}
	store := newMockStore()

	d := dialoguemanager.Dialogue{
		ID:           "d",
		ApprovedPlan: plan,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "sys prompt"},
			{Role: llm.RoleAssistant, Content: "Working on step 1..."},
			{Role: llm.RoleUser, Content: "continue"},
			{Role: llm.RoleAssistant, Content: "Done with step 1"},
		},
		Version: 1,
	}
	store.mu.Lock()
	store.data["d"] = d
	store.mu.Unlock()

	compactedMsgs, archivedID, err := CompactDialogue(context.Background(), svc, store, d)
	require.NoError(t, err)
	assert.NotEmpty(t, archivedID)

	require.Len(t, compactedMsgs, 3)
	assert.Equal(t, llm.RoleSystem, compactedMsgs[1].Role)
	assert.Contains(t, compactedMsgs[1].Content, "/tmp/plan.md")
	assert.Contains(t, compactedMsgs[1].Content, "Do X")
}

func TestCompactDialogueWithApprovedPlanUsesResumeProgressMessage(t *testing.T) {
	planContent := "Plan approved. Saved to /tmp/plan.md\n\n## Step 1\nDo X\n## Step 2\nDo Y"

	svc := &mockService{
		responses: []mockResponse{
			stopResponse("Current progress: Step 1 is complete. Step 2 is next."),
		},
	}
	store := newMockStore()

	d := dialoguemanager.Dialogue{
		ID:           "d",
		ApprovedPlan: &dialoguemanager.ApprovedPlan{Path: "/tmp/plan.md", Body: planContent},
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "sys prompt"},
			{Role: llm.RoleAssistant, Content: "Finished step 1 and preparing step 2."},
			{Role: llm.RoleUser, Content: "continue implementing"},
		},
		Version: 1,
	}
	store.mu.Lock()
	store.data["d"] = d
	store.mu.Unlock()

	compactedMsgs, archivedID, err := CompactDialogue(context.Background(), svc, store, d)
	require.NoError(t, err)
	assert.NotEmpty(t, archivedID)

	require.Len(t, compactedMsgs, 3)
	assert.Equal(t, llm.RoleSystem, compactedMsgs[0].Role)
	assert.Equal(t, llm.RoleSystem, compactedMsgs[1].Role)
	assert.Contains(t, compactedMsgs[1].Content, "/tmp/plan.md")
	assert.Equal(t, llm.RoleUser, compactedMsgs[2].Role)
	assert.Contains(t, compactedMsgs[2].Content, "Continue executing the approved plan immediately")
	assert.Contains(t, compactedMsgs[2].Content, "Do NOT create a new plan")
	assert.Contains(t, compactedMsgs[2].Content, "Current progress: Step 1 is complete. Step 2 is next.")
	assert.NotContains(t, compactedMsgs[2].Content, CompactSummaryPrefix)

	assert.Contains(t, compactedMsgs[2].Content, "Keep implementing")
}

func TestCompactDialoguePreservesSkillContent(t *testing.T) {
	skillContent := `<skill_content name="review">` + "\nReview the code.\n</skill_content>"

	svc := &mockService{
		responses: []mockResponse{
			stopResponse("Summary"),
		},
	}
	store := newMockStore()

	d := dialoguemanager.Dialogue{
		ID: "d",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "sys"},
			{Role: llm.RoleUser, Content: "review this"},
			{Role: llm.RoleTool, Content: skillContent, ToolCallID: "c1"},
			{Role: llm.RoleAssistant, Content: "reviewing..."},
		},
		Version: 1,
	}
	store.mu.Lock()
	store.data["d"] = d
	store.mu.Unlock()

	compactedMsgs, _, err := CompactDialogue(context.Background(), svc, store, d)
	require.NoError(t, err)

	var foundSkill bool
	for _, msg := range compactedMsgs {
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, `<skill_content name="review">`) {
			foundSkill = true
		}
	}
	assert.True(t, foundSkill, "skill content must survive CompactDialogue")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAgentRun_StoreGetError(t *testing.T) {
	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	store := newMockStore()
	store.getErr = errors.New("db connection failed")
	registry := NewRegistry()
	ag := NewAgent(svc, registry, noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	it := ag.Run(context.Background(), "d", "hi")
	events := collectEvents(t, it)

	errs := eventsByType(events, EventError)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error.Error(), "db connection failed")
	assert.False(t, hasEventType(events, EventDone))
	assert.Equal(t, 0, svc.getCallCount(), "should not call LLM if store.Get fails")
}

func TestAgentContextResources(t *testing.T) {
	t.Run("add and remove", func(t *testing.T) {
		ag := NewAgent(&mockService{responses: []mockResponse{stopResponse("ok")}},
			NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{SystemPrompt: "test"})
		ctx := context.Background()
		uri, err := workspaceapi.ParseURI("file:///test.go")
		require.NoError(t, err)

		require.NoError(t, ag.AddContextResource(ctx, uri, "data"))

		err = ag.RemoveContextResource(ctx, uri)
		assert.NoError(t, err)
	})

	t.Run("remove nonexistent returns error", func(t *testing.T) {
		ag := NewAgent(&mockService{responses: []mockResponse{stopResponse("ok")}},
			NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{SystemPrompt: "test"})
		ctx := context.Background()
		uri, err := workspaceapi.ParseURI("file:///nonexistent.go")
		require.NoError(t, err)

		err = ag.RemoveContextResource(ctx, uri)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestAgentCancellation(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (context.Context, context.CancelFunc, *mockService, []Tool)
		assertFn func(t *testing.T, events []Event, svc *mockService)
	}{
		{
			name: "pre-cancelled context prevents LLM call",
			setup: func() (context.Context, context.CancelFunc, *mockService, []Tool) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				svc := &mockService{
					responses: []mockResponse{stopResponse("should not see this")},
				}
				return ctx, cancel, svc, nil
			},
			assertFn: func(t *testing.T, events []Event, svc *mockService) {
				assert.False(t, hasEventType(events, EventDone))
			},
		},
		{
			name: "cancellation during tool execution stops loop",
			setup: func() (context.Context, context.CancelFunc, *mockService, []Tool) {
				ctx, cancel := context.WithCancel(context.Background())
				svc := &mockService{
					responses: []mockResponse{
						toolCallResponse("slow_tool", "{}", "c1"),
						stopResponse("should not reach"),
					},
				}
				tool := &mockTool{
					name: "slow_tool",
					executeFn: func(ctx context.Context, args string) ToolResult {
						cancel() // cancel while executing
						return ToolResult{Content: "ok"}
					},
				}
				return ctx, cancel, svc, []Tool{tool}
			},
			assertFn: func(t *testing.T, events []Event, svc *mockService) {
				// The agent might emit some events but should not reach Done
				// because the second LLM call will fail on cancelled context
				assert.False(t, hasEventType(events, EventDone))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, svc, tools := tt.setup()
			defer cancel()
			store := newMockStore()
			registry := NewRegistry(tools...)
			ag := NewAgent(svc, registry, noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

			it := ag.Run(ctx, "d", "hi")
			events := collectEventsWithTimeout(t, it, 5*time.Second)

			tt.assertFn(t, events, svc)
		})
	}
}

func TestAgentConcurrency(t *testing.T) {
	t.Run("concurrent runs on different dialogues", func(t *testing.T) {
		// Build enough responses for all concurrent runs
		responses := make([]mockResponse, 10)
		for i := range responses {
			responses[i] = stopResponse("ok")
		}
		svc := &mockService{responses: responses}
		store := newMockStore()
		registry := NewRegistry()
		ag := NewAgent(svc, registry, noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

		const goroutines = 5
		var wg sync.WaitGroup
		wg.Add(goroutines)
		errCh := make(chan error, goroutines)

		for i := range goroutines {
			go func(i int) {
				defer wg.Done()
				dialogueID := string(rune('a' + i))
				it := ag.Run(context.Background(), dialogueID, "hello")
				events := collectEvents(t, it)
				if !hasEventType(events, EventDone) {
					errCh <- errors.New("missing Done event for dialogue " + dialogueID)
				}
			}(i)
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			t.Error(err)
		}

		assert.Equal(t, goroutines, svc.getCallCount())
	})

	t.Run("concurrent add/remove context resources", func(t *testing.T) {
		ag := NewAgent(&mockService{}, NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{SystemPrompt: "test"})
		ctx := context.Background()

		const goroutines = 20
		var wg sync.WaitGroup
		wg.Add(goroutines * 2) // adds + removes

		for i := range goroutines {
			uri, _ := workspaceapi.ParseURI("file:///file" + string(rune('0'+i)) + ".go")
			go func() {
				defer wg.Done()
				_ = ag.AddContextResource(ctx, uri, "data")
			}()
			go func() {
				defer wg.Done()
				// May fail if add hasn't happened yet — that's fine
				_ = ag.RemoveContextResource(ctx, uri)
			}()
		}

		wg.Wait()
		// Should not panic — that's the main assertion
	})

	t.Run("tool execution happens in parallel within a dialogue", func(t *testing.T) {
		var executing atomic.Int32
		var maxConcurrent atomic.Int32

		svc := &mockService{
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
						{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
						{ID: "c3", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
					},
				},
				stopResponse("done"),
			},
		}
		tool := &mockTool{
			name: "t",
			executeFn: func(ctx context.Context, args string) ToolResult {
				cur := executing.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond) // simulate work — ensures overlap
				executing.Add(-1)
				return ToolResult{Content: "ok"}
			},
		}
		store := newMockStore()
		registry := NewRegistry(tool)
		ag := NewAgent(svc, registry, noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "go")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))
		// Tools within a single dialogue are now executed in parallel
		assert.Greater(t, maxConcurrent.Load(), int32(1),
			"tools should execute in parallel within a dialogue")
	})
}

func TestParallelToolExecution(t *testing.T) {
	t.Run("results are ordered by original call index", func(t *testing.T) {
		// Tool "fast" returns instantly, "slow" takes longer.
		// The LLM returns [slow, fast]; the tool result messages
		// fed to the second LLM call must still be [slow, fast].
		svc := &mockService{
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "slow", Arguments: "{}"}},
						{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "fast", Arguments: "{}"}},
					},
				},
				stopResponse("done"),
			},
		}
		slow := &mockTool{
			name: "slow",
			executeFn: func(ctx context.Context, args string) ToolResult {
				time.Sleep(50 * time.Millisecond)
				return ToolResult{Content: "slow-result"}
			},
		}
		fast := &mockTool{
			name: "fast",
			executeFn: func(ctx context.Context, args string) ToolResult {
				return ToolResult{Content: "fast-result"}
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(slow, fast), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		// Second LLM call must have tool messages in original call order.
		require.Equal(t, 2, svc.getCallCount())
		secondReq := svc.requests[1]
		var toolIDs []string
		for _, msg := range secondReq.Messages {
			if msg.Role == llm.RoleTool {
				toolIDs = append(toolIDs, msg.ToolCallID)
			}
		}
		assert.Equal(t, []string{"c1", "c2"}, toolIDs,
			"tool result messages must be in original call order")
	})

	t.Run("all EventToolCall emitted before any execution starts", func(t *testing.T) {
		started := make(chan struct{})
		svc := &mockService{
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
						{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
					},
				},
				stopResponse("done"),
			},
		}
		tool := &mockTool{
			name: "t",
			executeFn: func(ctx context.Context, args string) ToolResult {
				<-started // block until test releases
				return ToolResult{Content: "ok"}
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})
		it := ag.Run(context.Background(), "d", "go")
		ctx := context.Background()

		// Read events until we see 2 EventToolCall
		var toolCallCount int
		for toolCallCount < 2 {
			ev, ok := it.Next(ctx)
			require.True(t, ok, "should have more events")
			if ev.Type == EventToolCall {
				toolCallCount++
			}
			// Must not see a result before we release the tools
			assert.NotEqual(t, EventToolResult, ev.Type)
		}
		// Now let tools proceed
		close(started)
		// Drain remaining events
		events := collectEvents(t, it)
		results := eventsByType(events, EventToolResult)
		assert.Len(t, results, 2)
	})

	t.Run("parent tool call ID is propagated via context", func(t *testing.T) {
		var receivedIDs []string
		var mu sync.Mutex
		svc := &mockService{
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "call-abc", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "t", Arguments: "{}"}},
					},
				},
				stopResponse("done"),
			},
		}
		tool := &mockTool{
			name: "t",
			executeFn: func(ctx context.Context, args string) ToolResult {
				mu.Lock()
				receivedIDs = append(receivedIDs, ParentToolCallID(ctx))
				mu.Unlock()
				return ToolResult{Content: "ok"}
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})
		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, receivedIDs, 1)
		assert.Equal(t, "call-abc", receivedIDs[0])
	})
}

func TestAutoDiagnostics(t *testing.T) {
	t.Run("single-file apply_patch injects check_file_errors", func(t *testing.T) {
		// LLM calls apply_patch (which reports one touched file).
		// The agent should auto-inject a check_file_errors call.
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("apply_patch", `{"patch":"p"}`, "c1"),
				stopResponse("done"),
			},
		}
		patchTool := &mockTool{
			name: "apply_patch",
			result: ToolResult{
				Content:      "applied 1/1 operations successfully",
				TouchedFiles: []string{"/workspace/main.go"},
			},
		}
		diagTool := &mockTool{
			name:   "check_file_errors",
			result: ToolResult{Content: "no errors or warnings"},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(patchTool, diagTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		// Should have 2 EventToolCall: apply_patch + auto check_file_errors
		toolCalls := eventsByType(events, EventToolCall)
		require.Len(t, toolCalls, 2)
		assert.Equal(t, "apply_patch", toolCalls[0].ToolName)
		assert.Equal(t, "check_file_errors", toolCalls[1].ToolName)

		// Should have 2 EventToolResult in the same order
		toolResults := eventsByType(events, EventToolResult)
		require.Len(t, toolResults, 2)
		assert.Equal(t, "apply_patch", toolResults[0].ToolName)
		assert.Equal(t, "check_file_errors", toolResults[1].ToolName)
		assert.Equal(t, "auto-diag-c1", toolResults[1].ToolCallID)

		// Second LLM call should have both tool result messages
		require.Equal(t, 2, svc.getCallCount())
		secondReq := svc.requests[1]
		var toolIDs []string
		for _, msg := range secondReq.Messages {
			if msg.Role == llm.RoleTool {
				toolIDs = append(toolIDs, msg.ToolCallID)
			}
		}
		assert.Equal(t, []string{"c1", "auto-diag-c1"}, toolIDs)

		// The assistant message should have both tool calls
		var assistantToolCalls int
		for _, msg := range secondReq.Messages {
			if msg.Role == llm.RoleAssistant {
				assistantToolCalls = len(msg.ToolCalls)
			}
		}
		assert.Equal(t, 2, assistantToolCalls)

		// check_file_errors tool should have been executed exactly once
		assert.Equal(t, int32(1), diagTool.execCount.Load())
	})

	t.Run("multi-file apply_patch skips auto-diagnostics", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("apply_patch", `{"patch":"p"}`, "c1"),
				stopResponse("done"),
			},
		}
		patchTool := &mockTool{
			name: "apply_patch",
			result: ToolResult{
				Content:      "applied 2/2 operations successfully",
				TouchedFiles: []string{"/workspace/a.go", "/workspace/b.go"},
			},
		}
		diagTool := &mockTool{
			name:   "check_file_errors",
			result: ToolResult{Content: "no errors or warnings"},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(patchTool, diagTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		// Only 1 tool call (apply_patch), no auto-diagnostics
		toolCalls := eventsByType(events, EventToolCall)
		assert.Len(t, toolCalls, 1)
		assert.Equal(t, "apply_patch", toolCalls[0].ToolName)

		// check_file_errors should NOT have been called
		assert.Equal(t, int32(0), diagTool.execCount.Load())
	})

	t.Run("failed apply_patch skips auto-diagnostics", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("apply_patch", `{"patch":"p"}`, "c1"),
				stopResponse("done"),
			},
		}
		patchTool := &mockTool{
			name: "apply_patch",
			result: ToolResult{
				Content:      "applied 0/1 operations; errors:\nhunk mismatch",
				IsError:      true,
				TouchedFiles: nil, // no files touched on error
			},
		}
		diagTool := &mockTool{
			name:   "check_file_errors",
			result: ToolResult{Content: "no errors or warnings"},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(patchTool, diagTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		// Only 1 tool call (apply_patch)
		toolCalls := eventsByType(events, EventToolCall)
		assert.Len(t, toolCalls, 1)

		// check_file_errors should NOT have been called
		assert.Equal(t, int32(0), diagTool.execCount.Load())
	})

	t.Run("delete-only apply_patch skips auto-diagnostics", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("apply_patch", `{"patch":"p"}`, "c1"),
				stopResponse("done"),
			},
		}
		patchTool := &mockTool{
			name: "apply_patch",
			result: ToolResult{
				Content:      "applied 1/1 operations successfully",
				TouchedFiles: nil, // no non-delete files
			},
		}
		diagTool := &mockTool{
			name:   "check_file_errors",
			result: ToolResult{Content: "no errors or warnings"},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(patchTool, diagTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		toolCalls := eventsByType(events, EventToolCall)
		assert.Len(t, toolCalls, 1)
		assert.Equal(t, int32(0), diagTool.execCount.Load())
	})

	t.Run("diagnostics results are appended after original tool results", func(t *testing.T) {
		// LLM requests two tools: apply_patch and another tool.
		// The auto-diagnostics message should appear after all original results.
		svc := &mockService{
			responses: []mockResponse{
				{
					chunks:       []string{""},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{
						{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "apply_patch", Arguments: `{"patch":"p"}`}},
						{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "other", Arguments: "{}"}},
					},
				},
				stopResponse("done"),
			},
		}
		patchTool := &mockTool{
			name: "apply_patch",
			result: ToolResult{
				Content:      "applied 1/1 operations successfully",
				TouchedFiles: []string{"/workspace/main.go"},
			},
		}
		otherTool := &mockTool{
			name:   "other",
			result: ToolResult{Content: "other result"},
		}
		diagTool := &mockTool{
			name:   "check_file_errors",
			result: ToolResult{Content: "2:1 error: undefined: foo [compiler]"},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(patchTool, otherTool, diagTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		events := collectEvents(t, ag.Run(context.Background(), "d", "go"))
		assert.True(t, hasEventType(events, EventDone))

		// Second LLM call should have 3 tool messages: c1, c2, auto-diag-c1
		require.Equal(t, 2, svc.getCallCount())
		secondReq := svc.requests[1]
		var toolIDs []string
		for _, msg := range secondReq.Messages {
			if msg.Role == llm.RoleTool {
				toolIDs = append(toolIDs, msg.ToolCallID)
			}
		}
		assert.Equal(t, []string{"c1", "c2", "auto-diag-c1"}, toolIDs)
	})
}

func TestChannelIterator(t *testing.T) {
	t.Run("Err returns nil when no errors", func(t *testing.T) {
		ch := make(chan Event, 1)
		ch <- Event{Type: EventText, Text: "hi"}
		close(ch)

		done := make(chan struct{})
		close(done)
		it := &channelIterator{ch: ch, done: done}
		ev, ok := it.Next(context.Background())
		assert.True(t, ok)
		assert.Equal(t, "hi", ev.Text)

		_, ok = it.Next(context.Background())
		assert.False(t, ok)
		assert.NoError(t, it.Err())
	})

	t.Run("Err returns error from EventError", func(t *testing.T) {
		ch := make(chan Event, 1)
		ch <- Event{Type: EventError, Error: errors.New("bad")}
		close(ch)

		done := make(chan struct{})
		close(done)
		it := &channelIterator{ch: ch, done: done}
		ev, ok := it.Next(context.Background())
		assert.True(t, ok)
		assert.Equal(t, EventError, ev.Type)

		_, ok = it.Next(context.Background())
		assert.False(t, ok)
		assert.EqualError(t, it.Err(), "bad")
	})

	t.Run("Err returns context error on cancellation", func(t *testing.T) {
		ch := make(chan Event) // unbuffered, will block
		done := make(chan struct{})
		close(done)
		it := &channelIterator{ch: ch, done: done}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, ok := it.Next(ctx)
		assert.False(t, ok)
		assert.ErrorIs(t, it.Err(), context.Canceled)
	})

	t.Run("Close returns nil", func(t *testing.T) {
		ch := make(chan Event)
		close(ch)
		done := make(chan struct{})
		close(done)
		it := &channelIterator{ch: ch, done: done}
		assert.NoError(t, it.Close())
	})
}

func TestAgentCompact(t *testing.T) {
	t.Run("compact creates new dialogue and continues", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				// summarize() call — returns summary text
				stopResponse("Summary of work so far"),
				// post-compaction iteration
				stopResponse("Continuing from summary"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test system prompt"})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		// 3 LLM calls: main → summarize → post-compact
		require.Equal(t, 3, svc.getCallCount())

		// summarize call has no tools
		assert.Empty(t, svc.requests[1].Tools)

		// summarize call must not contain unpaired assistant tool_calls —
		// the OpenAI API rejects messages where an assistant message with
		// tool_calls is not followed by matching tool result messages.
		summarizeMsgs := svc.requests[1].Messages
		for i, msg := range summarizeMsgs {
			if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					found := false
					for j := i + 1; j < len(summarizeMsgs); j++ {
						if summarizeMsgs[j].Role == llm.RoleTool && summarizeMsgs[j].ToolCallID == tc.ID {
							found = true
							break
						}
					}
					assert.True(t, found,
						"summarize messages contain unpaired tool_call %q (would cause API error)", tc.ID)
				}
			}
		}

		// post-compact call has compacted summary as user message
		// (skills system message is injected at index 1)
		postMsgs := svc.requests[2].Messages
		require.Len(t, postMsgs, 3)
		assert.Equal(t, llm.RoleSystem, postMsgs[0].Role)
		assert.Equal(t, "test system prompt", postMsgs[0].Content)
		assert.Equal(t, llm.RoleSystem, postMsgs[1].Role) // skills
		assert.Equal(t, llm.RoleUser, postMsgs[2].Role)
		assert.Equal(t, CompactSummaryPrefix+"Summary of work so far", postMsgs[2].Content)

		// Archived dialogue exists with old messages
		archived, ok := store.getDialogue("d-archived")
		assert.True(t, ok)
		assert.NotEmpty(t, archived.Messages)

		// Current dialogue (same ID) has compacted messages
		// plus the post-compaction response
		d, ok := store.getDialogue("d")
		assert.True(t, ok)
		require.GreaterOrEqual(t, len(d.Messages), 3)
		assert.Equal(t, llm.RoleSystem, d.Messages[0].Role)
		assert.Equal(t, llm.RoleUser, d.Messages[1].Role)
		assert.True(t, strings.HasPrefix(d.Messages[1].Content, CompactSummaryPrefix))
		assert.Equal(t, llm.RoleAssistant, d.Messages[2].Role)

		// Events include compacting/compacted/done in correct order
		assert.True(t, hasEventType(events, EventCompacting))
		assert.True(t, hasEventType(events, EventCompacted))
		assert.True(t, hasEventType(events, EventDone))
	})

	t.Run("compact failure LLM error falls through", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				// summarize() fails
				{err: errors.New("api timeout")},
				// agent continues with error fed back
				stopResponse("I see the error"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		require.Equal(t, 3, svc.getCallCount())

		// Original messages + error tool result in third call
		thirdMsgs := svc.requests[2].Messages
		var foundErrorTool bool
		for _, msg := range thirdMsgs {
			if msg.Role == llm.RoleTool && contains(msg.Content, "Compaction failed") {
				foundErrorTool = true
			}
		}
		assert.True(t, foundErrorTool, "should contain error tool result")

		// EventCompacting emitted, but NOT EventCompacted
		assert.True(t, hasEventType(events, EventCompacting))
		assert.False(t, hasEventType(events, EventCompacted))
		assert.True(t, hasEventType(events, EventDone))

		// No archived dialogue in store (compaction failed)
		_, ok := store.getDialogue("d-archived")
		assert.False(t, ok)
	})

	t.Run("compact failure stream error falls through", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				// summarize() stream error
				{chunks: []string{"partial"}, finishReason: llm.FinishReasonStop, streamErr: errors.New("connection reset")},
				stopResponse("I see"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		assert.True(t, hasEventType(events, EventCompacting))
		assert.False(t, hasEventType(events, EventCompacted))
		assert.True(t, hasEventType(events, EventDone))

		_, ok := store.getDialogue("d-archived")
		assert.False(t, ok)
	})

	t.Run("Ctrl-C during compaction cancels gracefully", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				// summarize() fails because context is cancelled
				{err: context.Canceled},
				// never reached — context cancelled
				stopResponse("unreachable"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name: "compact",
			executeFn: func(_ context.Context, _ string) ToolResult {
				cancel() // simulate Ctrl-C
				return ToolResult{Content: "Compacting conversation...", Compact: true}
			},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(ctx, "d", "do stuff")
		events := collectEventsWithTimeout(t, it, 5*time.Second)

		// No compacted dialogue should be created
		assert.False(t, hasEventType(events, EventCompacted))
		assert.False(t, hasEventType(events, EventDone))

		// No archive created
		_, ok := store.getDialogue("d-archived")
		assert.False(t, ok)
	})

	t.Run("compact overwrites dialogue and archives old messages", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				stopResponse("Summary text"),
				stopResponse("Post-compact response"),
			},
		}
		store := newMockStore()
		// Pre-populate with history
		oldMessages := []llm.Message{
			{Role: llm.RoleSystem, Content: "sys"},
			{Role: llm.RoleUser, Content: "old question"},
			{Role: llm.RoleAssistant, Content: "old answer"},
		}
		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID: "d", Messages: oldMessages, Version: 1,
		}
		store.mu.Unlock()

		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "new question")
		_ = collectEvents(t, it)

		// Current dialogue (same ID) now has compacted messages
		curD, ok := store.getDialogue("d")
		assert.True(t, ok)
		require.GreaterOrEqual(t, len(curD.Messages), 2)
		assert.True(t, strings.HasPrefix(curD.Messages[1].Content, CompactSummaryPrefix),
			"dialogue should be overwritten with compacted summary")

		// Archived dialogue has old messages
		archivedD, ok := store.getDialogue("d-archived")
		assert.True(t, ok)
		assert.NotEmpty(t, archivedD.Messages)
	})

	t.Run("re-compaction does not overwrite previous archive", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				stopResponse("Re-summary"),
				stopResponse("Continuing"),
			},
		}
		store := newMockStore()
		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID: "d",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "prev summary"},
			},
			Version: 1,
		}
		// Pre-existing archive from a first compaction.
		store.data["d-archived"] = dialoguemanager.Dialogue{
			ID: "d-archived",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "original messages"},
			},
			Version: 1,
		}
		store.mu.Unlock()

		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "more work")
		events := collectEvents(t, it)

		compactedEvents := eventsByType(events, EventCompacted)
		require.Len(t, compactedEvents, 1)

		// The first archive must not have been overwritten.
		first, ok := store.getDialogue("d-archived")
		assert.True(t, ok, "first archive must still exist")
		assert.Equal(t, "original messages", first.Messages[1].Content,
			"first archive content must be preserved")

		// The second archive should be stored under d-archived-2.
		second, ok := store.getDialogue("d-archived-2")
		assert.True(t, ok, "second archive should exist as d-archived-2")
		assert.Equal(t, "prev summary", second.Messages[1].Content)

		// Event should reference the actual archived ID.
		assert.Equal(t, "d-archived-2", compactedEvents[0].ArchivedDialogueID)
	})

	t.Run("EventCompacting and EventCompacted emitted in correct order", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				stopResponse("Summary"),
				stopResponse("Done"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "go")
		events := collectEvents(t, it)

		var compactingIdx, compactedIdx, doneIdx int
		for i, ev := range events {
			switch ev.Type {
			case EventCompacting:
				compactingIdx = i
			case EventCompacted:
				compactedIdx = i
			case EventDone:
				doneIdx = i
			}
		}
		assert.Less(t, compactingIdx, compactedIdx, "compacting before compacted")
		assert.Less(t, compactedIdx, doneIdx, "compacted before done")
	})

	t.Run("compact emits EventDone before post-compaction events", func(t *testing.T) {
		// Reproduces the "com" bug: after compaction the agent loop
		// continues to a new LLM call without emitting EventDone.
		// The TUI never receives a turn boundary, so:
		//  - the compact Turn is never closed (currentTurn stays set)
		//  - any short text the LLM streams before its next tool
		//    call is finalized as a standalone message ("com")
		//  - the next tool call merges into the compact Turn
		//
		// The fix emits EventDone after successful compaction so the
		// TUI can close the compact Turn before new events arrive.
		svc := &mockService{
			responses: []mockResponse{
				// 1. LLM generates text, then calls compact
				{
					chunks:       []string{"Let me compact"},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{{
						ID: "c1", Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "compact", Arguments: "{}"},
					}},
				},
				// 2. summarize() call
				stopResponse("Summary of work"),
				// 3. post-compaction: LLM generates short text + tool call
				{
					chunks:       []string{"com"},
					finishReason: llm.FinishReasonToolCall,
					toolCalls: []llm.ToolCall{{
						ID: "c2", Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`},
					}},
				},
				// 4. after read_file result, LLM finishes
				stopResponse("Done"),
			},
		}
		store := newMockStore()
		compactTool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		readTool := &mockTool{
			name:   "read_file",
			result: ToolResult{Content: "file contents"},
		}
		ag := NewAgent(svc, NewRegistry(compactTool, readTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		// Find the compact tool result and the first post-compaction text.
		var compactResultIdx, firstPostTextIdx int
		compactResultFound := false
		for i, ev := range events {
			if ev.Type == EventToolResult && ev.ToolName == "compact" {
				compactResultIdx = i
				compactResultFound = true
			}
			if compactResultFound && ev.Type == EventText && ev.Text == "com" {
				firstPostTextIdx = i
				break
			}
		}
		require.True(t, compactResultFound, "should have compact tool result")
		require.NotZero(t, firstPostTextIdx, "should have post-compaction text")

		// There must be an EventDone between the compact result and
		// the post-compaction text, so the TUI can close the compact
		// Turn before new events arrive.
		var doneFound bool
		for i := compactResultIdx + 1; i < firstPostTextIdx; i++ {
			if events[i].Type == EventDone {
				doneFound = true
				break
			}
		}
		assert.True(t, doneFound,
			"EventDone must appear between compact tool result and post-compaction text")
	})

	t.Run("compact preserves activated skill content", func(t *testing.T) {
		// Simulate: user asks something, LLM activates skill tool,
		// then LLM calls compact. After compaction the skill content
		// should be re-injected into the compacted messages.
		svc := &mockService{
			responses: []mockResponse{
				// 1. LLM calls the skill tool
				toolCallResponse("skill", `{"name":"review"}`, "c1"),
				// 2. LLM calls compact
				toolCallResponse("compact", "{}", "c2"),
				// 3. Summarize call
				stopResponse("Summary of work"),
				// 4. Post-compaction
				stopResponse("Continuing"),
			},
		}
		store := newMockStore()

		skillContent := `<skill_content name="review">` +
			"\nReview the code.\n</skill_content>"
		skillTool := &mockTool{
			name:   "skill",
			result: ToolResult{Content: skillContent},
		}
		compactTool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "compacting...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(skillTool, compactTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "review this")
		_ = collectEvents(t, it)

		// Compacted dialogue (same ID) should have skill content at position 1.
		d, ok := store.getDialogue("d")
		require.True(t, ok)
		require.GreaterOrEqual(t, len(d.Messages), 3,
			"should have system + skill + summary")
		assert.Equal(t, llm.RoleSystem, d.Messages[1].Role)
		assert.Contains(t, d.Messages[1].Content, `<skill_content name="review">`)

		// Post-compaction LLM request should include the preserved skill.
		postReq := svc.requests[len(svc.requests)-1]
		var foundSkill bool
		for _, msg := range postReq.Messages {
			if msg.Role == llm.RoleSystem &&
				contains(msg.Content, `<skill_content name="review">`) {
				foundSkill = true
			}
		}
		assert.True(t, foundSkill,
			"post-compaction request should include preserved skill content")
	})

	t.Run("compact deduplicates multiple activations of same skill", func(t *testing.T) {
		msgs := []llm.Message{
			{Role: llm.RoleSystem, Content: "sys"},
			{Role: llm.RoleTool, Content: `<skill_content name="x">` + "\nbody\n</skill_content>", ToolCallID: "c1"},
			{Role: llm.RoleTool, Content: `<skill_content name="x">` + "\nbody\n</skill_content>", ToolCallID: "c2"},
			{Role: llm.RoleTool, Content: `<skill_content name="y">` + "\nother\n</skill_content>", ToolCallID: "c3"},
		}
		preserved := extractSkillContent(msgs)
		assert.Len(t, preserved, 2, "should deduplicate identical skill content")
	})

	t.Run("compact preserves resource messages after compaction", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("compact", "{}", "c1"),
				stopResponse("Summary"),
				stopResponse("Done"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "compact",
			result: ToolResult{Content: "Compacting conversation...", Compact: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		uri, _ := workspaceapi.ParseURI("file:///test.go")
		_ = ag.AddContextResource(context.Background(), uri, "package main")

		it := ag.Run(context.Background(), "d", "go")
		_ = collectEvents(t, it)

		// Post-compaction request should have resource message
		require.Equal(t, 3, svc.getCallCount())
		postMsgs := svc.requests[2].Messages
		var foundResource bool
		for _, msg := range postMsgs {
			if msg.Role == llm.RoleSystem && contains(msg.Content, "package main") {
				foundResource = true
			}
		}
		assert.True(t, foundResource, "post-compaction request should include resource messages")
	})
}

func TestClearContext(t *testing.T) {
	t.Run("wraps plan with markers and emits EventCompacted", func(t *testing.T) {
		planContent := "Plan approved. Saved to /tmp/plan.md\n\n## Step 1\nDo something"
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("exit_plan", "{}", "ep1"),
				// After clearContext the agent continues with a new LLM call.
				stopResponse("Starting work on the plan"),
			},
		}
		store := newMockStore()
		tool := &mockTool{
			name:   "exit_plan",
			result: ToolResult{Content: planContent, ApprovedPlan: &dialoguemanager.ApprovedPlan{Path: "/tmp/plan.md", Body: planContent}, ClearContext: true},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "create a plan")
		events := collectEvents(t, it)

		compactedEvents := eventsByType(events, EventCompacted)
		require.Len(t, compactedEvents, 1)
		assert.Equal(t, "d-archived", compactedEvents[0].ArchivedDialogueID)

		// The stored dialogue must persist the approved plan as first-class state.
		d, ok := store.getDialogue("d")
		require.True(t, ok)
		require.NotNil(t, d.ApprovedPlan, "stored dialogue should contain approved plan metadata")
		assert.Equal(t, "/tmp/plan.md", d.ApprovedPlan.Path)
		assert.Equal(t, planContent, d.ApprovedPlan.Body)
	})

	t.Run("plan survives auto-compaction with file path", func(t *testing.T) {
		planPath := "/tmp/plans/20260319-120000-my-feature.md"
		planBody := "## The Plan\nStep 1: Do X\nStep 2: Do Y"
		planContent := fmt.Sprintf("Plan approved. Saved to %s\n\n%s", planPath, planBody)
		countCalls := 0
		svc := &mockService{
			responses: []mockResponse{
				// 1st call (from the resumed agent after clearContext): summarize for auto-compact
				stopResponse("Summary of work so far"),
				// 2nd call: post-compaction response
				stopResponse("Continuing after compaction"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				countCalls++
				if countCalls <= 1 {
					return 900, nil // triggers auto-compact
				}
				return 100, nil
			},
		}
		store := newMockStore()

		// Pre-seed the store with approved-plan metadata plus some work.
		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID:           "d",
			ApprovedPlan: &dialoguemanager.ApprovedPlan{Path: planPath, Body: planContent},
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleAssistant, Content: "I'll start working on step 1..."},
			},
			Version: 1,
		}
		store.mu.Unlock()

		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "continue working")
		events := collectEvents(t, it)

		// Auto-compact should have triggered.
		assert.True(t, hasEventType(events, EventCompacted))

		// After compaction, the plan (including file path) must be in the stored dialogue.
		d, ok := store.getDialogue("d")
		require.True(t, ok)
		require.NotNil(t, d.ApprovedPlan, "approved plan metadata must survive auto-compaction")
		assert.Contains(t, d.ApprovedPlan.Path, planPath,
			"plan file path must be preserved after compaction")
		assert.Contains(t, d.ApprovedPlan.Body, planBody,
			"plan body must be preserved after compaction")
	})

	t.Run("plan survives repeated compactions", func(t *testing.T) {
		planContent := "Plan approved.\n\n## The Plan\nStep 1: Do X"
		countCalls := 0
		svc := &mockService{
			responses: []mockResponse{
				// 1st: summarize for first auto-compact
				stopResponse("First summary"),
				// 2nd: summarize for second auto-compact
				stopResponse("Second summary"),
				// 3rd: post-compaction response
				stopResponse("Done"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				countCalls++
				if countCalls <= 2 {
					return 900, nil // triggers auto-compact twice
				}
				return 100, nil
			},
		}
		store := newMockStore()

		// Pre-seed with approved-plan metadata (as if one compaction already happened).
		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID:           "d",
			ApprovedPlan: &dialoguemanager.ApprovedPlan{Body: planContent},
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: CompactSummaryPrefix + "Prior summary"},
				{Role: llm.RoleAssistant, Content: "Working on it..."},
			},
			Version: 1,
		}
		store.mu.Unlock()

		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "keep going")
		events := collectEvents(t, it)

		compactedEvents := eventsByType(events, EventCompacted)
		require.GreaterOrEqual(t, len(compactedEvents), 1,
			"at least one compaction should occur")

		// After repeated compaction, the plan must still be present.
		d, ok := store.getDialogue("d")
		require.True(t, ok)
		require.NotNil(t, d.ApprovedPlan)
		assert.Contains(t, d.ApprovedPlan.Body, planContent,
			"approved plan must survive repeated compactions")
	})

	t.Run("approved plan compaction keeps resume instructions across repeats", func(t *testing.T) {
		planContent := "Plan approved. Saved to /tmp/plan.md\n\n## The Plan\nStep 1: Do X"
		countCalls := 0
		svc := &mockService{
			responses: []mockResponse{
				stopResponse("Progress after compact 1"),
				stopResponse("Progress after compact 2"),
				stopResponse("Done"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				countCalls++
				if countCalls <= 2 {
					return 900, nil
				}
				return 100, nil
			},
		}
		store := newMockStore()

		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID:           "d",
			ApprovedPlan: &dialoguemanager.ApprovedPlan{Path: "/tmp/plan.md", Body: planContent},
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleAssistant, Content: "Working on it..."},
			},
			Version: 1,
		}
		store.mu.Unlock()

		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		it := ag.Run(context.Background(), "d", "keep going")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventCompacted))

		stored, ok := store.getDialogue("d")
		require.True(t, ok)
		require.GreaterOrEqual(t, len(stored.Messages), 3)
		require.NotNil(t, stored.ApprovedPlan)
		assert.Equal(t, planContent, stored.ApprovedPlan.Body)
		assert.Equal(t, llm.RoleUser, stored.Messages[2].Role)
		assert.Contains(t, stored.Messages[2].Content, "Continue executing the approved plan immediately")
		assert.Contains(t, stored.Messages[2].Content, "Do NOT create a new plan")
	})
}

// assertToolCallPresent verifies that a tool result message with the given
// ToolCallID exists and has the expected content.
func assertToolCallPresent(t *testing.T, msgs []llm.Message, toolCallID, wantContent string) {
	t.Helper()
	for _, msg := range msgs {
		if msg.Role == llm.RoleTool && msg.ToolCallID == toolCallID {
			assert.Equal(t, wantContent, msg.Content)
			return
		}
	}
	t.Errorf("expected tool result %q to be present, but not found", toolCallID)
}

func TestToolResultsNotDropped(t *testing.T) {
	t.Run("DropToolResultIDs returned by tools are ignored", func(t *testing.T) {
		// Turn 1: LLM calls read → gets content (call_r1)
		// Turn 2: LLM calls read again → returns DropToolResultIDs for call_r1
		// Turn 3: LLM produces final response
		// Verify: call_r1 is still present (drops are disabled for cache stability)
		var callCount int
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("read", `{"path":"a.txt"}`, "call_r1"),
				toolCallResponse("read", `{"path":"a.txt"}`, "call_r2"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		readTool := &mockTool{
			name: "read",
			executeFn: func(_ context.Context, _ string) ToolResult {
				callCount++
				if callCount == 1 {
					return ToolResult{Content: "file contents v1"}
				}
				return ToolResult{
					Content:           "file contents v2",
					DropToolResultIDs: []string{"call_r1"},
				}
			},
		}
		ag := NewAgent(svc, NewRegistry(readTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "read a.txt twice")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))

		// Both reads should be present — drops are disabled.
		require.Equal(t, 3, svc.getCallCount())
		thirdReq := svc.requests[2]
		assertToolCallPresent(t, thirdReq.Messages, "call_r1", "file contents v1")
		assertToolCallPresent(t, thirdReq.Messages, "call_r2", "file contents v2")
	})
}

func TestStaleReadsRetained(t *testing.T) {
	t.Run("read_file called twice retains both results", func(t *testing.T) {
		// Drops are disabled for prompt caching stability.
		// Both reads should remain in the conversation.
		var callCount int
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("read", `{"path":"a.txt"}`, "call_r1"),
				toolCallResponse("read", `{"path":"a.txt"}`, "call_r2"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		readTool := &mockTool{
			name: "read",
			executeFn: func(_ context.Context, _ string) ToolResult {
				callCount++
				if callCount == 1 {
					return ToolResult{Content: "file contents v1"}
				}
				return ToolResult{
					Content:           "file contents v2",
					DropToolResultIDs: []string{"call_r1"},
				}
			},
		}
		ag := NewAgent(svc, NewRegistry(readTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "read a.txt twice")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))

		require.Equal(t, 3, svc.getCallCount())
		thirdReq := svc.requests[2]
		assertToolCallPresent(t, thirdReq.Messages, "call_r1", "file contents v1")
		assertToolCallPresent(t, thirdReq.Messages, "call_r2", "file contents v2")
	})

	t.Run("apply_patch after read_file retains the read result", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("read", `{"path":"a.txt"}`, "call_r1"),
				toolCallResponse("patch", `{}`, "call_p1"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		readTool := &mockTool{
			name:   "read",
			result: ToolResult{Content: "file contents"},
		}
		patchTool := &mockTool{
			name: "patch",
			executeFn: func(_ context.Context, _ string) ToolResult {
				return ToolResult{
					Content:           "applied 1/1 operations successfully",
					DropToolResultIDs: []string{"call_r1"},
				}
			},
		}
		ag := NewAgent(svc, NewRegistry(readTool, patchTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "read then patch")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))

		require.Equal(t, 3, svc.getCallCount())
		thirdReq := svc.requests[2]
		assertToolCallPresent(t, thirdReq.Messages, "call_r1", "file contents")
	})
}

func TestSupersededErrorsRetained(t *testing.T) {
	t.Run("successful retry retains previous error", func(t *testing.T) {
		// Drops are disabled for prompt caching stability.
		// The errored result stays in the conversation alongside the success.
		var callCount int
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("run", `{"cmd":"go test"}`, "call_e1"),
				toolCallResponse("run", `{"cmd":"go test"}`, "call_s1"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		runTool := &mockTool{
			name: "run",
			executeFn: func(_ context.Context, _ string) ToolResult {
				callCount++
				if callCount == 1 {
					return ToolResult{Content: "FAIL: compilation error", IsError: true}
				}
				return ToolResult{Content: "PASS: all tests passed"}
			},
		}
		ag := NewAgent(svc, NewRegistry(runTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "run tests")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))

		require.Equal(t, 3, svc.getCallCount())
		thirdReq := svc.requests[2]
		assertToolCallPresent(t, thirdReq.Messages, "call_e1", "FAIL: compilation error")
		assertToolCallPresent(t, thirdReq.Messages, "call_s1", "PASS: all tests passed")
	})

	t.Run("multiple errors all retained after success", func(t *testing.T) {
		var callCount int
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("run", `{"cmd":"go test"}`, "call_e1"),
				toolCallResponse("run", `{"cmd":"go test"}`, "call_e2"),
				toolCallResponse("run", `{"cmd":"go test"}`, "call_s1"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		runTool := &mockTool{
			name: "run",
			executeFn: func(_ context.Context, _ string) ToolResult {
				callCount++
				if callCount <= 2 {
					return ToolResult{Content: "FAIL", IsError: true}
				}
				return ToolResult{Content: "PASS"}
			},
		}
		ag := NewAgent(svc, NewRegistry(runTool), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "run tests")
		events := collectEvents(t, it)
		assert.True(t, hasEventType(events, EventDone))

		require.Equal(t, 4, svc.getCallCount())
		fourthReq := svc.requests[3]
		assertToolCallPresent(t, fourthReq.Messages, "call_e1", "FAIL")
		assertToolCallPresent(t, fourthReq.Messages, "call_e2", "FAIL")
		assertToolCallPresent(t, fourthReq.Messages, "call_s1", "PASS")
	})
}

func TestNoTransientContextHint(t *testing.T) {
	// The dynamic context hint was removed because its varying content
	// (percentage, token count) landed on the system cache breakpoint
	// and busted the prompt cache every iteration. Compaction guidance
	// is now static in the system prompt.
	svc := &mockService{
		responses: []mockResponse{stopResponse("noted")},
		// Simulate 80% context usage.
		countTokensFn:  func([]llm.Message) (int, error) { return 8000, nil },
		contextWindowN: 10000,
	}
	store := newMockStore()
	store.mu.Lock()
	store.data["d"] = dialoguemanager.Dialogue{
		ID: "d", Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "sys"},
			{Role: llm.RoleUser, Content: "q"},
			{Role: llm.RoleAssistant, Content: "a"},
		}, Version: 1,
	}
	store.mu.Unlock()

	ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{SystemPrompt: "sys"})
	it := ag.Run(context.Background(), "d", "new")
	_ = collectEvents(t, it)

	require.Equal(t, 1, svc.getCallCount())
	for _, msg := range svc.requests[0].Messages {
		if msg.Role == llm.RoleSystem {
			assert.NotContains(t, msg.Content, "tokens)",
				"no transient context hint should be injected — compaction guidance is in the static system prompt")
		}
	}
}

func dirURI(dir string) workspaceapi.URI {
	u, _ := workspaceapi.ParseURI("file://" + dir)
	return u
}

type nopFileSystem struct{}

func (nopFileSystem) URI(path string) (workspaceapi.URI, error) { return workspaceapi.URI{}, nil }
func (nopFileSystem) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, os.ErrNotExist
}
func (nopFileSystem) Remove(string) error                   { return nil }
func (nopFileSystem) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (nopFileSystem) ReadDir(string) ([]os.DirEntry, error) { return nil, nil }
func (nopFileSystem) MkdirAll(string, os.FileMode) error    { return nil }

type osFileSystem struct{}

func (osFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.CurrentUserHostURI(path)
}
func (osFileSystem) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}
func (osFileSystem) Remove(path string) error                   { return os.Remove(path) }
func (osFileSystem) Stat(path string) (os.FileInfo, error)      { return os.Stat(path) }
func (osFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func TestDefaultSystemPrompt(t *testing.T) {
	prompt := DefaultSystemPrompt(dirURI("/workspace"))
	assert.Contains(t, prompt, "/workspace")
	assert.Contains(t, prompt, "coding assistant")
	assert.Contains(t, prompt, "tools")
	assert.Contains(t, prompt, "compact")

	// Tool strategy section recommends semantic tools.
	assert.Contains(t, prompt, "find_definition")
	assert.Contains(t, prompt, "outline_file")
	assert.Contains(t, prompt, "format_file")
	assert.Contains(t, prompt, "read_file")

	// Stronger directive language.
	assert.Contains(t, prompt, "always use")
	assert.Contains(t, prompt, "Do NOT")
}

func TestSkillsInjectedPerTurn(t *testing.T) {
	// Create a temp skill directory with a skill.
	skillDir := t.TempDir()
	dir := filepath.Join(skillDir, "greet")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: greet
description: Say hello
---
Greet the user warmly.
`), 0o644))

	registry := skills.NewRegistry(osFileSystem{}, dirURI(""), []string{skillDir}, nil)
	require.Len(t, registry.List(), 3) // greet + builtins (explore, plan)

	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	store := newMockStore()
	ag := NewAgent(svc, NewRegistry(), registry, store, NoMemory(), Config{
		SystemPrompt: "test",
	})

	it := ag.Run(context.Background(), "d", "hello")
	_ = collectEvents(t, it)

	// The LLM request should include a transient skills system message.
	require.Equal(t, 1, svc.getCallCount())
	req := svc.requests[0]
	require.True(t, len(req.Messages) >= 3, "expected at least 3 messages (system + skills + user)")
	assert.Equal(t, llm.RoleSystem, req.Messages[1].Role)
	assert.Contains(t, req.Messages[1].Content, "greet")
	assert.Contains(t, req.Messages[1].Content, "skill tool")

	// The persisted messages should NOT contain the skills message.
	d, ok := store.getDialogue("d")
	require.True(t, ok)
	for _, msg := range d.Messages {
		if msg.Role == llm.RoleSystem && msg.Content != "test" {
			t.Errorf("persisted messages should not contain transient skills message, found: %s", msg.Content)
		}
	}
}

func TestSkillsNotInjectedWhenEmpty(t *testing.T) {
	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	reg := skills.NewRegistry(nopFileSystem{}, dirURI(""), nil, nil)
	ag := NewAgent(svc, NewRegistry(), reg, newMockStore(), NoMemory(), Config{
		SystemPrompt: "test",
	})

	it := ag.Run(context.Background(), "d", "hello")
	_ = collectEvents(t, it)

	require.Equal(t, 1, svc.getCallCount())
	req := svc.requests[0]
	// system + skills (builtins) + user messages.
	assert.Equal(t, 3, len(req.Messages))
	assert.Equal(t, llm.RoleSystem, req.Messages[0].Role)
	assert.Equal(t, llm.RoleSystem, req.Messages[1].Role) // builtin skills
	assert.Equal(t, llm.RoleUser, req.Messages[2].Role)
	assert.Contains(t, req.Messages[1].Content, "explore")
	assert.Contains(t, req.Messages[1].Content, "plan")
}

func TestNewAgent_DefaultMaxIterations(t *testing.T) {
	ag := NewAgent(&mockService{}, NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{MaxIterations: 0})
	assert.Equal(t, 500, ag.config.MaxIterations)

	ag2 := NewAgent(&mockService{}, NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{MaxIterations: -5})
	assert.Equal(t, 500, ag2.config.MaxIterations)

	ag3 := NewAgent(&mockService{}, NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{MaxIterations: 10})
	assert.Equal(t, 10, ag3.config.MaxIterations)
}

func TestAgentMaxOutputTokens(t *testing.T) {
	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	ag := NewAgent(svc, NewRegistry(), noSkills(), newMockStore(), NoMemory(), Config{SystemPrompt: "test"})

	assert.Zero(t, ag.MaxOutputTokens())
	ag.SetMaxOutputTokens(8192)
	assert.Equal(t, 8192, ag.MaxOutputTokens())

	it := ag.Run(context.Background(), "d", "hello")
	_ = collectEvents(t, it)

	require.Equal(t, 1, svc.getCallCount())
	require.Len(t, svc.requests, 1)
	assert.Equal(t, 8192, svc.requests[0].MaxOutputTokens)
}

func TestPersistMessages_AppendFallback(t *testing.T) {
	// Test that when Create returns ErrAlreadyExists, AppendMessages is called
	store := newMockStore()
	ctx := context.Background()

	// Pre-populate store so Create returns ErrAlreadyExists
	store.mu.Lock()
	store.data["d"] = dialoguemanager.Dialogue{ID: "d", Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "old"},
	}}
	store.mu.Unlock()

	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	it := ag.Run(ctx, "d", "new message")
	_ = collectEvents(t, it)

	store.mu.Lock()
	assert.Contains(t, store.appended, "d", "should have called AppendMessages")
	store.mu.Unlock()
}

func TestPersistMessages_AppendError(t *testing.T) {
	// When AppendMessages fails, the agent should not panic — just log
	store := newMockStore()
	store.appendErr = errors.New("write error")

	// Pre-populate so Create → ErrAlreadyExists → AppendMessages → error
	store.mu.Lock()
	store.data["d"] = dialoguemanager.Dialogue{ID: "d", Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "old"},
	}}
	store.mu.Unlock()

	svc := &mockService{responses: []mockResponse{stopResponse("hi")}}
	ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	it := ag.Run(context.Background(), "d", "msg")
	events := collectEvents(t, it)

	// Should still produce Done (persistence error is logged, not emitted)
	assert.True(t, hasEventType(events, EventDone))
}

func TestPersistMessages_SaveOnCancelledContext(t *testing.T) {
	// When the session context is cancelled (e.g. tab closed), messages
	// should still be persisted using a background context.
	t.Run("new dialogue persisted after context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := newMockStore()
		svc := &mockService{
			responses: []mockResponse{stopResponse("hello back")},
		}
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

		// Cancel context right before the agent runs, so that
		// CreateCompletion fails with context.Canceled but the
		// user message should still be persisted.
		svc.beforeCompletion = func() { cancel() }

		it := ag.Run(ctx, "d", "hello")
		_ = collectEventsWithTimeout(t, it, 5*time.Second)

		d, ok := store.getDialogue("d")
		require.True(t, ok, "dialogue should exist in store after cancellation")
		// Should have at least the system prompt + user message
		assert.GreaterOrEqual(t, len(d.Messages), 2, "should persist system prompt + user message")
		assert.Equal(t, "hello", d.Messages[len(d.Messages)-1].Content)
	})

	t.Run("existing dialogue appended after context cancellation during tool execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := newMockStore()

		// Pre-populate store with an existing dialogue
		store.mu.Lock()
		store.data["d"] = dialoguemanager.Dialogue{
			ID: "d",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "test"},
				{Role: llm.RoleUser, Content: "old message"},
				{Role: llm.RoleAssistant, Content: "old response"},
			},
		}
		store.mu.Unlock()

		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("my_tool", `{"arg":"val"}`, "c1"),
				stopResponse("should not reach"),
			},
		}
		tool := &mockTool{
			name: "my_tool",
			executeFn: func(ctx context.Context, args string) ToolResult {
				cancel() // cancel context during tool execution (simulates tab close)
				return ToolResult{Content: "tool output"}
			},
		}
		ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

		it := ag.Run(ctx, "d", "use the tool")
		_ = collectEventsWithTimeout(t, it, 5*time.Second)

		d, ok := store.getDialogue("d")
		require.True(t, ok, "dialogue should exist in store after cancellation")
		// Should have the original messages + new user message + assistant tool call + tool result
		assert.GreaterOrEqual(t, len(d.Messages), 5,
			"should persist user msg, assistant tool call, and tool result: got %d messages", len(d.Messages))

		// Verify the append was called (not create)
		store.mu.Lock()
		assert.Contains(t, store.appended, "d", "should have used AppendMessages for existing dialogue")
		store.mu.Unlock()
	})
}

func TestAgentRun_CheckpointsAfterToolIteration(t *testing.T) {
	store := newMockStore()
	secondCallStarted := make(chan struct{})
	allowSecondCall := make(chan struct{})

	svc := &mockService{
		responses: []mockResponse{
			toolCallResponse("my_tool", `{"arg":"val"}`, "c1"),
			stopResponse("all done"),
		},
	}
	svc.beforeCompletion = func() {
		if svc.getCallCount() != 1 {
			return
		}
		close(secondCallStarted)
		<-allowSecondCall
	}

	tool := &mockTool{name: "my_tool", result: ToolResult{Content: "tool output"}}
	ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	it := ag.Run(context.Background(), "d", "use the tool")

	select {
	case <-secondCallStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second completion to start")
	}

	d, ok := store.getDialogue("d")
	require.True(t, ok, "dialogue should be checkpointed before the next completion starts")
	require.Len(t, d.Messages, 4)
	assert.Equal(t, llm.RoleSystem, d.Messages[0].Role)
	assert.Equal(t, llm.RoleUser, d.Messages[1].Role)
	assert.Equal(t, llm.RoleAssistant, d.Messages[2].Role)
	require.Len(t, d.Messages[2].ToolCalls, 1)
	assert.Equal(t, "c1", d.Messages[2].ToolCalls[0].ID)
	assert.Equal(t, llm.RoleTool, d.Messages[3].Role)
	assert.Equal(t, "c1", d.Messages[3].ToolCallID)
	assert.Equal(t, "tool output", d.Messages[3].Content)

	close(allowSecondCall)
	_ = collectEventsWithTimeout(t, it, 5*time.Second)
}

func TestRegistry(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		r := NewRegistry()
		assert.Empty(t, r.AllTools())
		_, ok := r.Get("anything", "")
		assert.False(t, ok)
	})

	t.Run("get registered tool", func(t *testing.T) {
		tool := &mockTool{name: "my_tool", result: ToolResult{Content: "ok"}}
		r := NewRegistry(tool)

		got, ok := r.Get("my_tool", "")
		assert.True(t, ok)
		assert.Equal(t, tool, got)

		tools := r.AllTools()
		assert.Len(t, tools, 1)
		assert.Equal(t, "my_tool", tools[0].Function.Name)
	})

	t.Run("multiple tools", func(t *testing.T) {
		t1 := &mockTool{name: "a"}
		t2 := &mockTool{name: "b"}
		t3 := &mockTool{name: "c"}
		r := NewRegistry(t1, t2, t3)

		assert.Len(t, r.AllTools(), 3)
		for _, name := range []string{"a", "b", "c"} {
			_, ok := r.Get(name, "")
			assert.True(t, ok, "should find tool %q", name)
		}
	})
}

type mockService struct {
	mu        sync.Mutex
	callCount int
	responses []mockResponse
	// captured requests for assertion
	requests []llm.Request

	// Optional overrides for token counting / context window.
	countTokensFn  func([]llm.Message) (int, error)
	contextWindowN int

	// beforeCompletion is called just before processing the mock
	// response, allowing tests to cancel contexts etc.
	beforeCompletion func()
}

type mockResponse struct {
	chunks            []string
	reasoningChunks   []string
	finishReason      llm.FinishReason
	toolCalls         []llm.ToolCall
	usage             llm.Usage
	err               error
	streamErr         error                // error to return from iterator.Err() after consuming
	rateLimitWarnings []*llm.RateLimitInfo // warnings to emit before text deltas
}

func (m *mockService) CreateCompletion(
	ctx context.Context, req llm.Request,
) (iterator.Iterator[llm.Event], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if m.beforeCompletion != nil {
		m.beforeCompletion()
	}

	// Re-check after hook — the hook may have cancelled the context.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	idx := m.callCount
	m.callCount++
	m.requests = append(m.requests, req)
	m.mu.Unlock()

	if idx >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}

	resp := m.responses[idx]
	if resp.err != nil {
		return nil, resp.err
	}

	var items []llm.Event

	// Emit rate limit warnings (before text/reasoning).
	for _, rl := range resp.rateLimitWarnings {
		items = append(items, llm.Event{
			Type:      llm.EventRateLimitWarning,
			RateLimit: rl,
		})
	}

	// Emit reasoning deltas.
	for i, chunk := range resp.reasoningChunks {
		if i < len(resp.chunks) {
			// Paired with a text chunk — will be emitted below.
			continue
		}
		items = append(items, llm.Event{Type: llm.EventReasoningDelta, Reasoning: chunk})
	}

	// Emit text deltas, interleaving any paired reasoning deltas.
	for i, chunk := range resp.chunks {
		if i < len(resp.reasoningChunks) {
			items = append(items, llm.Event{Type: llm.EventReasoningDelta, Reasoning: resp.reasoningChunks[i]})
		}
		if chunk != "" {
			items = append(items, llm.Event{Type: llm.EventTextDelta, Text: chunk})
		}
	}

	// Emit individual tool call done events.
	for i := range resp.toolCalls {
		tc := resp.toolCalls[i]
		items = append(items, llm.Event{Type: llm.EventToolCallDone, ToolCall: &tc})
	}

	// Build the final assistant message for DoneData.
	content := strings.Join(resp.chunks, "")
	reasoning := strings.Join(resp.reasoningChunks, "")
	msg := llm.Message{
		Role:             llm.RoleAssistant,
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        resp.toolCalls,
	}

	items = append(items, llm.Event{
		Type: llm.EventStreamDone,
		DoneData: &llm.DoneData{
			Message:      msg,
			FinishReason: resp.finishReason,
			Usage:        resp.usage,
		},
	})

	if resp.streamErr != nil {
		return &errorAfterIterator{
			inner: iterator.FromSlice(items),
			err:   resp.streamErr,
		}, nil
	}

	return iterator.FromSlice(items), nil
}

func (m *mockService) CountTokens(msgs []llm.Message) (int, error) {
	if m.countTokensFn != nil {
		return m.countTokensFn(msgs)
	}
	return 0, nil
}

func (m *mockService) ContextWindow() int {
	if m.contextWindowN > 0 {
		return m.contextWindowN
	}
	return math.MaxInt
}

func (m *mockService) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// errorAfterIterator wraps an iterator and returns an error after exhaustion.
type errorAfterIterator struct {
	inner iterator.Iterator[llm.Event]
	err   error
}

func (e *errorAfterIterator) Next(ctx context.Context) (llm.Event, bool) {
	return e.inner.Next(ctx)
}

func (e *errorAfterIterator) Err() error {
	if err := e.inner.Err(); err != nil {
		return err
	}
	return e.err
}

func (e *errorAfterIterator) Close() error {
	return e.inner.Close()
}

type mockStore struct {
	mu        sync.Mutex
	data      map[string]dialoguemanager.Dialogue
	created   []string
	appended  []string
	getErr    error // injected error for Get
	createErr error // injected error for Create (non-AlreadyExists)
	appendErr error // injected error for AppendMessages
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]dialoguemanager.Dialogue)}
}

func noSkills() *skills.SkillRegistry {
	return skills.NewRegistry(nopFileSystem{}, dirURI(""), nil, nil)
}

func (s *mockStore) Health(ctx context.Context) error { return nil }

func (s *mockStore) Create(ctx context.Context, d dialoguemanager.Dialogue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	if _, ok := s.data[d.ID]; ok {
		return storageapi.ErrAlreadyExists
	}
	s.data[d.ID] = d
	s.created = append(s.created, d.ID)
	return nil
}

func (s *mockStore) Get(ctx context.Context, id string) (dialoguemanager.Dialogue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return dialoguemanager.Dialogue{}, s.getErr
	}
	d, ok := s.data[id]
	if !ok {
		return dialoguemanager.Dialogue{}, storageapi.ErrNotFound
	}
	return d, nil
}

func (s *mockStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

func (s *mockStore) AppendMessages(ctx context.Context, d dialoguemanager.Dialogue, msgs []llm.Message, usage llm.DialogueUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.appendErr != nil {
		return s.appendErr
	}
	existing := s.data[d.ID]
	if existing.ApprovedPlan == nil {
		existing.ApprovedPlan = d.ApprovedPlan
	}
	existing.Messages = append(existing.Messages, msgs...)
	existing.Usage.TokensSent += usage.TokensSent
	existing.Usage.TokensReceived += usage.TokensReceived
	existing.Usage.TokensReasoned += usage.TokensReasoned
	existing.Usage.TokensCached += usage.TokensCached
	existing.Usage.Completions += usage.Completions
	existing.Usage.ToolCalls += usage.ToolCalls
	existing.Usage.TotalDuration += usage.TotalDuration
	existing.Usage.InferenceDuration += usage.InferenceDuration
	existing.Usage.ToolCallDuration += usage.ToolCallDuration
	s.data[d.ID] = existing
	s.appended = append(s.appended, d.ID)
	return nil
}

func (s *mockStore) List(ctx context.Context) (iterator.Iterator[dialoguemanager.DialogueHeader], error) {
	return iterator.FromSlice[dialoguemanager.DialogueHeader](nil), nil
}

func (s *mockStore) ArchiveAndReplace(_ context.Context, p dialoguemanager.ArchiveAndReplaceParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	archived := p.Dialogue
	archived.ID = p.ArchivedDialogueID
	s.data[p.ArchivedDialogueID] = archived
	replaced := p.Dialogue
	replaced.Messages = p.Messages
	replaced.ApprovedPlan = p.ApprovedPlan
	replaced.MessageCount = len(p.Messages)
	s.data[p.Dialogue.ID] = replaced
	return nil
}

func (s *mockStore) getDialogue(id string) (dialoguemanager.Dialogue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[id]
	return d, ok
}

type mockTool struct {
	name      string
	result    ToolResult
	summary   string
	execCount atomic.Int32
	executeFn func(ctx context.Context, arguments string) ToolResult
}

func (t *mockTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        t.name,
			Description: "mock tool",
			Parameters:  map[string]any{"type": "object"},
		},
	}
}

func (t *mockTool) Execute(ctx context.Context, arguments string) ToolResult {
	t.execCount.Add(1)
	if t.executeFn != nil {
		return t.executeFn(ctx, arguments)
	}
	return t.result
}

func (t *mockTool) Summary(_ string) string {
	return t.summary
}

func collectEvents(t *testing.T, it iterator.Iterator[Event]) []Event {
	t.Helper()
	ctx := context.Background()
	var events []Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		events = append(events, ev)
	}
	return events
}

func collectEventsWithTimeout(t *testing.T, it iterator.Iterator[Event], timeout time.Duration) []Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var events []Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		events = append(events, ev)
	}
	return events
}

func eventsByType(events []Event, typ EventType) []Event {
	var ret []Event
	for _, ev := range events {
		if ev.Type == typ {
			ret = append(ret, ev)
		}
	}
	return ret
}

func hasEventType(events []Event, typ EventType) bool {
	return len(eventsByType(events, typ)) > 0
}

func toolCallResponse(toolName, args, callID string) mockResponse {
	return mockResponse{
		chunks:       []string{""},
		finishReason: llm.FinishReasonToolCall,
		toolCalls: []llm.ToolCall{
			{
				ID:       callID,
				Type:     llm.ToolTypeFunction,
				Function: llm.FunctionCall{Name: toolName, Arguments: args},
			},
		},
	}
}

func TestStreamReset(t *testing.T) {
	t.Run("agent resets builders and emits EventDone on EventStreamReset", func(t *testing.T) {
		// Simulate a stream that delivers partial text, then resets (mid-stream
		// retry), then delivers the complete response.
		streamEvents := []llm.Event{
			{Type: llm.EventTextDelta, Text: "partial "},
			{Type: llm.EventReasoningDelta, Reasoning: "thinking..."},
			{Type: llm.EventStreamReset},
			{Type: llm.EventRateLimitWarning, RateLimit: &llm.RateLimitInfo{
				WaitDuration: time.Second,
				Message:      "Connection lost (connection reset by peer), retrying in 1s (attempt 1/3).",
			}},
			{Type: llm.EventTextDelta, Text: "complete answer"},
			{Type: llm.EventStreamDone, DoneData: &llm.DoneData{
				Message:      llm.Message{Role: llm.RoleAssistant, Content: "complete answer"},
				FinishReason: llm.FinishReasonStop,
			}},
		}

		svc := &streamResetMockService{events: streamEvents}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{
			SystemPrompt: "test",
		})

		events := collectEvents(t, ag.Run(context.Background(), "d1", "hello"))

		// Verify the event sequence contains:
		// 1. EventText "partial " (from before reset)
		// 2. EventReasoning "thinking..." (from before reset)
		// 3. EventDone (emitted by agent on reset)
		// 4. EventRateLimitWarning
		// 5. EventText "complete answer" (from retried stream)
		// 6. EventDone (final)

		var textEvents []string
		var doneCount int
		var warningCount int
		for _, ev := range events {
			switch ev.Type {
			case EventText:
				textEvents = append(textEvents, ev.Text)
			case EventDone:
				doneCount++
			case EventRateLimitWarning:
				warningCount++
				assert.Contains(t, ev.RateLimit.Message, "Connection lost")
			}
		}

		assert.Equal(t, []string{"partial ", "complete answer"}, textEvents)
		assert.Equal(t, 2, doneCount, "should have 2 done events: one for reset, one for completion")
		assert.Equal(t, 1, warningCount, "should have 1 rate limit warning")

		// Verify persisted message uses the final content (after reset), not partial.
		d, ok := store.getDialogue("d1")
		require.True(t, ok)
		// Last message should be the assistant message with "complete answer".
		lastMsg := d.Messages[len(d.Messages)-1]
		assert.Equal(t, llm.RoleAssistant, lastMsg.Role)
		assert.Equal(t, "complete answer", lastMsg.Content)
	})
}

// streamResetMockService returns a pre-built event sequence that includes
// EventStreamReset for testing mid-stream retry handling in the agent.
type streamResetMockService struct {
	events []llm.Event
}

func (m *streamResetMockService) CreateCompletion(
	_ context.Context, _ llm.Request,
) (iterator.Iterator[llm.Event], error) {
	return iterator.FromSlice(m.events), nil
}

func (m *streamResetMockService) CountTokens(_ []llm.Message) (int, error) {
	return 0, nil
}

func (m *streamResetMockService) ContextWindow() int {
	return 1_000_000
}

func stopResponse(chunks ...string) mockResponse {
	return mockResponse{
		chunks:       chunks,
		finishReason: llm.FinishReasonStop,
	}
}

func stopResponseWithReasoning(reasoning []string, chunks ...string) mockResponse {
	return mockResponse{
		chunks:          chunks,
		reasoningChunks: reasoning,
		finishReason:    llm.FinishReasonStop,
	}
}

func TestAgentRun_MultiContentInjectsSyntheticUserMessage(t *testing.T) {
	imageParts := []llm.ContentPart{
		{Type: llm.ContentPartTypeText, Text: "Read image file: photo.png"},
		{Type: llm.ContentPartTypeImageURL, ImageURL: "data:image/png;base64,AAAA"},
	}
	svc := &mockService{
		responses: []mockResponse{
			toolCallResponse("read_file", `{"path":"photo.png"}`, "call_1"),
			stopResponse("I can see the image!"),
		},
	}
	store := newMockStore()
	tool := &mockTool{
		name: "read_file",
		result: ToolResult{
			Content:      "Read image file: photo.png (100 bytes, image/png)",
			MultiContent: imageParts,
		},
	}
	ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	events := collectEvents(t, ag.Run(context.Background(), "d", "describe this image"))
	assert.True(t, hasEventType(events, EventDone))

	// The second LLM call should contain a synthetic user message with MultiContent.
	require.Equal(t, 2, svc.getCallCount())
	secondReq := svc.requests[1]
	var foundImageMsg bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleUser && len(msg.MultiContent) > 0 {
			foundImageMsg = true
			assert.Equal(t, imageParts, msg.MultiContent)
		}
	}
	assert.True(t, foundImageMsg, "second request should contain synthetic user message with image MultiContent")

	// Verify the synthetic user message is also persisted.
	d, ok := store.getDialogue("d")
	require.True(t, ok)
	var persistedImageMsg bool
	for _, msg := range d.Messages {
		if msg.Role == llm.RoleUser && len(msg.MultiContent) > 0 {
			persistedImageMsg = true
		}
	}
	assert.True(t, persistedImageMsg, "synthetic user message should be persisted")
}

func TestSwapService(t *testing.T) {
	t.Run("swaps service and model for next run", func(t *testing.T) {
		svc1 := &mockService{responses: []mockResponse{stopResponse("from svc1")}}
		svc2 := &mockService{responses: []mockResponse{stopResponse("from svc2")}}
		store := newMockStore()
		ag := NewAgent(svc1, NewRegistry(), noSkills(), store, NoMemory(), Config{
			SystemPrompt: "test",
			Model:        "model-a",
		})

		// First run uses svc1.
		events := collectEvents(t, ag.Run(context.Background(), "d1", "hello"))
		assert.True(t, hasEventType(events, EventDone))
		assert.Equal(t, 1, svc1.getCallCount())
		assert.Equal(t, 0, svc2.getCallCount())

		d1, ok := store.getDialogue("d1")
		require.True(t, ok)
		assert.Equal(t, "model-a", d1.Model)

		// Swap to svc2 / model-b.
		ag.SwapService(svc2, "model-b", "anthropic")
		assert.Equal(t, "model-b", ag.Model())

		// Second run uses svc2.
		events = collectEvents(t, ag.Run(context.Background(), "d2", "world"))
		assert.True(t, hasEventType(events, EventDone))
		assert.Equal(t, 1, svc1.getCallCount())
		assert.Equal(t, 1, svc2.getCallCount())

		d2, ok := store.getDialogue("d2")
		require.True(t, ok)
		assert.Equal(t, "model-b", d2.Model)
	})
}

func TestAutoCompact(t *testing.T) {
	t.Run("auto-compacts when usage exceeds threshold", func(t *testing.T) {
		countCalls := 0
		svc := &mockService{
			responses: []mockResponse{
				// 1st call: summarize (triggered by auto-compact)
				stopResponse("Summary of conversation"),
				// 2nd call: post-compaction normal response
				stopResponse("Continuing after compaction"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				countCalls++
				if countCalls <= 1 {
					// First count: 90% usage triggers auto-compact.
					return 900, nil
				}
				// After compaction, usage is low.
				return 100, nil
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test system prompt"})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		// 2 LLM calls: summarize + post-compact
		require.Equal(t, 2, svc.getCallCount())

		// Events include compacting and compacted signals.
		assert.True(t, hasEventType(events, EventCompacting))
		assert.True(t, hasEventType(events, EventCompacted))
		assert.True(t, hasEventType(events, EventDone))

		// Archived dialogue exists.
		_, ok := store.getDialogue("d-archived")
		assert.True(t, ok)
	})

	t.Run("no transient hint below auto-compact threshold", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				stopResponse("Done"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				// 80% usage: above old hint threshold but below auto-compact
				return 800, nil
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "test message")
		events := collectEvents(t, it)

		// Should not auto-compact.
		assert.False(t, hasEventType(events, EventCompacting))

		// No transient context hint — compaction guidance is static
		// in the system prompt to avoid busting the prompt cache.
		require.Equal(t, 1, svc.getCallCount())
		for _, msg := range svc.requests[0].Messages {
			if msg.Role == llm.RoleSystem {
				assert.NotContains(t, msg.Content, "tokens)",
					"no transient context hint should be injected")
			}
		}
	})

	t.Run("auto-compact failure degrades gracefully", func(t *testing.T) {
		callCount := 0
		svc := &mockService{
			responses: []mockResponse{
				// 1st call: summarize fails (returned as stream error)
				{chunks: []string{""}, finishReason: llm.FinishReasonStop,
					streamErr: errors.New("summarize failed")},
				// 2nd call: normal response (after fallthrough)
				stopResponse("Kept going despite compact failure"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				callCount++
				if callCount <= 1 {
					// First count: above threshold → triggers auto-compact
					return 900, nil
				}
				// After failed compact, report lower usage so we don't
				// trigger auto-compact again (simulates no change).
				return 100, nil
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "test"})

		it := ag.Run(context.Background(), "d", "test message")
		events := collectEvents(t, it)

		// Should still complete with a response.
		assert.True(t, hasEventType(events, EventDone))
		textEvents := eventsByType(events, EventText)
		require.NotEmpty(t, textEvents)
		assert.Equal(t, "Kept going despite compact failure", textEvents[0].Text)
	})

	t.Run("auto-compact with project instructions does not panic", func(t *testing.T) {
		// Regression test: after auto-compaction, userMsgIdx pointed past
		// the end of the (now shorter) messages slice, causing a panic
		// when project instructions were injected at that stale index.
		// The bug requires prior conversation history so userMsgIdx is high:
		//   messages = [system, user1, asst1, user2, asst2, userMsg] → userMsgIdx=5
		// After compact:
		//   messages = [system, summary] → len=2
		// but userMsgIdx is still 5, causing out-of-bounds access.
		countCalls := 0
		svc := &mockService{
			responses: []mockResponse{
				// 1st call: summarize (triggered by auto-compact)
				stopResponse("Summary of conversation"),
				// 2nd call: post-compaction normal response
				stopResponse("Continuing after compaction"),
			},
			contextWindowN: 1000,
			countTokensFn: func(msgs []llm.Message) (int, error) {
				countCalls++
				if countCalls <= 1 {
					// First count: 90% usage triggers auto-compact.
					return 900, nil
				}
				// After compaction, usage is low.
				return 100, nil
			},
		}
		store := newMockStore()
		// Pre-seed the dialogue with conversation history so userMsgIdx > 1.
		err := store.Create(context.Background(), dialoguemanager.Dialogue{
			ID: "d",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "test system prompt"},
				{Role: llm.RoleUser, Content: "first question"},
				{Role: llm.RoleAssistant, Content: "first answer"},
				{Role: llm.RoleUser, Content: "second question"},
				{Role: llm.RoleAssistant, Content: "second answer"},
			},
		})
		require.NoError(t, err)
		ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(),
			Config{
				SystemPrompt:        "test system prompt",
				ProjectInstructions: "You are working on project X.",
			})

		it := ag.Run(context.Background(), "d", "do stuff")
		events := collectEvents(t, it)

		// Must not panic and must complete successfully.
		assert.True(t, hasEventType(events, EventCompacting))
		assert.True(t, hasEventType(events, EventCompacted))
		assert.True(t, hasEventType(events, EventDone))

		// Post-compact call should have project instructions injected
		// into the user message (the summary).
		require.GreaterOrEqual(t, svc.getCallCount(), 2)
		postCompactMsgs := svc.requests[1].Messages
		var foundProjectInstructions bool
		for _, msg := range postCompactMsgs {
			if msg.Role == llm.RoleUser && contains(msg.Content, "project-instructions") {
				foundProjectInstructions = true
			}
		}
		assert.True(t, foundProjectInstructions,
			"post-compact user message should contain project instructions")
	})
}

func TestSummarizeEmptySummaryReturnsError(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
	}{
		{"empty string", []string{""}},
		{"whitespace only", []string{"  ", "\t", "  "}},
		{"newlines only", []string{"\n", "\n\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockService{
				responses: []mockResponse{stopResponse(tt.chunks...)},
			}
			msgs := []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "hello"},
				{Role: llm.RoleAssistant, Content: "hi there"},
			}
			_, err := Summarize(context.Background(), svc, msgs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "empty summary")
		})
	}
}

func TestCompactDialogueNormalizesMessagesBeforeSummarizing(t *testing.T) {
	t.Parallel()

	svc := &mockService{
		responses: []mockResponse{stopResponse("Summary of work so far")},
	}
	store := newMockStore()
	d := dialoguemanager.Dialogue{
		ID: "d",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "sys"},
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "", ToolCalls: []llm.ToolCall{{
				ID:       "orphaned_1",
				Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`},
			}}},
			{Role: llm.RoleUser, Content: "try again"},
		},
	}

	compacted, archivedID, err := CompactDialogue(context.Background(), svc, store, d)
	require.NoError(t, err)
	assert.Equal(t, ArchivedID("d"), archivedID)

	require.Equal(t, 1, svc.getCallCount())
	req := svc.requests[0]
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleUser, Content: "try again"},
		{Role: llm.RoleUser, Content: SummarizePrompt},
	}, req.Messages)

	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: "sys"},
		{Role: llm.RoleUser, Content: CompactSummaryPrefix + "Summary of work so far"},
	}, compacted)

	archived, ok := store.getDialogue(archivedID)
	require.True(t, ok)
	assert.Equal(t, d.Messages, archived.Messages)

	current, ok := store.getDialogue("d")
	require.True(t, ok)
	assert.Equal(t, compacted, current.Messages)
}

func TestCleanSummary(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "plain text passthrough",
			raw:  "Summary of work so far",
			want: "Summary of work so far",
		},
		{
			name: "analysis and summary tags",
			raw:  "<analysis>\nthinking about the task...\n</analysis>\n\n<summary>\n1. Primary Request\n2. Key Concepts\n</summary>",
			want: "Summary:\n1. Primary Request\n2. Key Concepts",
		},
		{
			name: "summary only no analysis",
			raw:  "<summary>\nContent here\n</summary>",
			want: "Summary:\nContent here",
		},
		{
			name: "analysis only no summary",
			raw:  "<analysis>\nthinking\n</analysis>\nSome leftover text",
			want: "Some leftover text",
		},
		{
			name: "multiple blank lines collapsed",
			raw:  "Line 1\n\n\n\nLine 2",
			want: "Line 1\nLine 2",
		},
		{
			name: "whitespace trimmed",
			raw:  "  \n Summary text \n  ",
			want: "Summary text",
		},
		{
			name: "empty after strip",
			raw:  "<analysis>\nthinking\n</analysis>\n\n",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanSummary(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptCacheKey(t *testing.T) {
	t.Run("sets PromptCacheKey to dialogue ID on every request", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("my_tool", "{}", "c1"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(&mockTool{
			name:   "my_tool",
			result: ToolResult{Content: "ok"},
		}), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		collectEvents(t, ag.Run(context.Background(), "my-dialogue", "go"))
		require.Equal(t, 2, svc.getCallCount())

		// Every request must carry the dialogue ID as cache key.
		assert.Equal(t, "my-dialogue", svc.requests[0].PromptCacheKey)
		assert.Equal(t, "my-dialogue", svc.requests[1].PromptCacheKey)
	})

	t.Run("sends full messages on every iteration", func(t *testing.T) {
		svc := &mockService{
			responses: []mockResponse{
				toolCallResponse("my_tool", "{}", "c1"),
				stopResponse("done"),
			},
		}
		store := newMockStore()
		ag := NewAgent(svc, NewRegistry(&mockTool{
			name:   "my_tool",
			result: ToolResult{Content: "ok"},
		}), noSkills(), store, NoMemory(),
			Config{SystemPrompt: "sys"})

		collectEvents(t, ag.Run(context.Background(), "d", "go"))
		require.Equal(t, 2, svc.getCallCount())

		// Second request must include the full conversation
		// (user, assistant, tool) — no delta optimization.
		req2 := svc.requests[1]
		var hasUser, hasAssistant, hasTool bool
		for _, msg := range req2.Messages {
			switch msg.Role {
			case llm.RoleUser:
				hasUser = true
			case llm.RoleAssistant:
				hasAssistant = true
			case llm.RoleTool:
				hasTool = true
			}
		}
		assert.True(t, hasUser)
		assert.True(t, hasAssistant)
		assert.True(t, hasTool)
	})
}

func TestContextSnapshotShowsCurrentNotCumulative(t *testing.T) {
	// Regression: ContextSnapshot.TokensSent used to accumulate across all
	// completions (cumulative), inflating the displayed context usage
	// (e.g. 8m tokens / 3834%). It should reflect the current completion's
	// sent token count, not the sum across every iteration.

	store := newMockStore()
	tool := &mockTool{name: "my_tool", result: ToolResult{Content: "ok"}}

	svc := &mockService{
		contextWindowN: 200_000,
		responses: []mockResponse{
			{
				chunks:       []string{""},
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:       "call_1",
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "my_tool", Arguments: `{}`},
				}},
				usage: llm.Usage{TokensSent: 50_000, TokensReceived: 500},
			},
			{
				chunks:       []string{"done"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 55_000, TokensReceived: 600},
			},
		},
	}

	ag := NewAgent(svc, NewRegistry(tool), noSkills(), store, NoMemory(),
		Config{SystemPrompt: "test"})

	events := collectEvents(t, ag.Run(context.Background(), "d", "go"))

	// The EventUsageUpdate after the first completion (tool call iteration).
	usageUpdates := eventsByType(events, EventUsageUpdate)
	require.NotEmpty(t, usageUpdates, "expected at least one EventUsageUpdate")
	first := usageUpdates[0]
	assert.Equal(t, 50_000, first.Context.TokensSent,
		"ContextSnapshot.TokensSent should be the current completion's sent tokens, not cumulative")
	assert.Equal(t, 500, first.Context.TokensReceived,
		"ContextSnapshot.TokensReceived should be the current completion's received tokens")

	// Cumulative usage in Usage field should still accumulate.
	assert.Equal(t, 50_000, first.Usage.TokensSent,
		"Usage.TokensSent should accumulate (first iteration only)")

	// The EventDone at the end should also use per-completion values.
	doneEvents := eventsByType(events, EventDone)
	require.NotEmpty(t, doneEvents)
	last := doneEvents[len(doneEvents)-1]
	assert.Equal(t, 55_000, last.Context.TokensSent,
		"final ContextSnapshot.TokensSent should be the last completion's sent tokens")
	assert.Equal(t, 600, last.Context.TokensReceived,
		"final ContextSnapshot.TokensReceived should be the last completion's received tokens")
}

// staticMemory is a MemoryRecaller that always returns the same memories.
type staticMemory struct {
	memories []Memory
}

func (m staticMemory) Recall(_ context.Context, _ []string, _ string) ([]Memory, error) {
	return m.memories, nil
}

func TestCachePrefixStability(t *testing.T) {
	// Simulates a realistic 4-iteration coding session:
	//   0: LLM calls read_file
	//   1: LLM calls apply_patch
	//   2: LLM calls bash (run tests)
	//   3: LLM responds "done"
	//
	// We capture every request and verify the message prefix is
	// strictly append-only — any prefix mutation between consecutive
	// requests means the prompt cache will miss.

	svc := &mockService{
		responses: []mockResponse{
			toolCallResponse("read_file", `{"path":"main.go"}`, "c1"),
			toolCallResponse("apply_patch", `{"patch":"..."}`, "c2"),
			toolCallResponse("bash", `{"cmd":"go test"}`, "c3"),
			stopResponse("All tests pass."),
		},
	}
	store := newMockStore()
	readTool := &mockTool{name: "read_file", result: ToolResult{Content: "package main\n"}}
	patchTool := &mockTool{name: "apply_patch", result: ToolResult{Content: "applied 1/1"}}
	bashTool := &mockTool{name: "bash", result: ToolResult{Content: "PASS"}}

	mem := staticMemory{memories: []Memory{
		{ID: "pref-1", Content: "User prefers concise code."},
	}}

	ag := NewAgent(svc, NewRegistry(readTool, patchTool, bashTool),
		noSkills(), store, mem,
		Config{SystemPrompt: "You are a coding assistant."})

	it := ag.Run(context.Background(), "d", "Fix the bug in main.go")
	events := collectEvents(t, it)
	require.True(t, hasEventType(events, EventDone))
	require.Equal(t, 4, svc.getCallCount(), "expected 4 LLM requests")

	// For each consecutive pair of requests, the earlier request's
	// messages must be an exact prefix of the later request's messages.
	// Any divergence means the prompt cache will miss.
	for i := 0; i+1 < len(svc.requests); i++ {
		prev := svc.requests[i].Messages
		curr := svc.requests[i+1].Messages

		if len(curr) < len(prev) {
			t.Errorf("request %d→%d: message count shrank (%d → %d); prefix destroyed",
				i, i+1, len(prev), len(curr))
			continue
		}

		// The previous request's messages must be a prefix of the
		// current request's messages (same role and content at each index).
		for j := range prev {
			if j >= len(curr) {
				break
			}
			if prev[j].Role != curr[j].Role {
				t.Errorf("request %d→%d: message[%d] role changed: %q → %q",
					i, i+1, j, prev[j].Role, curr[j].Role)
			}
			if prev[j].Content != curr[j].Content {
				// Truncate for readability.
				pc := prev[j].Content
				cc := curr[j].Content
				if len(pc) > 80 {
					pc = pc[:80] + "..."
				}
				if len(cc) > 80 {
					cc = cc[:80] + "..."
				}
				t.Errorf("request %d→%d: message[%d] (role=%s) content changed:\n  was:  %q\n  now:  %q",
					i, i+1, j, prev[j].Role, pc, cc)
			}
		}
	}

	// Log all requests for visibility.
	for i, req := range svc.requests {
		t.Logf("=== Request %d (%d messages) ===", i, len(req.Messages))
		for j, msg := range req.Messages {
			c := msg.Content
			if len(c) > 100 {
				c = c[:100] + "..."
			}
			t.Logf("  [%d] role=%-9s toolCallID=%-6s content=%q tcs=%d",
				j, msg.Role, msg.ToolCallID, c, len(msg.ToolCalls))
		}
	}
}

func TestNormalizeMessages(t *testing.T) {
	t.Run("drops empty assistant messages after stripping orphaned tool calls", func(t *testing.T) {
		// Simulates a model switch scenario: an assistant message has tool calls
		// but no text content (common for OpenAI). When tool calls are orphaned
		// (no matching tool results), stripping them leaves a completely empty
		// assistant message that would cause Anthropic's API to reject with
		// "text content blocks must be non-empty".
		msgs := normalizeMessages([]llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "", ToolCalls: []llm.ToolCall{
				{ID: "orphaned_1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}},
			}},
			{Role: llm.RoleUser, Content: "try again"},
		})
		// The assistant message should be dropped entirely.
		require.Len(t, msgs, 2)
		assert.Equal(t, llm.RoleUser, msgs[0].Role)
		assert.Equal(t, "hello", msgs[0].Content)
		assert.Equal(t, llm.RoleUser, msgs[1].Role)
		assert.Equal(t, "try again", msgs[1].Content)
	})

	t.Run("keeps assistant messages with content after stripping tool calls", func(t *testing.T) {
		msgs := normalizeMessages([]llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "Sure, let me check.", ToolCalls: []llm.ToolCall{
				{ID: "orphaned_1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}},
			}},
			{Role: llm.RoleUser, Content: "thanks"},
		})
		require.Len(t, msgs, 3)
		assert.Equal(t, llm.RoleAssistant, msgs[1].Role)
		assert.Equal(t, "Sure, let me check.", msgs[1].Content)
		assert.Empty(t, msgs[1].ToolCalls)
	})
}

func TestAgentRun_ContinueAfterReasoningOnlyTruncatedTurnWithRealStore(t *testing.T) {
	t.Parallel()
	const partialReasoning = "Let me analyze this step by step..."

	svc := &mockService{
		responses: []mockResponse{
			{
				reasoningChunks: []string{partialReasoning},
				finishReason:    llm.FinishReasonLength,
			},
			stopResponse("...doing X and Y."),
		},
	}

	store := dialoguemanager.NewStore(storagestub.NewInMemoryService(), t.TempDir())
	ag := NewAgent(svc, NewRegistry(), noSkills(), store, NoMemory(), Config{SystemPrompt: "test"})

	_ = collectEventsWithTimeout(t, ag.Run(context.Background(), "d", "explain this"), 5*time.Second)
	_ = collectEventsWithTimeout(t, ag.Run(context.Background(), "d", "please continue"), 5*time.Second)

	require.Equal(t, 2, svc.getCallCount(), "expected 2 LLM requests across both turns")
	secondReq := svc.requests[1]
	var foundPartialAssistant bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleAssistant && msg.Content == partialReasoning {
			foundPartialAssistant = true
			break
		}
	}
	assert.True(t, foundPartialAssistant,
		"second LLM request must contain the partial assistant message with reasoning as content")
}
