// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/anthropic"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	llmopenai "unstable.build/go-tui/cmd/rune-agent/llm/openai"
	runemcp "unstable.build/go-tui/cmd/rune-agent/mcp"
)

// builtinSkillsSystemMsg is the system message injected by the agent loop
// when the skill registry contains the 2 builtin skills (explore + plan).
var builtinSkillsSystemMsg = llm.Message{
	Role: llm.RoleSystem,
	Content: "<system-reminder>\n" +
		"The following skills are available for use with the skill tool:\n" +
		"\n\nAgent skills (spawn a sub-agent to perform the task):\n" +
		"\n- explore (agent): Fast, read-only research agent for exploring codebases. " +
		"Spawns a sub-agent that searches code, follows references, and reports findings without modifying files." +
		"\n- plan (agent): Read-only software architect agent for designing implementation plans. " +
		"Analyzes requirements, explores the codebase, and produces step-by-step plans. " +
		"Uses exit_plan_mode for user approval and plan persistence, then returns the plan for the parent agent to execute." +
		"\n</system-reminder>",
}

// testSystemPromptWithAddendum is the expected system prompt content for
// tests that use the "openai" provider. It includes the provider-specific
// tool selection addendum.
var testSystemPromptWithAddendum = "test system prompt" + agent.ProviderToolAddendum("openai")

func TestMakeOnCompactedCallsCompactFnWithSummaryPrefix(t *testing.T) {
	t.Parallel()
	// makeOnCompacted must invoke compactFn even when the compacted
	// dialogue contains a CompactSummaryPrefix message — this is the
	// normal auto-compaction case.
	compactedMsgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: agent.CompactSummaryPrefix + "Summary of work so far"},
	}
	store := &mockStore{dialogues: map[string]dialoguemanager.Dialogue{
		"sess-1": {ID: "sess-1", Messages: compactedMsgs},
	}}

	var gotMsgs []llm.Message
	onCompacted := makeOnCompacted(store, func(msgs []llm.Message) {
		gotMsgs = msgs
	})

	onCompacted("sess-1")

	assert.Equal(t, compactedMsgs, gotMsgs,
		"compactFn must be called even when compacted messages contain CompactSummaryPrefix")
}

func TestMakeOnCompactedStoreErrorSkipsCompactFn(t *testing.T) {
	t.Parallel()
	// When the store returns an error, compactFn must not be called.
	store := &mockStore{dialogues: map[string]dialoguemanager.Dialogue{}}

	called := false
	onCompacted := makeOnCompacted(store, func(_ []llm.Message) {
		called = true
	})

	onCompacted("nonexistent")

	assert.False(t, called, "compactFn must not be called when store.Get fails")
}

func TestGetModelURI(t *testing.T) {
	t.Parallel()
	suite := []struct {
		modelIn     string
		idIn        string
		expectedOut string
		expectedErr string
	}{
		{"llama3.2", "1234", "rune-agent://llama3.2/1234", ""},
		{"llama4:scout", "1234", "rune-agent://llama4_scout/1234", ""},
	}

	for _, test := range suite {
		actualOut, err := getModelUri(test.idIn, test.modelIn)
		if test.expectedErr != "" {
			assert.EqualError(t, err, test.expectedErr)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.expectedOut, actualOut.String())
	}
}

func TestAddMessage_ToolMessages(t *testing.T) {
	t.Parallel()
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(20, 12)

	// Simulate a stored dialogue: user → assistant (with tool call) → tool result → assistant
	stored := []llm.Message{
		{Role: llm.RoleUser, Content: "Read it"},
		{
			Role:    llm.RoleAssistant,
			Content: "Let me read.",
			ToolCalls: []llm.ToolCall{
				{
					ID:       "call_1",
					Type:     llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`},
				},
			},
		},
		{Role: llm.RoleTool, Content: "file data", ToolCallID: "call_1"},
		{Role: llm.RoleAssistant, Content: "Got it."},
	}

	pendingTools := make(map[string]llm.ToolCall)
	for _, msg := range stored {
		addMessage(comp, msg, pendingTools)
	}

	w := term.NewStringWriter(21, 13)
	comp.Draw(w)
	err := w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "Read it")
	assert.Contains(t, out, "Let me read.")
	assert.Contains(t, out, "\u2713 read_file")
	assert.Contains(t, out, "file data")
	assert.Contains(t, out, "Got it.")
}

func TestAddMessage_ToolMessageWithoutPendingMap(t *testing.T) {
	t.Parallel()
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(20, 10)

	// When pendingTools is nil, tool messages should still render with generic name
	addMessage(comp, llm.Message{
		Role: llm.RoleTool, Content: "output", ToolCallID: "call_x",
	}, nil)

	w := term.NewStringWriter(21, 11)
	comp.Draw(w)
	err := w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "\u2713 tool")
	assert.Contains(t, out, "output")
}

func TestAddMessage_MultipleToolCalls(t *testing.T) {
	t.Parallel()
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(30, 14)

	stored := []llm.Message{
		{Role: llm.RoleUser, Content: "Read both"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"a"}`}},
				{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"b"}`}},
			},
		},
		{Role: llm.RoleTool, Content: "data_a", ToolCallID: "c1"},
		{Role: llm.RoleTool, Content: "data_b", ToolCallID: "c2"},
		{Role: llm.RoleAssistant, Content: "Done."},
	}

	pendingTools := make(map[string]llm.ToolCall)
	for _, msg := range stored {
		addMessage(comp, msg, pendingTools)
	}
	assert.Empty(t, pendingTools, "all pending tool calls should be consumed")

	w := term.NewStringWriter(31, 15)
	comp.Draw(w)
	err := w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "Read both")
	assert.Contains(t, out, "data_a")
	assert.Contains(t, out, "data_b")
	assert.Contains(t, out, "Done.")
}

func TestAddMessage_AssistantOnlyToolCalls(t *testing.T) {
	t.Parallel()
	// Assistant message with no text content, only tool call metadata
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(20, 10)

	stored := []llm.Message{
		{
			Role: llm.RoleAssistant,
			// Content is empty — LLM returned only tool calls
			ToolCalls: []llm.ToolCall{
				{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "edit_file", Arguments: `{}`}},
			},
		},
		{Role: llm.RoleTool, Content: "ok", ToolCallID: "c1"},
	}

	pendingTools := make(map[string]llm.ToolCall)
	for _, msg := range stored {
		addMessage(comp, msg, pendingTools)
	}

	w := term.NewStringWriter(21, 11)
	comp.Draw(w)
	err := w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "\u2713 edit_file")
	assert.Contains(t, out, "ok")
}

func TestAddMessage_JSONDeserializedMetadata(t *testing.T) {
	t.Parallel()
	// After JSON round-trip, ToolCalls should deserialize correctly
	// Ensure addMessage still handles it correctly
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(20, 10)

	original := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{
			{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}},
		},
	}

	// Round-trip through JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)
	var deserialized llm.Message
	require.NoError(t, json.Unmarshal(data, &deserialized))

	pendingTools := make(map[string]llm.ToolCall)
	addMessage(comp, deserialized, pendingTools)

	require.Len(t, pendingTools, 1)
	call, ok := pendingTools["c1"]
	require.True(t, ok)
	assert.Equal(t, "read_file", call.Function.Name)
}

// recordingHandler records the last HandleCommand call for assertion.
type recordingHandler struct {
	lastCmd  repl.Command
	response string // if empty, returns empty iterator
}

func (r *recordingHandler) HandleCommand(_ context.Context, cmd repl.Command) (
	iterator.Iterator[component.Responsive], error,
) {
	r.lastCmd = cmd
	if r.response == "" {
		return iterator.FromSlice[component.Responsive](nil), nil
	}
	resp := component.NewResponsiveString(r.response, component.StringResponsiveConfig{})
	return iterator.FromSlice([]component.Responsive{resp}), nil
}

func (r *recordingHandler) Complete(context.Context, string, []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// mockWindowManager records Floating calls for testing.
type mockWindowManager struct {
	browserapi.WindowManager
	floatingCalled bool
}

func (m *mockWindowManager) Floating(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error) {
	m.floatingCalled = true
	return nil, nil
}

func (m *mockWindowManager) CloseWindow(browserapi.Window) error { return nil }

func TestCommandAdapterCompactInjectsID(t *testing.T) {
	t.Parallel()
	wm := &mockWindowManager{}
	rec := &recordingHandler{response: "compact output"}
	compactCalled := false
	adapter := &commandAdapter{
		handler:       rec,
		dialogueID:    "sess-1",
		wm:            wm,
		skillRegistry: skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil),
		compactFn:     func([]llm.Message) { compactCalled = true },
	}

	result, err := adapter.HandleCommand(context.Background(), "compact", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Display)

	// The iterator's Next blocks on the handler call then returns false.
	_, ok := result.Display.Next(context.Background())
	assert.False(t, ok)
	assert.NoError(t, result.Display.Err())

	// Verify the handler received the correct command with injected ID.
	assert.Equal(t, repl.Command{Name: "chats", Args: []string{"compact", "sess-1"}}, rec.lastCmd)

	// Close triggers compactFn only if store has the compacted dialogue.
	// store is nil here, so compactFn should NOT be called.
	_ = result.Display.Close()
	assert.False(t, compactCalled)
	assert.False(t, wm.floatingCalled, "compact should not open floating")
}

// mockStore is a minimal dialoguemanager.Store for testing.
type mockStore struct {
	dialoguemanager.Store
	dialogues map[string]dialoguemanager.Dialogue
}

func (m *mockStore) Get(_ context.Context, id string) (dialoguemanager.Dialogue, error) {
	d, ok := m.dialogues[id]
	if !ok {
		return dialoguemanager.Dialogue{}, fmt.Errorf("not found: %s", id)
	}
	return d, nil
}

func TestCommandAdapterCompactHappyPath(t *testing.T) {
	t.Parallel()
	wm := &mockWindowManager{}
	rec := &recordingHandler{response: "compact output"}

	compactedMsgs := []llm.Message{
		{Role: llm.RoleUser, Content: "summary"},
		{Role: llm.RoleAssistant, Content: "ok"},
	}
	store := &mockStore{dialogues: map[string]dialoguemanager.Dialogue{
		"sess-1": {ID: "sess-1", Messages: compactedMsgs},
	}}

	var gotMsgs []llm.Message
	adapter := &commandAdapter{
		handler:       rec,
		dialogueID:    "sess-1",
		wm:            wm,
		store:         store,
		skillRegistry: skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil),
		compactFn: func(msgs []llm.Message) {
			gotMsgs = msgs
		},
	}

	result, err := adapter.HandleCommand(context.Background(), "compact", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Display)

	// Next blocks on the handler call, then returns false.
	_, ok := result.Display.Next(context.Background())
	assert.False(t, ok)
	require.NoError(t, result.Display.Err())

	// Close triggers compactFn with compacted messages.
	_ = result.Display.Close()
	assert.Equal(t, compactedMsgs, gotMsgs)
	assert.False(t, wm.floatingCalled, "compact should not open floating")
}

func TestCompactReplayRendersMarkdown(t *testing.T) {
	t.Parallel()
	// The compacted summary is stored as a RoleUser message with the
	// CompactSummaryPrefix. addMessage detects the prefix and renders
	// it via AddReceiveMessageChunk (markdown) instead of AddSendMessage
	// (plain text).
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(40, 20)

	// Simulate pre-compact state.
	addMessage(comp, llm.Message{Role: llm.RoleUser, Content: "hello"}, nil)
	addMessage(comp, llm.Message{Role: llm.RoleAssistant, Content: "world"}, nil)

	// Simulate compact: reset then replay with CompactDialogue format.
	comp.Reset()

	compactedMsgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.RoleUser, Content: agent.CompactSummaryPrefix + "- item one\n- item two\n\n**bold text**"},
	}
	pending := make(map[string]llm.ToolCall)
	for _, msg := range compactedMsgs {
		addMessage(comp, msg, pending)
	}

	w := term.NewStringWriter(41, 21)
	comp.Draw(w)
	require.NoError(t, w.Flush())

	out := w.String()
	// Markdown bullet list should render with • not raw -.
	assert.Contains(t, out, "•", "bullets should render as • not raw -")
	assert.NotContains(t, out, "- item", "raw markdown bullet should not appear")
}

// --- mock types for createAgentCompletions tests ---

// agentMockService is a minimal llm.Service for testing the agent flow.
type agentMockService struct {
	mu            sync.Mutex
	callCount     int
	responses     []agentMockResponse
	requests      []llm.Request
	contextWindow int // 0 uses math.MaxInt

	// Optional: validates each request before processing. Return a non-nil
	// error to simulate an API rejection (e.g. empty content blocks).
	validateRequest func(llm.Request) error
}

type agentMockResponse struct {
	chunks          []string
	reasoningChunks []string // Optional: reasoning content deltas.
	finishReason    llm.FinishReason
	toolCalls       []llm.ToolCall
	err             error

	// Optional: arbitrary events emitted as-is (bypasses chunk/done generation).
	rawEvents []llm.Event
	// Optional: rate limit warnings emitted before text deltas.
	rateLimitWarnings []*llm.RateLimitInfo
	// Optional: stream error emitted instead of done (simulates failed stream).
	streamError error
	// Optional: blocks CreateCompletion until the channel is closed.
	gate <-chan struct{}
	// Optional: usage data returned in DoneData.
	usage llm.Usage
}

func (m *agentMockService) CreateCompletion(
	ctx context.Context, req llm.Request,
) (iterator.Iterator[llm.Event], error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	idx := m.callCount
	m.callCount++
	m.requests = append(m.requests, req)
	m.mu.Unlock()

	if idx >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}

	resp := m.responses[idx]
	if resp.gate != nil {
		select {
		case <-resp.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.validateRequest != nil {
		if err := m.validateRequest(req); err != nil {
			return nil, err
		}
	}
	if resp.err != nil {
		return nil, resp.err
	}

	// If rawEvents is set, return them directly.
	if len(resp.rawEvents) > 0 {
		return iterator.FromSlice(resp.rawEvents), nil
	}

	var items []llm.Event

	// Emit rate limit warnings first.
	for _, rl := range resp.rateLimitWarnings {
		items = append(items, llm.Event{
			Type:      llm.EventRateLimitWarning,
			RateLimit: rl,
		})
	}

	for _, chunk := range resp.reasoningChunks {
		if chunk != "" {
			items = append(items, llm.Event{Type: llm.EventReasoningDelta, Reasoning: chunk})
		}
	}
	for _, chunk := range resp.chunks {
		if chunk != "" {
			items = append(items, llm.Event{Type: llm.EventTextDelta, Text: chunk})
		}
	}
	for i := range resp.toolCalls {
		tc := resp.toolCalls[i]
		items = append(items, llm.Event{Type: llm.EventToolCallDone, ToolCall: &tc})
	}

	// If streamError is set, emit it instead of done.
	if resp.streamError != nil {
		items = append(items, llm.Event{Type: llm.EventStreamError, Error: resp.streamError})
		return iterator.FromSlice(items), nil
	}

	var content string
	for _, chunk := range resp.chunks {
		content += chunk
	}
	var reasoningContent string
	for _, chunk := range resp.reasoningChunks {
		reasoningContent += chunk
	}
	msg := llm.Message{
		Role:             llm.RoleAssistant,
		Content:          content,
		ReasoningContent: reasoningContent,
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

	return iterator.FromSlice(items), nil
}

func (m *agentMockService) CountTokens([]llm.Message) (int, error) { return 0, nil }
func (m *agentMockService) ContextWindow() int {
	if m.contextWindow > 0 {
		return m.contextWindow
	}
	return math.MaxInt
}

func (m *agentMockService) getRequests() []llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]llm.Request(nil), m.requests...)
}

func agentToolCallResponse(toolName, args, callID string) agentMockResponse {
	return agentMockResponse{
		finishReason: llm.FinishReasonToolCall,
		toolCalls: []llm.ToolCall{{
			ID:   callID,
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{
				Name:      toolName,
				Arguments: args,
			},
		}},
	}
}

// newTestDialogueStore returns a real dialoguemanager.Store backed by an
// in-memory storageapi.Service. This lets e2e tests exercise the actual
// persistence path (indexing, versioning, serialization) instead of a mock.
func newTestDialogueStore() dialoguemanager.Store {
	return dialoguemanager.NewStore(storagestub.NewInMemoryService())
}

// normalizeMessages zeroes out empty-but-non-nil slices that result from
// BSON round-tripping so that test assertions can use plain llm.Message
// literals (where those fields are nil).
func normalizeMessages(msgs []llm.Message) []llm.Message {
	out := make([]llm.Message, len(msgs))
	for i, m := range msgs {
		if len(m.MultiContent) == 0 {
			m.MultiContent = nil
		}
		if len(m.ToolCalls) == 0 {
			m.ToolCalls = nil
		}
		out[i] = m
	}
	return out
}

// agentMockTool is a minimal agent.Tool for testing.
type agentMockTool struct {
	name      string
	desc      string // optional; defaults to "mock tool"
	executeFn func(ctx context.Context, arguments string) agent.ToolResult
}

func (t *agentMockTool) Definition() llm.Tool {
	desc := t.desc
	if desc == "" {
		desc = "mock tool"
	}
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        t.name,
			Description: desc,
			Parameters:  map[string]any{"type": "object"},
		},
	}
}

func (t *agentMockTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	if t.executeFn != nil {
		return t.executeFn(ctx, arguments)
	}
	return agent.ToolResult{Content: "ok"}
}

func (t *agentMockTool) Summary(_ string) string { return "" }

// nopNotifications implements browserapi.Notifications.
type nopNotifications struct{}

func (nopNotifications) Notify(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (nopNotifications) NotifyOnce(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (nopNotifications) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

type noopProgressUpdater struct{}

func (noopProgressUpdater) UpdateTaskProgress(context.Context, taskstore.Task) {}

type noopPrompter struct{}

func (noopPrompter) Prompt(context.Context, agent.PromptRequest) (agent.PromptResponse, error) {
	return agent.PromptResponse{}, nil
}

func TestCreateAgentCompletions_SendsBreakOnCancel(t *testing.T) {
	t.Parallel()
	// A tool that blocks until its context is cancelled.
	blockingTool := &agentMockTool{
		name: "slow_tool",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			<-ctx.Done()
			return agent.ToolResult{Content: "cancelled"}
		},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{""},
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "slow_tool", Arguments: "{}"}},
				},
			},
			// Second response (after tool result) — should not be reached.
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry(blockingTool)
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		nil, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
	}()

	// Send a request with its own cancellable context.
	reqCtx, reqCancel := context.WithCancel(ctx)
	reqRx <- completionRequest{msg: "hello", ctx: reqCtx}

	// Read events until we see the tool call, then cancel.
	sawToolCall := false
	for !sawToolCall {
		select {
		case ev := <-tx:
			if ev.Type == dialoguetui.MessageEventToolCall {
				sawToolCall = true
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for EventToolCall")
		}
	}

	// Cancel the request (simulates Ctrl-C).
	reqCancel()

	// The fix: we should receive MessageEventBreak even after cancellation.
	sawBreak := false
	timeout := time.After(5 * time.Second)
	for !sawBreak {
		select {
		case ev := <-tx:
			if ev.Type == dialoguetui.MessageEventBreak {
				sawBreak = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for MessageEventBreak after cancellation")
		}
	}

	assert.True(t, sawBreak, "MessageEventBreak should be sent after request cancellation")

	// Shut down.
	cancel()
	<-done
}

func TestCreateAgentCompletions_NormalFlowSendsBreak(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"Hello!"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry()
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		nil, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	done := make(chan struct{})
	go func() {
		defer close(done)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
	}()

	// Send a simple request.
	reqRx <- completionRequest{msg: "hello", ctx: ctx}

	// Collect events until we see the break.
	var sawText, sawBreak bool
	timeout := time.After(5 * time.Second)
	for !sawBreak {
		select {
		case ev := <-tx:
			switch ev.Type {
			case dialoguetui.MessageEventText:
				sawText = true
			case dialoguetui.MessageEventBreak:
				sawBreak = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for events")
		}
	}

	assert.True(t, sawText, "should have received text event")
	assert.True(t, sawBreak, "should have received break event")

	// Shut down.
	cancel()
	<-done
}

// --- stubConfig for newLLMService tests ---

type stubConfig struct {
	strings map[string]string
	floats  map[string]float64
	ints    map[string]int
	bools   map[string]bool
	configs map[string]config.Config
}

func (c stubConfig) GetString(key string) (string, error) {
	v, ok := c.strings[key]
	if !ok {
		return "", config.ErrNotFound
	}
	return v, nil
}

func (c stubConfig) GetFloat(key string) (float64, error) {
	v, ok := c.floats[key]
	if !ok {
		return 0, config.ErrNotFound
	}
	return v, nil
}

func (c stubConfig) GetInt(key string) (int, error) {
	v, ok := c.ints[key]
	if !ok {
		return 0, config.ErrNotFound
	}
	return v, nil
}

func (c stubConfig) GetBool(key string) (bool, error) {
	v, ok := c.bools[key]
	if !ok {
		return false, config.ErrNotFound
	}
	return v, nil
}
func (c stubConfig) GetConfig(key string) (config.Config, error) {
	v, ok := c.configs[key]
	if !ok {
		return nil, config.ErrNotFound
	}
	return v, nil
}
func (c stubConfig) GetMap(string) (map[string]interface{}, error) { return nil, config.ErrNotFound }
func (c stubConfig) GetAttribute(string) (tcell.AttrMask, error)   { return 0, config.ErrNotFound }
func (c stubConfig) GetColor(string) (tcell.Color, error)          { return 0, config.ErrNotFound }
func (c stubConfig) GetRune(string) (rune, error)                  { return 0, config.ErrNotFound }
func (c stubConfig) GetSlice(string) ([]interface{}, error)        { return nil, config.ErrNotFound }
func (c stubConfig) Iterate(func(k string, value interface{}))     {}

func TestNewLLMService_resolves_model_from_registry(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
		BaseURL:       "https://custom.api.example.com/v1/",
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "sk-test-key"},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "test-model", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, 128000, svc.ContextWindow())
}

func TestNewLLMService_unknown_model_returns_error(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	cfg := stubConfig{}

	_, err := newLLMService(cfg, reg, "nonexistent-model", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in registry")
}

func TestNewLLMService_registry_base_url_overrides_config(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "local-model",
		Provider:      "ollama",
		ContextWindow: 4096,
		BaseURL:       "http://localhost:11434/v1/",
	})

	cfg := stubConfig{
		strings: map[string]string{
			"base_url": "https://api.openai.com/v1/",
		},
	}

	svc, err := newLLMService(cfg, reg, "local-model", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	// The registry entry's BaseURL should be used, not the config's.
	// We verify indirectly: the service was created successfully with
	// context window from the registry entry.
	assert.Equal(t, 4096, svc.ContextWindow())
}

func TestNewLLMService_missing_api_key_returns_error(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-opus-4-6",
		Provider:      "anthropic",
		ContextWindow: 200000,
		BaseURL:       "https://api.anthropic.com/v1/",
	})

	cfg := stubConfig{} // no provider config at all

	_, err := newLLMService(cfg, reg, "claude-opus-4-6", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no api_key configured for provider")
	assert.Contains(t, err.Error(), "anthropic")
}

func TestNewLLMService_custom_provider_model(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "my-custom-model",
		Provider:      "custom_provider",
		ContextWindow: 32000,
		BaseURL:       "https://custom.example.com/v1/",
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"custom_provider": config.MapConfig(map[string]interface{}{
				"api_key": "sk-custom-key",
			}),
		},
	}

	svc, err := newLLMService(cfg, reg, "my-custom-model", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, 32000, svc.ContextWindow())
}

func TestNewLLMService_empty_registry_base_url_keeps_config(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "gpt-4",
		Provider:      "openai",
		ContextWindow: 8192,
		BaseURL:       "", // empty — use config's base_url
	})

	cfg := stubConfig{
		strings: map[string]string{
			"base_url": "https://custom-proxy.example.com/v1/",
		},
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "sk-test-key"},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "gpt-4", nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, 8192, svc.ContextWindow())
}

// TestNewLLMService_config_change_applies_to_next_call is an integration test
// that verifies config changes (like those from agentshell's config set) are
// picked up by the next call to newLLMService. This ensures we don't cache
// config values at startup.
func TestNewLLMService_config_change_applies_to_next_call(t *testing.T) {
	t.Parallel()
	// Track API keys received by the server across requests.
	var mu sync.Mutex
	var receivedKeys []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth := r.Header.Get("Authorization")
		receivedKeys = append(receivedKeys, strings.TrimPrefix(auth, "Bearer "))
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
		BaseURL:       srv.URL + "/v1/",
	})

	// Start with api_key "key-alpha".
	providerCfg := stubConfig{
		strings: map[string]string{"api_key": "key-alpha"},
	}
	cfg := stubConfig{
		configs: map[string]config.Config{"openai": providerCfg},
	}

	// First service creation + completion: should use key-alpha.
	svc1, err := newLLMService(cfg, reg, "test-model", nil, nil)
	require.NoError(t, err)
	it1, err := svc1.CreateCompletion(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	for { // drain iterator
		_, ok := it1.Next(context.Background())
		if !ok {
			break
		}
	}
	require.NoError(t, it1.Err())
	_ = it1.Close()

	// Simulate config change: update the API key to "key-beta".
	// Because stubConfig.configs is a map (reference type), this
	// mutation is visible through the same cfg value stored in the
	// handler, just like a real config.Config backed by the editor.
	cfg.configs["openai"] = stubConfig{
		strings: map[string]string{"api_key": "key-beta"},
	}

	// Second service creation + completion: should use key-beta.
	svc2, err := newLLMService(cfg, reg, "test-model", nil, nil)
	require.NoError(t, err)
	it2, err := svc2.CreateCompletion(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	for {
		_, ok := it2.Next(context.Background())
		if !ok {
			break
		}
	}
	require.NoError(t, it2.Err())
	_ = it2.Close()

	// Verify each call used the API key that was current at that time.
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, receivedKeys, 2,
		"expected exactly 2 requests (one per service creation)")
	assert.Equal(t, "key-alpha", receivedKeys[0],
		"first request should use the original API key")
	assert.Equal(t, "key-beta", receivedKeys[1],
		"second request should use the updated API key")
}

func TestNewLLMService_openai_reasoning_effort_from_provider_config(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "gpt-test",
		Provider:      "openai",
		ContextWindow: 8192,
	})

	var got llmopenai.Config
	cfg := stubConfig{
		strings: map[string]string{
			"reasoning_effort": "bogus",
		},
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{
					"api_key":          "sk-test-key",
					"reasoning_effort": string(llm.ReasoningEffortMedium),
				},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "gpt-test",
		func(_ string, c llmopenai.Config, models map[string]int) llm.Service {
			got = c
			return &agentMockService{contextWindow: models[c.Model]}
		}, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, string(llm.ReasoningEffortMedium), got.ReasoningEffort)
}

func TestNewLLMService_openai_global_reasoning_effort_ignored(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "gpt-test",
		Provider:      "openai",
		ContextWindow: 8192,
	})

	var got llmopenai.Config
	cfg := stubConfig{
		strings: map[string]string{
			"reasoning_effort": string(llm.ReasoningEffortXHigh),
		},
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{
					"api_key": "sk-test-key",
				},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "gpt-test",
		func(_ string, c llmopenai.Config, models map[string]int) llm.Service {
			got = c
			return &agentMockService{contextWindow: models[c.Model]}
		}, nil)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Empty(t, got.ReasoningEffort)
}

func TestNewLLMService_anthropic_reasoning_effort_from_provider_config(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-test",
		Provider:      "anthropic",
		ContextWindow: 200000,
	})

	var got anthropic.Config
	cfg := stubConfig{
		strings: map[string]string{
			"reasoning_effort": "bogus",
		},
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{
					"api_key":          "sk-ant-test",
					"reasoning_effort": string(llm.ReasoningEffortMax),
				},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "claude-test", nil,
		func(_ string, c anthropic.Config, models map[string]int) llm.Service {
			got = c
			return &agentMockService{contextWindow: models[c.Model]}
		})
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, string(llm.ReasoningEffortMax), got.ReasoningEffort)
}

func TestNewLLMService_anthropic_global_reasoning_effort_ignored(t *testing.T) {
	t.Parallel()
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-test",
		Provider:      "anthropic",
		ContextWindow: 200000,
	})

	var got anthropic.Config
	cfg := stubConfig{
		strings: map[string]string{
			"reasoning_effort": string(llm.ReasoningEffortMax),
		},
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{
					"api_key": "sk-ant-test",
				},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "claude-test", nil,
		func(_ string, c anthropic.Config, models map[string]int) llm.Service {
			got = c
			return &agentMockService{contextWindow: models[c.Model]}
		})
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Empty(t, got.ReasoningEffort)
}

func TestNewLLMService_anthropic_base_url_from_registry(t *testing.T) {
	t.Parallel()
	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = "http://" + r.Host + r.RequestURI
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, anthropicMinimalSSE("hi"))
	}))
	t.Cleanup(srv.Close)

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-test",
		Provider:      "anthropic",
		ContextWindow: 200000,
		BaseURL:       srv.URL,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{"api_key": "sk-ant-test"},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "claude-test", nil, nil)
	require.NoError(t, err)

	it, err := svc.CreateCompletion(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()
	for {
		if _, ok := it.Next(context.Background()); !ok {
			break
		}
	}
	require.NoError(t, it.Err())
	assert.Contains(t, receivedURL, srv.URL, "request should be sent to registry base URL")
}

func TestNewLLMService_anthropic_base_url_from_config(t *testing.T) {
	t.Parallel()
	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = "http://" + r.Host + r.RequestURI
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, anthropicMinimalSSE("hi"))
	}))
	t.Cleanup(srv.Close)

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-test",
		Provider:      "anthropic",
		ContextWindow: 200000,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{
					"base_url": srv.URL,
					"api_key":  "sk-ant-test",
				},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "claude-test", nil, nil)
	require.NoError(t, err)

	it, err := svc.CreateCompletion(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()
	for {
		if _, ok := it.Next(context.Background()); !ok {
			break
		}
	}
	require.NoError(t, it.Err())
	assert.Contains(t, receivedURL, srv.URL, "request should be sent to config base URL")
}

func TestNewLLMService_anthropic_registry_base_url_overrides_config(t *testing.T) {
	t.Parallel()
	var receivedURL string
	registrySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = "http://" + r.Host + r.RequestURI
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, anthropicMinimalSSE("hi"))
	}))
	t.Cleanup(registrySrv.Close)

	configSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = "http://" + r.Host + r.RequestURI
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, anthropicMinimalSSE("hi"))
	}))
	t.Cleanup(configSrv.Close)

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-test",
		Provider:      "anthropic",
		ContextWindow: 200000,
		BaseURL:       registrySrv.URL,
	})

	cfg := stubConfig{
		strings: map[string]string{
			"base_url": configSrv.URL,
		},
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{"api_key": "sk-ant-test"},
			},
		},
	}

	svc, err := newLLMService(cfg, reg, "claude-test", nil, nil)
	require.NoError(t, err)

	it, err := svc.CreateCompletion(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()
	for {
		if _, ok := it.Next(context.Background()); !ok {
			break
		}
	}
	require.NoError(t, it.Err())
	assert.Contains(t, receivedURL, registrySrv.URL, "registry base URL should take precedence over config")
}

// anthropicMinimalSSE returns a minimal Anthropic-format SSE response.
func anthropicMinimalSSE(text string) string {
	return fmt.Sprintf(`event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`, text)
}

// --- e2e stubs ---

// e2eWindow is a minimal browserapi.Window.
type e2eWindow uint64

func (w e2eWindow) WindowID() uint64 { return uint64(w) }

// capturingWindowManager captures the handlers passed to Floating() and Tab().
type capturingWindowManager struct {
	mu            sync.Mutex
	floatingCalls int
	tabCalls      int
	lastFloating  browserapi.Floating
	lastTab       browserapi.Handler
	lastTabURI    workspaceapi.URI
}

func (m *capturingWindowManager) Focus() (browserapi.Window, error) {
	return e2eWindow(0), nil
}
func (m *capturingWindowManager) Split(_ browserapi.Orientation, _ browserapi.Window, _ browserapi.Handler) (browserapi.Window, error) {
	return e2eWindow(0), nil
}
func (m *capturingWindowManager) Floating(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.floatingCalls++
	m.lastFloating = h
	return e2eWindow(1), nil
}
func (m *capturingWindowManager) Bar(_ browserapi.BarConfig, _ tui.Handler) error {
	return nil
}
func (m *capturingWindowManager) Tab(uri workspaceapi.URI, _ rune, _ string, h browserapi.Handler) (browserapi.Handler, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tabCalls++
	m.lastTab = h
	m.lastTabURI = uri
	return h, nil
}
func (m *capturingWindowManager) SetWindowContent(browserapi.Window, browserapi.Handler) error {
	return nil
}
func (m *capturingWindowManager) CloseWindow(browserapi.Window) error { return nil }

// nopFileSystem is a minimal workspaceapi.FileSystem for testing.
type nopFileSystem struct{}

func (nopFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}
func (nopFileSystem) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, os.ErrNotExist
}
func (nopFileSystem) Remove(string) error                   { return nil }
func (nopFileSystem) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (nopFileSystem) ReadDir(string) ([]os.DirEntry, error) { return nil, nil }
func (nopFileSystem) MkdirAll(string, os.FileMode) error    { return nil }

// testAIEditorDeps groups the handler and its captured test dependencies.
type testAIEditorDeps struct {
	handler     *aiEditorHandler
	wm          *capturingWindowManager
	store       dialoguemanager.Store
	svc         *agentMockService
	interruptCh chan struct{}
}

const testStorageProtoFieldKey = "X"

type testDocumentStoreServer struct {
	docpb.UnimplementedDocumentStoreServer
	backend   storageapi.Service
	marshaler storageapi_docmarshaler
}

type storageapi_docmarshaler interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
	DefaultLowerCase() bool
}

func decodeProtoFieldValue(m storageapi_docmarshaler, raw []byte) (any, error) {
	var slab map[string]any
	if err := storageapi.SafeDecode(m, &slab, raw); err != nil {
		return nil, err
	}
	return slab[testStorageProtoFieldKey], nil
}

func (s *testDocumentStoreServer) Create(ctx context.Context, req *docpb.CreateDocumentRequest) (*docpb.CreateDocumentResponse, error) {
	var doc map[string]any
	if err := storageapi.SafeDecode(s.marshaler, &doc, req.GetData()); err != nil {
		return nil, err
	}
	err := s.backend.Create(ctx, req.GetId(), doc)
	return &docpb.CreateDocumentResponse{AlreadyExists: errors.Is(err, storageapi.ErrAlreadyExists)}, nil
}

func (s *testDocumentStoreServer) Set(ctx context.Context, req *docpb.SetDocumentRequest) (*docpb.DocumentResponse, error) {
	var doc map[string]any
	if err := storageapi.SafeDecode(s.marshaler, &doc, req.GetData()); err != nil {
		return nil, err
	}
	if err := s.backend.Set(ctx, req.GetId(), doc); err != nil {
		return nil, err
	}
	return &docpb.DocumentResponse{}, nil
}

func (s *testDocumentStoreServer) Update(ctx context.Context, req *docpb.UpdateDocumentRequest) (*docpb.UpdateDocumentResponse, error) {
	updates := make([]storageapi.Update, 0, len(req.GetUpdates()))
	for _, field := range req.GetUpdates() {
		v, err := decodeProtoFieldValue(s.marshaler, field.GetData())
		if err != nil {
			return nil, err
		}
		updates = append(updates, storageapi.Update{FieldPath: field.GetFieldPath(), Value: v})
	}
	preconditions := make([]storageapi.Precondition, 0, len(req.GetPreconditions()))
	for _, field := range req.GetPreconditions() {
		v, err := decodeProtoFieldValue(s.marshaler, field.GetData())
		if err != nil {
			return nil, err
		}
		preconditions = append(preconditions, storageapi.Precondition{FieldPath: field.GetFieldPath(), Value: v})
	}
	err := s.backend.Update(ctx, req.GetId(), updates, preconditions...)
	return &docpb.UpdateDocumentResponse{
		NotFound:           errors.Is(err, storageapi.ErrNotFound),
		PreconditionFailed: errors.Is(err, storageapi.ErrPreconditionFailed),
	}, nil
}

func (s *testDocumentStoreServer) Get(ctx context.Context, req *docpb.GetDocumentRequest) (*docpb.GetDocumentResponse, error) {
	var doc map[string]any
	err := s.backend.Get(ctx, req.GetId(), &doc)
	if errors.Is(err, storageapi.ErrNotFound) {
		return &docpb.GetDocumentResponse{NotFound: true}, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := s.marshaler.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return &docpb.GetDocumentResponse{Data: raw}, nil
}

func (s *testDocumentStoreServer) Delete(ctx context.Context, req *docpb.DeleteDocumentRequest) (*docpb.DocumentResponse, error) {
	if err := s.backend.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &docpb.DocumentResponse{}, nil
}

func (s *testDocumentStoreServer) List(req *docpb.ListDocumentRequest, stream docpb.DocumentStore_ListServer) error {
	filters := make([]storageapi.Filter, 0, len(req.GetFilters()))
	for _, f := range req.GetFilters() {
		v, err := decodeProtoFieldValue(s.marshaler, f.GetData())
		if err != nil {
			return err
		}
		filters = append(filters, storageapi.Filter{
			Field: storageapi.Field{FieldPath: f.GetFieldPath(), Value: v},
			Op:    storageapi.Op(f.GetOperation()),
		})
	}
	it, err := s.backend.List(stream.Context(), filters)
	if err != nil {
		return err
	}
	defer func() { _ = it.Close() }()
	for it.HasNext() {
		var doc map[string]any
		if err := it.NextTo(&doc); err != nil {
			return stream.Send(&docpb.ListDocumentResponse{Error: err.Error()})
		}
		raw, err := s.marshaler.Marshal(doc)
		if err != nil {
			return err
		}
		if err := stream.Send(&docpb.ListDocumentResponse{Data: raw}); err != nil {
			return err
		}
	}
	return nil
}

// waitUntilIdle drains interrupt signals until no signal arrives for idleTimeout.
// If maxWait is positive, it caps total wall-clock time even if interrupts keep arriving.
func waitUntilIdle(ch <-chan struct{}, idleTimeout, maxWait time.Duration) {
	idle := time.NewTimer(idleTimeout)
	defer idle.Stop()

	var deadline <-chan time.Time
	if maxWait > 0 {
		d := time.NewTimer(maxWait)
		defer d.Stop()
		deadline = d.C
	}

	for {
		select {
		case <-ch:
			idle.Reset(idleTimeout)
		case <-idle.C:
			return
		case <-deadline:
			return
		}
	}
}

func newTestAIEditorHandler(t *testing.T, svc *agentMockService) testAIEditorDeps {
	t.Helper()

	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
	})
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model-2",
		Provider:      "openai",
		ContextWindow: 64000,
	})

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "test-key"},
			},
		},
	}

	h := &aiEditorHandler{
		defaultModel:      "test-model",
		queryDefaultModel: "test-model",
		defaultEffort:     llm.ReasoningEffortHigh,
		modelRegistry:     reg,
		dialogueStore:     store,
		newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
			return svc
		},
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		baseTools:     nil,
		toolRegistry:  agent.NewRegistry(),
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig:  agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	return testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		svc:         svc,
		interruptCh: interruptCh,
	}
}

// --- rendering helpers ---

const (
	e2eWidth  = 40
	e2eHeight = 10
)

func e2ePad(s string) string {
	n := utf8.RuneCountInString(s)
	if n >= e2eWidth {
		return s
	}
	return s + strings.Repeat(" ", e2eWidth-n)
}

func e2eExpected(numBlank int, lines ...string) string {
	all := make([]string, 0, numBlank+len(lines))
	bl := strings.Repeat(" ", e2eWidth)
	for range numBlank {
		all = append(all, bl)
	}
	for _, l := range lines {
		all = append(all, e2ePad(l))
	}
	return strings.Join(all, "\n")
}

// asyncFlusher wraps a tui.Handler and waits for async processing
// to settle between Handle and Draw, by draining an interrupt channel.
type asyncFlusher struct {
	inner       tui.Handler
	interruptCh <-chan struct{}
	idleTimeout time.Duration
	maxWait     time.Duration // optional: cap total wait even if interrupts keep coming
}

func (f *asyncFlusher) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = f.inner.Handle(ev)
	waitUntilIdle(f.interruptCh, f.idleTimeout, f.maxWait)
	return
}
func (f *asyncFlusher) Resize(w, h int)    { f.inner.Resize(w, h) }
func (f *asyncFlusher) Draw(w term.Writer) { f.inner.Draw(w) }
func (f *asyncFlusher) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return f.inner.Cursor()
}
func (f *asyncFlusher) Selection() (string, bool) { return f.inner.Selection() }

// newTestAIEditorHandlerWithServer is like newTestAIEditorHandler but
// wires the handler through the real newService → newLLMService path
// using a config and model registry that point at the given httptest
// server URL. This exercises the full service-creation code path.
func newTestAIEditorHandlerWithServer(t *testing.T, serverURL string) testAIEditorDeps {
	t.Helper()

	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          llmopenai.GPT3Dot5Turbo,
		Provider:      "openai",
		ContextWindow: 16000,
		BaseURL:       serverURL,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "test-key"},
			},
		},
	}

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	// Create the initial service via the real path for queryAgent.
	svc, err := newLLMService(cfg, reg, llmopenai.GPT3Dot5Turbo, nil, nil)
	require.NoError(t, err)

	h := &aiEditorHandler{
		defaultModel:      llmopenai.GPT3Dot5Turbo,
		queryDefaultModel: llmopenai.GPT3Dot5Turbo,
		defaultEffort:     llm.ReasoningEffortHigh,
		modelRegistry:     reg,
		dialogueStore:     store,
		// No newClient override — exercises the full newService → newLLMService → openai.NewClient path.
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		baseTools:     nil,
		toolRegistry:  agent.NewRegistry(),
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig:  agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        llmopenai.GPT3Dot5Turbo,
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	return testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		interruptCh: interruptCh,
	}
}

// newTestAIEditorHandlerWithServerAndRealStore is like
// newTestAIEditorHandlerWithServer but uses the real indexed dialogue store
// backed by storagestub. This exercises dialogue index updates, including
// optimistic preconditions in upsertIndex.
func newTestAIEditorHandlerWithServerAndRealStore(t *testing.T, serverURL string) testAIEditorDeps {
	t.Helper()

	store := dialoguemanager.NewStore(storagestub.NewInMemoryService())
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          llmopenai.GPT3Dot5Turbo,
		Provider:      "openai",
		ContextWindow: 16000,
		BaseURL:       serverURL,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "test-key"},
			},
		},
	}

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	svc, err := newLLMService(cfg, reg, llmopenai.GPT3Dot5Turbo, nil, nil)
	require.NoError(t, err)

	h := &aiEditorHandler{
		defaultModel:      llmopenai.GPT3Dot5Turbo,
		queryDefaultModel: llmopenai.GPT3Dot5Turbo,
		modelRegistry:     reg,
		dialogueStore:     store,
		wm:                wm,
		n:                 nopNotifications{},
		p:                 interrupter,
		clip:              clipboard.NewInMemory(),
		mcpManager:        runemcp.NewManager(),
		baseTools:         nil,
		toolRegistry:      agent.NewRegistry(),
		systemPrompt:      "test system prompt",
		skillRegistry:     skillReg,
		cwd:               cwd,
		fs:                nopFileSystem{},
		config:            cfg,
		resources:         make(map[string]string),
		agentsConfig:      agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        llmopenai.GPT3Dot5Turbo,
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	return testAIEditorDeps{
		handler:     h,
		wm:          wm,
		interruptCh: interruptCh,
	}
}

// newTestAIEditorHandlerWithServerAndRPCStore is like
// newTestAIEditorHandlerWithServerAndRealStore but routes dialogue storage
// through the storagerpc client/server path to better match production.
func newTestAIEditorHandlerWithServerAndRPCStore(t *testing.T, serverURL string) testAIEditorDeps {
	t.Helper()

	marshaler := docbson.Marshaler()
	backend := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
	grpcSrv := grpc.NewServer()
	storagerpc.RegisterCollectionDocumentService(grpcSrv, &testDocumentStoreServer{
		backend:   backend,
		marshaler: marshaler,
	}, "dialogues")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		_ = grpcSrv.Serve(ln)
	}()
	t.Cleanup(func() {
		grpcSrv.Stop()
		_ = ln.Close()
	})

	conn, err := grpc.NewClient(ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	rpcBackend := new(storagerpc.Client)
	rpcBackend.InitWithCollection(conn, marshaler, "dialogues")
	store := dialoguemanager.NewStore(rpcBackend)

	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          llmopenai.GPT3Dot5Turbo,
		Provider:      "openai",
		ContextWindow: 16000,
		BaseURL:       serverURL,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{
				strings: map[string]string{"api_key": "test-key"},
			},
		},
	}

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	svc, err := newLLMService(cfg, reg, llmopenai.GPT3Dot5Turbo, nil, nil)
	require.NoError(t, err)

	h := &aiEditorHandler{
		defaultModel:      llmopenai.GPT3Dot5Turbo,
		queryDefaultModel: llmopenai.GPT3Dot5Turbo,
		modelRegistry:     reg,
		dialogueStore:     store,
		wm:                wm,
		n:                 nopNotifications{},
		p:                 interrupter,
		clip:              clipboard.NewInMemory(),
		mcpManager:        runemcp.NewManager(),
		baseTools:         nil,
		toolRegistry:      agent.NewRegistry(),
		systemPrompt:      "test system prompt",
		skillRegistry:     skillReg,
		cwd:               cwd,
		fs:                nopFileSystem{},
		config:            cfg,
		resources:         make(map[string]string),
		agentsConfig:      agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        llmopenai.GPT3Dot5Turbo,
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	return testAIEditorDeps{
		handler:     h,
		wm:          wm,
		interruptCh: interruptCh,
	}
}

// --- query tests ---

func TestAIEditorHandler_query_creates_floating_window(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"response"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	assert.Equal(t, 1, deps.wm.floatingCalls)
	assert.NotNil(t, deps.wm.lastFloating)
	deps.wm.mu.Unlock()
}

func TestAIEditorHandler_query_renders_user_message(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"response"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello", "world"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	floating := deps.wm.lastFloating
	deps.wm.mu.Unlock()
	require.NotNil(t, floating)

	floating.Resize(e2eWidth, e2eHeight)
	out := handlertest.DrawHandler(floating, e2eWidth, e2eHeight)
	assert.Contains(t, out, "hello world")
}

func TestAIEditorHandler_query_renders_LLM_response(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"test response text"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	// Wait for async LLM response to arrive.
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	deps.wm.mu.Lock()
	floating := deps.wm.lastFloating
	deps.wm.mu.Unlock()
	require.NotNil(t, floating)

	handlertest.RunHandlerSequence(t, floating, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"test response text",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

// --- chat tests ---

func TestAIEditorHandler_chat_creates_tab(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"default"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	assert.Equal(t, 1, deps.wm.tabCalls)
	deps.wm.mu.Unlock()
}

func TestAIEditorHandler_chat_tab_URI_contains_model(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"default"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	uri := deps.wm.lastTabURI
	deps.wm.mu.Unlock()

	assert.Contains(t, uri.String(), "test-model")
	assert.Contains(t, uri.String(), "default")
}

func TestAIEditorHandler_chat_model_as_first_arg_error(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"test-model"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model must be passed as a second argument")
}

func TestAIEditorHandler_chat_invalid_model_error(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"default", "nonexistent-model"},
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-model")
	assert.Contains(t, err.Error(), "not supported")
}

func TestAIEditorHandler_chat_replays_history(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			// Session 1: user sends "previous question" → agent replies "previous answer".
			{chunks: []string{"previous answer"}, finishReason: llm.FinishReasonStop},
			// Session 2: (unused) — no new message is sent; we only verify the replayed history.
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	// Session 1: open a chat, send a message, get a response, close the tab.
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"mysess"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	flusher1 := &asyncFlusher{
		inner:       deps.wm.lastTab,
		interruptCh: deps.interruptCh,
		idleTimeout: 50 * time.Millisecond,
	}

	handlertest.RunHandlerSequence(t, flusher1, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "previous<space>question<enter>",
			Expected: e2eExpected(0,
				"previous question",
				"previous answer",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Close session 1 tab.
	deps.wm.mu.Lock()
	tab1 := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NoError(t, tab1.Close())

	// Session 2: reopen the same chat and verify history is replayed.
	cmd = textapi.Command{
		Name:   commandChat,
		Args:   []string{"mysess"},
		Window: e2eWindow(0),
	}
	err = deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	waitUntilIdle(deps.interruptCh, 100*time.Millisecond, 0)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	handlertest.RunHandlerSequence(t, tab, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"previous question",
				"previous answer",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_persists_multiple_turns_and_replays_full_history(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"alpha"}, finishReason: llm.FinishReasonStop},
			{chunks: []string{"beta"}, finishReason: llm.FinishReasonStop},
			{chunks: []string{"gamma"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	const dialogueID = "default"

	sendKeysToFlusher(t, flusher, "one<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "one"},
		{Role: llm.RoleAssistant, Content: "alpha"},
	})

	sendKeysToFlusher(t, flusher, "two<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "one"},
		{Role: llm.RoleAssistant, Content: "alpha"},
		{Role: llm.RoleUser, Content: "two"},
		{Role: llm.RoleAssistant, Content: "beta"},
	})

	sendKeysToFlusher(t, flusher, "three<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "one"},
		{Role: llm.RoleAssistant, Content: "alpha"},
		{Role: llm.RoleUser, Content: "two"},
		{Role: llm.RoleAssistant, Content: "beta"},
		{Role: llm.RoleUser, Content: "three"},
		{Role: llm.RoleAssistant, Content: "gamma"},
	})

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)
	require.NoError(t, tab.Close())

	replay := openChatAndGetTab(t, deps)
	const replayHeight = 12
	handlertest.RunHandlerSequence(t, replay, e2eWidth, replayHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"one",
				"alpha",
				"",
				"two",
				"beta",
				"",
				"three",
				"gamma",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

// --- shell tests ---

func TestAIEditorHandler_shell_creates_tab(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandShell,
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	assert.Equal(t, 1, deps.wm.tabCalls)
	deps.wm.mu.Unlock()
}

func TestAIEditorHandler_shell_shows_prompt(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	cmd := textapi.Command{
		Name:   commandShell,
		Window: e2eWindow(0),
	}

	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	handlertest.RunHandlerSequence(t, tab, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected:      e2eExpected(9, "agent> ▐"),
		},
	})
}

// --- dream tests ---

// capturingNotifications records Notify and UpdateNotificationProgress calls.
type capturingNotifications struct {
	mu       sync.Mutex
	notified []string
	levels   []browserapi.NotificationLevel
	progress []capturedProgress
}

type capturedProgress struct {
	id       string
	message  string
	progress int64
	total    int64
}

func (c *capturingNotifications) Notify(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	formatted := fmt.Sprintf(msg, args...)
	c.notified = append(c.notified, formatted)
	c.levels = append(c.levels, level)
	return fmt.Sprintf("notif-%d", len(c.notified)), nil
}

func (c *capturingNotifications) NotifyOnce(_ browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return c.Notify(0, msg, args...)
}

func (c *capturingNotifications) UpdateNotificationProgress(id, message string, progress, total int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.progress = append(c.progress, capturedProgress{id, message, progress, total})
	return nil
}

// testLocalFS is a workspaceapi.FileSystem backed by the real OS.
type testLocalFS struct{}

func (testLocalFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}
func (testLocalFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}
func (testLocalFS) Remove(path string) error                     { return os.Remove(path) }
func (testLocalFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (testLocalFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (testLocalFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

func TestAIEditorHandler_dream_via_shell(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	// Pre-initialize the memory workspace so bootstrap returns actionNoop,
	// skipping go mod tidy (which requires network and a live Go toolchain).
	// bootstrap.go checks: if go.mod exists and version >= templateVersion (3),
	// return actionNoop without running go mod tidy.
	memPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(memPath, "go.mod"), []byte("module memories\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(memPath, "version"), []byte("3\n"), 0o644))

	noti := &capturingNotifications{}
	deps.handler.n = noti
	deps.handler.db = storagestub.NewInMemoryService()
	deps.handler.exec = testLocalExec{}
	deps.handler.fs = testLocalFS{}
	deps.handler.memoryDataPath = memPath

	// Open the agent shell tab.
	cmd := textapi.Command{Name: commandShell, Window: e2eWindow(0)}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	f := &asyncFlusher{
		inner:       tab,
		interruptCh: deps.interruptCh,
		idleTimeout: 500 * time.Millisecond,
		maxWait:     5 * time.Second,
	}

	handlertest.RunHandlerSequence(t, f, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "dream<enter>",
			Expected: e2eExpected(7,
				"agent> dream",
				"Done: Dream complete",
				"agent> \u2590",
			),
		},
	})

	noti.mu.Lock()
	notifyCount := len(noti.notified)
	noti.mu.Unlock()
	assert.GreaterOrEqual(t, notifyCount, 1, "dream should create a notification")

	// Verify git repository was initialised in the memory workspace.
	if _, err := exec.LookPath("git"); err == nil {
		_, statErr := os.Stat(filepath.Join(memPath, ".git"))
		assert.NoError(t, statErr, "memory workspace should be a git repo")

		out, gitErr := exec.Command("git", "-C", memPath, "log", "--oneline", "--format=%s").CombinedOutput()
		require.NoError(t, gitErr, "git log should succeed")
		commits := strings.Split(strings.TrimSpace(string(out)), "\n")
		assert.Equal(t, []string{"initialize memory module"}, commits)
	}
}

func TestAIEditorHandler_dream_unknown_model_via_shell(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	// Open the agent shell tab.
	cmd := textapi.Command{Name: commandShell, Window: e2eWindow(0)}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	f := &asyncFlusher{
		inner:       tab,
		interruptCh: deps.interruptCh,
		idleTimeout: 200 * time.Millisecond,
	}

	// The error "model \"nonexistent\" not found in registry" is 41 chars,
	// which wraps at e2eWidth=40: "...registr" on line 1, "y" on line 2.
	handlertest.RunHandlerSequence(t, f, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "dream<space>--model<space>nonexistent<enter>",
			Expected: e2eExpected(6,
				"agent> dream --model nonexistent",
				"model \"nonexistent\" not found in registr",
				"y",
				"agent> \u2590",
			),
		},
	})
}

// --- reset tests ---

func TestAIEditorHandler_reset_resets_open_chat(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"hi"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	// Open a chat first.
	openCmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"resettable"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), openCmd)
	require.NoError(t, err)

	// Verify it's stored in openChats.
	_, ok := deps.handler.openChats.Load("resettable")
	require.True(t, ok, "chat should be in openChats")

	// Reset it.
	resetCmd := textapi.Command{
		Name: commandResetChat,
		Args: []string{"resettable"},
	}
	err = deps.handler.HandleCommand(context.Background(), resetCmd)
	require.NoError(t, err)
}

func TestAIEditorHandler_reset_nonexistent_returns_error(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)

	resetCmd := textapi.Command{
		Name: commandResetChat,
		Args: []string{"nonexistent"},
	}
	err := deps.handler.HandleCommand(context.Background(), resetCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// --- editor event tests ---

func TestAIEditorHandler_events(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	h := deps.handler

	uri, err := workspaceapi.ParseURI("file:///test/main.go")
	require.NoError(t, err)

	t.Run("open adds resource", func(t *testing.T) {
		h.Handle(context.Background(), textapi.Event{
			Type:    textapi.EventTypeOpen,
			URI:     uri,
			Content: "package main",
		})
		assert.Equal(t, "package main", h.resources[uri.String()])
	})

	t.Run("flush updates resource", func(t *testing.T) {
		h.Handle(context.Background(), textapi.Event{
			Type:    textapi.EventTypeFlush,
			URI:     uri,
			Content: "package main\nfunc main() {}",
		})
		assert.Equal(t, "package main\nfunc main() {}", h.resources[uri.String()])
	})

	t.Run("focus does not panic", func(t *testing.T) {
		h.Handle(context.Background(), textapi.Event{
			Type: textapi.EventTypeFocus,
			URI:  uri,
		})
	})

	t.Run("unfocus does not panic", func(t *testing.T) {
		h.Handle(context.Background(), textapi.Event{
			Type: textapi.EventTypeUnfocus,
			URI:  uri,
		})
	})

	t.Run("close removes resource", func(t *testing.T) {
		h.Handle(context.Background(), textapi.Event{
			Type: textapi.EventTypeClose,
			URI:  uri,
		})
		_, ok := h.resources[uri.String()]
		assert.False(t, ok, "resource should be removed after close")
	})
}

// --- complete tests ---

func TestAIEditorHandler_complete(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	h := deps.handler

	t.Run("aichat returns dialogue IDs", func(t *testing.T) {
		it, err := h.Complete(context.Background(), commandChat, nil)
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("aichat arg2 returns models", func(t *testing.T) {
		it, err := h.Complete(context.Background(), commandChat, []string{"sess", ""})
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		require.Len(t, items, 2)
		sort.Strings(items)
		assert.Equal(t, "test-model", items[0])
		assert.Equal(t, "test-model-2", items[1])
	})

	t.Run("airesetchat returns dialogue IDs", func(t *testing.T) {
		it, err := h.Complete(context.Background(), commandResetChat, nil)
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("unknown command returns empty", func(t *testing.T) {
		it, err := h.Complete(context.Background(), commandQuery, nil)
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("aichat excess args returns empty", func(t *testing.T) {
		it, err := h.Complete(context.Background(), commandChat, []string{"a", "b", "c"})
		require.NoError(t, err)
		items, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Empty(t, items)
	})
}

func TestAIEditorHandler_complete_returns_created_chats(t *testing.T) {
	t.Parallel()
	// Verify that a chat created through the normal extension flow
	// (opening a tab and sending a message) shows up in completions.
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"reply"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	ctx := context.Background()

	// Before any chat is opened, completions should be empty.
	it, err := deps.handler.Complete(ctx, commandChat, nil)
	require.NoError(t, err)
	items, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Empty(t, items, "no dialogues before any chat is opened")

	// Open a chat, send a message to trigger persistence.
	flusher := openChatAndGetTab(t, deps)
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hi<enter>",
			Expected: e2eExpected(0,
				"hi",
				"reply",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// After the chat round-trip, the completer should return "default".
	it, err = deps.handler.Complete(ctx, commandChat, nil)
	require.NoError(t, err)
	items, err = iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{"default"}, items, "dialogue should appear in completions after chat flow")
}

func TestAIEditorHandler_complete_filters_sub_agent_dialogues(t *testing.T) {
	t.Parallel()
	// End-to-end: user sends a message → main agent calls the agent
	// tool → sub-agent runs and replies → both dialogues are persisted.
	// Then we verify that the completer only returns the main
	// conversation, not the sub-agent dialogue.

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → calls agent tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-agent",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "agent",
						Arguments: `{"description":"do research","prompt":"look into the codebase"}`,
					},
				}},
			},
			// 1: sub-agent → replies immediately
			{
				chunks:       []string{"Found it"},
				finishReason: llm.FinishReasonStop,
			},
			// 2: main agent → uses sub-agent result, replies to user
			{
				chunks:       []string{"Here is what the sub-agent found."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Configure agents so the spawner can create sub-agents.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	// Drive the conversation: user sends "hello" which triggers the
	// full main-agent → agent tool → sub-agent → main-agent flow.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"hello",
				"✓ agent do research",
				"Found it",
				"Here is what the sub-agent found.",
				"", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Wait for async persistence of both dialogues.
	mainDialogueID := "default"
	require.Eventually(t, func() bool {
		_, err := deps.store.Get(context.Background(), mainDialogueID)
		ok := err == nil
		return ok
	}, 5*time.Second, 20*time.Millisecond, "main dialogue was not persisted")

	// Find the sub-agent dialogue(s) in the store.
	require.Eventually(t, func() bool {
		it, err := deps.store.List(context.Background())
		if err != nil {
			return false
		}
		defer it.Close() //nolint:errcheck
		for {
			h, ok := it.Next(context.Background())
			if !ok {
				return false
			}
			if h.ID != mainDialogueID && h.SubAgent {
				return true
			}
		}
	}, 5*time.Second, 20*time.Millisecond, "sub-agent dialogue was not persisted")

	// Verify the SubAgent flag: the main dialogue must have
	// SubAgent==false and the sub-agent dialogue must have
	// SubAgent==true.
	mainDialogue, err := deps.store.Get(context.Background(), mainDialogueID)
	require.NoError(t, err)

	listIt, err := deps.store.List(context.Background())
	require.NoError(t, err)
	defer listIt.Close() //nolint:errcheck
	var subAgentIDs []string
	for {
		h, ok := listIt.Next(context.Background())
		if !ok {
			break
		}
		if h.SubAgent {
			subAgentIDs = append(subAgentIDs, h.ID)
		}
	}

	assert.False(t, mainDialogue.SubAgent, "main dialogue must not be marked as sub-agent")
	require.Len(t, subAgentIDs, 1, "expected exactly one sub-agent dialogue")
	assert.Contains(t, subAgentIDs[0], "sub-agent-", "sub-agent dialogue ID should have the sub-agent prefix")

	// Insert synthetic dialogues for legacy and foreign-workspace tiers
	// directly into the store — these represent pre-existing conversations
	// that can't be created through the current extension flow.
	now := time.Now()

	// Legacy dialogue (no workspace).
	require.NoError(t, deps.store.Create(context.Background(), dialoguemanager.Dialogue{
		ID:        "old-chat",
		UpdatedAt: now.Add(-1 * time.Hour),
	}))
	// Foreign-workspace dialogue.
	require.NoError(t, deps.store.Create(context.Background(), dialoguemanager.Dialogue{
		ID:           "remote-chat",
		WorkspaceURI: "ssh://host/other/project",
		UpdatedAt:    now.Add(-2 * time.Hour),
	}))

	// Without --all: local dialogues first, then legacy; foreign
	// and sub-agent dialogues must be excluded.
	ctx := context.Background()
	it, err := deps.handler.Complete(ctx, commandChat, nil)
	require.NoError(t, err)
	items, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)

	assert.Equal(t, []string{
		mainDialogueID,      // tier 0 — current workspace (bare ID)
		"<legacy>:old-chat", // tier 1 — no workspace, prefixed
	}, items, "without --all")

	// With --all: foreign dialogues appear at the bottom, displayed
	// with the full workspace URI.
	it, err = deps.handler.Complete(ctx, commandChat, []string{"--all"})
	require.NoError(t, err)
	items, err = iterator.ToSlice(ctx, it)
	require.NoError(t, err)

	assert.Equal(t, []string{
		mainDialogueID,                         // tier 0
		"<legacy>:old-chat",                    // tier 1
		"ssh://host/other/project:remote-chat", // tier 2 — foreign, full URI
	}, items, "with --all")
}

func TestMakeCommandCompleter_filters_by_prefix(t *testing.T) {
	t.Parallel()
	ch := &stubCommandHandler{
		completions: []string{"gpt-4o", "gpt-4o-mini", "opus", "sonnet", "haiku"},
	}
	completer := makeCommandCompleter(context.Background(), ch)

	tests := []struct {
		name      string
		line      string
		pos       int
		wantHead  string
		wantComps []string
		wantTail  string
	}{
		{
			name:      "filters by typed prefix",
			line:      "/model gpt",
			pos:       10,
			wantHead:  "/model ",
			wantComps: []string{"gpt-4o", "gpt-4o-mini"},
			wantTail:  "",
		},
		{
			name:      "case insensitive match",
			line:      "/model GPT",
			pos:       10,
			wantHead:  "/model ",
			wantComps: []string{"gpt-4o", "gpt-4o-mini"},
			wantTail:  "",
		},
		{
			name:      "no prefix returns all in source order",
			line:      "/model ",
			pos:       7,
			wantHead:  "/model ",
			wantComps: []string{"gpt-4o", "gpt-4o-mini", "opus", "sonnet", "haiku"},
			wantTail:  "",
		},
		{
			name:      "no matches returns empty",
			line:      "/model xyz",
			pos:       10,
			wantHead:  "/model ",
			wantComps: nil,
			wantTail:  "",
		},
		{
			name:      "command name filtered by prefix",
			line:      "/mod",
			pos:       4,
			wantHead:  "/",
			wantComps: nil,
			wantTail:  "",
		},
		{
			name:      "cursor in middle with tail",
			line:      "/model gpt extra",
			pos:       10,
			wantHead:  "/model ",
			wantComps: []string{"gpt-4o", "gpt-4o-mini"},
			wantTail:  " extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			head, comps, tail := completer(tt.line, tt.pos)
			assert.Equal(t, tt.wantHead, head)
			assert.Equal(t, tt.wantComps, comps)
			assert.Equal(t, tt.wantTail, tail)
		})
	}
}

func TestAIEditorHandler_chat_save_on_close(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{"Hello!"},
				finishReason: llm.FinishReasonStop,
				gate:         gate, // blocks until closed or context cancelled
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	// Type a message and press Enter — inference blocks on the gate.
	keys, err := term.ParseKeys("save<space>me<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Wait for an interrupt to confirm the agent goroutine is running
	// (the hint ticker fires while inference is in progress).
	select {
	case <-deps.interruptCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hint ticker — agent not running")
	}

	// Close the tab while the LLM call is still blocked.
	// This cancels the session context, causing CreateCompletion to
	// return ctx.Err(). The agent's persistMessages should detect the
	// cancelled context, switch to a background context, and save the
	// conversation state.
	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)
	require.NoError(t, tab.Close())

	// The agent goroutine is now unwinding asynchronously: it calls
	// persistMessages with a background-context fallback. Poll the
	// store until the dialogue appears.
	dialogueID := "default"
	require.Eventually(t, func() bool {
		_, err := deps.store.Get(context.Background(), dialogueID)
		ok := err == nil
		return ok
	}, 5*time.Second, 20*time.Millisecond, "dialogue was not persisted after tab close")

	// Verify persisted messages contain system prompt + user message.
	d, err := deps.store.Get(context.Background(), dialogueID)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(d.Messages), 2,
		"expected at least system prompt + user message, got %d messages", len(d.Messages))
	assert.Equal(t, llm.RoleSystem, d.Messages[0].Role)
	assert.Equal(t, testSystemPromptWithAddendum, d.Messages[0].Content)
	assert.Equal(t, llm.RoleUser, d.Messages[1].Role)
	assert.Contains(t, d.Messages[1].Content, "save me")
}

func TestAIEditorHandler_chat_checkpoints_between_tool_iterations(t *testing.T) {
	t.Parallel()

	gateSecondIteration := make(chan struct{})
	gateFinalIteration := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gateSecondIteration:
		default:
			close(gateSecondIteration)
		}
		select {
		case <-gateFinalIteration:
		default:
			close(gateFinalIteration)
		}
	})

	secondIteration := agentToolCallResponse("checkpoint_tool", `{"step":2}`, "c2")
	secondIteration.gate = gateSecondIteration
	finalIteration := agentMockResponse{
		chunks:       []string{"tool turn done"},
		finishReason: llm.FinishReasonStop,
		gate:         gateFinalIteration,
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"ready"}, finishReason: llm.FinishReasonStop},
			agentToolCallResponse("checkpoint_tool", `{"step":1}`, "c1"),
			secondIteration,
			finalIteration,
			{chunks: []string{"welcome"}, finishReason: llm.FinishReasonStop},
		},
	}

	var toolExecs atomic.Int32
	tool := &agentMockTool{
		name: "checkpoint_tool",
		executeFn: func(context.Context, string) agent.ToolResult {
			n := toolExecs.Add(1)
			return agent.ToolResult{Content: fmt.Sprintf("tool output %d", n)}
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{tool}
	deps.handler.toolRegistry = agent.NewRegistry(tool)

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 100 * time.Millisecond
	flusher.maxWait = 500 * time.Millisecond

	const dialogueID = "default"

	sendKeysToFlusher(t, flusher, "hello<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "ready"},
	})

	sendKeysToFlusher(t, flusher, "run<space>tools<enter>")

	require.Eventually(t, func() bool {
		return len(svc.getRequests()) >= 3
	}, 5*time.Second, 20*time.Millisecond, "timed out waiting for second tool iteration to start")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "ready"},
		{Role: llm.RoleUser, Content: "run tools"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c1",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":1}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 1", ToolCallID: "c1"},
	})

	close(gateSecondIteration)
	require.Eventually(t, func() bool {
		return len(svc.getRequests()) >= 4
	}, 5*time.Second, 20*time.Millisecond, "timed out waiting for final tool iteration to start")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "ready"},
		{Role: llm.RoleUser, Content: "run tools"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c1",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":1}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 1", ToolCallID: "c1"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c2",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":2}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 2", ToolCallID: "c2"},
	})

	close(gateFinalIteration)
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "ready"},
		{Role: llm.RoleUser, Content: "run tools"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c1",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":1}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 1", ToolCallID: "c1"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c2",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":2}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 2", ToolCallID: "c2"},
		{Role: llm.RoleAssistant, Content: "tool turn done"},
	})

	sendKeysToFlusher(t, flusher, "thanks<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "ready"},
		{Role: llm.RoleUser, Content: "run tools"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c1",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":1}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 1", ToolCallID: "c1"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c2",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{"step":2}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output 2", ToolCallID: "c2"},
		{Role: llm.RoleAssistant, Content: "tool turn done"},
		{Role: llm.RoleUser, Content: "thanks"},
		{Role: llm.RoleAssistant, Content: "welcome"},
	})

	assert.Equal(t, int32(2), toolExecs.Load())
}

func TestAIEditorHandler_mcp_tool_no_redundant_summary(t *testing.T) {
	t.Parallel()

	// Set up an in-memory MCP server with a "get_file" tool under server name "obsidian".
	// The LLM-facing tool name will be "obsidian_get_file" (serverName + "_" + toolName).
	// With Summary() returning "", the collapsed TUI line must show the tool name
	// followed by formatted args — NOT a redundant "obsidian:get_file" summary.
	mcpServer := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-mcp", Version: "v1.0.0"}, nil,
	)
	gomcp.AddTool(mcpServer, &gomcp.Tool{
		Name:        "get_file",
		Description: "Get file contents",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, _ any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.TextContent{Text: "file body"},
			},
		}, nil, nil
	})

	var mu sync.Mutex
	var sessions []*gomcp.ServerSession
	factory := runemcp.TransportFactory(func(_ string, _ runemcp.ServerConfig) (gomcp.Transport, error) {
		st, ct := gomcp.NewInMemoryTransports()
		ss, err := mcpServer.Connect(context.Background(), st, nil)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		sessions = append(sessions, ss)
		mu.Unlock()
		return ct, nil
	})
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, ss := range sessions {
			_ = ss.Wait()
		}
	})

	mgr := runemcp.NewManagerWithTransport(factory)
	mcpTools := mgr.Connect(context.Background(), runemcp.Config{
		MCPServers: map[string]runemcp.ServerConfig{
			"obsidian": {Type: "stdio", Command: "dummy"},
		},
	})
	require.Len(t, mcpTools, 1)

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "obsidian_get_file",
						Arguments: `{"path":"notes.md"}`,
					},
				}},
			},
			{
				chunks:       []string{"Done."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.mcpManager = mgr
	deps.handler.baseTools = mcpTools
	deps.handler.toolRegistry = agent.NewRegistry(mcpTools...)
	deps.handler.cfg.StartCollapsed = true
	deps.handler.cfg.DurationPrecision = time.Hour
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "go<enter>",
			Expected: e2eExpected(0,
				"go",
				"└─ ✓ obsidian_get_file 0s path=notes.md",
				"Press <ctrl-o> to expand",
				"",
				"Done.",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

type stubCommandHandler struct {
	completions []string
}

func (s *stubCommandHandler) HandleCommand(context.Context, string, []string) (dialoguetui.CommandResult, error) {
	return dialoguetui.CommandResult{}, nil
}

func (s *stubCommandHandler) Complete(context.Context, string, []string) (iterator.Iterator[string], error) {
	return iterator.FromSlice(s.completions), nil
}

// --- chat slash command tests (InputSequence-driven) ---

// openChatAndGetTab opens a chat tab and returns an asyncFlusher wrapping it.
func openChatAndGetTab(t *testing.T, deps testAIEditorDeps) *asyncFlusher {
	t.Helper()
	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"default"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	return &asyncFlusher{
		inner:       tab,
		interruptCh: deps.interruptCh,
		idleTimeout: 50 * time.Millisecond,
	}
}

func sendKeysToFlusher(t *testing.T, flusher *asyncFlusher, seq string) {
	t.Helper()
	keys, err := term.ParseKeys(seq)
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}
}

func assertStoredDialogueMessages(t *testing.T, store dialoguemanager.Store, dialogueID string, expected []llm.Message) {
	t.Helper()
	require.Eventually(t, func() bool {
		d, err := store.Get(context.Background(), dialogueID)
		if err != nil {
			return false
		}
		return assert.ObjectsAreEqual(normalizeMessages(expected), normalizeMessages(d.Messages))
	}, 5*time.Second, 20*time.Millisecond, "dialogue %q did not match expected messages", dialogueID)

	d, err := store.Get(context.Background(), dialogueID)
	require.NoError(t, err)
	assert.Equal(t, normalizeMessages(expected), normalizeMessages(d.Messages))
}

func latestFloatingAndGetHandler(t *testing.T, deps testAIEditorDeps) browserapi.Floating {
	t.Helper()
	deps.wm.mu.Lock()
	defer deps.wm.mu.Unlock()
	require.NotNil(t, deps.wm.lastFloating)
	return deps.wm.lastFloating
}

func floatingFlusher(deps testAIEditorDeps, floating browserapi.Floating) *asyncFlusher {
	return &asyncFlusher{
		inner:       floating,
		interruptCh: deps.interruptCh,
		idleTimeout: 50 * time.Millisecond,
	}
}

// --- transient error / rate limit rendering tests ---
//
// These use real openai.Client instances pointed at httptest.Server to
// verify the retry logic actually fires and the TUI renders the result.

func TestAIEditorHandler_query_retries_429_and_renders_response(t *testing.T) {
	t.Parallel()
	// Server returns 429 on the first request, then succeeds.
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	deps := newTestAIEditorHandlerWithServer(t, srv.URL)

	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitUntilIdle(deps.interruptCh, 500*time.Millisecond, 0)

	deps.wm.mu.Lock()
	floating := deps.wm.lastFloating
	deps.wm.mu.Unlock()
	require.NotNil(t, floating)

	assert.Equal(t, int32(2), attempts.Load(), "should have retried once")

	handlertest.RunHandlerSequence(t, floating, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"Retrying in 0s (attempt 1/3).",
				"hello",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_query_retries_mid_stream_drop_and_renders_response(t *testing.T) {
	t.Parallel()
	// Server sends partial data then drops the connection on the first
	// request, then succeeds on the second.
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
			flusher.Flush()
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	deps := newTestAIEditorHandlerWithServer(t, srv.URL)

	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitUntilIdle(deps.interruptCh, 2*time.Second, 0)

	deps.wm.mu.Lock()
	floating := deps.wm.lastFloating
	deps.wm.mu.Unlock()
	require.NotNil(t, floating)

	assert.Equal(t, int32(2), attempts.Load(), "should have retried once after mid-stream drop")

	handlertest.RunHandlerSequence(t, floating, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"partial",
				"",
				"Connection lost (unexpected EOF),",
				"retrying in 1s (attempt 1/3).",
				"hello",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_query_renders_error_after_exhausted_retries(t *testing.T) {
	t.Parallel()
	// Mock service that emits retry warnings and then a stream error,
	// simulating what happens when retries exhaust (e.g. repeated 502s).
	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				rateLimitWarnings: []*llm.RateLimitInfo{
					{Message: "Retrying in 0s (attempt 1/3)."},
					{Message: "Retrying in 0s (attempt 2/3)."},
					{Message: "Retrying in 0s (attempt 3/3)."},
				},
				streamError: fmt.Errorf("502 Bad Gateway"),
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	cmd := textapi.Command{
		Name:   commandQuery,
		Args:   []string{"hello"},
		Window: e2eWindow(0),
	}
	err := deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitUntilIdle(deps.interruptCh, 500*time.Millisecond, 0)

	deps.wm.mu.Lock()
	floating := deps.wm.lastFloating
	deps.wm.mu.Unlock()
	require.NotNil(t, floating)

	handlertest.RunHandlerSequence(t, floating, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"Retrying in 0s (attempt 1/3).",
				"Retrying in 0s (attempt 2/3).",
				"Retrying in 0s (attempt 3/3).",
				"! agent: stream: 502 Bad Gateway",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_model_shows_current(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<enter>",
			Expected: e2eExpected(0,
				"/model",
				"test-model",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_model_switches(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<space>test-model-2<enter>",
			Expected: e2eExpected(0,
				"/model test-model-2",
				"Switched to model test-model-2",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_model_invalid(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<space>nonexistent<enter>",
			Expected: e2eExpected(0,
				"/model nonexistent",
				"! model \"nonexistent\" is not available.",
				"Available models: test-model,",
				"test-model-2",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_command_hint_survives(t *testing.T) {
	t.Parallel()
	// Regression test: typing a slash command (e.g. /model) while an
	// agent turn is in flight must not remove the status hint (spinner).
	// Before the fix, AddSendMessage unconditionally called
	// RemoveReceiveMessageHint, destroying the hint for the rest of
	// the turn.

	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		contextWindow: 128_000,
		responses: []agentMockResponse{
			// 0: gated response — blocks so we can type a slash
			// command while the turn is in flight.
			{
				gate:         gate,
				chunks:       []string{"hello"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 400, TokensReceived: 30},
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.cfg.DurationPrecision = time.Hour
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	// Step 1: Send a user message to start the agent turn, which
	// blocks on the gate. The hint should be visible.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hi<enter>",
			Expected: e2eExpected(0,
				"hi",
				"⠙ sending (0s)",
				"",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Step 2: Type a slash command while the turn is still in flight.
	// The hint must survive.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<enter>",
			Expected: e2eExpected(0,
				"hi",
				"/model",
				"⠹ sending (0s)",
				"test-model",
				"", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Step 3: Release the gate so the turn completes normally.
	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 2*time.Second)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hi",
				"/model",
				"test-model",
				"",
				"hello",
				"",
				"0s · 400 tokens sent · context: 430 (0%)",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_model_switch_propagates_to_agent_skill(t *testing.T) {
	t.Parallel()
	// Regression test: after /model switches to a different provider,
	// agent-type skills (like /explore) must spawn sub-agents using the
	// user's current model — not the startup default.
	//
	// Before the fix, the spawner always used def.Model from the
	// agent definition, so sub-agents would hit the wrong API endpoint
	// (e.g. OpenAI instead of Anthropic) after a /model switch.

	var mu sync.Mutex
	var requestedModels []string

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: sub-agent explores via read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			// 1: sub-agent returns findings
			{
				chunks:       []string{"Found it"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
	})
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-3-haiku",
		Provider:      "anthropic",
		ContextWindow: 200000,
		BaseURL:       "https://api.anthropic.com/v1/",
	})

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	// Load real skills from the repository so /explore is available.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	_, err = skillReg.AddDir(repoSkillsDir)
	require.NoError(t, err)

	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai":    stubConfig{strings: map[string]string{"api_key": "sk-openai"}},
			"anthropic": stubConfig{strings: map[string]string{"api_key": "sk-anthropic"}},
		},
	}

	h := &aiEditorHandler{
		defaultModel:      "test-model",
		queryDefaultModel: "test-model",
		modelRegistry:     reg,
		dialogueStore:     store,
		newClient: func(_ string, c llmopenai.Config, models map[string]int) llm.Service {
			mu.Lock()
			requestedModels = append(requestedModels, c.Model)
			mu.Unlock()
			return svc
		},
		newAnthropicClient: func(_ string, cfg anthropic.Config, _ map[string]int) llm.Service {
			mu.Lock()
			requestedModels = append(requestedModels, cfg.Model)
			mu.Unlock()
			return svc
		},
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		toolRegistry:  agent.NewRegistry(),
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig: agent.NewConfig([]agent.Definition{{
			ID:       "default",
			Name:     "default",
			Model:    "test-model",
			AllowAny: true,
		}}),
	}
	// Add mock read_file tool so the sub-agent can execute it.
	h.baseTools = []agent.Tool{&agentMockTool{
		name: "read_file",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "package main"}
		},
	}}

	h.ctx, h.cancelCtx = context.WithCancel(context.Background())
	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
		},
	)
	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	deps := testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		svc:         svc,
		interruptCh: interruptCh,
	}
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	// Step 1: Switch model to Anthropic.
	// Step 2: Invoke /explore (agent-type skill) which spawns a sub-agent.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<space>claude-3-haiku<enter>",
			Expected: e2eExpected(0,
				"/model claude-3-haiku",
				"Switched to model claude-3-haiku",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/explore<space>find<space>main<enter>",
			Expected: e2eExpected(0,
				"Switched to model claude-3-haiku",
				"",
				"/explore find main",
				"✓ read_file",
				"package main",
				"Found it",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// The sub-agent must have been created with the switched model,
	// not the startup default.
	mu.Lock()
	defer mu.Unlock()
	// requestedModels includes: initial chat service, /model switch,
	// and sub-agent creation. The last entry must be the sub-agent.
	require.NotEmpty(t, requestedModels)
	last := requestedModels[len(requestedModels)-1]
	assert.Equal(t, "claude-3-haiku", last,
		"sub-agent spawned by agent-type skill must use the model "+
			"the user switched to, not the startup default")
}

func TestAIEditorHandler_chat_slash_effort_shows_current(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/effort<enter>",
			Expected: e2eExpected(0,
				"/effort",
				"Current effort level: high",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_effort_sets_and_propagates(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Step 1: set effort to low.
		{
			InputSequence: "/effort<space>low<enter>",
			Expected: e2eExpected(0,
				"/effort low",
				"Set effort level to low",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 2: send a message that triggers the agent loop.
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"/effort low",
				"Set effort level to low",
				"",
				"hello",
				"done",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Verify the request sent to the LLM had ReasoningEffort set.
	reqs := svc.getRequests()
	require.NotEmpty(t, reqs, "expected at least one LLM request")
	assert.Equal(t, llm.ReasoningEffortLow, reqs[0].ReasoningEffort)
}

func TestAIEditorHandler_chat_slash_effort_invalid(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/effort<space>turbo<enter>",
			Expected: e2eExpected(0,
				"/effort turbo",
				"! invalid effort level \"turbo\": must be",
				"none, minimal, low, medium, high,",
				"xhigh, or max",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_default_effort_propagated(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
			{chunks: []string{"done2"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)

	// Set the global default effort before opening a chat.
	deps.handler.setDefaultEffort(llm.ReasoningEffortLow)

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Step 1: send a message — the agent should use the propagated effort.
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"hello",
				"done",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 2: override effort to high via /effort in the chat.
		{
			InputSequence: "/effort<space>high<enter>",
			Expected: e2eExpected(0,
				"hello",
				"done",
				"",
				"/effort high",
				"Set effort level to high",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 3: send another message — should use the overridden high effort.
		{
			InputSequence: "world<enter>",
			Expected: e2eExpected(1,
				"/effort high",
				"Set effort level to high",
				"",
				"world",
				"done2",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Verify the requests sent to the LLM.
	reqs := svc.getRequests()
	require.Len(t, reqs, 2, "expected two LLM requests")
	assert.Equal(t, llm.ReasoningEffortLow, reqs[0].ReasoningEffort,
		"first request should use propagated default effort")
	assert.Equal(t, llm.ReasoningEffortHigh, reqs[1].ReasoningEffort,
		"second request should use overridden effort")
}

// --- plan skill integration test ---

func TestAIEditorHandler_plan_skill_feedback_then_approve(t *testing.T) {
	t.Parallel()
	// This test exercises the full plan skill flow through the TUI:
	//
	//   1. User sends a message → main agent calls `skill` tool with plan
	//   2. Sub-agent explores (read_file) → calls exit_plan_mode (first plan)
	//   3. User rejects with feedback via prompt → sub-agent refines
	//   4. Sub-agent explores again → calls exit_plan_mode (improved plan)
	//   5. User approves via prompt → context cleared → sub-agent returns
	//   6. Main agent produces final response
	//
	// We verify:
	//   - Prompt interactions (feedback/approve) work through the TUI
	//   - Context clearing removes old messages from rendering
	//   - Dialogue store contains both original and cleared dialogues
	//   - The approved plan is in the final display

	const (
		// Use JSON-safe escape sequences: raw backtick strings keep
		// the literal characters \n which are valid JSON escapes.
		firstPlanJSON    = `## Plan v1\n1. Step one`
		improvedPlanJSON = `## Plan v2\n1. Better step`
	)

	// Mock LLM responses, shared by main agent and sub-agent.
	// Call order:
	//   0: main  → skill tool call
	//   1: sub   → read_file (exploration)
	//   2: sub   → exit_plan_mode (first plan)
	//   3: sub   → read_file (more exploration, after feedback)
	//   4: sub   → exit_plan_mode (improved plan)
	//   5: sub   → final text after ClearContext
	//   6: main  → final text
	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → call skill tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-skill",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "skill",
						Arguments: `{"name":"plan","args":"Plan a feature"}`,
					},
				}},
			},
			// 1: sub-agent → exploration: read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			// 2: sub-agent → exit_plan_mode (first plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"Feature plan","plan":"` + firstPlanJSON + `"}`,
					},
				}},
			},
			// 3: sub-agent (after feedback) → exploration: read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read2",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"util.go"}`,
					},
				}},
			},
			// 4: sub-agent → exit_plan_mode (improved plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit2",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"Feature plan","plan":"` + improvedPlanJSON + `"}`,
					},
				}},
			},
			// 5: sub-agent → after ClearContext, final text
			{
				chunks:       []string{improvedPlanJSON},
				finishReason: llm.FinishReasonStop,
			},
			// 6: main agent → final response
			{
				chunks:       []string{"Plan received. Implementing now."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	// Create handler with plan skill registered and agents config.
	deps := newTestAIEditorHandler(t, svc)

	// Register the real plan skill from the repository.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	// Configure agents so the spawner can run sub-agents.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	// Add a mock read_file tool to baseTools so the sub-agent can use it.
	deps.handler.baseTools = []agent.Tool{&agentMockTool{
		name: "read_file",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "package main\nfunc main() {}"}
		},
	}}

	// Deterministic plan paths – use a short fixed directory so plan
	// paths are short and fit in expected outputs as literals.
	planDir := filepath.Join(os.TempDir(), "rune-test-plans1")
	require.NoError(t, os.MkdirAll(planDir, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(planDir) })
	deps.handler.plansDir = planDir
	deps.handler.generatePlanPath = func() func(string) string {
		n := 0
		return func(title string) string {
			n++
			return filepath.Join(planDir, fmt.Sprintf("p%d.md", n))
		}
	}()
	deps.handler.generateDialogueID = func(_ context.Context, _ string) string { return "test-sub" }
	planPath2 := filepath.Join(planDir, "p2.md")

	// Open a chat tab. Default dialogue ID is "default".
	flusher := openChatAndGetTab(t, deps)
	// The agent blocks on exit_plan_mode prompts while the waiting
	// animation keeps firing interrupts, preventing idleTimeout from
	// ever settling. maxWait caps total wall-clock time per Handle call
	// so the test can proceed to answer the prompt.
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second
	const dialogueID = "default"

	// Use a wider viewport so prompt options and plan text fit.
	const pw, ph = 60, 20

	writer := term.NewStringWriter(pw, ph)
	handlertest.RunHandlerSequenceWriter(t, writer, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Step 1: Send user message. The agent runs through
		// skill → sub-agent → read_file → exit_plan_mode and
		// blocks on the prompt with "Approve" selected.
		// Sub-agent tool calls are visible as nested child events.
		{
			InputSequence: "Plan<space>a<space>feature<enter>",
			Expected: "" +
				"args=Plan a feature name=plan                               \n" +
				"✓ read_file                                                 \n" +
				"package main                                                \n" +
				"func main() {}                                              \n" +
				"⚙ exit_plan_mode Feature plan                               \n" +
				"plan=## Plan v1 1. Step one title=Feature plan              \n" +
				"                                                            \n" +
				"Plan v1                                                     \n" +
				"                                                            \n" +
				"1.Step one                                                  \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"> Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"  Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
		// Step 2: Move selection down to "Give feedback".
		// Plan body (v1) still visible.
		{
			InputSequence: "<down>",
			Expected: "" +
				"args=Plan a feature name=plan                               \n" +
				"✓ read_file                                                 \n" +
				"package main                                                \n" +
				"func main() {}                                              \n" +
				"⚙ exit_plan_mode Feature plan                               \n" +
				"plan=## Plan v1 1. Step one title=Feature plan              \n" +
				"                                                            \n" +
				"Plan v1                                                     \n" +
				"                                                            \n" +
				"1.Step one                                                  \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"  Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"> Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
		// Step 3: Enter opens text input for "Give feedback", then
		// typing feedback + enter submits. The agent processes it, runs
		// more tools, and reaches the second exit_plan_mode prompt
		// with the improved plan body (v2).
		{
			InputSequence: "<enter>Need<space>more<space>detail<enter>",
			Expected: "" +
				"Please revise the plan and call exit_plan_mode again.       \n" +
				"✓ read_file                                                 \n" +
				"package main                                                \n" +
				"func main() {}                                              \n" +
				"⚙ exit_plan_mode Feature plan                               \n" +
				"plan=## Plan v2 1. Better step title=Feature plan           \n" +
				"                                                            \n" +
				"Plan v2                                                     \n" +
				"                                                            \n" +
				"1.Better step                                               \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"> Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"  Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	// Step 4: Approve (select "Approve" = Enter on default).
	// Context is cleared, plan displayed at the top, main agent finishes.
	// Width=200 so the plan path fits on one line.
	{
		keys, keyErr := term.ParseKeys("<enter>")
		require.NoError(t, keyErr)
		for _, key := range keys {
			flusher.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
		}

		handlertest.RunHandlerSequence(t, flusher, 200, ph, []handlertest.SequenceTestCase{
			{
				InputSequence: "",
				Expected: fmt.Sprintf("%-200s", "Plan approved. Saved to "+planPath2) + "\n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"Plan v2                                                                                                                                                                                                 \n" +
					"                                                                                                                                                                                                        \n" +
					"1.Better step                                                                                                                                                                                           \n" +
					"                                                                                                                                                                                                        \n" +
					"✓ skill plan Plan a feature                                                                                                                                                                             \n" +
					"## Plan v2\\n1. Better step                                                                                                                                                                              \n" +
					"Plan received. Implementing now.                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  \n" +
					"                │▐                                                                                                                                                                   │                  \n" +
					"                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  ",
			},
		})
	}

	// --- Verify dialogue store state. ---
	// The original dialogue should exist.
	_, err = deps.store.Get(context.Background(), dialogueID)
	assert.NoError(t, err, "original dialogue should exist in store")

	// An archived dialogue should also exist (created by clearContext).
	// The sub-agent's archived dialogue has a generated ID, but
	// we can verify the main agent's dialogue got persisted with
	// the final response.
	var foundArchived bool
	archiveIt, err := deps.store.List(context.Background())
	require.NoError(t, err)
	defer archiveIt.Close() //nolint:errcheck
	for {
		h, ok := archiveIt.Next(context.Background())
		if !ok {
			break
		}
		if strings.Contains(h.ID, "archived") {
			foundArchived = true
			break
		}
	}
	assert.True(t, foundArchived, "archived dialogue should exist in store")

	// --- Verify plan file was written. ---
	_, err = os.Stat(filepath.Join(planDir, "p1.md"))
	assert.NoError(t, err, "first plan file should exist")
	_, err = os.Stat(planPath2)
	assert.NoError(t, err, "second plan file should exist")
}

func TestAIEditorHandler_plan_feedback_reaches_llm(t *testing.T) {
	t.Parallel()
	// This test verifies that user feedback typed in the prompt input box
	// is actually forwarded to the LLM service as a tool-result message.
	//
	// Flow:
	//   1. User sends a message → main agent calls skill tool (plan)
	//   2. Sub-agent calls exit_plan_mode (first plan)
	//   3. User navigates to "Give feedback", types feedback, submits
	//   4. Sub-agent receives feedback as tool result → calls exit_plan_mode again
	//   5. User approves → sub-agent returns → main agent finishes
	//
	// We verify:
	//   - Each TUI state renders correctly through the sequence
	//   - svc.getRequests() contains the feedback text in a RoleTool message

	const (
		firstPlanJSON    = `## Draft\n1. Do something`
		improvedPlanJSON = `## Final\n1. Do it better`
		feedbackDSL      = "Add<space>error<space>handling" // input DSL
		feedbackPlain    = "Add error handling"             // plain text the LLM sees
	)

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → call skill tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-skill",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "skill",
						Arguments: `{"name":"plan","args":"Build a widget"}`,
					},
				}},
			},
			// 1: sub-agent → exit_plan_mode (first plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"Widget plan","plan":"` + firstPlanJSON + `"}`,
					},
				}},
			},
			// 2: sub-agent (after feedback) → exit_plan_mode (improved plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit2",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"Widget plan","plan":"` + improvedPlanJSON + `"}`,
					},
				}},
			},
			// 3: sub-agent → after ClearContext, final text
			{
				chunks:       []string{improvedPlanJSON},
				finishReason: llm.FinishReasonStop,
			},
			// 4: main agent → final response
			{
				chunks:       []string{"Widget built."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Register the real plan skill.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	deps.handler.plansDir = t.TempDir()
	// Truncate durations to 1h so elapsed always renders as "0s".
	deps.handler.cfg.DurationPrecision = time.Hour
	// Deterministic sub-agent dialogue ID.
	deps.handler.generateDialogueID = func(_ context.Context, _ string) string { return "test-sub" }
	// Deterministic plan paths.
	plansDir2 := deps.handler.plansDir
	deps.handler.generatePlanPath = func() func(string) string {
		n := 0
		return func(title string) string {
			n++
			return filepath.Join(plansDir2, fmt.Sprintf("widget-plan-%d.md", n))
		}
	}()
	planPath1 := filepath.Join(plansDir2, "widget-plan-1.md")
	planPath2 := filepath.Join(plansDir2, "widget-plan-2.md")

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 20

	// Steps 1–4: deterministic rendering at 60×20.
	// Step 1 sends the user message, the agent runs through
	// skill → sub-agent → exit_plan_mode and blocks on the prompt.
	// Step 2 moves selection to "Give feedback".
	// Step 3 presses Enter to open the text input box, types feedback,
	// and submits. The agent processes feedback and shows the improved
	// plan prompt with "Approve" selected.
	// Step 4 approves the revised plan → context cleared, agent finishes.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Step 1: Send user message → first plan prompt with "Approve" selected.
		{
			InputSequence: "Build<space>a<space>widget<enter>",
			Expected: "" +
				"Build a widget                                              \n" +
				"⚙ skill plan Build a widget                                 \n" +
				"args=Build a widget name=plan                               \n" +
				"⚙ exit_plan_mode Widget plan                                \n" +
				"plan=## Draft 1. Do something title=Widget plan             \n" +
				"                                                            \n" +
				"Draft                                                       \n" +
				"                                                            \n" +
				"1.Do something                                              \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"> Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"  Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
		// Step 2: Move selection down to "Give feedback".
		{
			InputSequence: "<down>",
			Expected: "" +
				"Build a widget                                              \n" +
				"⚙ skill plan Build a widget                                 \n" +
				"args=Build a widget name=plan                               \n" +
				"⚙ exit_plan_mode Widget plan                                \n" +
				"plan=## Draft 1. Do something title=Widget plan             \n" +
				"                                                            \n" +
				"Draft                                                       \n" +
				"                                                            \n" +
				"1.Do something                                              \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"  Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"> Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	handlertest.RunHandlerSequence(t, flusher, 200, ph, []handlertest.SequenceTestCase{
		// Step 3: Enter opens text input, type feedback, submit.
		// Agent processes feedback → shows improved plan prompt.
		{
			InputSequence: "<enter>" + feedbackDSL + "<enter>",
			Expected: "✓ exit_plan_mode Widget plan                                                                                                                                                                            \n" +
				fmt.Sprintf("%-200s", "Plan saved to "+planPath1+". User feedback: Add error handling") + "\n" +
				"                                                                                                                                                                                                        \n" +
				"Please revise the plan and call exit_plan_mode again.                                                                                                                                                   \n" +
				"⚙ exit_plan_mode Widget plan                                                                                                                                                                            \n" +
				"plan=## Final 1. Do it better title=Widget plan                                                                                                                                                         \n" +
				"                                                                                                                                                                                                        \n" +
				"Final                                                                                                                                                                                                   \n" +
				"                                                                                                                                                                                                        \n" +
				"1.Do it better                                                                                                                                                                                          \n" +
				"                                                                                                                                                                                                        \n" +
				"Plan ready for review                                                                                                                                                                             [Plan]\n" +
				"                                                                                                                                                                                                        \n" +
				"> Approve                                                                                                                                                                                               \n" +
				"      Accept the plan and start implementing                                                                                                                                                            \n" +
				"  Give feedback                                                                                                                                                                                         \n" +
				"      Suggest changes to the plan                                                                                                                                                                       \n" +
				"                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  \n" +
				"                │                                                                                                                                                                    │                  \n" +
				"                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  ",
		},
		// Step 4: Approve the revised plan.
		{
			InputSequence: "<enter>",
			Expected: fmt.Sprintf("%-200s", "Plan approved. Saved to "+planPath2) + "\n" +
				`                                                                                                                                                                                                        
                                                                                                                                                                                                        
Final                                                                                                                                                                                                   
                                                                                                                                                                                                        
1.Do it better                                                                                                                                                                                          
                                                                                                                                                                                                        
✓ skill plan Build a widget                                                                                                                                                                             
## Final\n1. Do it better                                                                                                                                                                               
Widget built.                                                                                                                                                                                           
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                                                                                                                                                                                                        
                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  
                │▐                                                                                                                                                                   │                  
                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  `,
		},
	})

	// Verify both plan files were written.
	_, err = os.Stat(planPath1)
	assert.NoError(t, err, "first plan file should exist")
	_, err = os.Stat(planPath2)
	assert.NoError(t, err, "second plan file should exist")

	// Verify the final rendered state at width=200 so the plan path
	// fits on a single line.
	handlertest.RunHandlerSequence(t, flusher, 200, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: fmt.Sprintf("%-200s", "Plan approved. Saved to "+planPath2) + "\n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"Final                                                                                                                                                                                                   \n" +
				"                                                                                                                                                                                                        \n" +
				"1.Do it better                                                                                                                                                                                          \n" +
				"                                                                                                                                                                                                        \n" +
				"✓ skill plan Build a widget                                                                                                                                                                             \n" +
				"## Final\\n1. Do it better                                                                                                                                                                               \n" +
				"Widget built.                                                                                                                                                                                           \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                                                                                                                                                                                                        \n" +
				"                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  \n" +
				"                │▐                                                                                                                                                                   │                  \n" +
				"                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  ",
		},
	})

	// --- Verify the feedback reached the LLM. ---
	reqs := svc.getRequests()
	// We search all requests for a tool-result message containing the
	// user's feedback. The exit_plan_mode tool formats it as:
	//   "Plan saved to <path>. User feedback: <text>\n\n
	//    Please revise the plan and call exit_plan_mode again."
	var foundFeedback bool
	for _, req := range reqs {
		for _, msg := range req.Messages {
			if msg.Role == llm.RoleTool && strings.Contains(msg.Content, "User feedback: "+feedbackPlain) {
				foundFeedback = true
				assert.Contains(t, msg.Content, "Please revise the plan and call exit_plan_mode again.")
				break
			}
		}
		if foundFeedback {
			break
		}
	}
	assert.True(t, foundFeedback, "feedback text %q should appear in a RoleTool message sent to the LLM", feedbackPlain)
}

func TestAIEditorHandler_explore_slash_command(t *testing.T) {
	t.Parallel()
	// Exercises the explore skill (agent-type) invoked as a /slash command:
	//
	//   1. User types /explore find main → commandAdapter recognizes skill
	//   2. Agent-type skill: spawner.Run creates a sub-agent
	//   3. Sub-agent calls read_file (exploring) internally
	//   4. Sub-agent returns findings as text reply
	//
	// We verify:
	//   - The slash command is intercepted before reaching agentshell
	//   - The skill body is used as the sub-agent's system prompt
	//   - The sub-agent's reply text is rendered in the TUI

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: sub-agent explores via read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			// 1: sub-agent returns findings
			{
				chunks:       []string{"Found main at cmd/main.go"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Register the real explore skill from the repository.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	// Configure agents so sub-agent spawning works.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	// Add mock read_file tool.
	deps.handler.baseTools = []agent.Tool{&agentMockTool{
		name: "read_file",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "package main\nfunc main() {}"}
		},
	}}

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/explore<space>find<space>main<enter>",
			Expected: e2eExpected(0,
				"/explore find main",
				"✓ read_file",
				"package main",
				"func main() {}",
				"Found main at cmd/main.go",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Verify the sub-agent received the skill body as system prompt
	// and the user args as the task message.
	reqs := svc.getRequests()
	require.NotEmpty(t, reqs)
	// The sub-agent's system prompt should contain the skill body.
	assert.Contains(t, reqs[0].Messages[0].Content,
		"You are a fast, read-only code research specialist.")
	// The user message should contain the args.
	var userMsg string
	for _, m := range reqs[0].Messages {
		if m.Role == llm.RoleUser {
			userMsg = m.Content
			break
		}
	}
	assert.Contains(t, userMsg, "find main")
}

func TestAIEditorHandler_chat_agent_error_is_visible(t *testing.T) {
	t.Parallel()
	// Exercises the agent tool when the sub-agent fails immediately:
	//
	//   1. User sends a message → main agent calls agent tool
	//   2. Sub-agent's CreateCompletion returns an error
	//   3. Sub-agent emits EventError, run() returns
	//   4. consumeSubAgent propagates the error as the tool result
	//   5. Main agent sees the error and reports it to the user
	//
	// We verify:
	//   - The error is visible in the TUI (not silently swallowed)
	//   - The main agent can respond after the sub-agent failure

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → calls agent tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-agent",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "agent",
						Arguments: `{"description":"do research","prompt":"research the codebase"}`,
					},
				}},
			},
			// 1: sub-agent → CreateCompletion fails
			{
				err: fmt.Errorf("create completion: connection refused"),
			},
			// 2: main agent → sees error, responds to user
			{
				chunks:       []string{"The sub-agent failed. Let me try directly."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Configure agents so the spawner can create sub-agents.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"hello",
				"✗ agent do research",
				"sub-agent error: create completion:",
				"create completion: connection refused",
				"The sub-agent failed. Let me try",
				"directly.",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_agent_reasoning_only_is_visible(t *testing.T) {
	t.Parallel()
	// Exercises the agent tool when the sub-agent produces only
	// reasoning (extended thinking) with no text content:
	//
	//   1. User sends a message → main agent calls agent tool
	//   2. Sub-agent LLM returns reasoning-only response (FinishReasonStop)
	//   3. consumeSubAgent captures reasoning as fallback reply
	//   4. Main agent sees the reasoning and responds
	//
	// We verify:
	//   - The sub-agent's reasoning output is not lost
	//   - The main agent can use the reasoning in its response

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → calls agent tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-agent",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "agent",
						Arguments: `{"description":"analyze code","prompt":"analyze the code"}`,
					},
				}},
			},
			// 1: sub-agent → returns reasoning only (no text deltas)
			{
				rawEvents: []llm.Event{
					{Type: llm.EventReasoningDelta, Reasoning: "The code uses a factory pattern."},
					{Type: llm.EventStreamDone, DoneData: &llm.DoneData{
						Message: llm.Message{
							Role:             llm.RoleAssistant,
							ReasoningContent: "The code uses a factory pattern.",
						},
						FinishReason: llm.FinishReasonStop,
					}},
				},
			},
			// 2: main agent → uses the reasoning reply
			{
				chunks:       []string{"Analysis: factory pattern found."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"hello",
				"✓ agent analyze code",
				"The code uses a factory pattern.",
				"Analysis: factory pattern found.",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_compact_archives_and_replays(t *testing.T) {
	t.Parallel()
	// Exercises /chats compact through the TUI:
	//   1. User sends a message → agent responds with text
	//   2. User types /chats compact → agentshell calls CompactDialogue
	//   3. CompactDialogue archives old messages, overwrites current ID
	//   4. compactIterator.Close fetches from the same dialogue ID
	//
	// We verify:
	//   - Archived dialogue exists under ArchivedID
	//   - Current dialogue (same ID) contains compacted summary
	//   - The TUI replays the summary visually

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent responds to user message
			{
				chunks:       []string{"Hello from agent"},
				finishReason: llm.FinishReasonStop,
			},
			// 1: Summarize call (from CompactDialogue)
			{
				chunks:       []string{"Summary of our conversation"},
				finishReason: llm.FinishReasonStop,
			},
			// 2: post-compact agent response
			{
				chunks:       []string{"Continuing work"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)
	const dialogueID = "default"

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Step 1: Send a message, agent responds.
		{
			InputSequence: "hello<enter>",
			Expected: e2eExpected(0,
				"hello",
				"Hello from agent",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 2: /chats compact replays the summary.
		{
			InputSequence: "/chats<space>compact<enter>",
			Expected: e2eExpected(0,
				"This session is being continued from a",
				"previous conversation that ran out of",
				"context. The summary below covers the",
				"earlier portion of the conversation.",
				"",
				"Summary of our conversation",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 3: Send a follow-up message after compact.
		{
			InputSequence: "continue<enter>",
			Expected: e2eExpected(0,
				"earlier portion of the conversation.",
				"",
				"Summary of our conversation",
				"",
				"continue",
				"Continuing work",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	summaryMsg := agent.CompactSummaryPrefix + "Summary of our conversation"

	// Verify store state.
	archivedID := agent.ArchivedID(dialogueID)
	archived, err := deps.store.Get(context.Background(), archivedID)
	require.NoError(t, err)
	current, err := deps.store.Get(context.Background(), dialogueID)
	require.NoError(t, err)

	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "Hello from agent"},
	}, normalizeMessages(archived.Messages))
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: summaryMsg},
		{Role: llm.RoleUser, Content: "continue"},
		{Role: llm.RoleAssistant, Content: "Continuing work"},
	}, normalizeMessages(current.Messages))

	// The post-compact LLM call (index 2) must end with a user message
	// (the summary is a user message, no assistant prefill).
	reqs := svc.getRequests()
	require.Len(t, reqs, 3)
	postCompactMsgs := reqs[2].Messages
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		builtinSkillsSystemMsg,
		{Role: llm.RoleUser, Content: summaryMsg},
		{Role: llm.RoleUser, Content: "continue"},
	}, normalizeMessages(postCompactMsgs))
}

func TestAIEditorHandler_chat_compact_normalizes_stored_history_before_summarize(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{"Summary of malformed conversation"},
				finishReason: llm.FinishReasonStop,
			},
		},
		validateRequest: func(req llm.Request) error {
			for _, msg := range req.Messages {
				if msg.Role == llm.RoleAssistant && msg.Content == "" && len(msg.ToolCalls) == 0 {
					return fmt.Errorf("messages.11.content: Field required")
				}
			}
			return nil
		},
	}

	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
	})
	reg.Register(llmregistry.ModelEntry{
		Name:          "claude-3-haiku",
		Provider:      "anthropic",
		ContextWindow: 200000,
	})

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai":    stubConfig{strings: map[string]string{"api_key": "sk-openai"}},
			"anthropic": stubConfig{strings: map[string]string{"api_key": "sk-anthropic"}},
		},
	}

	h := &aiEditorHandler{
		defaultModel:      "test-model",
		queryDefaultModel: "test-model",
		modelRegistry:     reg,
		dialogueStore:     store,
		newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
			return svc
		},
		newAnthropicClient: func(_ string, _ anthropic.Config, _ map[string]int) llm.Service {
			return svc
		},
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		toolRegistry:  agent.NewRegistry(),
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig:  agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())
	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
		},
	)
	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	deps := testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		svc:         svc,
		interruptCh: interruptCh,
	}

	err = deps.store.Create(context.Background(), dialoguemanager.Dialogue{
		ID: "default",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: ""},
			{Role: llm.RoleUser, Content: "continue"},
		},
	})
	require.NoError(t, err)

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	keys, err := term.ParseKeys("/model<space>claude-3-haiku<enter>")
	require.NoError(t, err)
	for _, key := range keys {
		flusher.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
	}

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/chats<space>compact<enter>",
			Expected: e2eExpected(0,
				"This session is being continued from a",
				"previous conversation that ran out of",
				"context. The summary below covers the",
				"earlier portion of the conversation.",
				"",
				"Summary of malformed conversation",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	reqs := svc.getRequests()
	require.Len(t, reqs, 1)
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleUser, Content: "continue"},
		{Role: llm.RoleUser, Content: agent.SummarizePrompt},
	}, normalizeMessages(reqs[0].Messages))

	archived, err := deps.store.Get(context.Background(), agent.ArchivedID("default"))
	require.NoError(t, err)
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: ""},
		{Role: llm.RoleUser, Content: "continue"},
	}, normalizeMessages(archived.Messages))

	current, err := deps.store.Get(context.Background(), "default")
	require.NoError(t, err)
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: agent.CompactSummaryPrefix + "Summary of malformed conversation"},
	}, normalizeMessages(current.Messages))
}

func TestAIEditorHandler_agent_compact_no_assistant_prefill(t *testing.T) {
	t.Parallel()
	// Reproduces bug: after the agent calls the compact tool mid-loop,
	// CompactDialogue returns [system, user, assistant] and the loop
	// continues to the next LLM call without appending a user message.
	// Providers like Anthropic Opus 4.6 reject this because the
	// conversation ends with an assistant message (prefill).
	compactTool := &agentMockTool{
		name: "compact",
		desc: "Compact the conversation",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "Compacting...", Compact: true}
		},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent calls the compact tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "compact",
						Arguments: "{}",
					},
				}},
			},
			// 1: Summarize call (from CompactDialogue)
			{
				chunks:       []string{"Summary of conversation"},
				finishReason: llm.FinishReasonStop,
			},
			// 2: post-compact response
			{
				chunks:       []string{"Continuing work"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{compactTool}
	deps.handler.toolRegistry = agent.NewRegistry(compactTool)
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "do<space>stuff<enter>",
			Expected: e2eExpected(0,
				"Conversation compacted. Old",
				"conversation stored as",
				"**default-archived**.",
				"✓ compact",
				"Compacting...",
				"Continuing work",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// The post-compact LLM call must end with a user message,
	// not an assistant message (which would be rejected as prefill
	// by providers like Anthropic Opus 4.6).
	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 3,
		"expected at least 3 LLM calls: main → summarize → post-compact")
	postCompactMsgs := reqs[2].Messages
	lastMsg := postCompactMsgs[len(postCompactMsgs)-1]
	assert.Equal(t, llm.RoleUser, lastMsg.Role,
		"post-compact request must end with user message (not assistant prefill)")
}

func TestAIEditorHandler_agent_compact_hint_survives(t *testing.T) {
	// After the compact tool call, the agent must re-enter the LLM
	// inference loop. While the post-compact LLM request is in flight,
	// the status hint (spinner + phase) must still be visible. Before
	// the fix, compactFn called comp.Reset() which removed the hint,
	// and the replay called AddSendMessageMarkdown which also removes
	// it. The hint was never restored, leaving a blank screen until
	// the next response arrived.
	compactTool := &agentMockTool{
		name: "compact",
		desc: "Compact the conversation",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "Compacting...", Compact: true}
		},
	}

	gate := make(chan struct{})
	svc := &agentMockService{
		contextWindow: 128_000,
		responses: []agentMockResponse{
			// 0: agent calls the compact tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "compact",
						Arguments: "{}",
					},
				}},
				usage: llm.Usage{TokensSent: 500, TokensReceived: 20},
			},
			// 1: Summarize call (from CompactDialogue)
			{
				chunks:       []string{"Summary of conversation"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 200, TokensReceived: 30},
			},
			// 2: post-compact response (gated so we can inspect
			// the TUI state while the hint should be visible)
			{
				gate:         gate,
				chunks:       []string{"Continuing work"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 100, TokensReceived: 50},
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{compactTool}
	deps.handler.toolRegistry = agent.NewRegistry(compactTool)
	// Truncate durations to 1h so elapsed always renders as "0s".
	deps.handler.cfg.DurationPrecision = time.Hour
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	// Step 1: Type the message. After Enter the agent starts,
	// compacts, and blocks on the gated post-compact LLM request.
	// The flusher returns after maxWait because the hint's tick
	// goroutine keeps sending interrupts. The status hint must
	// still be visible (with token counts from response 0).
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "do<space>stuff<enter>",
			Expected: e2eExpected(0,
				"",
				"Conversation compacted. Old",
				"conversation stored as",
				"**default-archived**.",
				"✓ compact",
				"Compacting...",
				"⠙ sending (0s · ↑ 500 tokens · ↓ 20 toke",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Step 2: Release the gate so the turn completes. The
	// status hint is replaced by the post-turn context hint
	// showing accumulated token usage and context window stats.
	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 2*time.Second)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"conversation stored as",
				"**default-archived**.",
				"✓ compact",
				"Compacting...",
				"Continuing work",
				"",
				"0s · 600 tokens sent · context: 150 (0%)",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_plan_survives_compact(t *testing.T) {
	t.Parallel()
	// Full round-trip: user sends a message, agent enters plan mode
	// via the plan skill, proposes a plan, user approves, ClearContext
	// collapses old messages, then the sub-agent compacts the context.
	// After the turn finishes we close the tab and re-open the same
	// dialogue. The replay must show the plan (with the file path)
	// at the top.
	//
	// LLM call sequence (shared mock service):
	//   0: main  → skill(plan)
	//   1: sub   → read_file (exploration)
	//   2: sub   → exit_plan_mode (blocks on prompt, user approves)
	//   3: sub   → compact tool (after ClearContext)
	//   4: sub   → Summarize (from CompactDialogue)
	//   5: sub   → post-compact final text
	//   6: main  → final text

	const planJSON = `## Step 1\n1. Do the thing`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main → skill(plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-skill",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "skill",
						Arguments: `{"name":"plan","args":"Plan a feature"}`,
					},
				}},
			},
			// 1: sub → read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			// 2: sub → exit_plan_mode (blocks on prompt)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"Feature plan","plan":"` + planJSON + `"}`,
					},
				}},
			},
			// 3: sub → compact (after ClearContext)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-compact",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "compact",
						Arguments: "{}",
					},
				}},
			},
			// 4: Summarize (from CompactDialogue)
			{
				chunks:       []string{"Summary of work"},
				finishReason: llm.FinishReasonStop,
			},
			// 5: sub → post-compact final text
			{
				chunks:       []string{"Done implementing"},
				finishReason: llm.FinishReasonStop,
			},
			// 6: main → final text
			{
				chunks:       []string{"All done."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Override GenerateID for predictable sub-agent dialogue ID.
	deps.handler.generateDialogueID = func(_ context.Context, _ string) string { return "test-sub" }

	// Register the real plan skill.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	// Configure agents so the spawner can run sub-agents.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID: "default", Name: "default", Model: "test-model", AllowAny: true,
	}})

	compactTool := &agentMockTool{
		name: "compact",
		desc: "Compact the conversation",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			// Small delay so child events (compact tool call/result)
			// are fully drained before the sub-agent returns and the
			// main agent resumes, making event ordering deterministic.
			time.Sleep(50 * time.Millisecond)
			return agent.ToolResult{Content: "Compacting...", Compact: true}
		},
	}
	deps.handler.baseTools = []agent.Tool{
		&agentMockTool{
			name: "read_file",
			executeFn: func(_ context.Context, _ string) agent.ToolResult {
				return agent.ToolResult{Content: "package main\nfunc main() {}"}
			},
		},
		compactTool,
	}
	// Use a short, fixed plans dir so the path fits on one line
	// at 120 chars width without word-wrap splitting.
	plansDir := filepath.Join(t.TempDir(), "plans")
	deps.handler.plansDir = plansDir
	// Deterministic plan paths.
	deps.handler.generatePlanPath = func() func(string) string {
		n := 0
		return func(title string) string {
			n++
			return filepath.Join(plansDir, fmt.Sprintf("feature-plan-%d.md", n))
		}
	}()
	planPath := filepath.Join(plansDir, "feature-plan-1.md")

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 20
	writer := term.NewStringWriter(pw, ph)

	// Step 1: Send user message. Agent enters plan mode via skill,
	// sub-agent explores, then calls exit_plan_mode. The prompt
	// appears with "Approve" selected.
	handlertest.RunHandlerSequenceWriter(t, writer, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "Plan<space>a<space>feature<enter>",
			Expected: "" +
				"args=Plan a feature name=plan                               \n" +
				"✓ read_file                                                 \n" +
				"package main                                                \n" +
				"func main() {}                                              \n" +
				"⚙ exit_plan_mode Feature plan                               \n" +
				"plan=## Step 1 1. Do the thing title=Feature plan           \n" +
				"                                                            \n" +
				"Step 1                                                      \n" +
				"                                                            \n" +
				"1.Do the thing                                              \n" +
				"                                                            \n" +
				"Plan ready for review                                 [Plan]\n" +
				"                                                            \n" +
				"> Approve                                                   \n" +
				"      Accept the plan and start implementing                \n" +
				"  Give feedback                                             \n" +
				"      Suggest changes to the plan                           \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	// Step 2: Approve. ClearContext collapses old sub-agent messages.
	// The plan renders at the top, then final text from sub-agent
	// and main agent. Width=200 so plan path fits on one line.
	{
		keys, keyErr := term.ParseKeys("<enter>")
		require.NoError(t, keyErr)
		for _, key := range keys {
			flusher.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
		}

		handlertest.RunHandlerSequence(t, flusher, 200, ph, []handlertest.SequenceTestCase{
			{
				InputSequence: "",
				Expected: fmt.Sprintf("%-200s", "Plan approved. Saved to "+planPath) + "\n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"Step 1                                                                                                                                                                                                  \n" +
					"                                                                                                                                                                                                        \n" +
					"1.Do the thing                                                                                                                                                                                          \n" +
					"                                                                                                                                                                                                        \n" +
					"This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.                                                 \n" +
					"                                                                                                                                                                                                        \n" +
					"Summary of work                                                                                                                                                                                         \n" +
					"                                                                                                                                                                                                        \n" +
					"✓ skill plan Plan a feature                                                                                                                                                                             \n" +
					"Done implementing                                                                                                                                                                                       \n" +
					"All done.                                                                                                                                                                                               \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  \n" +
					"                │▐                                                                                                                                                                   │                  \n" +
					"                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  ",
			},
		})
	}

	// Step 3: Close and re-open the sub-agent dialogue. The replay
	// must show the plan (with file path) at the top, then the
	// compacted summary, then the post-compact response.
	deps.wm.mu.Lock()
	tab1 := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NoError(t, tab1.Close())

	cmd := textapi.Command{
		Name:   commandChat,
		Args:   []string{"test-sub"},
		Window: e2eWindow(0),
	}
	require.NoError(t, deps.handler.HandleCommand(context.Background(), cmd))

	deps.wm.mu.Lock()
	tab2 := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab2)

	flusher2 := &asyncFlusher{
		inner:       tab2,
		interruptCh: deps.interruptCh,
		idleTimeout: 50 * time.Millisecond,
	}

	// Step 3: The re-opened dialogue renders the plan at the top.
	// Width=200 so the plan path fits on one line.
	{

		handlertest.RunHandlerSequence(t, flusher2, 200, 20, []handlertest.SequenceTestCase{
			{
				InputSequence: "",
				Expected: fmt.Sprintf("%-200s", "Plan approved. Saved to "+planPath) + "\n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"Step 1                                                                                                                                                                                                  \n" +
					"                                                                                                                                                                                                        \n" +
					"1.Do the thing                                                                                                                                                                                          \n" +
					"                                                                                                                                                                                                        \n" +
					"This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.                                                 \n" +
					"                                                                                                                                                                                                        \n" +
					"Summary of work                                                                                                                                                                                         \n" +
					"                                                                                                                                                                                                        \n" +
					"Done implementing                                                                                                                                                                                       \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                                                                                                                                                                                                        \n" +
					"                ┌────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐                  \n" +
					"                │▐                                                                                                                                                                   │                  \n" +
					"                └────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘                  ",
			},
		})
	}
}

func TestAIEditorHandler_chat_slash_clear(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/model<enter>",
			Expected: e2eExpected(0,
				"/model",
				"test-model",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/clear<enter>",
			Expected: e2eExpected(7,
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_slash_fork_picker_flow(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)
	noti := &capturingNotifications{}
	deps.handler.n = noti

	_, err := deps.store.Get(context.Background(), "default")
	if errors.Is(err, storageapi.ErrNotFound) {
		require.NoError(t, deps.store.Create(context.Background(), dialoguemanager.Dialogue{
			ID:           "default",
			AgentID:      "default",
			Model:        "test-model",
			WorkspaceURI: "file:///test/workspace",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
				{Role: llm.RoleUser, Content: "first user message"},
				{Role: llm.RoleAssistant, Content: "first assistant reply"},
				{Role: llm.RoleUser, Content: "second user message"},
				{Role: llm.RoleAssistant, Content: "second assistant reply"},
			},
		}))
	} else {
		require.NoError(t, err)
	}

	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/fork<enter>",
			Expected: e2eExpected(0,
				"first user message",
				"first assistant reply",
				"",
				"second user message",
				"second assistant reply",
				"",
				"/fork",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	floating := latestFloatingAndGetHandler(t, deps)
	picker := floatingFlusher(deps, floating)

	handlertest.RunHandlerSequence(t, picker, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				" first user message",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				"  first user message",
			),
		},
	})

	// Move focus down once and ensure the picker remains interactive.
	handlertest.RunHandlerSequence(t, picker, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "<down>",
			Expected: e2eExpected(0,
				" first assistant reply",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				" 󰚩 first assistant reply",
			),
		},
	})

	// Select the focused message.
	handlertest.RunHandlerSequence(t, picker, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "<enter>",
			Expected: e2eExpected(0,
				" first assistant reply",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				" 󰚩 first assistant reply",
			),
		},
	})

	// Verify a cloned dialogue now exists and notification wording is clear.
	it, err := deps.store.List(context.Background())
	require.NoError(t, err)
	defer func() { _ = it.Close() }()
	ids, err := iterator.ToSlice(context.Background(), iterator.Map(it, func(h dialoguemanager.DialogueHeader) string { return h.ID }))
	require.NoError(t, err)
	require.Contains(t, ids, "default-fork")

	noti.mu.Lock()
	defer noti.mu.Unlock()
	require.NotEmpty(t, noti.notified)
	assert.Contains(t, noti.notified[len(noti.notified)-1], "Cloned conversation at the selected message. Open default-fork to resume it.")
}

func TestAIEditorHandler_chat_slash_fork_picker_clones_selected_message(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}
	deps := newTestAIEditorHandler(t, svc)

	require.NoError(t, deps.store.Create(context.Background(), dialoguemanager.Dialogue{
		ID:           "default",
		AgentID:      "default",
		Model:        "test-model",
		WorkspaceURI: "file:///test/workspace",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
			{Role: llm.RoleUser, Content: "keep me"},
			{Role: llm.RoleAssistant, Content: "keep assistant"},
			{Role: llm.RoleUser, Content: "fork here"},
			{Role: llm.RoleAssistant, Content: "drop me"},
		},
	}))

	flusher := openChatAndGetTab(t, deps)
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "/fork<enter>",
			Expected: e2eExpected(0,
				"keep me",
				"keep assistant",
				"",
				"fork here",
				"drop me",
				"",
				"/fork",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	picker := floatingFlusher(deps, latestFloatingAndGetHandler(t, deps))
	handlertest.RunHandlerSequence(t, picker, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "<down>",
			Expected: e2eExpected(0,
				" keep assistant",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				" 󰚩 keep assistant",
			),
		},
		{
			InputSequence: "<down>",
			Expected: e2eExpected(0,
				" fork here",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				"  fork here",
			),
		},
		{
			InputSequence: "<enter>",
			Expected: e2eExpected(0,
				" fork here",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				" ──────────────────────────────────────",
				"  fork here",
			),
		},
	})

	forked, err := deps.store.Get(context.Background(), "default-fork")
	require.NoError(t, err)
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "keep me"},
		{Role: llm.RoleAssistant, Content: "keep assistant"},
		{Role: llm.RoleUser, Content: "fork here"},
	}, normalizeMessages(forked.Messages))
}

func TestAIEditorHandler_chat_clear_archives_and_empties(t *testing.T) {
	t.Parallel()
	// Exercises /chats clear <id> through the TUI:
	//   1. User sends a message → agent responds
	//   2. User types /chats clear default → agentshell archives + empties
	//
	// We verify:
	//   - Archived dialogue exists under ArchivedID
	//   - Current dialogue has empty messages

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent responds to user message
			{
				chunks:       []string{"Agent reply"},
				finishReason: llm.FinishReasonStop,
			},
			// 1: post-clear agent response
			{
				chunks:       []string{"Fresh start"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)
	const dialogueID = "default"

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Step 1: Send a message, agent responds.
		{
			InputSequence: "hi<enter>",
			Expected: e2eExpected(0,
				"hi",
				"Agent reply",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 2: /chats clear default archives and empties.
		// Output goes to a floating window, so inline view
		// only shows the command text.
		{
			InputSequence: "/chats<space>clear<space>default<enter>",
			Expected: e2eExpected(0,
				"hi",
				"Agent reply",
				"",
				"/chats clear default",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 3: Send a follow-up after clear.
		{
			InputSequence: "new<space>topic<enter>",
			Expected: e2eExpected(0,
				"hi",
				"Agent reply",
				"",
				"/chats clear default",
				"new topic",
				"Fresh start",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Verify store state.
	archivedID := agent.ArchivedID(dialogueID)
	archived, err := deps.store.Get(context.Background(), archivedID)
	require.NoError(t, err)
	current, err := deps.store.Get(context.Background(), dialogueID)
	require.NoError(t, err)

	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hi"},
		{Role: llm.RoleAssistant, Content: "Agent reply"},
	}, normalizeMessages(archived.Messages))
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "new topic"},
		{Role: llm.RoleAssistant, Content: "Fresh start"},
	}, normalizeMessages(current.Messages))

	// The post-clear LLM call (index 1) must have only the
	// system prompt + new user message — no old conversation.
	reqs := svc.getRequests()
	require.Len(t, reqs, 2)
	postClearMsgs := reqs[1].Messages
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		builtinSkillsSystemMsg,
		{Role: llm.RoleUser, Content: "new topic"},
	}, normalizeMessages(postClearMsgs))
}

func TestAIEditorHandler_chat_clear_no_args_archives_and_clears(t *testing.T) {
	t.Parallel()
	// Exercises /clear (no args) through the TUI:
	//   1. Open chat, send message → agent responds
	//   2. /clear → archives conversation + clears store + visual reset
	//   3. Close tab, reopen → chat is empty, archived dialogue exists
	//   4. Send another message, /clear again → second archive created
	//   5. Close tab, reopen → chat is empty, two archived dialogues exist

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"First reply"}, finishReason: llm.FinishReasonStop},
			{chunks: []string{"Second reply"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	const dialogueID = "default"

	// === Session 1: send a message, then /clear ===
	flusher1 := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher1, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "first<space>msg<enter>",
			Expected: e2eExpected(0,
				"first msg",
				"First reply",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/clear<enter>",
			Expected: e2eExpected(0,
				"Cleared default (archived as",
				"default-archived)",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Close the tab.
	deps.wm.mu.Lock()
	tab1 := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NoError(t, tab1.Close())

	// Verify: archived dialogue has the first conversation.
	archivedID := agent.ArchivedID(dialogueID)
	archived1, err := deps.store.Get(context.Background(), archivedID)
	require.NoError(t, err)
	current, err := deps.store.Get(context.Background(), dialogueID)
	require.NoError(t, err)

	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "first msg"},
		{Role: llm.RoleAssistant, Content: "First reply"},
	}, normalizeMessages(archived1.Messages))
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
	}, normalizeMessages(current.Messages))

	// === Session 2: reopen, verify clean, add more, clear again ===
	flusher2 := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher2, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Chat should be empty after /clear.
		{
			InputSequence: "",
			Expected: e2eExpected(7,
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Send another message.
		{
			InputSequence: "second<space>msg<enter>",
			Expected: e2eExpected(0,
				"second msg",
				"Second reply",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Clear again.
		{
			InputSequence: "/clear<enter>",
			Expected: e2eExpected(0,
				"Cleared default (archived as",
				"default-archived-2)",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Close the tab.
	deps.wm.mu.Lock()
	tab2 := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NoError(t, tab2.Close())

	// Verify: two archived chats.
	archived1, err = deps.store.Get(context.Background(), archivedID)
	require.NoError(t, err)
	archived2, err := deps.store.Get(context.Background(), archivedID+"-2")
	require.NoError(t, err)
	current, err = deps.store.Get(context.Background(), dialogueID)
	require.NoError(t, err)

	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "first msg"},
		{Role: llm.RoleAssistant, Content: "First reply"},
	}, normalizeMessages(archived1.Messages))
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "second msg"},
		{Role: llm.RoleAssistant, Content: "Second reply"},
	}, normalizeMessages(archived2.Messages))
	assert.Equal(t, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
	}, normalizeMessages(current.Messages))

	// === Session 3: reopen, verify still clean ===
	flusher3 := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher3, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(7,
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_clear_after_normal_turn_with_real_store(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	deps := newTestAIEditorHandlerWithServerAndRealStore(t, srv.URL)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "silly<space>message<enter>",
			Expected: e2eExpected(0,
				"silly message",
				"hello",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/clear<enter>",
			Expected: e2eExpected(0,
				"Cleared default (archived as",
				"default-archived)",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_clear_after_normal_turn_with_rpc_store(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	deps := newTestAIEditorHandlerWithServerAndRPCStore(t, srv.URL)
	flusher := openChatAndGetTab(t, deps)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "silly<space>message<enter>",
			Expected: e2eExpected(0,
				"silly message",
				"hello",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/clear<enter>",
			Expected: e2eExpected(0,
				"Cleared default (archived as",
				"default-archived)",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestFormatStatusDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{61 * time.Second, "1m 1s"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
		{59*time.Minute + 59*time.Second, "59m 59s"},
		{60 * time.Minute, "1h 0m"},
		{90 * time.Minute, "1h 30m"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatStatusDuration(tt.d)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTokenCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 tokens"},
		{1, "1 tokens"},
		{999, "999 tokens"},
		{1000, "1.0k tokens"},
		{1500, "1.5k tokens"},
		{18900, "18.9k tokens"},
		{999999, "1000.0k tokens"},
		{1000000, "1.0m tokens"},
		{2500000, "2.5m tokens"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatTokenCount(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatusPhaseString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "sending", phaseSending.String())
	assert.Equal(t, "thinking", phaseThinking.String())
	assert.Equal(t, "receiving", phaseReceiving.String())
	assert.Equal(t, "tool calling", phaseToolCalling.String())
	assert.Equal(t, "compacting", phaseCompacting.String())
	assert.Equal(t, "rate limited", phaseRateLimited.String())
}

func TestStatusPhaseArrow(t *testing.T) {
	t.Parallel()
	assert.Equal(t, '↑', phaseSending.arrow())
	assert.Equal(t, '↓', phaseThinking.arrow())
	assert.Equal(t, '↓', phaseReceiving.arrow())
	assert.Equal(t, '↑', phaseToolCalling.arrow())
	assert.Equal(t, '↓', phaseCompacting.arrow())
	assert.Equal(t, '↓', phaseRateLimited.arrow())
}

func TestStatusHint_Draw(t *testing.T) {
	t.Parallel()
	hint := newStatusHint(term.NopInterrupter(), term.Attributes{}, 0, nil)
	defer func() { _ = hint.Close() }()

	hint.Resize(80, 1)
	w := term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got := strings.TrimRight(w.String(), " \n")

	// First draw: phase=sending, tokens=0, duration≈0s
	// Should match: "⠙ sending (0s)" (drawCount=1 → frame index 1)
	assert.Contains(t, got, "sending")
	assert.Contains(t, got, "0s)")
	assert.NotContains(t, got, "tokens")
}

func TestStatusHint_DrawWithTokens(t *testing.T) {
	t.Parallel()
	hint := newStatusHint(term.NopInterrupter(), term.Attributes{}, 0, nil)
	defer func() { _ = hint.Close() }()

	hint.setTokens(18900, 3200)
	hint.Resize(80, 1)
	w := term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got := strings.TrimRight(w.String(), " \n")

	assert.Contains(t, got, "sending")
	assert.Contains(t, got, "↑ 18.9k tokens")
	assert.Contains(t, got, "↓ 3.2k tokens")
}

func TestStatusHint_PhaseChange(t *testing.T) {
	t.Parallel()
	hint := newStatusHint(term.NopInterrupter(), term.Attributes{}, 0, nil)
	defer func() { _ = hint.Close() }()

	hint.setPhase(phaseToolCalling)
	hint.setTokens(5000, 800)
	hint.Resize(80, 1)
	w := term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got := strings.TrimRight(w.String(), " \n")

	assert.Contains(t, got, "tool calling")
	assert.Contains(t, got, "↑ 5.0k tokens")
	assert.Contains(t, got, "↓ 800 tokens")
}

func TestStatusHint_DrawUsesActiveFormFn(t *testing.T) {
	t.Parallel()
	activeForm := ""
	hint := newStatusHint(term.NopInterrupter(), term.Attributes{}, 0, func() string {
		return activeForm
	})
	defer func() { _ = hint.Close() }()

	hint.setPhase(phaseToolCalling)
	hint.Resize(80, 1)

	// When activeFormFn returns empty, falls back to phase label.
	w := term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got := strings.TrimRight(w.String(), " \n")
	assert.Contains(t, got, "tool calling")

	// When activeFormFn returns a value, it replaces the phase label.
	activeForm = "Writing tests"
	w = term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got = strings.TrimRight(w.String(), " \n")
	assert.Contains(t, got, "Writing tests")
	assert.NotContains(t, got, "tool calling")
}

func TestStatusHint_DrawUsesPhaseWhenTaskListVisible(t *testing.T) {
	t.Parallel()

	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.UpdateTaskProgress(dialoguetui.ProgressTaskEntry{
		ID:         "1",
		Subject:    "Write tests",
		ActiveForm: "Writing tests",
		Status:     "in_progress",
	})

	hint := newStatusHint(term.NopInterrupter(), term.Attributes{}, 0, comp.TaskActiveForm)
	defer func() { _ = hint.Close() }()

	hint.setPhase(phaseToolCalling)
	hint.Resize(80, 1)

	w := term.NewStringWriter(80, 1)
	hint.Draw(w)
	_ = w.Flush()
	got := strings.TrimRight(w.String(), " \n")

	assert.Contains(t, got, "tool calling")
	assert.NotContains(t, got, "Writing tests")
}

func TestAIEditorHandler_chat_hint_visible_during_inference(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{"Hello!"},
				finishReason: llm.FinishReasonStop,
				gate:         gate,
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	// Type hello and press enter — inference blocks on gate.
	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Wait for at least one hint ticker interrupt to confirm the hint is active.
	select {
	case <-deps.interruptCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hint ticker")
	}

	// Verify hint is visible during inference.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"⠙ sending (0s)",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Unblock inference and let it complete.
	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	// After completion, hint should be removed.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"Hello!",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_hint_shows_tool_calling_phase(t *testing.T) {
	t.Parallel()
	blockingTool := &agentMockTool{
		name: "slow_tool",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			<-ctx.Done()
			return agent.ToolResult{Content: "cancelled"}
		},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{""},
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "slow_tool", Arguments: "{}"}},
				},
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{blockingTool}
	flusher := openChatAndGetTab(t, deps)

	// Type hello and press enter — inference completes,
	// then the agent dispatches the blocking tool.
	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Wait for the tool to start executing: the hint ticker fires
	// at 125ms intervals. A 50ms idle timeout returns between ticks.
	// maxWait guarantees we wait past the first tick at least.
	waitUntilIdle(deps.interruptCh, 50*time.Millisecond, 500*time.Millisecond)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"⚙ slow_tool",
				"⠙ tool calling (0s)",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_context_hint_after_turn(t *testing.T) {
	t.Parallel()
	const width = 100

	svc := &agentMockService{
		contextWindow: 100000,
		responses: []agentMockResponse{
			{
				chunks:       []string{"Hi!"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 5000, TokensReceived: 200},
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= width {
			return s
		}
		return s + strings.Repeat(" ", width-n)
	}
	expected := func(lines ...string) string {
		padded := make([]string, len(lines))
		for i, l := range lines {
			padded[i] = pad(l)
		}
		return strings.Join(padded, "\n")
	}

	// After turn, static context hint should be visible.
	handlertest.RunHandlerSequence(t, flusher, width, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: expected(
				"hello",
				"Hi!",
				"",
				"0s · 5.0k tokens sent · context: 5k (5%) · compacts at 85k",
				"",
				"",
				"",
				"        ┌─────────────────────────────────────────────────────────────────────────────────┐",
				"        │▐                                                                                │",
				"        └─────────────────────────────────────────────────────────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_context_hint_after_turn_accumulates_sent_tokens(t *testing.T) {
	t.Parallel()
	const width = 100

	mockTool := &agentMockTool{
		name: "mock_tool",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "ok"}
		},
	}

	svc := &agentMockService{
		contextWindow: 100000,
		responses: []agentMockResponse{
			{
				chunks:       []string{""},
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "mock_tool", Arguments: "{}"}},
				},
				usage: llm.Usage{TokensSent: 3000, TokensReceived: 500},
			},
			{
				chunks:       []string{"answer"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 4000, TokensReceived: 200},
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{mockTool}
	flusher := openChatAndGetTab(t, deps)

	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= width {
			return s
		}
		return s + strings.Repeat(" ", width-n)
	}
	expected := func(lines ...string) string {
		padded := make([]string, len(lines))
		for i, l := range lines {
			padded[i] = pad(l)
		}
		return strings.Join(padded, "\n")
	}

	// "tokens sent" accumulates across completions (3000+4000 = 7000).
	// "context" shows the last completion's window usage (sent 4000 + received 200 = 4200).
	handlertest.RunHandlerSequence(t, flusher, width, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: expected(
				"hello",
				"✓ mock_tool",
				"ok",
				"answer",
				"",
				"0s · 7.0k tokens sent · context: 4k (4%) · compacts at 85k",
				"",
				"        ┌─────────────────────────────────────────────────────────────────────────────────┐",
				"        │▐                                                                                │",
				"        └─────────────────────────────────────────────────────────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_context_hint_cleared_on_next_turn(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		contextWindow: 100000,
		responses: []agentMockResponse{
			{
				chunks:       []string{"First"},
				finishReason: llm.FinishReasonStop,
				usage:        llm.Usage{TokensSent: 5000, TokensReceived: 200},
			},
			{
				chunks:       []string{"Second"},
				finishReason: llm.FinishReasonStop,
				gate:         gate,
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	// First turn: send and wait for completion.
	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	// Second turn: start inference (blocks on gate).
	keys, err = term.ParseKeys("again<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Wait for spinner hint to replace context hint.
	select {
	case <-deps.interruptCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hint ticker")
	}

	// The context hint should be replaced by the spinner.
	// Previous turn's messages are still visible above.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"First",
				"",
				"again",
				"⠙ sending (0s)",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)
}

func TestAIEditorHandler_chat_no_context_hint_without_usage(t *testing.T) {
	t.Parallel()
	// When the mock returns zero usage, no context hint should appear.
	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{"Hello!"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)

	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 0)

	// No context hint — the hint row should be blank.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"Hello!",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_retains_all_reads_for_cache_stability(t *testing.T) {
	t.Parallel()
	// Scenario: LLM calls read_file, then check_file_errors, then
	// read_file again on the same file. All results are retained for
	// prompt caching stability (drops are disabled).
	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "call_r1", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`},
				}},
			},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "call_c1", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "check_file_errors", Arguments: `{"path":"a.txt"}`},
				}},
			},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "call_r2", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`},
				}},
			},
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	tracker := agentools.NewFileTracker()
	readTool := &agentMockTool{
		name: "read_file",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{
				Content:           "file contents",
				DropToolResultIDs: tracker.TrackRead(ctx, "read_file", "/ws/a.txt", ""),
			}
		},
	}
	checkTool := &agentMockTool{
		name: "check_file_errors",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{
				Content:           "no errors",
				DropToolResultIDs: tracker.TrackRead(ctx, "check_file_errors", "/ws/a.txt", ""),
			}
		},
	}
	deps.handler.baseTools = []agent.Tool{readTool, checkTool}

	flusher := openChatAndGetTab(t, deps)
	flusher.Resize(e2eWidth, e2eHeight)

	keys, err := term.ParseKeys("test<space>stale<space>reads<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 4, "expected at least 4 LLM requests")
	finalReq := reqs[3]

	// All tool results should be retained for prompt caching stability.
	var r1Found, c1Found, r2Found bool
	for _, msg := range finalReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_r1" {
			r1Found = true
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_c1" {
			c1Found = true
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_r2" {
			r2Found = true
		}
	}
	assert.True(t, r1Found, "first read_file result should be retained")
	assert.True(t, c1Found, "check_file_errors result should be retained")
	assert.True(t, r2Found, "second read_file result should be retained")
}

func TestAIEditorHandler_chat_applyPatchAutoInjectsDiagnostics(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "call_patch",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "apply_patch",
						Arguments: `{"patch":"*** Begin Patch\n*** Update File: main.go\n@@\n-old\n+new\n*** End Patch"}`,
					},
				}},
			},
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	patchTool := &agentMockTool{
		name: "apply_patch",
		executeFn: func(context.Context, string) agent.ToolResult {
			return agent.ToolResult{
				Content:      "applied 1/1 operations successfully",
				TouchedFiles: []string{"/test/workspace/main.go"},
			}
		},
	}
	diagTool := &agentMockTool{
		name: "check_file_errors",
		executeFn: func(context.Context, string) agent.ToolResult {
			return agent.ToolResult{Content: "2:1 error: undefined: foo [compiler]\n"}
		},
	}
	deps.handler.baseTools = []agent.Tool{patchTool, diagTool}

	flusher := openChatAndGetTab(t, deps)
	flusher.Resize(e2eWidth, e2eHeight)

	keys, err := term.ParseKeys("apply<space>the<space>patch<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 2, "expected at least 2 LLM requests")
	secondReq := reqs[1]

	var patchFound, diagFound, syntheticCallFound bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleAssistant {
			for _, tc := range msg.ToolCalls {
				if tc.ID == "auto-diag-call_patch" && tc.Function.Name == "check_file_errors" {
					syntheticCallFound = true
				}
			}
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_patch" {
			patchFound = true
			assert.Contains(t, msg.Content, "applied 1/1 operations successfully")
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "auto-diag-call_patch" {
			diagFound = true
			assert.Contains(t, msg.Content, "undefined: foo")
		}
	}
	assert.True(t, patchFound, "second request should contain apply_patch tool result")
	assert.True(t, diagFound, "second request should contain injected diagnostics tool result")
	assert.True(t, syntheticCallFound, "assistant tool_calls should include synthetic diagnostics call")

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"✓ apply_patch",
				"applied 1/1 operations successfully",
				"✓ check_file_errors",
				"2:1 error: undefined: foo [compiler]",
				"",
				"done",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_read_file_image(t *testing.T) {
	t.Parallel()
	// Scenario: LLM asks to read an image file via read_file.
	// The real read_file tool detects the image extension, base64-encodes
	// the file, and returns MultiContent. The agent loop injects a
	// synthetic user message with the image data. We verify that the
	// second LLM request contains the base64-encoded image in the
	// expected data-URI format.
	dir := t.TempDir()

	// Create a minimal valid PNG (1×1 red pixel).
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&pngBuf, img))
	pngData := pngBuf.Bytes()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.png"), pngData, 0o644))

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: LLM calls read_file on the image.
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "call_img",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"photo.png","offset":null,"limit":null}`,
					},
				}},
			},
			// 1: Final text response.
			{chunks: []string{"I can see a red pixel."}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Replace nopFileSystem with a real local FS backed by the OS.
	deps.handler.fs = testLocalFS{}
	cwd, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)
	deps.handler.cwd = cwd

	// Wire real tools via DefaultTools so the real read_file processes images.
	tools, _ := agentools.DefaultTools(testLocalFS{}, nopExecutor{}, cwd, agentools.Config{})
	deps.handler.baseTools = tools

	flusher := openChatAndGetTab(t, deps)
	flusher.Resize(e2eWidth, e2eHeight)

	// Send a message to trigger the agent loop.
	keys, err := term.ParseKeys("describe<space>photo.png<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Verify the second LLM request.
	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 2, "expected at least 2 LLM requests")
	secondReq := reqs[1]

	// The tool-role message should contain the short summary.
	var foundToolMsg bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_img" {
			foundToolMsg = true
			assert.Contains(t, msg.Content, "Read image file: photo.png")
			assert.Contains(t, msg.Content, "image/png")
		}
	}
	assert.True(t, foundToolMsg, "second request should contain read_file tool result")

	// A synthetic user message with MultiContent should carry the image.
	wantPrefix := "data:image/png;base64,"
	wantDataURI := wantPrefix + base64.StdEncoding.EncodeToString(pngData)
	var foundImageMsg bool
	for _, msg := range secondReq.Messages {
		if msg.Role != llm.RoleUser || len(msg.MultiContent) == 0 {
			continue
		}
		foundImageMsg = true
		require.Len(t, msg.MultiContent, 2)
		assert.Equal(t, llm.ContentPartTypeText, msg.MultiContent[0].Type)
		assert.Contains(t, msg.MultiContent[0].Text, "Read image file: photo.png")
		assert.Equal(t, llm.ContentPartTypeImageURL, msg.MultiContent[1].Type)
		assert.Equal(t, wantDataURI, msg.MultiContent[1].ImageURL,
			"image data URI must be data:<mime>;base64,<full file>")
	}
	assert.True(t, foundImageMsg,
		"second request should contain a synthetic user message with image MultiContent")
}

func TestAIEditorHandler_chat_read_file_with_spaces_renders_success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fs := testLocalFS{}
	filePath := filepath.Join(dir, "file with spaces.txt")
	f, err := fs.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	require.NoError(t, err)
	_, err = f.Write([]byte("hello spaced world\nsecond line\n"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "call_spaces",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"file with spaces.txt","offset":null,"limit":null}`,
					},
				}},
			},
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.fs = fs
	cwd, err := workspaceapi.CurrentUserHostURI(dir)
	require.NoError(t, err)
	deps.handler.cwd = cwd

	tools, _ := agentools.DefaultTools(fs, nopExecutor{}, cwd, agentools.Config{})
	deps.handler.baseTools = tools
	deps.handler.toolRegistry = agent.NewRegistry(tools...)
	deps.handler.queryAgent = agent.NewAgent(
		svc, deps.handler.toolRegistry, deps.handler.skillRegistry, deps.store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
		},
	)

	flusher := openChatAndGetTab(t, deps)
	flusher.Resize(e2eWidth, e2eHeight)

	keys, err := term.ParseKeys("read<space>the<space>file<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 2, "expected at least 2 LLM requests")
	secondReq := reqs[1]

	var foundToolMsg bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_spaces" {
			foundToolMsg = true
			assert.Contains(t, msg.Content, "L1: hello spaced world")
			assert.Contains(t, msg.Content, "L2: second line")
		}
	}
	assert.True(t, foundToolMsg, "second request should contain read_file tool result")

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"✓ read_file file with spaces.txt",
				"L1: hello spaced world",
				"L2: second line",
				"L3:",
				"",
				"done",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

// nopExecutor implements workspaceapi.Executor as a no-op for tests
// that only exercise read-only tools.
type nopExecutor struct{}

func (nopExecutor) Start(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return 0, fmt.Errorf("nopExecutor: not implemented")
}
func (nopExecutor) Signal(workspaceapi.Pid, syscall.Signal) error { return nil }
func (nopExecutor) Close() error                                  { return nil }

func TestAIEditorHandler_audit_log_with_tool_drops_and_plan_mode(t *testing.T) {
	t.Parallel()
	// Exercises the audit log through a flow that includes:
	//   - Tool result drops (read_file stale reads)
	//   - Plan mode (exit_plan_mode → approve)
	//   - Context clearing
	//
	// Flow (shared mock responses):
	//   0: main agent → skill(plan)
	//   1: sub-agent → read_file(a.txt)
	//   2: sub-agent → read_file(a.txt) again → triggers drop of first read
	//   3: sub-agent → exit_plan_mode
	//   4: sub-agent → final text (after approve + ClearContext)
	//   5: main agent → final text
	//
	// We verify the audit log entries persisted in storage match the
	// expected messages, finish reasons, and tool-drop behavior.

	const planJSON = `## Plan\n1. Do the thing`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main → skill(plan)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-skill",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "skill",
						Arguments: `{"name":"plan","args":"Plan a feature"}`,
					},
				}},
			},
			// 1: sub → read_file
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"a.txt"}`,
					},
				}},
			},
			// 2: sub → read_file again (triggers drop of first)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read2",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"a.txt"}`,
					},
				}},
			},
			// 3: sub → exit_plan_mode
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-exit",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exit_plan_mode",
						Arguments: `{"title":"My plan","plan":"` + planJSON + `"}`,
					},
				}},
			},
			// 4: sub → final text after ClearContext
			{
				chunks:       []string{"plan approved"},
				finishReason: llm.FinishReasonStop,
			},
			// 5: main → final text
			{
				chunks:       []string{"done"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Override GenerateID for predictable sub-agent dialogue ID.
	deps.handler.generateDialogueID = func(_ context.Context, _ string) string { return "test-sub" }
	const subDialogueID = "test-sub"

	// Set up audit store with in-memory backend.
	auditDB := storagestub.NewInMemoryService()
	deps.handler.auditStore = llm.NewAuditStore(auditDB)

	// Register the real plan skill from the repository.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	// Configure agents so the spawner can run sub-agents.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID: "default", Name: "default", Model: "test-model", AllowAny: true,
	}})

	// Add read_file tool with file tracker for stale-read drops.
	tracker := agentools.NewFileTracker()
	deps.handler.baseTools = []agent.Tool{&agentMockTool{
		name: "read_file",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{
				Content:           "file contents",
				DropToolResultIDs: tracker.TrackRead(ctx, "read_file", "/ws/a.txt", ""),
			}
		},
	}}

	deps.handler.plansDir = t.TempDir()

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	// Resize for adequate content display.
	flusher.Resize(60, 20)

	// Step 1: Send user message. The agent processes through
	// skill → sub-agent → read_file × 2 → exit_plan_mode and
	// blocks on the approval prompt.
	keys, err := term.ParseKeys("Plan<space>a<space>feature<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Step 2: Approve the plan (Enter on default "Approve" selection).
	keys, err = term.ParseKeys("<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// --- Assert audit log entries in storage. ---
	ctx := context.Background()

	// Main agent ("default"): 2 completions (skill tool call + final text).
	mainEntries, err := deps.handler.auditStore.Get(ctx, "default")
	require.NoError(t, err)
	require.Len(t, mainEntries, 2, "main agent should have 2 audit entries")

	assert.Equal(t, llm.FinishReasonToolCall, mainEntries[0].FinishReason)
	require.NotNil(t, mainEntries[0].Response)
	require.Len(t, mainEntries[0].Response.ToolCalls, 1)
	assert.Equal(t, "skill", mainEntries[0].Response.ToolCalls[0].Function.Name)

	assert.Equal(t, llm.FinishReasonStop, mainEntries[1].FinishReason)
	require.NotNil(t, mainEntries[1].Response)
	assert.Equal(t, "done", mainEntries[1].Response.Content)

	// Sub-agent: 4 completions (read_file, read_file, exit_plan_mode, final).
	subEntries, err := deps.handler.auditStore.Get(ctx, subDialogueID)
	require.NoError(t, err)
	require.Len(t, subEntries, 4, "sub-agent should have 4 audit entries")

	// Sub entry 0: first read_file call.
	assert.Equal(t, llm.FinishReasonToolCall, subEntries[0].FinishReason)
	require.NotNil(t, subEntries[0].Response)
	require.Len(t, subEntries[0].Response.ToolCalls, 1)
	assert.Equal(t, "read_file", subEntries[0].Response.ToolCalls[0].Function.Name)

	// Sub entry 1: second read_file call.
	assert.Equal(t, llm.FinishReasonToolCall, subEntries[1].FinishReason)

	// Sub entry 2: exit_plan_mode call.
	// Both read_file results should be retained (drops are disabled
	// for prompt caching stability).
	assert.Equal(t, llm.FinishReasonToolCall, subEntries[2].FinishReason)
	var r1Found, r2Found bool
	for _, msg := range subEntries[2].Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "c-read1" {
			r1Found = true
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "c-read2" {
			r2Found = true
		}
	}
	assert.True(t, r1Found, "first read_file result should be retained in exit_plan_mode request")
	assert.True(t, r2Found, "second read_file result should be retained in exit_plan_mode request")

	// Sub entry 3: final text after ClearContext.
	assert.Equal(t, llm.FinishReasonStop, subEntries[3].FinishReason)
	require.NotNil(t, subEntries[3].Response)
	assert.Equal(t, "plan approved", subEntries[3].Response.Content)
	// After ClearContext, the message count should be reduced
	// (only system prompt + compact summary or minimal messages).
	assert.Less(t, len(subEntries[3].Messages), len(subEntries[2].Messages),
		"messages after ClearContext should be fewer than before")
}

func TestCommandAdapterModelSwitchPreservesAudit(t *testing.T) {
	t.Parallel()
	// After /model switches the LLM backend, completions through the
	// swapped service must still be recorded in the audit store.
	inner := &agentMockService{
		responses: []agentMockResponse{
			// First response consumed by the agent during Run.
			{chunks: []string{"hello"}, finishReason: llm.FinishReasonStop},
		},
	}

	auditDB := storagestub.NewInMemoryService()
	auditStore := llm.NewAuditStore(auditDB)

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name: "test-model", Provider: "openai", ContextWindow: 128000,
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{strings: map[string]string{"api_key": "k"}},
		},
	}

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	adapter := &commandAdapter{
		auditStore:    auditStore,
		modelRegistry: reg,
		config:        cfg,
		newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
			return inner
		},
		skillRegistry: skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{}),
	}

	// Create the service the way /model would.
	svc, err := adapter.newService("test-model")
	require.NoError(t, err)

	// Run a completion with an audit dialogue ID.
	ctx := llm.WithAuditDialogueID(context.Background(), "audit-test")
	it, err := svc.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)

	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()

	// The audit store must have recorded the completion.
	entries, err := auditStore.Get(ctx, "audit-test")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "audit entry should be recorded for service created by commandAdapter")
}

func TestAIEditorHandler_chat_model_switch_uses_provider_tools(t *testing.T) {
	t.Parallel()
	// Verifies that after switching to a model with a different provider,
	// the LLM request contains provider-specific tool overrides and
	// excludes tools marked for exclusion, while preserving all other
	// base tools.
	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: response to first message (anthropic model)
			{chunks: []string{"anthropic reply"}, finishReason: llm.FinishReasonStop},
			// 1: response to second message (openai model)
			{chunks: []string{"openai reply"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "anthropic",
		ContextWindow: 128000,
	})
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model-2",
		Provider:      "openai",
		ContextWindow: 64000,
	})

	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai":    stubConfig{strings: map[string]string{"api_key": "test-key"}},
			"anthropic": stubConfig{strings: map[string]string{"api_key": "test-key"}},
		},
	}

	// Base tools: three mock tools with distinct names and descriptions.
	baseTools := []agent.Tool{
		&agentMockTool{name: "read_file", desc: "base read_file"},
		&agentMockTool{name: "search_content", desc: "base search_content"},
		&agentMockTool{name: "bash", desc: "base bash"},
	}

	toolRegistry := agent.NewRegistry(baseTools...)
	// Override search_content for openai with a different description.
	toolRegistry.RegisterOverrides("openai",
		&agentMockTool{name: "search_content", desc: "openai search_content"},
	)
	// Exclude bash for openai.
	toolRegistry.RegisterExclusions("openai", "bash")

	h := &aiEditorHandler{
		defaultModel:      "test-model",
		queryDefaultModel: "test-model",
		modelRegistry:     reg,
		dialogueStore:     store,
		newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
			return svc
		},
		newAnthropicClient: func(string, anthropic.Config, map[string]int) llm.Service {
			return svc
		},
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		baseTools:     baseTools,
		toolRegistry:  toolRegistry,
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig:  agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
			Provider:     "anthropic",
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	deps := testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		svc:         svc,
		interruptCh: interruptCh,
	}

	flusher := openChatAndGetTab(t, deps)

	// Step 1: Send first message with default model (anthropic).
	// Step 2: Switch to openai model.
	// Step 3: Send second message with new model.
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "first<enter>",
			Expected: e2eExpected(0,
				"first",
				"anthropic reply",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "/model<space>test-model-2<enter>",
			Expected: e2eExpected(0,
				"first",
				"anthropic reply",
				"",
				"/model test-model-2",
				"Switched to model test-model-2",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		{
			InputSequence: "do<space>this<enter>",
			Expected: e2eExpected(0,
				"",
				"/model test-model-2",
				"Switched to model test-model-2",
				"",
				"do this",
				"openai reply",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	reqs := svc.getRequests()
	require.Len(t, reqs, 2, "expected exactly 2 LLM requests (one per user message)")

	// Request 0: anthropic provider — should have all 3 base tools,
	// plus session/skill tools. Verify the 3 base tools are present
	// with their base descriptions.
	toolNames0 := make(map[string]string) // name → description
	for _, tool := range reqs[0].Tools {
		toolNames0[tool.Function.Name] = tool.Function.Description
	}
	assert.Equal(t, "base read_file", toolNames0["read_file"],
		"anthropic request should have base read_file")
	assert.Equal(t, "base search_content", toolNames0["search_content"],
		"anthropic request should have base search_content")
	assert.Equal(t, "base bash", toolNames0["bash"],
		"anthropic request should have base bash")

	// Request 1: openai provider — search_content should be overridden,
	// bash should be excluded, read_file should be unchanged.
	toolNames1 := make(map[string]string) // name → description
	for _, tool := range reqs[1].Tools {
		toolNames1[tool.Function.Name] = tool.Function.Description
	}
	assert.Equal(t, "base read_file", toolNames1["read_file"],
		"openai request should have base read_file")
	assert.Equal(t, "openai search_content", toolNames1["search_content"],
		"openai request should have overridden search_content")
	_, hasBash := toolNames1["bash"]
	assert.False(t, hasBash,
		"openai request should NOT have bash (excluded)")
}

func TestAIEditorHandler_exec_command_and_write_stdin_integration(t *testing.T) {
	t.Parallel()
	// Verifies that the openai provider gets exec_command and write_stdin
	// tools (replacing bash), and that the LLM can start a persistent
	// session with exec_command and then interact with it via write_stdin.
	//
	// Flow:
	//   0: LLM calls exec_command → starts "cat" (blocks on stdin, yields session_id 1000)
	//   1: LLM calls write_stdin(session 1000) → sends data, gets echo
	//   2: LLM returns final text response
	const fixedSessionID = 1000

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: call exec_command to start cat (yields quickly)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "call_exec", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "exec_command",
						Arguments: `{"cmd":"cat","yield_time_ms":250}`,
					},
				}},
			},
			// 1: call write_stdin with the known session ID
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "call_write", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "write_stdin",
						Arguments: fmt.Sprintf(`{"session_id":%d,"chars":"hello from LLM\n","yield_time_ms":1000}`, fixedSessionID),
					},
				}},
			},
			// 2: final text
			{chunks: []string{"Process interaction complete"}, finishReason: llm.FinishReasonStop},
		},
	}

	// Build handler with real SessionManager and real exec_command/write_stdin tools.
	store := newTestDialogueStore()
	wm := &capturingWindowManager{}
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "test-model",
		Provider:      "openai",
		ContextWindow: 128000,
	})

	// Use a real temp dir so that exec_command can actually run processes.
	workDir := t.TempDir()
	cwd, err := workspaceapi.ParseURI("file://" + workDir)
	require.NoError(t, err)

	skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
	interruptCh := make(chan struct{}, 100)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	cfg := stubConfig{
		configs: map[string]config.Config{
			"openai": stubConfig{strings: map[string]string{"api_key": "test-key"}},
		},
	}

	// Create real SessionManager with a local executor and
	// deterministic ID + clock so the Expected strings are stable.
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessionMgr := agentools.NewSessionManager(context.Background(), testLocalExec{}, nil)
	sessionMgr.GenerateID = func() int { return fixedSessionID }
	sessionMgr.Clock = func() time.Time { return fixedTime }
	t.Cleanup(func() { _ = sessionMgr.Close() })

	execTool := agentools.NewExecCommand(sessionMgr, cwd)
	writeTool := agentools.NewWriteStdin(sessionMgr)

	toolRegistry := agent.NewRegistry()
	toolRegistry.RegisterOverrides("openai", execTool, writeTool)

	h := &aiEditorHandler{
		defaultModel:      "test-model",
		queryDefaultModel: "test-model",
		modelRegistry:     reg,
		dialogueStore:     store,
		newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
			return svc
		},
		wm:            wm,
		n:             nopNotifications{},
		p:             interrupter,
		clip:          clipboard.NewInMemory(),
		mcpManager:    runemcp.NewManager(),
		sessionMgr:    sessionMgr,
		baseTools:     nil,
		toolRegistry:  toolRegistry,
		systemPrompt:  "test system prompt",
		skillRegistry: skillReg,
		cwd:           cwd,
		fs:            nopFileSystem{},
		config:        cfg,
		resources:     make(map[string]string),
		agentsConfig:  agent.NewConfig(nil),
	}
	h.ctx, h.cancelCtx = context.WithCancel(context.Background())

	h.queryAgent = agent.NewAgent(
		svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
			SystemPrompt: "test query prompt",
			SessionKey:   "query",
			AgentID:      "query",
			Model:        "test-model",
			Provider:     "openai",
		},
	)

	t.Cleanup(func() {
		h.cancelCtx()
		_ = h.mcpManager.Close()
	})

	deps := testAIEditorDeps{
		handler:     h,
		wm:          wm,
		store:       store,
		svc:         svc,
		interruptCh: interruptCh,
	}

	// Open the chat tab and build a flusher with a longer idle
	// timeout — real tool calls (exec_command yield, write_stdin
	// yield) introduce gaps between interrupt signals.
	const pw, ph = 80, 15
	cmd := textapi.Command{Name: commandChat, Window: e2eWindow(0)}
	require.NoError(t, deps.handler.HandleCommand(context.Background(), cmd))
	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	flusher := &asyncFlusher{
		inner:       tab,
		interruptCh: interruptCh,
		idleTimeout: 2 * time.Second,
		maxWait:     15 * time.Second,
	}

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= pw {
			return s
		}
		return s + strings.Repeat(" ", pw-n)
	}
	bl := strings.Repeat(" ", pw)
	_ = bl

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "run<space>cat<enter>",
			Expected: strings.Join([]string{
				pad("run cat"),
				pad(`✓ exec_command cat`),
				pad(`{"session_id":1000,"output":"","wall_time_seconds":0}`),
				pad(`✓ write_stdin session 1000: hello from LLM`),
				pad(`{"session_id":1000,"output":"hello from LLM\n","wall_time_seconds":0}`),
				pad("Process interaction complete"),
				bl,
				bl,
				bl,
				bl,
				bl,
				bl,
				pad("      ┌────────────────────────────────────────────────────────────────┐"),
				pad("      │▐                                                               │"),
				pad("      └────────────────────────────────────────────────────────────────┘"),
			}, "\n"),
		},
	})

	// Also verify the LLM requests for tool correctness.
	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 3, "expected at least 3 LLM requests")

	// Request 0: openai provider tools should include exec_command/write_stdin, not bash.
	toolNames := make(map[string]bool)
	for _, tool := range reqs[0].Tools {
		toolNames[tool.Function.Name] = true
	}
	assert.True(t, toolNames["exec_command"],
		"openai request should have exec_command tool")
	assert.True(t, toolNames["write_stdin"],
		"openai request should have write_stdin tool")
	assert.False(t, toolNames["bash"],
		"openai request should NOT have bash (excluded by exec_command)")

	// Request 1: exec_command tool result contains session_id.
	var execResult struct {
		SessionID int `json:"session_id"`
	}
	for _, msg := range reqs[1].Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_exec" {
			require.NoError(t, json.Unmarshal([]byte(msg.Content), &execResult))
			break
		}
	}
	assert.Equal(t, fixedSessionID, execResult.SessionID,
		"exec_command should return the deterministic session_id")

	// Request 2: write_stdin tool result contains echoed output.
	var writeResult struct {
		Output string `json:"output"`
	}
	for _, msg := range reqs[2].Messages {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_write" {
			require.NoError(t, json.Unmarshal([]byte(msg.Content), &writeResult))
			break
		}
	}
	assert.Contains(t, writeResult.Output, "hello from LLM",
		"write_stdin result should contain the echoed input")
}

// testLocalExec implements workspaceapi.Executor using os/exec for handler tests.
type testLocalExec struct{}

func (testLocalExec) Start(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Dir = cmd.Dir
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	c.Stderr = cmd.Stderr
	c.Env = cmd.Env

	if err := c.Start(); err != nil {
		return 0, err
	}
	pid := workspaceapi.Pid(c.Process.Pid)

	go func() {
		err := c.Wait()
		if cmd.Watcher != nil {
			cmd.Watcher.WatchProcess() <- err
		}
	}()

	return pid, nil
}

func (testLocalExec) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (testLocalExec) Close() error { return nil }

func TestAIEditorHandler_chat_request_user_input_renders_prompt(t *testing.T) {
	t.Parallel()
	// Exercises the OpenAI-flavored request_user_input tool end-to-end:
	//
	//   1. User sends a message → agent calls request_user_input
	//   2. Tool blocks on prompter → inline selection UI appears
	//   3. User selects an option (Enter) → tool completes
	//   4. Agent receives result → produces final text response
	//
	// We verify:
	//   - Prompt tool call renders with "?" prefix (not spinner)
	//   - Prompt options render correctly
	//   - After selection, tool completes and final response appears

	const requestUserInputArgs = `{"questions":[{"id":"db","header":"DB","question":"Which DB?","options":[{"label":"Postgres","description":"Mature"},{"label":"SQLite","description":"Light"}]}]}`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent calls request_user_input
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-rui",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "request_user_input",
						Arguments: requestUserInputArgs,
					},
				}},
			},
			// 1: agent final text after receiving user choice
			{
				chunks:       []string{"You picked Postgres."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.plansDir = t.TempDir()

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 16

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Step 1: Send message. Agent calls request_user_input,
		// tool blocks on prompt → inline selection UI appears.
		// Verify: "?" prefix, prompt title with [DB] header,
		// options with "Other" auto-appended.
		{
			InputSequence: "choose<space>a<space>db<enter>",
			Expected: "" +
				"choose a db                                                 \n" +
				"? request_user_input Which DB?                              \n" +
				"questions=[{\"header\":\"DB\",\"id\":\"db\",\"options\":[{\"description\n" +
				"\":\"Mature\",\"label\":\"Postgres\"},{\"descrip...                 \n" +
				"Which DB?                                               [DB]\n" +
				"                                                            \n" +
				"> Postgres                                                  \n" +
				"      Mature                                                \n" +
				"  SQLite                                                    \n" +
				"      Light                                                 \n" +
				"  Other                                                     \n" +
				"      None of the above                                     \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
		// Step 2: Press Enter to select "Postgres" (first option).
		// Tool completes with ✓ prefix, result JSON shown,
		// then final text response.
		{
			InputSequence: "<enter>",
			Expected: "" +
				"choose a db                                                 \n" +
				"✓ request_user_input Which DB?                              \n" +
				"{\"answers\":{\"db\":{\"answers\":[\"Postgres\"]}}}                 \n" +
				"You picked Postgres.                                        \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})
}

func TestAIEditorHandler_chat_request_user_input_free_form(t *testing.T) {
	t.Parallel()
	// Exercises the request_user_input tool with empty options (free-form
	// text input) end-to-end:
	//
	//   1. User sends a message → agent calls request_user_input with options=[]
	//   2. Tool blocks on prompter → question rendered in chat, main inputbox active
	//   3. User types a response and presses Enter → tool completes
	//   4. Agent receives result → produces final text response
	//
	// We verify:
	//   - Prompt tool call renders with "?" prefix
	//   - Question text appears in messages area
	//   - Cursor is visible (main inputbox mode, not hidden selection)
	//   - After typing and Enter, tool completes with ✓ prefix
	//   - Typed answer appears as sent message
	//   - Final agent response appears

	const freeFormArgs = `{"questions":[{"id":"name","header":"Name","question":"What is your name?","options":[]}]}`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent calls request_user_input with empty options
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-ff",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "request_user_input",
						Arguments: freeFormArgs,
					},
				}},
			},
			// 1: agent final text after receiving user's typed answer
			{
				chunks:       []string{"Hello, Alice!"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.plansDir = t.TempDir()

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 12

	// Step 1: Send message, agent calls request_user_input with empty options.
	// The question appears in messages, inputbox stays active.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Verify: "?" prefix, question text with [Name] header rendered
		// as a receive message, no selection widget, cursor visible.
		{
			InputSequence: "greet<space>me<enter>",
			Expected: "" +
				"greet me                                                    \n" +
				"? request_user_input What is your name?                     \n" +
				"questions=[{\"header\":\"Name\",\"id\":\"name\",\"options\":[],\"questi\n" +
				"on\":\"What is your name?\"}]                                  \n" +
				"Name: What is your name?                                    \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	// Verify cursor is visible (free-form prompt uses main inputbox).
	_, _, cursorOK := flusher.Cursor()
	assert.True(t, cursorOK, "cursor should be visible during free-form prompt")

	// Step 2: Type "Alice" and press Enter.
	// Tool completes with ✓ prefix, typed answer appears as sent message,
	// result JSON and final response shown.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "Alice<enter>",
			Expected: "" +
				"greet me                                                    \n" +
				"✓ request_user_input What is your name?                     \n" +
				"{\"answers\":{\"name\":{\"answers\":[\"Alice\"]}}}                  \n" +
				"Alice                                                       \n" +
				"Hello, Alice!                                               \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

}

func TestAIEditorHandler_chat_request_user_input_free_form_esc_dismisses(t *testing.T) {
	t.Parallel()
	// Exercises Esc dismissal during a free-form request_user_input prompt:
	//
	//   1. User sends a message → agent calls request_user_input with options=[]
	//   2. Tool blocks on prompter → question rendered, inputbox active
	//   3. User presses Esc → prompt dismissed, tool returns error
	//   4. Agent receives error → produces error-handling response

	const freeFormArgs = `{"questions":[{"id":"name","header":"Name","question":"What is your name?","options":[]}]}`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent calls request_user_input with empty options
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-ff",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "request_user_input",
						Arguments: freeFormArgs,
					},
				}},
			},
			// 1: agent handles the dismissal error
			{
				chunks:       []string{"No worries."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.plansDir = t.TempDir()

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 12

	// Step 1: Send message, wait for prompt.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Verify: "?" prefix, question rendered, cursor visible.
		{
			InputSequence: "ask<space>me<enter>",
			Expected: "" +
				"ask me                                                      \n" +
				"? request_user_input What is your name?                     \n" +
				"questions=[{\"header\":\"Name\",\"id\":\"name\",\"options\":[],\"questi\n" +
				"on\":\"What is your name?\"}]                                  \n" +
				"Name: What is your name?                                    \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	// Step 2: Press Esc to dismiss the prompt.
	// The tool receives an error and the agent handles it gracefully.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Verify: "✗" prefix (error), dismissal message, final response.
		{
			InputSequence: "<esc>",
			Expected: "" +
				"ask me                                                      \n" +
				"✗ request_user_input What is your name?                     \n" +
				"prompt dismissed: prompt dismissed                          \n" +
				"No worries.                                                 \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})
}

func TestAIEditorHandler_chat_ask_user_question_hint_survives(t *testing.T) {
	t.Parallel()
	// Exercises the full prompt-tool cycle with a gated follow-up inference
	// to verify the status hint (spinner) is still visible after the user
	// answers a prompt and the turn is NOT yet done.
	//
	//   1. User sends "hi" → agent calls request_user_input
	//   2. Tool blocks on prompter → inline selection UI appears (hint saved)
	//   3. User presses Enter → tool completes, hint must be restored
	//   4. Agent starts second inference which blocks on gate
	//   5. We verify the hint is active (ticker fires interrupts)
	//   6. Unblock gate → turn completes
	//
	// This covers ask_user_question / request_user_input / request_skill
	// which all share the same AddPrompt → removePrompt → hint restore path.

	const askArgs = `{"questions":[{"id":"color","header":"Color","question":"Which color?","options":[{"label":"Red","description":"Warm"},{"label":"Blue","description":"Cool"}]}]}`

	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		contextWindow: 200000,
		responses: []agentMockResponse{
			// 0: agent calls request_user_input
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-ask",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "request_user_input",
						Arguments: askArgs,
					},
				}},
				usage: llm.Usage{TokensSent: 500, TokensReceived: 50},
			},
			// 1: second inference blocks on gate, then produces text
			{
				chunks:       []string{"You chose Red."},
				finishReason: llm.FinishReasonStop,
				gate:         gate,
				usage:        llm.Usage{TokensSent: 600, TokensReceived: 30},
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.cfg.DurationPrecision = time.Minute
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	const pw, ph = 100, 15

	// Step 1: Send message → prompt appears.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "hi<enter>",
			Expected: "" +
				"hi                                                                                                  \n" +
				"? request_user_input Which color?                                                                   \n" +
				"questions=[{\"header\":\"Color\",\"id\":\"color\",\"options\":[{\"description\":\"Warm\",\"label\":\"Red\"},{\"descript\n" +
				"...                                                                                                 \n" +
				"Which color?                                                                                 [Color]\n" +
				"                                                                                                    \n" +
				"> Red                                                                                               \n" +
				"      Warm                                                                                          \n" +
				"  Blue                                                                                              \n" +
				"      Cool                                                                                          \n" +
				"  Other                                                                                             \n" +
				"      None of the above                                                                             \n" +
				"        ┌─────────────────────────────────────────────────────────────────────────────────┐         \n" +
				"        │                                                                                 │         \n" +
				"        └─────────────────────────────────────────────────────────────────────────────────┘         ",
		},
	})

	// Step 2: Press Enter to select "Red". The prompt dismisses,
	// hint is restored, and the second inference blocks on the gate.
	// The flusher returns after maxWait because the hint ticker
	// keeps sending interrupts. The spinner must be visible.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "<enter>",
			Expected: "" +
				"hi                                                                                                  \n" +
				"✓ request_user_input Which color?                                                                   \n" +
				"{\"answers\":{\"color\":{\"answers\":[\"Red\"]}}}                                                           \n" +
				"⠙ sending (0s · ↑ 500 tokens · ↓ 50 tokens)                                                         \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"        ┌─────────────────────────────────────────────────────────────────────────────────┐         \n" +
				"        │▐                                                                                │         \n" +
				"        └─────────────────────────────────────────────────────────────────────────────────┘         ",
		},
	})

	// Step 3: Unblock the gate → turn completes. The spinner is
	// replaced by the context usage line.
	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 2*time.Second)

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: "" +
				"hi                                                                                                  \n" +
				"✓ request_user_input Which color?                                                                   \n" +
				"{\"answers\":{\"color\":{\"answers\":[\"Red\"]}}}                                                           \n" +
				"You chose Red.                                                                                      \n" +
				"                                                                                                    \n" +
				"0s · 1.1k tokens sent · context: 630 (0%) · compacts at 170k                                        \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"                                                                                                    \n" +
				"        ┌─────────────────────────────────────────────────────────────────────────────────┐         \n" +
				"        │▐                                                                                │         \n" +
				"        └─────────────────────────────────────────────────────────────────────────────────┘         ",
		},
	})
}

// TestAIEditorHandler_chat_request_skill_install exercises the full
// request_skill → install → reload → use flow:
//
//  1. User sends a message → agent calls request_skill("web_search")
//  2. Tool blocks on prompter → inline selection UI appears
//  3. User selects "Done" → mock FS is updated with web_search/SKILL.md
//  4. Agent loop iteration → Reload() picks up the new skill
//  5. Agent calls skill("web_search") → skill is loaded successfully
//  6. Agent produces final text response
//
// We verify:
//   - Prompt tool call renders with "?" prefix and options
//   - After "Done", tool completes with ✓ prefix
//   - The skill tool succeeds (not "unknown skill")
//   - Final response text appears
func TestAIEditorHandler_chat_request_skill_install(t *testing.T) {
	t.Parallel()
	const requestSkillArgs = `{"skill":"web_search","description":"Search the web"}`

	// The skill tool call that the agent makes after the user installs web_search.
	const useSkillArgs = `{"name":"web_search","args":null}`

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: agent calls request_skill
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-rs",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "request_skill",
						Arguments: requestSkillArgs,
					},
				}},
			},
			// 1: after user says "Done", agent tries to use the skill
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-skill",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "skill",
						Arguments: useSkillArgs,
					},
				}},
			},
			// 2: agent final text
			{
				chunks:       []string{"Web search is ready."},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	// Mutable mock filesystem. Initially the skills directory is empty.
	// After the user selects "Done", we add web_search/SKILL.md.
	skillsDir := "/test/skills"
	mockFS := &mutableMockFS{
		dirs: map[string][]os.DirEntry{
			skillsDir: {}, // initially empty
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.plansDir = t.TempDir()

	// Replace the skill registry with one backed by the mock FS that
	// tracks our skills directory.
	cwd, _ := workspaceapi.ParseURI("file:///test/workspace")
	deps.handler.skillRegistry = skills.NewRegistry(
		mockFS, cwd, []string{skillsDir}, nil,
	)

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const pw, ph = 60, 20

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Step 1: Send message → request_skill prompt appears.
		// Verify: "?" prefix, prompt body, Done/Won't do options.
		{
			InputSequence: "need<space>web_search<enter>",
			Expected: "" +
				"need web_search                                             \n" +
				"? request_skill web_search                                  \n" +
				"description=Search the web skill=web_search                 \n" +
				"Search the web                                              \n" +
				"                                                            \n" +
				"Skill \"web_search\" is not available                  [Skill]\n" +
				"                                                            \n" +
				"> Done                                                      \n" +
				"      I have installed the skill                            \n" +
				"  Won't do                                                  \n" +
				"      Skip this — continue without it                       \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │                                                │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})

	// Simulate out-of-band skill installation: add web_search/SKILL.md
	// to the mock FS so the next Reload() finds it.
	mockFS.installSkill(skillsDir, "web_search", `---
name: web_search
description: Search the web
---
You can search the web with this skill.`)

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		// Step 2: Press Enter to select "Done".
		// request_skill completes with ✓, agent calls skill("web_search")
		// which succeeds because Reload() picked up the new SKILL.md
		// from the mock FS. Skill content and final text appear.
		{
			InputSequence: "<enter>",
			Expected: "" +
				"need web_search                                             \n" +
				"✓ request_skill web_search                                  \n" +
				"The user has installed the \"web_search\" skill. It should    \n" +
				"now be available — check the skills list and proceed.       \n" +
				"✓ skill web_search                                          \n" +
				"<skill_content name=\"web_search\">                           \n" +
				"You can search the web with this skill.                     \n" +
				"                                                            \n" +
				"Skill directory: /test/skills/web_search                    \n" +
				"Relative paths in this skill are relative to the skill      \n" +
				"directory.                                                  \n" +
				"...                                                         \n" +
				"Web search is ready.                                        \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"                                                            \n" +
				"     ┌────────────────────────────────────────────────┐     \n" +
				"     │▐                                               │     \n" +
				"     └────────────────────────────────────────────────┘     ",
		},
	})
}

// mutableMockFS is a thread-safe mock FileSystem whose contents can be
// updated at runtime to simulate out-of-band skill installation.
type mutableMockFS struct {
	mu    sync.Mutex
	dirs  map[string][]os.DirEntry // dir path → entries
	files map[string][]byte        // file path → content
}

func (m *mutableMockFS) installSkill(parentDir, name, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	// Add the skill subdirectory entry to the parent.
	m.dirs[parentDir] = append(m.dirs[parentDir], mockDirEntry{name: name})
	// Store the SKILL.md content.
	m.files[filepath.Join(parentDir, name, "SKILL.md")] = []byte(content)
}

func (m *mutableMockFS) ReadDir(name string) ([]os.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, ok := m.dirs[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return entries, nil
}

func (m *mutableMockFS) OpenFile(path string, _ int, _ os.FileMode) (workspaceapi.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &mockFile{data: data}, nil
}

func (m *mutableMockFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.URI{}, nil
}
func (m *mutableMockFS) Remove(string) error                { return nil }
func (m *mutableMockFS) Stat(string) (os.FileInfo, error)   { return nil, os.ErrNotExist }
func (m *mutableMockFS) MkdirAll(string, os.FileMode) error { return nil }

// mockDirEntry implements os.DirEntry for a directory.
type mockDirEntry struct{ name string }

func (e mockDirEntry) Name() string               { return e.name }
func (e mockDirEntry) IsDir() bool                { return true }
func (e mockDirEntry) Type() os.FileMode          { return os.ModeDir }
func (e mockDirEntry) Info() (os.FileInfo, error) { return nil, nil }

// mockFile is a minimal in-memory File for reading skill content.
type mockFile struct {
	data   []byte
	offset int
}

func (f *mockFile) Read(p []byte) (int, error) {
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += n
	return n, nil
}

func (f *mockFile) Close() error                      { return nil }
func (f *mockFile) Name() string                      { return "" }
func (f *mockFile) Stat() (os.FileInfo, error)        { return nil, nil }
func (f *mockFile) Sync() error                       { return nil }
func (f *mockFile) Truncate(int64) error              { return nil }
func (f *mockFile) Fd() uintptr                       { return 0 }
func (f *mockFile) Seek(int64, int) (int64, error)    { return 0, nil }
func (f *mockFile) Write([]byte) (int, error)         { return 0, nil }
func (f *mockFile) ReadAt([]byte, int64) (int, error) { return 0, nil }

// TestAIEditorHandler_chat_prompt_cache_key verifies that the agent
// sets PromptCacheKey on every LLM request and sends the full
// conversation history each iteration (no delta optimization).
func TestAIEditorHandler_chat_prompt_cache_key(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "c1", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "my_tool", Arguments: `{}`},
				}},
			},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID: "c2", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "my_tool", Arguments: `{}`},
				}},
			},
			{
				chunks:       []string{"all done"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{&agentMockTool{
		name: "my_tool",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "tool output"}
		},
	}}

	flusher := openChatAndGetTab(t, deps)
	flusher.Resize(e2eWidth, e2eHeight)

	keys, err := term.ParseKeys("go<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	reqs := svc.getRequests()
	require.Equal(t, 3, len(reqs), "expected 3 LLM requests")

	// Every request must carry the same PromptCacheKey (the dialogue ID).
	for i, req := range reqs {
		assert.NotEmpty(t, req.PromptCacheKey,
			"request %d must have a PromptCacheKey", i)
	}
	assert.Equal(t, reqs[0].PromptCacheKey, reqs[1].PromptCacheKey)
	assert.Equal(t, reqs[1].PromptCacheKey, reqs[2].PromptCacheKey)

	// All requests send full history — request 2 must include
	// user, assistant, and tool messages (not just a delta).
	var hasUser, hasAssistant, hasTool bool
	for _, msg := range reqs[2].Messages {
		switch msg.Role {
		case llm.RoleUser:
			hasUser = true
		case llm.RoleAssistant:
			hasAssistant = true
		case llm.RoleTool:
			hasTool = true
		}
	}
	assert.True(t, hasUser, "request must include user messages")
	assert.True(t, hasAssistant, "request must include assistant messages")
	assert.True(t, hasTool, "request must include tool messages")
}

// mockMemoryRecaller is a test double for agent.MemoryRecaller.
type mockMemoryRecaller struct {
	memories []agent.Memory
	delay    time.Duration // simulated recall latency
}

func (m *mockMemoryRecaller) Recall(_ context.Context, _ []string, _ string) ([]agent.Memory, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.memories, nil
}

func TestAgent_MemoryInjectedOncePerRun(t *testing.T) {
	t.Parallel()

	echoTool := &agentMockTool{
		name: "echo",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "echoed"}
		},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			// Turn 1: tool call
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "echo", Arguments: "{}"}},
				},
			},
			// Turn 2: tool call
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c2", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "echo", Arguments: "{}"}},
				},
			},
			// Turn 3: tool call
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c3", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "echo", Arguments: "{}"}},
				},
			},
			// Turn 4: final stop
			{chunks: []string{"Done!"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry(echoTool)
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	memRecaller := &mockMemoryRecaller{memories: []agent.Memory{
		{ID: "handler-refactor", Content: "When renaming handler methods, update mock registry."},
	}}
	ag := agent.NewAgent(svc, registry, skillReg, store, memRecaller, agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		memRecaller, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent, 50)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
	}()

	reqRx <- completionRequest{msg: "do the thing", ctx: ctx}

	// Drain events until the agent signals completion.
	timeout := time.After(10 * time.Second)
	sawBreak := false
	for !sawBreak {
		select {
		case ev := <-tx:
			if ev.Type == dialoguetui.MessageEventBreak {
				sawBreak = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for completion")
		}
	}

	cancel()
	<-doneCh

	reqs := svc.getRequests()
	require.Len(t, reqs, 4, "expected 4 LLM requests (3 tool calls + 1 stop)")

	// Memory is recalled once per Run() and prepended transiently to the
	// last message in reqMessages each iteration. It must not accumulate
	// across iterations — each LLM request must contain exactly one message
	// with <memory-context>.
	wantContent := agent.FormatMemoryContent(memRecaller.memories)
	for i, req := range reqs {
		count := 0
		for _, msg := range req.Messages {
			if strings.Contains(msg.Content, "<memory-context>") {
				count++
				assert.Contains(t, msg.Content, wantContent,
					"request %d: injected memory must match FormatMemoryContent output", i)
			}
		}
		assert.Equal(t, 1, count,
			"request %d: expected exactly 1 message with <memory-context>, got %d", i, count)
	}
}

// TestAgent_MemoryRecallRenderedInTree verifies that recalled memories
// appear in the collapsed TUI tree as a parent "memory recall" node
// with individual memory entries nested underneath, and that the
// memories are injected into LLM requests.
func TestAgent_MemoryRecallRenderedInTree(t *testing.T) {
	t.Parallel()
	memories := []agent.Memory{
		{ID: "handler-refactor", Content: "(coding) Update mock registry."},
		{ID: "iterator-bug", Content: "(debugging) Close before reuse."},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"Done!"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry()
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	memRecaller := &mockMemoryRecaller{memories: memories}
	ag := agent.NewAgent(svc, registry, skillReg, store, memRecaller, agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		memRecaller, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent, 50)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{StartCollapsed: true})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
	}()

	reqRx <- completionRequest{msg: "hello", ctx: ctx}

	// Drain events until the agent signals completion, feeding each
	// event into the component so it builds its rendering state.
	timeout := time.After(10 * time.Second)
	sawBreak := false
	for !sawBreak {
		select {
		case ev := <-tx:
			mu.Lock()
			switch ev.Type {
			case dialoguetui.MessageEventMemoryRecall:
				comp.AddMemoryRecall(ev.Memories, ev.MemoryDuration)
			case dialoguetui.MessageEventText:
				comp.AddReceiveMessageChunk(ev.Text)
			case dialoguetui.MessageEventBreak:
				comp.AddReceiveMessageBreak()
				sawBreak = true
			}
			mu.Unlock()
		case <-timeout:
			t.Fatal("timed out waiting for completion")
		}
	}

	cancel()
	<-doneCh

	// Render and verify the collapsed tree.
	mu.Lock()
	w := term.NewStringWriter(40, 12)
	comp.Resize(40, 12)
	comp.Draw(w)
	mu.Unlock()
	require.NoError(t, w.Flush())

	out := w.String()
	lines := strings.Split(out, "\n")
	require.True(t, len(lines) >= 6, "expected at least 6 lines, got %d:\n%s", len(lines), out)

	// The collapsed tree should show the memory recall parent
	// with children nested underneath:
	//   └─ 󰍛 memory recall <dur>
	//      ├─ 󰍛 handler-refactor
	//      └─ 󰍛 iterator-bug
	//   Press <ctrl-o> to expand
	//
	// NOTE: assert.Contains is used here (instead of exact Expected
	// strings via comptest/handlertest) because the parent node
	// includes a non-deterministic duration suffix (e.g. "2µs").
	// Do NOT copy this pattern for rendering tests where all values
	// are deterministic — use exact Expected strings instead.
	assert.Contains(t, lines[0], "└─ 󰍛 memory recall")
	assert.Contains(t, lines[1], "├─ 󰍛 handler-refactor")
	assert.Contains(t, lines[2], "└─ 󰍛 iterator-bug")
	assert.Contains(t, lines[3], "Press <ctrl-o> to expand")

	// Verify the LLM response text appears after the tree.
	assert.Contains(t, out, "Done!")

	// Verify memory content was injected into the LLM request.
	reqs := svc.getRequests()
	require.NotEmpty(t, reqs)
	wantContent := agent.FormatMemoryContent(memories)
	found := false
	for _, msg := range reqs[0].Messages {
		if strings.Contains(msg.Content, "<memory-context>") {
			found = true
			assert.Contains(t, msg.Content, wantContent)
		}
	}
	assert.True(t, found, "LLM request must contain <memory-context>")
}

func TestTuiPrompter_cancel_dismisses_prompt(t *testing.T) {
	t.Parallel()
	tx := make(chan dialoguetui.MessageEvent, 2)
	tp := &tuiPrompter{tx: tx, noti: nopNotifications{}}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := tp.Prompt(ctx, agent.PromptRequest{
			Title: "Plan ready",
			Options: []agent.PromptOption{
				{Label: "Approve", Value: "approve"},
			},
		})
		errCh <- err
	}()

	// Consume the prompt event.
	<-tx

	// Explicit cancel (e.g. session teardown).
	cancel()

	err := <-errCh
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCreateAgentCompletions_NotifiesTurnCompleted(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"Hello!"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry()
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		nil, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}
	noti := &capturingNotifications{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, noti, nil)
	}()

	// Drain tx concurrently to prevent blocking on the deferred break.
	go func() {
		for range tx {
		}
	}()

	reqRx <- completionRequest{msg: "hello", ctx: ctx}

	// The notification fires synchronously after the iterator is exhausted,
	// before the deferred break. Wait for it, then shut down.
	require.Eventually(t, func() bool {
		noti.mu.Lock()
		defer noti.mu.Unlock()
		return len(noti.notified) > 0
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	<-done

	noti.mu.Lock()
	defer noti.mu.Unlock()
	require.Len(t, noti.notified, 1)
	assert.Equal(t, "Turn completed", noti.notified[0])
	assert.Equal(t, browserapi.LevelSuccess, noti.levels[0])
}

func TestCreateAgentCompletions_NoNotificationOnCancel(t *testing.T) {
	t.Parallel()
	blockingTool := &agentMockTool{
		name: "slow_tool",
		executeFn: func(ctx context.Context, _ string) agent.ToolResult {
			<-ctx.Done()
			return agent.ToolResult{Content: "cancelled"}
		},
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				chunks:       []string{""},
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "slow_tool", Arguments: "{}"}},
				},
			},
			{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry(blockingTool)
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		nil, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}
	noti := &capturingNotifications{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, noti, nil)
	}()

	reqCtx, reqCancel := context.WithCancel(ctx)
	reqRx <- completionRequest{msg: "hello", ctx: reqCtx}

	// Wait for tool call, then cancel the request (simulates Ctrl-C).
	for ev := range tx {
		if ev.Type == dialoguetui.MessageEventToolCall {
			break
		}
	}
	reqCancel()

	// Drain remaining events, then shut down.
	go func() {
		for range tx {
		}
	}()
	cancel()
	<-done

	noti.mu.Lock()
	defer noti.mu.Unlock()
	assert.Empty(t, noti.notified, "no turn-done notification should be sent on cancellation")
}

func TestCreateAgentCompletions_NotifiesOnError(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{
		responses: []agentMockResponse{
			{err: fmt.Errorf("connection refused")},
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry()
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		nil, "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}
	noti := &capturingNotifications{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, noti, nil)
	}()

	go func() {
		for range tx {
		}
	}()

	reqRx <- completionRequest{msg: "hello", ctx: ctx}

	require.Eventually(t, func() bool {
		noti.mu.Lock()
		defer noti.mu.Unlock()
		return len(noti.notified) > 0
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	<-done

	noti.mu.Lock()
	defer noti.mu.Unlock()
	require.NotEmpty(t, noti.notified)
	assert.Contains(t, noti.notified[0], "connection refused")
	assert.Equal(t, browserapi.LevelError, noti.levels[0])
	// No success notification should follow an error.
	for i, level := range noti.levels {
		assert.NotEqual(t, browserapi.LevelSuccess, level, "unexpected success notification at index %d: %s", i, noti.notified[i])
	}
}

func TestTuiPrompter_NotifiesInputRequired(t *testing.T) {
	t.Parallel()
	tx := make(chan dialoguetui.MessageEvent, 2)
	noti := &capturingNotifications{}
	tp := &tuiPrompter{tx: tx, noti: noti}

	ctx := context.Background()

	resCh := make(chan agent.PromptResponse, 1)
	go func() {
		resp, _ := tp.Prompt(ctx, agent.PromptRequest{
			Title: "Plan approval",
			Options: []agent.PromptOption{
				{Label: "Approve", Value: "approve"},
				{Label: "Reject", Value: "reject"},
			},
		})
		resCh <- resp
	}()

	// Consume the prompt event and answer it.
	ev := <-tx
	require.Equal(t, dialoguetui.MessageEventPrompt, ev.Type)
	ev.PromptResult <- []string{"Approve"}

	<-resCh

	noti.mu.Lock()
	defer noti.mu.Unlock()
	require.Len(t, noti.notified, 1)
	assert.Equal(t, "Input required: Plan approval", noti.notified[0])
	assert.Equal(t, browserapi.LevelWarn, noti.levels[0])
}

func TestAIEditorHandler_agent_tool_resolves_skill(t *testing.T) {
	t.Parallel()
	// Exercises the agent tool's skill fallback: when the LLM
	// calls the agent tool with a subagent_type that matches an
	// agent-type skill (case-insensitive), the spawner resolves
	// it via the skill registry and runs a sub-agent.
	//
	//   1. User sends message → main agent calls `agent` tool
	//      with subagent_type="Explore" (uppercase to test case folding)
	//   2. Spawner falls back to skill registry → finds "explore" skill
	//   3. Sub-agent calls read_file (exploring)
	//   4. Sub-agent returns findings → agent tool returns result
	//   5. Main agent produces final response
	//
	// We verify:
	//   - The sub-agent is spawned despite "Explore" not being
	//     a configured agent type
	//   - The skill's body is used as the sub-agent's system prompt
	//   - The sub-agent's reply flows back through the agent tool
	//   - The main agent's final text is rendered in the TUI

	svc := &agentMockService{
		responses: []agentMockResponse{
			// 0: main agent → calls the agent tool
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-agent",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "agent",
						Arguments: `{"description":"explore code","prompt":"find the entry point","subagent_type":"Explore"}`,
					},
				}},
			},
			// 1: sub-agent → read_file (exploring)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c-read",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			// 2: sub-agent → returns findings
			{
				chunks:       []string{"Entry point is cmd/main.go"},
				finishReason: llm.FinishReasonStop,
			},
			// 3: main agent → final response after receiving agent tool result
			{
				chunks:       []string{"The entry point is in cmd/main.go"},
				finishReason: llm.FinishReasonStop,
			},
		},
	}

	deps := newTestAIEditorHandler(t, svc)

	// Register the real explore skill from the repository.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoSkillsDir := filepath.Join(filepath.Dir(thisFile), "..", "skills")
	_, err := deps.handler.skillRegistry.AddDir(repoSkillsDir)
	require.NoError(t, err)

	// Configure agents — "Explore" is NOT in the config, so the
	// spawner must fall back to the skill registry.
	deps.handler.agentsConfig = agent.NewConfig([]agent.Definition{{
		ID:       "default",
		Name:     "default",
		Model:    "test-model",
		AllowAny: true,
	}})

	// Add mock read_file tool.
	deps.handler.baseTools = []agent.Tool{&agentMockTool{
		name: "read_file",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "package main\nfunc main() {}"}
		},
	}}

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "find<space>entry<space>point<enter>",
			Expected: e2eExpected(0,
				"✓ agent explore code",
				"Entry point is cmd/main.go",
				"✓ read_file",
				"package main",
				"func main() {}",
				"The entry point is in cmd/main.go",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Verify the sub-agent received the explore skill body as
	// its system prompt.
	reqs := svc.getRequests()
	require.GreaterOrEqual(t, len(reqs), 2,
		"need at least main agent + sub-agent requests")
	// Sub-agent request (index 1) should have the explore skill
	// body in its system message.
	assert.Contains(t, reqs[1].Messages[0].Content,
		"You are a fast, read-only code research specialist.")
}

// TestAgent_PartialReasoningPreservedOnContinue verifies that when the
// first LLM response finishes with FinishReasonLength (output truncated)
// and the assistant message contains only reasoning (no text content),
// the conversation can still continue. The partial reasoning must be
// carried forward so the LLM can resume, and the message must not
// trigger an API rejection for empty text content blocks.
func TestAgent_PartialReasoningPreservedOnContinue(t *testing.T) {
	t.Parallel()
	const partialReasoning = "Let me analyze this step by step..."

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				reasoningChunks: []string{partialReasoning},
				// No text chunks — only reasoning was produced before
				// the output was truncated by FinishReasonLength.
				finishReason: llm.FinishReasonLength,
			},
			{
				chunks:       []string{"...doing X and Y."},
				finishReason: llm.FinishReasonStop,
			},
		},
		// Simulate the real API: reject requests that contain
		// assistant messages with empty text content blocks.
		validateRequest: func(req llm.Request) error {
			for _, msg := range req.Messages {
				if msg.Role == llm.RoleAssistant && msg.Content == "" {
					return fmt.Errorf(
						"400: Bad Request: type: invalid_request_error, " +
							"message: text content blocks must be non-empty")
				}
			}
			return nil
		},
	}

	store := newTestDialogueStore()
	registry := agent.NewRegistry()
	skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "test"})

	spawner := agent.NewGoroutineSpawner(
		store, func(string) (llm.Service, string, error) { return svc, "test", nil },
		agent.NewConfig(nil), skillReg,
		agent.NoMemory(), "", "test", "test",
	)
	childEvents := make(chan agent.ChildEvent, 64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tx := make(chan dialoguetui.MessageEvent, 50)
	reqRx := make(chan completionRequest)

	mu := &sync.Mutex{}
	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	comp.Resize(80, 24)
	h := &aiEditorHandler{p: term.NopInterrupter()}
	sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
	}()

	reqRx <- completionRequest{msg: "explain this", ctx: ctx}

	timeout := time.After(10 * time.Second)
	sawBreak := false
	for !sawBreak {
		select {
		case ev := <-tx:
			if ev.Type == dialoguetui.MessageEventBreak {
				sawBreak = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for first turn completion")
		}
	}

	reqRx <- completionRequest{msg: "please continue", ctx: ctx}

	sawBreak = false
	for !sawBreak {
		select {
		case ev := <-tx:
			if ev.Type == dialoguetui.MessageEventBreak {
				sawBreak = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for second turn completion")
		}
	}

	cancel()
	<-doneCh

	reqs := svc.getRequests()
	require.Len(t, reqs, 2, "expected 2 LLM requests (truncated + continue)")

	// The second request must include the partial assistant message with
	// the reasoning carried forward as non-empty content (since a bare
	// empty-content assistant message would be rejected by the API).
	secondReq := reqs[1]
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

// TestAgent_NormalizeStoredDialogueBeforeLLMCall stores one dialogue per
// normalization issue (reasoning-only assistant, orphaned tool calls,
// orphaned tool results). The mock service rejects any request that
// still contains malformed messages with a 400 error. The test verifies
// that normalization cleans each dialogue before the first LLM call.
func TestAgent_NormalizeStoredDialogueBeforeLLMCall(t *testing.T) {
	t.Parallel()
	// rejectMalformed returns a 400 error if the request contains any of
	// the message issues that normalizeMessages is supposed to fix.
	rejectMalformed := func(req llm.Request) error {
		callIDs := make(map[string]struct{})
		resultIDs := make(map[string]struct{})
		for _, msg := range req.Messages {
			for _, tc := range msg.ToolCalls {
				callIDs[tc.ID] = struct{}{}
			}
			if msg.Role == llm.RoleTool && msg.ToolCallID != "" {
				resultIDs[msg.ToolCallID] = struct{}{}
			}
		}
		for _, msg := range req.Messages {
			if msg.Role == llm.RoleAssistant && msg.Content == "" {
				return fmt.Errorf("400: text content blocks must be non-empty")
			}
			for _, tc := range msg.ToolCalls {
				if _, ok := resultIDs[tc.ID]; !ok {
					return fmt.Errorf("400: tool use id %s not found in tool results", tc.ID)
				}
			}
			if msg.Role == llm.RoleTool {
				if _, ok := callIDs[msg.ToolCallID]; !ok {
					return fmt.Errorf("400: tool result %s references unknown tool call", msg.ToolCallID)
				}
			}
		}
		return nil
	}

	tests := []struct {
		name   string
		stored []llm.Message
	}{
		{
			name: "reasoning-only assistant message",
			stored: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "explain"},
				{Role: llm.RoleAssistant, ReasoningContent: "deep thoughts..."},
			},
		},
		{
			name: "orphaned tool calls",
			stored: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "fix the bug"},
				{
					Role:    llm.RoleAssistant,
					Content: "I'll read the file",
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`}},
					},
				},
			},
		},
		{
			name: "orphaned tool results",
			stored: []llm.Message{
				{Role: llm.RoleSystem, Content: "sys"},
				{Role: llm.RoleUser, Content: "fix the bug"},
				{Role: llm.RoleTool, Content: "package main", ToolCallID: "call_missing"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &agentMockService{
				responses: []agentMockResponse{
					{chunks: []string{"done"}, finishReason: llm.FinishReasonStop},
				},
				validateRequest: rejectMalformed,
			}

			store := newTestDialogueStore()
			require.NoError(t, store.Create(context.Background(), dialoguemanager.Dialogue{
				ID:       "d",
				Messages: tt.stored,
				Version:  1,
			}))

			registry := agent.NewRegistry()
			skillReg := skills.NewRegistry(nopFileSystem{}, workspaceapi.URI{}, nil, nil)
			ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{SystemPrompt: "sys"})

			spawner := agent.NewGoroutineSpawner(
				store, func(string) (llm.Service, string, error) { return svc, "test", nil },
				agent.NewConfig(nil), skillReg,
				agent.NoMemory(), "", "test", "test",
			)
			childEvents := make(chan agent.ChildEvent, 64)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tx := make(chan dialoguetui.MessageEvent, 50)
			reqRx := make(chan completionRequest)

			mu := &sync.Mutex{}
			comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
			comp.Resize(80, 24)
			h := &aiEditorHandler{p: term.NopInterrupter()}
			sc := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

			doneCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				createAgentCompletions(ctx, cancel, tx, reqRx, ag, spawner, childEvents, skillReg, "d", sc, nopNotifications{}, nil)
			}()

			reqRx <- completionRequest{msg: "continue", ctx: ctx}

			timeout := time.After(10 * time.Second)
			var sawError bool
			for {
				select {
				case ev := <-tx:
					if ev.Type == dialoguetui.MessageEventError {
						sawError = true
					}
					if ev.Type == dialoguetui.MessageEventBreak {
						goto done
					}
				case <-timeout:
					t.Fatal("timed out waiting for turn completion")
				}
			}
		done:
			cancel()
			<-doneCh

			assert.False(t, sawError,
				"normalization must clean the stored dialogue before the LLM call")
			require.Len(t, svc.getRequests(), 1,
				"CreateCompletion must be called exactly once")
		})
	}
}

func TestAIEditorHandler_chat_task_progress_renders_checklist(t *testing.T) {
	t.Parallel()
	const width = 50
	const height = 20

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= width {
			return s
		}
		return s + strings.Repeat(" ", width-n)
	}
	expected := func(lines ...string) string {
		padded := make([]string, len(lines))
		for i, l := range lines {
			padded[i] = pad(l)
		}
		return strings.Join(padded, "\n")
	}

	// Helper to build a TaskCreate tool call.
	taskCreate := func(id, subject, desc string) llm.ToolCall {
		args, _ := json.Marshal(map[string]string{
			"subject":     subject,
			"description": desc,
		})
		return llm.ToolCall{
			ID:       id,
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "TaskCreate", Arguments: string(args)},
		}
	}

	// Helper to build a TaskUpdate tool call.
	taskUpdate := func(id, taskID, status string) llm.ToolCall {
		args, _ := json.Marshal(map[string]string{
			"taskId": taskID,
			"status": status,
		})
		return llm.ToolCall{
			ID:       id,
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "TaskUpdate", Arguments: string(args)},
		}
	}

	mockTool := &agentMockTool{
		name: "do_work",
		executeFn: func(_ context.Context, _ string) agent.ToolResult {
			return agent.ToolResult{Content: "done"}
		},
	}
	workCall := func(id string) llm.ToolCall {
		return llm.ToolCall{
			ID:       id,
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "do_work", Arguments: `{}`},
		}
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			// User "plan" — turn 1: create 3 tasks (sequential to preserve order)
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls:    []llm.ToolCall{taskCreate("tc1", "Create store", "Implement task store")},
			},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls:    []llm.ToolCall{taskCreate("tc2", "Add tools", "Implement 4 tools")},
			},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls:    []llm.ToolCall{taskCreate("tc3", "Write tests", "Add test coverage")},
			},
			{chunks: []string{"Plan ready."}, finishReason: llm.FinishReasonStop},

			// User "1" — turn 2: work on task 1
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu1", "1", "in_progress")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{workCall("wc1")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu2", "1", "completed")}},
			{chunks: []string{"Store done."}, finishReason: llm.FinishReasonStop},

			// User "2" — turn 3: work on task 2
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu3", "2", "in_progress")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{workCall("wc2")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu4", "2", "completed")}},
			{chunks: []string{"Tools done."}, finishReason: llm.FinishReasonStop},

			// User "3" — turn 4: work on task 3
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu5", "3", "in_progress")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{workCall("wc3")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu6", "3", "completed")}},
			{chunks: []string{"Tests done."}, finishReason: llm.FinishReasonStop},

			// User "x" — turn 5: delete all tasks
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu7", "1", "deleted")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu8", "2", "deleted")}},
			{finishReason: llm.FinishReasonToolCall, toolCalls: []llm.ToolCall{taskUpdate("tu9", "3", "deleted")}},
			{chunks: []string{"All clear."}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          "anthropic-test-model",
		Provider:      "anthropic",
		ContextWindow: 128000,
	})
	deps.handler.modelRegistry = reg
	deps.handler.config = stubConfig{
		configs: map[string]config.Config{
			"anthropic": stubConfig{
				strings: map[string]string{"api_key": "test-key"},
			},
		},
	}
	deps.handler.defaultModel = "anthropic-test-model"
	deps.handler.queryDefaultModel = "anthropic-test-model"
	deps.handler.newAnthropicClient = func(string, anthropic.Config, map[string]int) llm.Service {
		return svc
	}
	deps.handler.baseTools = []agent.Tool{mockTool}
	deps.handler.cfg.DurationPrecision = time.Second
	flusher := openChatAndGetTab(t, deps)
	flusher.maxWait = 2 * time.Second

	handlertest.RunHandlerSequence(t, flusher, width, height, []handlertest.SequenceTestCase{
		// Create 3 tasks. Task tools are invisible — only "Plan ready." text shows.
		// Progress widget at the bottom shows 3 pending tasks with descriptions.
		{
			InputSequence: "plan<enter><c-o>",
			Expected: expected(
				"plan",
				"Plan ready.",
				"",
				"󰏥 Create store",
				"  Implement task store",
				"󰏥 Add tools",
				"  Implement 4 tools",
				"󰏥 Write tests",
				"  Add test coverage",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
		// User submits "1" — previous progress is cleared. Task 1
		// goes in_progress → completed, so it appears with description.
		{
			InputSequence: "1<enter>",
			Expected: expected(
				"plan",
				"Plan ready.",
				"",
				"1",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Store done.",
				"",
				"󰗠 Create store",
				"  Implement task store",
				"",
				"",
				"",
				"",
				"",
				"",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
		// User submits "2" — progress cleared. Task 2 with description.
		{
			InputSequence: "2<enter>",
			Expected: expected(
				"plan",
				"Plan ready.",
				"",
				"1",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Store done.",
				"",
				"2",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Tools done.",
				"",
				"󰗠 Add tools",
				"  Implement 4 tools",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
		// User submits "3" — progress cleared. Task 3 with description.
		{
			InputSequence: "3<enter>",
			Expected: expected(
				"",
				"Store done.",
				"",
				"2",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Tools done.",
				"",
				"3",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Tests done.",
				"",
				"󰗠 Write tests",
				"  Add test coverage",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
		// User submits "x" — progress cleared. Deleted tasks on a
		// fresh widget are no-ops, so no progress shows at all.
		{
			InputSequence: "x<enter>",
			Expected: expected(
				"Store done.",
				"",
				"2",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Tools done.",
				"",
				"3",
				"└─ ✓ do_work 0s",
				"Press <ctrl-o> to expand",
				"",
				"Tests done.",
				"",
				"x",
				"All clear.",
				"",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
	})
}

func TestAIEditorHandler_chat_update_plan_renders_checklist(t *testing.T) {
	t.Parallel()
	const width = 50
	const height = 20

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= width {
			return s
		}
		return s + strings.Repeat(" ", width-n)
	}
	expected := func(lines ...string) string {
		padded := make([]string, len(lines))
		for i, l := range lines {
			padded[i] = pad(l)
		}
		return strings.Join(padded, "\n")
	}

	updatePlan := func(id string, explanation string, plan []map[string]string) llm.ToolCall {
		payload := map[string]any{"plan": plan}
		if explanation != "" {
			payload["explanation"] = explanation
		}
		args, _ := json.Marshal(payload)
		return llm.ToolCall{
			ID:       id,
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "update_plan", Arguments: string(args)},
		}
	}

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{updatePlan("up1", "Implement feature", []map[string]string{
					{"step": "Inspect handlers", "status": "completed"},
					{"step": "Patch update_plan", "status": "in_progress"},
					{"step": "Add tests", "status": "pending"},
				})},
			},
			{chunks: []string{"Plan updated."}, finishReason: llm.FinishReasonStop},
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{updatePlan("up2", "Revise plan", []map[string]string{
					{"step": "Patch update_plan", "status": "completed"},
					{"step": "Add tests", "status": "in_progress"},
				})},
			},
			{chunks: []string{"Revised."}, finishReason: llm.FinishReasonStop},
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)
	flusher.maxWait = 2 * time.Second

	handlertest.RunHandlerSequence(t, flusher, width, height, []handlertest.SequenceTestCase{
		{
			InputSequence: "plan<enter><c-o>",
			Expected: expected(
				"plan",
				"Plan updated.",
				"",
				"󰗠 Inspect handlers",
				"󰐌 Patch update_plan",
				"󰏥 Add tests",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
		{
			InputSequence: "revise<enter>",
			Expected: expected(
				"plan",
				"Plan updated.",
				"",
				"revise",
				"Revised.",
				"",
				"󰗠 Patch update_plan",
				"󰐌 Add tests",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"    ┌───────────────────────────────────────┐",
				"    │▐                                      │",
				"    └───────────────────────────────────────┘",
			),
		},
	})
}

func TestOpenAIRegistry_keeps_exit_plan_and_replaces_task_tools(t *testing.T) {
	t.Parallel()
	registry := agent.NewRegistry(
		&agentMockTool{name: "TaskCreate"},
		&agentMockTool{name: "TaskUpdate"},
		&agentMockTool{name: "TaskGet"},
		&agentMockTool{name: "TaskList"},
		&agentMockTool{name: "exit_plan_mode"},
		&agentMockTool{name: "ask_user_question"},
	)
	registry.RegisterOverrides(llmopenai.LLMProvider,
		agentools.NewUpdatePlan(noopProgressUpdater{}),
		agentools.NewRequestUserInput(noopPrompter{}),
	)
	registry.RegisterExclusions(llmopenai.LLMProvider,
		"TaskCreate", "TaskUpdate", "TaskGet", "TaskList", "ask_user_question",
	)

	_, ok := registry.Get("exit_plan_mode", llmopenai.LLMProvider)
	assert.True(t, ok, "exit_plan_mode should remain available for openai")

	_, ok = registry.Get("update_plan", llmopenai.LLMProvider)
	assert.True(t, ok, "update_plan should be available for openai")

	_, ok = registry.Get("request_user_input", llmopenai.LLMProvider)
	assert.True(t, ok, "request_user_input should be available for openai")

	_, ok = registry.Get("TaskCreate", llmopenai.LLMProvider)
	assert.False(t, ok, "TaskCreate should be excluded for openai")
	_, ok = registry.Get("TaskUpdate", llmopenai.LLMProvider)
	assert.False(t, ok, "TaskUpdate should be excluded for openai")
	_, ok = registry.Get("TaskGet", llmopenai.LLMProvider)
	assert.False(t, ok, "TaskGet should be excluded for openai")
	_, ok = registry.Get("TaskList", llmopenai.LLMProvider)
	assert.False(t, ok, "TaskList should be excluded for openai")

	tools := registry.Tools(llmopenai.LLMProvider)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	assert.Contains(t, names, "exit_plan_mode")
	assert.Contains(t, names, "update_plan")
	assert.NotContains(t, names, "TaskCreate")
	assert.NotContains(t, names, "TaskUpdate")
	assert.NotContains(t, names, "TaskGet")
	assert.NotContains(t, names, "TaskList")
}

func TestAIEditorHandler_shell_skills_list_dynamic(t *testing.T) {
	t.Parallel()
	svc := &agentMockService{}

	// Create a real temp directory with two initial skills.
	skillsDir := t.TempDir()
	writeSkill := func(name, desc string) {
		dir := filepath.Join(skillsDir, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "SKILL.md"),
			[]byte(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s body", name, desc, name)),
			0o644,
		))
	}
	removeSkill := func(name string) {
		require.NoError(t, os.RemoveAll(filepath.Join(skillsDir, name)))
	}
	writeSkill("alpha", "Alpha skill")
	writeSkill("beta", "Beta skill")

	deps := newTestAIEditorHandler(t, svc)

	// Replace the skill registry with one backed by the real FS
	// that tracks our temp skills directory.
	cwd, err := workspaceapi.ParseURI("file:///test/workspace")
	require.NoError(t, err)
	deps.handler.skillRegistry = skills.NewRegistry(
		testLocalFS{}, cwd, []string{skillsDir}, nil,
	)

	// Open the agent shell tab.
	cmd := textapi.Command{Name: commandShell, Window: e2eWindow(0)}
	err = deps.handler.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)

	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)

	flusher := &asyncFlusher{
		inner:       tab,
		interruptCh: deps.interruptCh,
		idleTimeout: 50 * time.Millisecond,
	}

	const pw, ph = 60, 30

	pad := func(s string) string {
		n := utf8.RuneCountInString(s)
		if n >= pw {
			return s
		}
		return s + strings.Repeat(" ", pw-n)
	}
	expected := func(lines ...string) string {
		padded := make([]string, len(lines))
		for i, l := range lines {
			padded[i] = pad(l)
		}
		return strings.Join(padded, "\n")
	}

	// Step 1: skills list — should show alpha, beta + builtins.
	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>list<enter>",
			Expected: expected(
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• beta — Beta skill",
				"• explore — Fast, read-only research agent for expl...",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> ▐",
			),
		},
	})

	// Step 2: Add a third skill on disk and verify it appears.
	writeSkill("gamma", "Gamma skill")

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>list<enter>",
			Expected: expected(
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• beta — Beta skill",
				"• explore — Fast, read-only research agent for expl...",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• beta — Beta skill",
				"• explore — Fast, read-only research agent for expl...",
				"• gamma — Gamma skill",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> ▐",
			),
		},
	})

	// Step 3: Remove beta from disk and verify it disappears.
	removeSkill("beta")

	handlertest.RunHandlerSequence(t, flusher, pw, ph, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>list<enter>",
			Expected: expected(
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• beta — Beta skill",
				"• explore — Fast, read-only research agent for expl...",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• beta — Beta skill",
				"• explore — Fast, read-only research agent for expl...",
				"• gamma — Gamma skill",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> skills list",
				"",
				"Skills",
				"",
				"• alpha — Alpha skill",
				"• explore — Fast, read-only research agent for expl...",
				"• gamma — Gamma skill",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> ▐",
			),
		},
	})
}

// TestAIEditorHandler_model_switch_no_empty_text_blocks verifies that
// switching from an OpenAI model to a Claude model does not produce
// empty text content blocks in the request sent to the LLM.
//
// The Anthropic API rejects empty text blocks with:
//
//	"text content blocks must be non-empty"
//
// Two scenarios cause this:
//  1. An interrupted turn left orphaned tool calls on an assistant
//     message with no text. normalizeMessages strips the orphaned
//     calls, leaving an empty assistant message.
//  2. A stored assistant message that already has empty content and
//     no tool calls (e.g. the prior model returned an empty
//     response).
func TestAIEditorHandler_model_switch_no_empty_text_blocks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		messages []llm.Message
	}{
		{
			name: "orphaned tool calls stripped leaves empty assistant message",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "test system prompt"},
				{Role: llm.RoleUser, Content: "read main.go"},
				{
					Role:    llm.RoleAssistant,
					Content: "",
					ToolCalls: []llm.ToolCall{{
						ID:   "orphaned_call",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{
							Name:      "read_file",
							Arguments: `{"path":"main.go"}`,
						},
					}},
				},
				// No tool result for "orphaned_call" — simulates interrupted turn.
				{Role: llm.RoleUser, Content: "never mind"},
				{Role: llm.RoleAssistant, Content: "OK."},
			},
		},
		{
			name: "stored empty assistant message",
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "test system prompt"},
				{Role: llm.RoleUser, Content: "hello"},
				{
					Role:    llm.RoleAssistant,
					Content: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock service validates every request: if any assistant
			// message has empty content and no tool calls, it returns an
			// error (mimicking the Anthropic API rejection).
			svc := &agentMockService{
				responses: []agentMockResponse{
					{chunks: []string{"hello"}, finishReason: llm.FinishReasonStop},
				},
				validateRequest: func(req llm.Request) error {
					for _, msg := range req.Messages {
						if msg.Role == llm.RoleAssistant &&
							msg.Content == "" &&
							len(msg.ToolCalls) == 0 {
							return fmt.Errorf(
								"text content blocks must be non-empty: " +
									"assistant message has empty content and no tool calls",
							)
						}
					}
					return nil
				},
			}

			store := newTestDialogueStore()
			wm := &capturingWindowManager{}
			reg := llmregistry.NewStatic()
			reg.Register(llmregistry.ModelEntry{
				Name:          "test-model",
				Provider:      "openai",
				ContextWindow: 128000,
			})
			reg.Register(llmregistry.ModelEntry{
				Name:          "claude-3-haiku",
				Provider:      "anthropic",
				ContextWindow: 200000,
			})

			cwd, err := workspaceapi.ParseURI("file:///test/workspace")
			require.NoError(t, err)

			skillReg := skills.NewRegistry(nopFileSystem{}, cwd, nil, nopNotifications{})
			interruptCh := make(chan struct{}, 100)
			interrupter := term.FuncInterrupter(func(context.Context) error {
				select {
				case interruptCh <- struct{}{}:
				default:
				}
				return nil
			})

			cfg := stubConfig{
				configs: map[string]config.Config{
					"openai":    stubConfig{strings: map[string]string{"api_key": "sk-openai"}},
					"anthropic": stubConfig{strings: map[string]string{"api_key": "sk-anthropic"}},
				},
			}

			h := &aiEditorHandler{
				defaultModel:      "test-model",
				queryDefaultModel: "test-model",
				modelRegistry:     reg,
				dialogueStore:     store,
				newClient: func(string, llmopenai.Config, map[string]int) llm.Service {
					return svc
				},
				newAnthropicClient: func(_ string, _ anthropic.Config, _ map[string]int) llm.Service {
					return svc
				},
				wm:            wm,
				n:             nopNotifications{},
				p:             interrupter,
				clip:          clipboard.NewInMemory(),
				mcpManager:    runemcp.NewManager(),
				toolRegistry:  agent.NewRegistry(),
				systemPrompt:  "test system prompt",
				skillRegistry: skillReg,
				cwd:           cwd,
				fs:            nopFileSystem{},
				config:        cfg,
				resources:     make(map[string]string),
				agentsConfig:  agent.NewConfig(nil),
			}
			h.ctx, h.cancelCtx = context.WithCancel(context.Background())
			h.queryAgent = agent.NewAgent(
				svc, h.toolRegistry, skillReg, store, agent.NoMemory(), agent.Config{
					SystemPrompt: "test query prompt",
					SessionKey:   "query",
					AgentID:      "query",
					Model:        "test-model",
				},
			)
			t.Cleanup(func() {
				h.cancelCtx()
				_ = h.mcpManager.Close()
			})

			// Pre-populate the store with the problematic conversation.
			err = store.Create(context.Background(), dialoguemanager.Dialogue{
				ID:       "sess",
				Messages: tt.messages,
			})
			require.NoError(t, err)

			// Open chat with the existing session.
			cmd := textapi.Command{
				Name:   commandChat,
				Args:   []string{"sess"},
				Window: e2eWindow(0),
			}
			err = h.HandleCommand(context.Background(), cmd)
			require.NoError(t, err)
			waitUntilIdle(interruptCh, 100*time.Millisecond, 0)

			wm.mu.Lock()
			tab := wm.lastTab
			wm.mu.Unlock()
			require.NotNil(t, tab)

			flusher := &asyncFlusher{
				inner:       tab,
				interruptCh: interruptCh,
				idleTimeout: 200 * time.Millisecond,
				maxWait:     3 * time.Second,
			}
			flusher.Resize(e2eWidth, e2eHeight)

			// Switch to Anthropic model, then send a user message.
			// If the fix is missing, validateRequest will fire and the
			// agent loop will emit an error event instead of the
			// expected "hello" response.
			//
			// We drive the handler directly (no rendering assertions)
			// because we only care about the LLM request content.
			for _, seq := range []string{
				"/model<space>claude-3-haiku<enter>",
				"test<enter>",
			} {
				keys, err := term.ParseKeys(seq)
				require.NoError(t, err)
				for _, key := range keys {
					flusher.Handle(term.Event{
						Ch: key.Ch, Mod: key.Mod, Key: key.Key,
						Type: term.EventKey,
					})
				}
			}

			// Verify the LLM was actually called (i.e. the request
			// passed validation and wasn't rejected).
			svc.mu.Lock()
			callCount := svc.callCount
			reqs := append([]llm.Request(nil), svc.requests...)
			svc.mu.Unlock()
			require.GreaterOrEqual(t, callCount, 1,
				"LLM should have been called at least once after model switch")

			// Double-check: none of the captured requests contain an
			// empty assistant message.
			for i, req := range reqs {
				for j, msg := range req.Messages {
					if msg.Role == llm.RoleAssistant &&
						msg.Content == "" &&
						len(msg.ToolCalls) == 0 {
						t.Errorf("request[%d].Messages[%d]: empty assistant message "+
							"would produce invalid text content block", i, j)
					}
				}
			}
		})
	}
}

// TestAIEditorHandler_chat_selection_focus_transition verifies that
// Selection() returns the correct text depending on whether the
// inputbox or the messages area was last interacted with via mouse.
//
// Flow:
//  1. Open aichat, send "hi", receive "Hello!" from the agent.
//  2. Type "world" in the inputbox, select "wor" with Shift+Right.
//     → Selection() returns "wor".
//  3. Triple-click on "Hello!" in the messages area.
//     → Selection() returns "Hello!", inputbox inverse is stripped.
//  4. Click back in the inputbox area.
//     → Selection() returns empty (inputbox has no new selection),
//     messages selection is cleared.
func TestAIEditorHandler_chat_selection_focus_transition(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"Hello!"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 200 * time.Millisecond
	flusher.maxWait = 1 * time.Second

	const width, height = e2eWidth, e2eHeight
	w := term.NewStringWriter(width, height)

	// Step 0: Send "hi" and wait for "Hello!" response.
	handlertest.RunHandlerSequenceWriter(t, w, flusher, width, height, []handlertest.SequenceTestCase{
		{
			InputSequence: "hi<enter>",
			Expected: e2eExpected(0,
				"hi",
				"Hello!",
				"",
				"",
				"",
				"",
				"",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Step 1: Type "world" in inputbox, then Home + 3×Shift+Right
	// to select "wor".
	keys, err := term.ParseKeys("world<home><s-right><s-right><s-right>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	sel, ok := flusher.Selection()
	assert.True(t, ok, "step 1: inputbox selection should be returned")
	assert.Equal(t, "wor", sel, "step 1: inputbox selected text")

	// Verify the selected text is rendered with AttrReverse.
	_ = w.Clear(term.Attributes{})
	flusher.Draw(w)
	_ = w.Flush()
	// The inputbox content row is at Y=8 (inside the frame).
	// "wor" starts at X=4 (after "   │" frame border).
	for x := 4; x < 7; x++ {
		cell := w.Cells()[8*width+x]
		assert.NotZero(t, cell.Attrs&tcell.AttrReverse,
			"step 1: cell (%d,8) should have AttrReverse", x)
	}

	// Step 2: Triple-click on "Hello!" at row 1 in the messages area.
	// This switches focus to messages and clears the inputbox
	// selection visually.
	for _, ev := range []term.Event{
		{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 2, MouseY: 1},
		{Type: term.EventMouse, Key: term.MouseRelease, MouseX: 2, MouseY: 1},
		{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 2, MouseY: 1},
		{Type: term.EventMouse, Key: term.MouseRelease, MouseX: 2, MouseY: 1},
		{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 2, MouseY: 1},
		{Type: term.EventMouse, Key: term.MouseRelease, MouseX: 2, MouseY: 1},
	} {
		flusher.Handle(ev)
	}

	sel, ok = flusher.Selection()
	assert.True(t, ok, "step 2: messages selection should be returned")
	assert.Equal(t, "Hello!", sel, "step 2: messages selected text")

	// Verify "wor" in the inputbox no longer has AttrReverse.
	_ = w.Clear(term.Attributes{})
	flusher.Draw(w)
	_ = w.Flush()
	for x := 4; x < 7; x++ {
		cell := w.Cells()[8*width+x]
		assert.Zero(t, cell.Attrs&tcell.AttrReverse,
			"step 2: cell (%d,8) should NOT have AttrReverse", x)
	}

	// Step 3: Click back in the inputbox area (Y=8, inside the frame).
	// This clears the messages selection and returns focus to the
	// inputbox.
	flusher.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 4, MouseY: 8})
	flusher.Handle(term.Event{Type: term.EventMouse, Key: term.MouseRelease, MouseX: 4, MouseY: 8})

	sel, ok = flusher.Selection()
	assert.False(t, ok, "step 3: no selection after click in inputbox")
	assert.Empty(t, sel, "step 3: empty selection text")
}

// TestAIEditorHandler_chat_queue_while_busy exercises the message-queue feature
// end-to-end through the aichat command.
//
// Flow:
//  1. Send "hello" → agent starts processing (stays busy; gate blocks completion).
//  2. While busy, type "msg1" + Enter → queued with ⏳ indicator.
//  3. Type "msg2" + Enter → second queued message.
//  4. Arrow-Up on empty input → recalls "msg2" into the inputbox.
//  5. Ctrl-U clears input, Arrow-Up → recalls "msg1".
//  6. Ctrl-U clears input → no more queued messages.
//  7. Close the gate → agent replies with "Hi!".
func TestAIEditorHandler_chat_queue_while_busy(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"Hi!"}, finishReason: llm.FinishReasonStop, gate: gate},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	deps.handler.cfg.DurationPrecision = time.Hour
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 100 * time.Millisecond
	flusher.maxWait = 300 * time.Millisecond

	// Step 1: Send "hello" — starts the agent turn, which blocks on the gate.
	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	// Steps 2–6 via RunHandlerSequence.
	// Spinner frame advances with each Draw: draw 1=⠙, 2=⠹, 3=⠸, 4=⠼, 5=⠴, 6=⠦
	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Step 2: queue "msg1"
		{
			InputSequence: "msg1<enter>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  msg1",
				"⠙ sending (0s)",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 3: queue "msg2"
		{
			InputSequence: "msg2<enter>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  msg1",
				"⏳  msg2",
				"⠹ sending (0s)",
				"", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 4: Arrow-Up recalls "msg2" (newest) into input
		{
			InputSequence: "<up>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  msg1",
				"⠸ sending (0s)",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │msg2▐                          │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 5: Ctrl-U clears input, Arrow-Up recalls "msg1"
		{
			InputSequence: "<c-u><up>",
			Expected: e2eExpected(0,
				"hello",
				"⠼ sending (0s)",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │msg1▐                          │",
				"   └───────────────────────────────┘",
			),
		},
		// Step 6: Ctrl-U clears input → empty, no more queued
		{
			InputSequence: "<c-u>",
			Expected: e2eExpected(0,
				"hello",
				"⠴ sending (0s)",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	// Step 7: close gate → agent responds with "Hi!".
	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 2*time.Second)

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected: e2eExpected(0,
				"hello",
				"Hi!",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})
}

// TestAIEditorHandler_chat_queue_two_then_delete_both verifies that two queued
// messages can each be recalled via Arrow-Up and discarded (Ctrl-U) one at a
// time, leaving an empty queue.
func TestAIEditorHandler_chat_queue_two_then_delete_both(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	})

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"reply"}, finishReason: llm.FinishReasonStop, gate: gate},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	deps.handler.cfg.DurationPrecision = time.Hour
	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 100 * time.Millisecond
	flusher.maxWait = 300 * time.Millisecond

	// Start an agent turn that blocks on the gate.
	keys, err := term.ParseKeys("hello<enter>")
	require.NoError(t, err)
	for _, k := range keys {
		flusher.Handle(term.Event{Ch: k.Ch, Mod: k.Mod, Key: k.Key, Type: term.EventKey})
	}

	handlertest.RunHandlerSequence(t, flusher, e2eWidth, e2eHeight, []handlertest.SequenceTestCase{
		// Queue "first".
		{
			InputSequence: "first<enter>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  first",
				"⠙ sending (0s)",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Queue "second".
		{
			InputSequence: "second<enter>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  first",
				"⏳  second",
				"⠹ sending (0s)",
				"", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Arrow-Up → recall "second" (newest).
		{
			InputSequence: "<up>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  first",
				"⠸ sending (0s)",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │second▐                        │",
				"   └───────────────────────────────┘",
			),
		},
		// Ctrl-U discards "second".
		{
			InputSequence: "<c-u>",
			Expected: e2eExpected(0,
				"hello",
				"⏳  first",
				"⠼ sending (0s)",
				"", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Arrow-Up → recall "first".
		{
			InputSequence: "<up>",
			Expected: e2eExpected(0,
				"hello",
				"⠴ sending (0s)",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │first▐                         │",
				"   └───────────────────────────────┘",
			),
		},
		// Ctrl-U discards "first" → queue empty.
		{
			InputSequence: "<c-u>",
			Expected: e2eExpected(0,
				"hello",
				"⠦ sending (0s)",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
		// Arrow-Up with empty queue → no change.
		{
			InputSequence: "<up>",
			Expected: e2eExpected(0,
				"hello",
				"⠧ sending (0s)",
				"", "", "", "", "",
				"   ┌───────────────────────────────┐",
				"   │▐                              │",
				"   └───────────────────────────────┘",
			),
		},
	})

	close(gate)
	waitUntilIdle(deps.interruptCh, 200*time.Millisecond, 2*time.Second)
}
