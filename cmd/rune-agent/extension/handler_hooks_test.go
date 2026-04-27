// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/hooks"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// TestAIEditorHandler_hooks_fire_on_chat_turn drives a chat tab
// end-to-end and asserts that the hooks system fires the expected
// events, in order, with payloads built from the user's turn:
//
//	SessionStart (startup) → UserPromptSubmit → PostToolUse → Stop →
//	SessionEnd (tab_close).
//
// The hooks dispatch through the recording Executor, which is the
// only place command-type hooks should be invoked from in production.
func TestAIEditorHandler_hooks_fire_on_chat_turn(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{
				finishReason: llm.FinishReasonToolCall,
				toolCalls: []llm.ToolCall{{
					ID:   "c1",
					Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{
						Name: "checkpoint_tool", Arguments: `{}`,
					},
				}},
			},
			{chunks: []string{"all done"}, finishReason: llm.FinishReasonStop},
		},
	}

	tool := &agentMockTool{
		name: "checkpoint_tool",
		executeFn: func(context.Context, string) agent.ToolResult {
			return agent.ToolResult{Content: "tool output"}
		},
	}

	deps := newTestAIEditorHandler(t, svc)
	deps.handler.baseTools = []agent.Tool{tool}
	deps.handler.toolRegistry = agent.NewRegistry(tool)
	rec := installHookRunner(t, deps)

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 100 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	const dialogueID = "default"

	sendKeysToFlusher(t, flusher, "hello<enter>")
	assertStoredDialogueMessages(t, deps.store, dialogueID, []llm.Message{
		{Role: llm.RoleSystem, Content: testSystemPromptWithAddendum},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID:       "c1",
			Type:     llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "checkpoint_tool", Arguments: `{}`},
		}}},
		{Role: llm.RoleTool, Content: "tool output", ToolCallID: "c1"},
		{Role: llm.RoleAssistant, Content: "all done"},
	})

	// Close the tab to trigger SessionEnd (tab_close).
	deps.wm.mu.Lock()
	tab := deps.wm.lastTab
	deps.wm.mu.Unlock()
	require.NotNil(t, tab)
	require.NoError(t, tab.Close())

	require.Eventually(t, func() bool {
		for _, c := range rec.snapshot() {
			if c.Payload.HookEventName == hooks.EventSessionEnd {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond, "SessionEnd hook never fired")

	calls := rec.snapshot()

	// Each invocation should have launched the configured shell
	// (default "bash") with `-c <command>`.
	for _, c := range calls {
		assert.Equal(t, "bash", c.Path, "hook command must dispatch via Executor (not exec.Command)")
		assert.Equal(t, []string{"-c", "/bin/dummy"}, c.Args)
		assert.Contains(t, c.Env, "RUNE_HOOK_EVENT="+string(c.Payload.HookEventName))
	}

	// Build an ordered list of fired event names.
	var ordered []hooks.Event
	for _, c := range calls {
		ordered = append(ordered, c.Payload.HookEventName)
	}

	// Expected ordering across the turn. UserPromptSubmit fires
	// from the extension just before ag.Run; SessionStart fires
	// once Agent.run loads the dialogue, so it follows. We do not
	// assert about Notification because internal noti.Notify calls
	// fan out unpredictably.
	want := []hooks.Event{
		hooks.EventUserPromptSubmit,
		hooks.EventSessionStart,
		hooks.EventPostToolUse,
		hooks.EventStop,
		hooks.EventSessionEnd,
	}
	assertSubsequence(t, ordered, want)

	// SessionStart must be `startup` for a brand-new dialogue.
	first := firstByEvent(t, calls, hooks.EventSessionStart)
	assert.Equal(t, "startup", first.Payload.Source)

	// UserPromptSubmit must carry the user's prompt verbatim.
	ups := firstByEvent(t, calls, hooks.EventUserPromptSubmit)
	assert.Equal(t, "hello", ups.Payload.Prompt)

	// PostToolUse must carry the tool name + tool_use_id from the
	// model's tool call.
	ptu := firstByEvent(t, calls, hooks.EventPostToolUse)
	assert.Equal(t, "checkpoint_tool", ptu.Payload.ToolName)
	assert.Equal(t, "c1", ptu.Payload.ToolUseID)
	assert.Contains(t, string(ptu.Payload.ToolResponse), "tool output")

	// SessionEnd carries the reason discriminator.
	se := firstByEvent(t, calls, hooks.EventSessionEnd)
	assert.Equal(t, "tab_close", se.Payload.Reason)
}

// TestAIEditorHandler_hooks_user_prompt_submit_block exercises the
// blocking path of UserPromptSubmit: the command exits with code 2
// and stderr "no thanks", so the agent must not call the LLM at all
// and must surface the reason as an error to the TUI (visible by the
// fact that the model is never invoked on this turn).
func TestAIEditorHandler_hooks_user_prompt_submit_block(t *testing.T) {
	t.Parallel()

	svc := &agentMockService{
		responses: []agentMockResponse{
			{chunks: []string{"unreachable"}, finishReason: llm.FinishReasonStop},
		},
	}
	deps := newTestAIEditorHandler(t, svc)
	rec := installHookRunner(t, deps)
	rec.respond = func(call recordedCall) execResponse {
		if call.Payload.HookEventName == hooks.EventUserPromptSubmit {
			return execResponse{stderr: "no thanks", exitCode: 2}
		}
		return execResponse{}
	}

	flusher := openChatAndGetTab(t, deps)
	flusher.idleTimeout = 100 * time.Millisecond
	flusher.maxWait = 2 * time.Second

	sendKeysToFlusher(t, flusher, "hi<enter>")

	require.Eventually(t, func() bool {
		for _, c := range rec.snapshot() {
			if c.Payload.HookEventName == hooks.EventUserPromptSubmit {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond, "UserPromptSubmit hook never fired")

	// The LLM must not have been called for this turn.
	require.Equal(t, 0, len(svc.getRequests()),
		"LLM should not be called when UserPromptSubmit blocks")

	// And the agent loop should never have started, so PostToolUse /
	// Stop must not have fired either.
	for _, c := range rec.snapshot() {
		switch c.Payload.HookEventName {
		case hooks.EventPostToolUse, hooks.EventStop:
			t.Fatalf("unexpected hook %s after UserPromptSubmit block", c.Payload.HookEventName)
		}
	}
}

// --- helpers ---

// recordingExec is a workspaceapi.Executor stub that captures every
// hook command invocation (Path, Args, Env, stdin payload). It writes
// a configurable response to the command's Stdout/Stderr and returns
// the configured exit error to the watcher channel, allowing the test
// to drive different response shapes per hook event.
type recordingExec struct {
	mu    sync.Mutex
	calls []recordedCall
	// respond is consulted (under no lock) to compute the response
	// for a given recorded call. The default response is "exit 0
	// with no output".
	respond func(call recordedCall) execResponse
}

type recordedCall struct {
	Path    string
	Args    []string
	Env     []string
	Stdin   []byte // captured payload bytes
	Payload hooks.Payload
}

type execResponse struct {
	stdout   string
	stderr   string
	exitCode int
}

func (r *recordingExec) Start(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	var stdin []byte
	if cmd.Stdin != nil {
		stdin, _ = io.ReadAll(cmd.Stdin)
	}
	var p hooks.Payload
	_ = json.Unmarshal(stdin, &p)

	call := recordedCall{
		Path:    cmd.Path,
		Args:    append([]string(nil), cmd.Args...),
		Env:     append([]string(nil), cmd.Env...),
		Stdin:   stdin,
		Payload: p,
	}

	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()

	resp := execResponse{}
	if r.respond != nil {
		resp = r.respond(call)
	}

	if cmd.Stdout != nil && resp.stdout != "" {
		_, _ = io.Copy(cmd.Stdout, bytes.NewBufferString(resp.stdout))
	}
	if cmd.Stderr != nil && resp.stderr != "" {
		_, _ = io.Copy(cmd.Stderr, bytes.NewBufferString(resp.stderr))
	}
	if cmd.Watcher != nil {
		var werr error
		if resp.exitCode != 0 {
			werr = &mockExitError{code: resp.exitCode}
		}
		cmd.Watcher.WatchProcess() <- werr
	}
	return 1, nil
}

func (r *recordingExec) Signal(workspaceapi.Pid, syscall.Signal) error { return nil }
func (r *recordingExec) Close() error                                  { return nil }

func (r *recordingExec) snapshot() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// mockExitError mimics *exec.ExitError so the runner's
// exitCodeFromError type-assertion succeeds.
type mockExitError struct{ code int }

func (e *mockExitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e *mockExitError) ExitCode() int { return e.code }

// installHookRunner wires a recordingExec + hooks.Runner with command
// hooks for every event we want to observe in the test.
func installHookRunner(t *testing.T, deps testAIEditorDeps) *recordingExec {
	t.Helper()
	rec := &recordingExec{}
	cfg := hooks.Config{}
	const dummy = "/bin/dummy"
	mk := func() []hooks.Group {
		return []hooks.Group{{Hooks: []hooks.Hook{{
			Type:    hooks.HookTypeCommand,
			Command: dummy,
			Timeout: 2 * time.Second,
		}}}}
	}
	cfg.SessionStart = mk()
	cfg.SessionEnd = mk()
	cfg.UserPromptSubmit = mk()
	cfg.PostToolUse = mk()
	cfg.Stop = mk()
	cfg.PreCompact = mk()
	cfg.SubagentStop = mk()
	cfg.Notification = mk()
	deps.handler.hookRunner = hooks.NewRunner(cfg, rec, nil, "/test/workspace")
	return rec
}

// assertSubsequence verifies that want appears as a subsequence of
// got, ignoring intervening events.
func assertSubsequence(t *testing.T, got []hooks.Event, want []hooks.Event) {
	t.Helper()
	i := 0
	for _, ev := range got {
		if i < len(want) && ev == want[i] {
			i++
		}
	}
	if i != len(want) {
		t.Fatalf("missing hook events in order: got %v, want subsequence %v",
			got, want)
	}
}

// firstByEvent returns the first recorded call for the given event,
// or fails the test.
func firstByEvent(t *testing.T, calls []recordedCall, ev hooks.Event) recordedCall {
	t.Helper()
	for _, c := range calls {
		if c.Payload.HookEventName == ev {
			return c
		}
	}
	t.Fatalf("no call recorded for event %s", ev)
	return recordedCall{}
}
