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

package agentshell

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/mcp"
)

// syncScheduler collects callbacks scheduled via
// scheduleNextTick so they can be flushed
// synchronously in tests.
type syncScheduler struct {
	pending []func()
}

func (s *syncScheduler) schedule(fn func()) bool {
	s.pending = append(s.pending, fn)
	return true
}

// flush runs all pending callbacks, including any
// callbacks scheduled during execution, until the
// queue is empty.
func (s *syncScheduler) flush() {
	for len(s.pending) > 0 {
		batch := s.pending
		s.pending = nil
		for _, fn := range batch {
			fn()
		}
	}
}

// flusher wraps a repl.Handler so that async command
// output is drained synchronously after every Handle call.
// This makes command results visible when RunHandlerSequence
// calls Draw immediately after Handle.
type flusher struct {
	h     *repl.Handler
	sched *syncScheduler
}

func (f *flusher) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = f.h.Handle(ev)
	f.h.Wait()
	f.sched.flush()
	return
}

func (f *flusher) Resize(w, h int)    { f.h.Resize(w, h) }
func (f *flusher) Draw(w term.Writer) { f.h.Draw(w) }
func (f *flusher) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return f.h.Cursor()
}
func (f *flusher) Selection() (string, bool) { return f.h.Selection() }

const (
	testWidth  = 60
	testHeight = 30
)

func newTestFlusher(deps *testDeps) *flusher {
	sh := New(
		deps.wm,
		deps.svc,
		deps.modelRegistry,
		deps.defaultModel,
		deps.store,
		deps.registry,
		deps.agentsConfig,
		deps.cfg,
		deps.skillRegistry,
		deps.workspaceRoot,
		deps.fs,
		deps.storage, nil, nil, nil,
		deps.notifications,
		deps.dataPath,
		deps.opts...,
	)
	sched := &syncScheduler{}
	h := repl.New(sh, sched.schedule, term.NopInterrupter(),
		repl.WithPrompt("agent> "),
	)
	return &flusher{h: h, sched: sched}
}

// pad right-pads s with spaces to testWidth runes.
func pad(s string) string {
	n := utf8.RuneCountInString(s)
	if n >= testWidth {
		return s
	}
	return s + strings.Repeat(" ", testWidth-n)
}

// mkExpected builds the full draw string by prepending
// numBlank empty lines, padding each content line, and
// joining all lines with newlines.
func mkExpected(numBlank int, contentLines ...string) string {
	lines := make([]string, 0, numBlank+len(contentLines))
	bl := strings.Repeat(" ", testWidth)
	for range numBlank {
		lines = append(lines, bl)
	}
	for _, l := range contentLines {
		lines = append(lines, pad(l))
	}
	return strings.Join(lines, "\n")
}

func TestIntegrationInitial(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "",
			Expected:      mkExpected(29, "agent> \u2590"),
		},
	})
}

func TestIntegrationHelp(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "help<enter>",
			Expected: mkExpected(10,
				"agent> help",
				"agents — List configured agent definitions.",
				"chats <list|show|log|export|clear|compact|fork> [args] — Ins",
				"pect, export, compact, clear, and fork saved conversations.",
				"config — Show current LLM config parameters.",
				"dream [--model MODEL] — Run memory consolidation on unproces",
				"sed dialogues.",
				"effort [none|minimal|low|medium|high|xhigh|max] — Show or se",
				"t default reasoning effort.",
				"exit — Exit the shell.",
				"help [command ...] — Show usage for agent commands.",
				"mcp — Show MCP server status and tool stats.",
				"model [dialogue_id] — Show the default model or a conversati",
				"on's assigned model.",
				"models — List available models with context window sizes.",
				"skills <list|show|list-dirs|add-dir|remove-dir> [args] — Ins",
				"pect discovered skills and configured skill directories.",
				"system-prompt [agent] — Show the system prompt for an agent.",
				"tools — List registered agent tools.",
				"agent> \u2590",
			),
		},
		{
			InputSequence: "agent<enter>",
			Expected: mkExpected(0,
				"exit — Exit the shell.",
				"help [command ...] — Show usage for agent commands.",
				"mcp — Show MCP server status and tool stats.",
				"model [dialogue_id] — Show the default model or a conversati",
				"on's assigned model.",
				"models — List available models with context window sizes.",
				"skills <list|show|list-dirs|add-dir|remove-dir> [args] — Ins",
				"pect discovered skills and configured skill directories.",
				"system-prompt [agent] — Show the system prompt for an agent.",
				"tools — List registered agent tools.",
				"agent> agent",
				"agents — List configured agent definitions.",
				"chats <list|show|log|export|clear|compact|fork> [args] — Ins",
				"pect, export, compact, clear, and fork saved conversations.",
				"config — Show current LLM config parameters.",
				"dream [--model MODEL] — Run memory consolidation on unproces",
				"sed dialogues.",
				"effort [none|minimal|low|medium|high|xhigh|max] — Show or se",
				"t default reasoning effort.",
				"exit — Exit the shell.",
				"help [command ...] — Show usage for agent commands.",
				"mcp — Show MCP server status and tool stats.",
				"model [dialogue_id] — Show the default model or a conversati",
				"on's assigned model.",
				"models — List available models with context window sizes.",
				"skills <list|show|list-dirs|add-dir|remove-dir> [args] — Ins",
				"pect discovered skills and configured skill directories.",
				"system-prompt [agent] — Show the system prompt for an agent.",
				"tools — List registered agent tools.",
				"agent> ▐",
			),
		},
	})
}

func TestIntegrationModels(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "models<enter>",
			Expected: mkExpected(22,
				"agent> models",
				"",
				"Models",
				"",
				"• gpt-3.5 — openai, 16384 tokens",
				"• gpt-4 — openai, 128000 tokens (default)",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationModelDefault(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "model<enter>",
			Expected: mkExpected(26,
				"agent> model",
				"gpt-4",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationModelSession(t *testing.T) {
	deps := newTestDeps()
	deps.store.set(dialoguemanager.Dialogue{
		ID:    "sess-1",
		Model: "gpt-3.5",
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "model<space>sess-1<enter>",
			Expected: mkExpected(26,
				"agent> model sess-1",
				"gpt-3.5",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationTools(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "tools<enter>",
			Expected: mkExpected(22,
				"agent> tools",
				"",
				"Tools",
				"",
				"• read_file — Reads a file",
				"• write_file — Writes a file",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationAgents(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "agents<enter>",
			Expected: mkExpected(23,
				"agent> agents",
				"",
				"Agents",
				"",
				"• default — Default Agent, model: gpt-4",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationChatsList(t *testing.T) {
	deps := newTestDeps()
	deps.store.set(dialoguemanager.Dialogue{
		ID: "chat-1",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "hi"},
		},
		UpdatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "chats<space>list<enter>",
			Expected: mkExpected(23,
				"agent> chats list",
				"",
				"Conversations",
				"",
				"• chat-1 — 2 messages, updated 2025-01-15 10:30",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationChatsListEmpty(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "chats<space>list<enter>",
			Expected: mkExpected(26,
				"agent> chats list",
				"(no conversations)",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationChatsShow(t *testing.T) {
	deps := newTestDeps()
	deps.store.set(dialoguemanager.Dialogue{
		ID: "chat-1",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "world"},
		},
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "chats<space>show<space>chat-1<enter>",
			Expected: mkExpected(28,
				"agent> chats show chat-1",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationChatsClear(t *testing.T) {
	deps := newTestDeps()
	deps.store.set(dialoguemanager.Dialogue{
		ID:       "chat-1",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "chats<space>clear<space>chat-1<enter>",
			Expected: mkExpected(26,
				"agent> chats clear chat-1",
				"Cleared chat-1 (archived as chat-1-archived)",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationSystemPrompt(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "system-prompt<enter>",
			Expected: mkExpected(26,
				"agent> system-prompt",
				"You are a helpful assistant",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationSystemPromptUnknown(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "system-prompt<space>nope<enter>",
			Expected: mkExpected(26,
				"agent> system-prompt nope",
				"Agent nope not found",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationConfig(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "config<enter>",
			Expected: mkExpected(12,
				"agent> config",
				"",
				"Configuration",
				"",
				"• openai.api_key: **************1234",
				"• anthropic.api_key:",
				"• gemini.api_key:",
				"• base_url: https://api.openai.com",
				"• default_model: gpt-4",
				"• temperature: 0.7",
				"• top_p:",
				"• frequency_penalty:",
				"• presence_penalty:",
				"• max_tokens:",
				"• max_completion_tokens:",
				"• reasoning_effort:",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationUnknownCommand(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "bogus<enter>",
			Expected: mkExpected(27,
				"agent> bogus",
				"unknown command: bogus",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationAgentsMultiple(t *testing.T) {
	deps := newTestDeps()
	deps.agentsConfig = agent.NewConfig([]agent.Definition{
		{
			ID: "default", Name: "Default Agent",
			Model: "gpt-4", SystemPrompt: "You are helpful",
			AllowAny: true,
		},
		{
			ID: "coder", Name: "Coding Agent",
			Model: "gpt-3.5", SystemPrompt: "You write code",
		},
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "agents<enter>",
			Expected: mkExpected(22,
				"agent> agents",
				"",
				"Agents",
				"",
				"• default — Default Agent, model: gpt-4",
				"• coder — Coding Agent, model: gpt-3.5",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationSequence(t *testing.T) {
	deps := newTestDeps()
	deps.store.set(dialoguemanager.Dialogue{
		ID: "s1",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hi"},
		},
		UpdatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		// List conversations.
		{
			InputSequence: "chats<space>list<enter>",
			Expected: mkExpected(23,
				"agent> chats list",
				"",
				"Conversations",
				"",
				"• s1 — 1 messages, updated 2025-06-01 00:00",
				"",
				"agent> \u2590",
			),
		},
		// Clear the conversation (archives it).
		{
			InputSequence: "chats<space>clear<space>s1<enter>",
			Expected: mkExpected(20,
				"agent> chats list",
				"",
				"Conversations",
				"",
				"• s1 — 1 messages, updated 2025-06-01 00:00",
				"",
				"agent> chats clear s1",
				"Cleared s1 (archived as s1-archived)",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationMCPNoServers(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "mcp<enter>",
			Expected: mkExpected(26,
				"agent> mcp",
				"(no MCP servers configured)",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationMCPConnected(t *testing.T) {
	deps := newTestDeps()
	deps.opts = append(deps.opts, WithMCPInfo(&stubMCPInfo{
		servers: []*mcp.ServerInfo{
			{
				Name:        "rune",
				Command:     "runectl",
				Status:      mcp.StatusConnected,
				ConnectedAt: time.Now(),
				ToolCount:   3,
				ToolNames:   []string{"read", "search", "write"},
			},
		},
	}))
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "mcp<enter>",
			Expected: mkExpected(16,
				"agent> mcp",
				"",
				"MCP Servers",
				"",
				"",
				"rune",
				"",
				"• Status: connected",
				"• Command: runectl",
				"• Tools: 3",
				"• Calls: 0, Errors: 0",
				"• Uptime: 0s",
				"",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationSkillsList(t *testing.T) {
	const w, h = 150, 20
	deps := newTestDeps()
	deps.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{deps.skillDir}, nil)

	sh := New(
		deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
		deps.store, deps.registry, deps.agentsConfig, deps.cfg,
		deps.skillRegistry, deps.workspaceRoot, deps.fs,
		deps.storage, nil, nil, nil, deps.notifications, deps.dataPath,
		deps.opts...,
	)
	sched := &syncScheduler{}
	rh := repl.New(sh, sched.schedule, term.NopInterrupter(), repl.WithPrompt("agent> "))
	f := &flusher{h: rh, sched: sched}

	p := func(s string) string { return padN(s, w) }
	bl := strings.Repeat(" ", w)

	handlertest.RunHandlerSequence(t, f, w, h, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>list<enter>",
			Expected: strings.Join([]string{
				bl,
				bl,
				bl,
				bl,
				bl,
				bl,
				bl,
				bl,
				p("agent> skills list"),
				bl,
				p("Skills"),
				bl,
				p("• debug — Debug issues step by step"),
				p("• explore — Fast, read-only research agent for expl..."),
				p("• lint — Run linters and fix warnings"),
				p("• plan — Read-only software architect agent for ..."),
				p("• refactor — Refactor code safely"),
				p("• test-skill — A test skill"),
				bl,
				p("agent> ▐"),
			}, "\n"),
		},
	})
}

// padN right-pads s with spaces to n runes.
func padN(s string, n int) string {
	c := utf8.RuneCountInString(s)
	if c >= n {
		return s
	}
	return s + strings.Repeat(" ", n-c)
}

func TestIntegrationSkillsListEmpty(t *testing.T) {
	f := newTestFlusher(newTestDeps())
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>list<enter>",
			Expected: mkExpected(22,
				"agent> skills list",
				"",
				"Skills",
				"",
				"• explore — Fast, read-only research agent for expl...",
				"• plan — Read-only software architect agent for ...",
				"",
				"agent> ▐",
			),
		},
	})
}

func TestIntegrationSkillsShow(t *testing.T) {
	deps := newTestDeps()
	deps.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{deps.skillDir}, nil)
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "skills<space>show<space>test-skill<enter>",
			Expected: mkExpected(28,
				"agent> skills show test-skill",
				"agent> \u2590",
			),
		},
	})
}

func TestIntegrationMCPError(t *testing.T) {
	deps := newTestDeps()
	deps.opts = append(deps.opts, WithMCPInfo(&stubMCPInfo{
		servers: []*mcp.ServerInfo{
			{
				Name:    "bad",
				Command: "nope",
				Status:  mcp.StatusError,
				Error:   "connection refused",
			},
		},
	}))
	f := newTestFlusher(deps)
	handlertest.RunHandlerSequence(t, f, testWidth, testHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "mcp<enter>",
			Expected: mkExpected(17,
				"agent> mcp",
				"",
				"MCP Servers",
				"",
				"",
				"bad",
				"",
				"• Status: error: connection refused",
				"• Command: nope",
				"• Tools: 0",
				"• Calls: 0, Errors: 0",
				"",
				"agent> \u2590",
			),
		},
	})
}
