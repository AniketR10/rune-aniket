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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp"
	"unstable.build/go-tui/cmd/rune-agent/llm/codex"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"unstable.build/go-tui/cmd/rune-agent/mcp"
)

func TestHandleCommand(t *testing.T) {
	tests := []struct {
		name         string
		cmd          repl.Command
		setup        func(*testDeps)
		wantErr      bool
		wantFloating int
		assertOut    func(t *testing.T, text string)
	}{
		{
			name: "help lists all commands",
			cmd:  repl.Command{Name: "help"},
			assertOut: func(t *testing.T, text string) {
				for _, cmd := range commandNames {
					if !strings.Contains(text, cmd) {
						t.Errorf("help output missing command %q", cmd)
					}
				}
			},
		},
		{
			name: "agent parent help lists nested commands",
			cmd:  repl.Command{Name: CommandName},
			assertOut: func(t *testing.T, text string) {
				for _, cmd := range commandNames {
					if !strings.Contains(text, cmd) {
						t.Errorf("parent help output missing command %q", cmd)
					}
				}
				if strings.Contains(text, "agent chats <") ||
					strings.Contains(text, "agent tools —") ||
					strings.Contains(text, "agent skills <") {
					t.Errorf("parent help should not prefix subcommands with agent: %q", text)
				}
			},
		},
		{
			name: "agent parent dispatches subcommands",
			cmd:  repl.Command{Name: CommandName, Args: []string{"models"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "gpt-4") {
					t.Error("expected gpt-4 in models output")
				}
			},
		},
		{
			name: "models lists available models",
			cmd:  repl.Command{Name: "models"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "gpt-4") {
					t.Error("expected gpt-4 in models output")
				}
				if !strings.Contains(text, "gpt-3.5") {
					t.Error("expected gpt-3.5 in models output")
				}
				// default model should be marked
				if !strings.Contains(text, "(default)") {
					t.Error("expected default model gpt-4 to be marked with (default)")
				}
			},
		},
		{
			name: "model with no args shows default model",
			cmd:  repl.Command{Name: "model"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "gpt-4") {
					t.Errorf("expected default model gpt-4, got %q", text)
				}
			},
		},
		{
			name: "model with session ID shows session model",
			cmd:  repl.Command{Name: "model", Args: []string{"sess-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID:    "sess-1",
					Model: "gpt-3.5",
				})
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "gpt-3.5") {
					t.Errorf("expected session model gpt-3.5, got %q", text)
				}
			},
		},
		{
			name:    "model with unknown arg returns error",
			cmd:     repl.Command{Name: "model", Args: []string{"unknown"}},
			wantErr: true,
		},
		{
			name: "tools lists registered tools",
			cmd:  repl.Command{Name: "tools"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "read_file") {
					t.Error("expected read_file in tools output")
				}
				if !strings.Contains(text, "Reads a file") {
					t.Error("expected tool description in output")
				}
			},
		},
		{
			name: "agents lists configured agents",
			cmd:  repl.Command{Name: "agents"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "default") {
					t.Error("expected default agent in output")
				}
				if !strings.Contains(text, "Default Agent") {
					t.Error("expected agent name in output")
				}
			},
		},
		{
			name: "chats list shows saved conversations",
			cmd:  repl.Command{Name: "chats", Args: []string{"list"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "hello"},
					},
					UpdatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
				})
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "conv-1") {
					t.Error("expected conv-1 in output")
				}
				if !strings.Contains(text, "1 messages") {
					t.Error("expected message count in output")
				}
			},
		},
		{
			name: "chats list shows empty message when none exist",
			cmd:  repl.Command{Name: "chats", Args: []string{"list"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "(no conversations)") {
					t.Errorf("expected no conversations message, got %q", text)
				}
			},
		},
		{
			name: "chats show opens floating markdown viewer",
			cmd:  repl.Command{Name: "chats", Args: []string{"show", "conv-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "hello"},
						{Role: llm.RoleAssistant, Content: "hi there"},
					},
				})
			},
			wantFloating: 1,
		},
		{
			name:    "chats show without ID returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"show"}},
			wantErr: true,
		},
		{
			name: "chats clear archives and clears conversation",
			cmd:  repl.Command{Name: "chats", Args: []string{"clear", "conv-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "hello"},
					},
				})
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "Cleared") {
					t.Error("expected cleared confirmation")
				}
				if !strings.Contains(text, "archived") {
					t.Error("expected archived mention")
				}
			},
		},
		{
			name:    "chats clear without ID returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"clear"}},
			wantErr: true,
		},
		{
			name:    "chats without subcommand returns error",
			cmd:     repl.Command{Name: "chats"},
			wantErr: true,
		},
		{
			name: "system-prompt shows default agent prompt",
			cmd:  repl.Command{Name: "system-prompt"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "You are a helpful assistant") {
					t.Errorf("expected system prompt, got %q", text)
				}
			},
		},
		{
			name: "system-prompt with agent ID",
			cmd:  repl.Command{Name: "system-prompt", Args: []string{"default"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "You are a helpful assistant") {
					t.Errorf("expected system prompt, got %q", text)
				}
			},
		},
		{
			name: "system-prompt with unknown agent",
			cmd:  repl.Command{Name: "system-prompt", Args: []string{"unknown"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "not found") {
					t.Errorf("expected not found message, got %q", text)
				}
			},
		},
		{
			name: "config shows parameters",
			cmd:  repl.Command{Name: "config"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "openai.api_key") {
					t.Error("expected openai.api_key in config output")
				}
				if !strings.Contains(text, "default_model") {
					t.Error("expected default_model in config output")
				}
				// api_key should be masked
				if strings.Contains(text, "sk-secret-key-1234") {
					t.Error("api_key should be masked")
				}
				if !strings.Contains(text, "1234") {
					t.Error("expected last 4 chars of api_key visible")
				}
			},
		},
		{
			name: "max_tokens writes global config",
			cmd:  repl.Command{Name: "max_tokens", Args: []string{"4096"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "4096") {
					t.Fatalf("expected new max_tokens in output, got %q", text)
				}
			},
		},
		{
			name: "max_tokens without value shows global config",
			cmd:  repl.Command{Name: "max_tokens"},
			setup: func(d *testDeps) {
				d.cfg = stubConfig{ints: map[string]int{"max_tokens": 8192}}
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "8192") {
					t.Fatalf("expected current max_tokens in output, got %q", text)
				}
			},
		},
		{
			name:    "max_tokens with invalid value returns error",
			cmd:     repl.Command{Name: "max_tokens", Args: []string{"nope"}},
			wantErr: true,
		},
		{
			name: "mcp with nil mcpInfo shows no servers",
			cmd:  repl.Command{Name: "mcp"},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "(no MCP servers configured)") {
					t.Errorf("expected no servers message, got %q", text)
				}
			},
		},
		{
			name: "mcp with empty servers shows no servers",
			cmd:  repl.Command{Name: "mcp"},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithMCPInfo(&stubMCPInfo{}))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "(no MCP servers configured)") {
					t.Errorf("expected no servers message, got %q", text)
				}
			},
		},
		{
			name: "mcp with connected server shows info",
			cmd:  repl.Command{Name: "mcp"},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithMCPInfo(&stubMCPInfo{
					servers: []*mcp.ServerInfo{
						{
							Name:        "rune",
							Command:     "runectl",
							Status:      mcp.StatusConnected,
							ConnectedAt: time.Now().Add(-5 * time.Minute),
							ToolCount:   3,
							ToolNames:   []string{"search", "read", "write"},
						},
					},
				}))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "rune") {
					t.Error("expected server name in output")
				}
				if !strings.Contains(text, "connected") {
					t.Error("expected connected status in output")
				}
				if !strings.Contains(text, "runectl") {
					t.Error("expected command in output")
				}
				if !strings.Contains(text, "Tools: 3") {
					t.Error("expected tool count in output")
				}
			},
		},
		{
			name: "mcp with error server shows error",
			cmd:  repl.Command{Name: "mcp"},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithMCPInfo(&stubMCPInfo{
					servers: []*mcp.ServerInfo{
						{
							Name:    "bad",
							Command: "nope",
							Status:  mcp.StatusError,
							Error:   "connection refused",
						},
					},
				}))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "bad") {
					t.Error("expected server name in output")
				}
				if !strings.Contains(text, "connection refused") {
					t.Error("expected error message in output")
				}
			},
		},
		{
			name:    "chats compact without ID returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"compact"}},
			wantErr: true,
		},
		{
			name: "chats compact creates compacted conversation",
			cmd:  repl.Command{Name: "chats", Args: []string{"compact", "conv-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "You are a helpful assistant"},
						{Role: llm.RoleUser, Content: "hello"},
						{Role: llm.RoleAssistant, Content: "hi there"},
					},
				})
				d.svc = &stubService{response: "Summary of conversation"}
			},
			assertOut: func(t *testing.T, text string) {
				if text != "" {
					t.Errorf("expected empty output, got %q", text)
				}
			},
		},
		{
			name:    "chats compact with unknown conversation returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"compact", "unknown"}},
			wantErr: true,
		},
		// --- chats fork tests ---
		{
			name: "chats fork opens floating picker",
			cmd:  repl.Command{Name: "chats", Args: []string{"fork", "conv-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "You are a helpful assistant"},
						{Role: llm.RoleUser, Content: "hello"},
						{Role: llm.RoleAssistant, Content: "hi there"},
						{Role: llm.RoleUser, Content: "how are you"},
					},
				})
			},
			wantFloating: 1,
		},
		{
			name:    "chats fork rejects extra arguments",
			cmd:     repl.Command{Name: "chats", Args: []string{"fork", "conv-1", "2"}},
			wantErr: true,
		},
		{
			name:    "chats fork unknown conversation returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"fork", "missing"}},
			wantErr: true,
		},
		{
			name: "chats fork opens floating picker even without selectable messages",
			cmd:  repl.Command{Name: "chats", Args: []string{"fork", "conv-1"}},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleSystem, Content: "You are a helpful assistant"},
					},
				})
			},
			wantFloating: 1,
		},
		// --- chats export tests ---
		{
			name:    "chats export without ID returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"export"}},
			wantErr: true,
		},
		{
			name: "chats export writes conversation markdown",
			cmd:  repl.Command{Name: "chats", Args: []string{"export", "conv-1"}},
			setup: func(d *testDeps) {
				_ = d.store.Create(context.Background(), dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "hello"},
						{Role: llm.RoleAssistant, Content: "hi there"},
						{Role: llm.RoleUser, Content: "how are you"},
						{Role: llm.RoleAssistant, Content: "doing well"},
					},
				})
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "Exported conversation to") {
					t.Errorf("expected export confirmation, got %q", text)
				}
				if !strings.Contains(text, ".md") {
					t.Errorf("expected .md extension in output, got %q", text)
				}
			},
		},
		{
			name: "chats export skips tool messages",
			cmd:  repl.Command{Name: "chats", Args: []string{"export", "conv-1"}},
			setup: func(d *testDeps) {
				_ = d.store.Create(context.Background(), dialoguemanager.Dialogue{
					ID: "conv-1",
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "do something"},
						{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
							{ID: "tc1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"foo.go"}`}},
						}},
						{Role: llm.RoleTool, ToolCallID: "tc1", Content: "file contents"},
						{Role: llm.RoleAssistant, Content: "here is the file"},
					},
				})
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "Exported conversation to") {
					t.Errorf("expected export confirmation, got %q", text)
				}
			},
		},
		{
			name:    "chats export --audit without audit store returns error",
			cmd:     repl.Command{Name: "chats", Args: []string{"export", "--audit", "conv-1"}},
			wantErr: true,
		},
		{
			name: "chats export --audit without ID returns error",
			cmd:  repl.Command{Name: "chats", Args: []string{"export", "--audit"}},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithAuditStore(llm.NewAuditStore(
					storagestub.NewInMemoryService(),
				)))
			},
			wantErr: true,
		},
		{
			name: "chats export --audit with no entries shows empty message",
			cmd:  repl.Command{Name: "chats", Args: []string{"export", "--audit", "conv-1"}},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithAuditStore(llm.NewAuditStore(
					storagestub.NewInMemoryService(),
				)))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "(no completions recorded)") {
					t.Errorf("expected no completions message, got %q", text)
				}
			},
		},
		{
			name: "chats export --audit writes JSONL file",
			cmd:  repl.Command{Name: "chats", Args: []string{"export", "--audit", "conv-1"}},
			setup: func(d *testDeps) {
				store := llm.NewAuditStore(storagestub.NewInMemoryService())
				store.Append(context.Background(), "conv-1", llm.AuditEntry{
					EstimatedTokens: 100,
					ContextWindow:   4096,
					Tools:           2,
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "hello"},
					},
					Response: &llm.Message{Role: llm.RoleAssistant, Content: "hi"},
					Usage:    llm.Usage{TokensSent: 50, TokensReceived: 10},
				})
				store.Append(context.Background(), "conv-1", llm.AuditEntry{
					EstimatedTokens: 200,
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "how are you"},
					},
				})
				d.opts = append(d.opts, WithAuditStore(store))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "Exported 2 audit entries to") {
					t.Errorf("expected export confirmation, got %q", text)
				}
				if !strings.Contains(text, ".jsonl") {
					t.Errorf("expected .jsonl extension in output, got %q", text)
				}
			},
		},
		{
			name:    "exit returns ErrExit",
			cmd:     repl.Command{Name: "exit"},
			wantErr: true,
		},
		{
			name:    "unknown command returns error",
			cmd:     repl.Command{Name: "unknown"},
			wantErr: true,
		},
		// --- skills subcommand tests ---
		{
			name:    "skills with no args returns usage error",
			cmd:     repl.Command{Name: "skills"},
			wantErr: true,
		},
		{
			name: "skills list with no filesystem skills still shows builtins",
			cmd:  repl.Command{Name: "skills", Args: []string{"list"}},
			assertOut: func(t *testing.T, text string) {
				for _, name := range []string{"explore", "plan"} {
					if !strings.Contains(text, name) {
						t.Errorf("expected builtin skill %q in output, got %q", name, text)
					}
				}
			},
		},
		{
			name: "skills list with skills",
			cmd:  repl.Command{Name: "skills", Args: []string{"list"}},
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			assertOut: func(t *testing.T, text string) {
				for _, name := range []string{"debug", "lint", "refactor", "test-skill"} {
					if !strings.Contains(text, name) {
						t.Errorf("expected skill %q in output", name)
					}
				}
			},
		},
		{
			name: "skills show with valid name opens floating",
			cmd:  repl.Command{Name: "skills", Args: []string{"show", "test-skill"}},
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			wantFloating: 1,
		},
		{
			name: "skills show with unknown name returns error",
			cmd:  repl.Command{Name: "skills", Args: []string{"show", "nonexistent"}},
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			wantErr: true,
		},
		{
			name:    "skills show without name returns error",
			cmd:     repl.Command{Name: "skills", Args: []string{"show"}},
			wantErr: true,
		},
		{
			name: "skills list-dirs shows configured dirs",
			cmd:  repl.Command{Name: "skills", Args: []string{"list-dirs"}},
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			assertOut: func(t *testing.T, text string) {
				if text == "" {
					t.Error("expected non-empty output")
				}
			},
		},
		{
			name: "skills list-dirs with no dirs",
			cmd:  repl.Command{Name: "skills", Args: []string{"list-dirs"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "(no skill directories configured)") {
					t.Errorf("expected no dirs message, got %q", text)
				}
			},
		},
		{
			name:    "skills add-dir without dir returns error",
			cmd:     repl.Command{Name: "skills", Args: []string{"add-dir"}},
			wantErr: true,
		},
		{
			name:    "skills remove-dir without dir returns error",
			cmd:     repl.Command{Name: "skills", Args: []string{"remove-dir"}},
			wantErr: true,
		},
		{
			name:    "skills unknown subcommand returns error",
			cmd:     repl.Command{Name: "skills", Args: []string{"unknown"}},
			wantErr: true,
		},
		// --- dream command tests ---
		{
			name: "dream runs when required deps are provided",
			cmd:  repl.Command{Name: "dream"},
			setup: func(d *testDeps) {
				d.storage = storagestub.NewInMemoryService()
				d.fs = osFileSystem{}
				d.notifications = stubNotifications{}
				d.opts = append(d.opts, withExecutor(testShellExec{}), withLSP(&stubTestLSP{}))

				memPath := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(memPath, "go.mod"), []byte("module memories\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(memPath, "version"), []byte("3\n"), 0o644))
				d.dataPath = memPath
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "Done: Dream complete") {
					t.Errorf("expected dream success output, got %q", text)
				}
			},
		},
		{
			name: "dream with no data path returns error",
			cmd:  repl.Command{Name: "dream"},
			setup: func(d *testDeps) {
				d.storage = &stubStorage{}
				d.opts = append(d.opts, withExecutor(testShellExec{}), withLSP(&stubTestLSP{}))
			},
			wantErr: true,
		},
		{
			name:    "dream --model without value returns error",
			cmd:     repl.Command{Name: "dream", Args: []string{"--model"}},
			wantErr: true,
		},
		{
			name:    "dream with unknown flag returns error",
			cmd:     repl.Command{Name: "dream", Args: []string{"--unknown"}},
			wantErr: true,
		},
		{
			name:    "dream with unknown model returns error",
			cmd:     repl.Command{Name: "dream", Args: []string{"--model", "nonexistent"}},
			wantErr: true,
		},
		// --- effort tests ---
		{
			name: "effort shows current level default high",
			cmd:  repl.Command{Name: "effort"},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithEffort(
					func() llm.ReasoningEffort { return "" },
					func(llm.ReasoningEffort) {},
				))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "high") {
					t.Errorf("expected default effort 'high', got %q", text)
				}
			},
		},
		{
			name: "effort shows current non-default level",
			cmd:  repl.Command{Name: "effort"},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithEffort(
					func() llm.ReasoningEffort { return llm.ReasoningEffortLow },
					func(llm.ReasoningEffort) {},
				))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "low") {
					t.Errorf("expected effort 'low', got %q", text)
				}
			},
		},
		{
			name: "effort sets level",
			cmd:  repl.Command{Name: "effort", Args: []string{"medium"}},
			setup: func(d *testDeps) {
				var current llm.ReasoningEffort
				d.opts = append(d.opts, WithEffort(
					func() llm.ReasoningEffort { return current },
					func(e llm.ReasoningEffort) { current = e },
				))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "medium") {
					t.Errorf("expected 'medium' in output, got %q", text)
				}
			},
		},
		{
			name: "effort rejects invalid level",
			cmd:  repl.Command{Name: "effort", Args: []string{"turbo"}},
			setup: func(d *testDeps) {
				d.opts = append(d.opts, WithEffort(
					func() llm.ReasoningEffort { return "" },
					func(llm.ReasoningEffort) {},
				))
			},
			wantErr: true,
		},
		{
			name:    "effort without WithEffort returns error",
			cmd:     repl.Command{Name: "effort"},
			wantErr: true,
		},
		{
			name:    "providers with no args returns usage error",
			cmd:     repl.Command{Name: "providers"},
			wantErr: true,
		},
		{
			name:    "providers unknown provider returns error",
			cmd:     repl.Command{Name: "providers", Args: []string{"unknown"}},
			wantErr: true,
		},
		{
			name:    "providers codex with no command returns usage error",
			cmd:     repl.Command{Name: "providers", Args: []string{"codex"}},
			wantErr: true,
		},
		{
			name: "providers codex status unauthenticated",
			cmd:  repl.Command{Name: "providers", Args: []string{"codex", "status"}},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "not authenticated") {
					t.Fatalf("expected not authenticated status, got %q", text)
				}
			},
		},
		{
			name: "agent parent dispatches providers codex status",
			cmd:  repl.Command{Name: CommandName, Args: []string{"providers", "codex", "status"}},
			setup: func(d *testDeps) {
				require.NoError(t, codex.SaveCredential(context.Background(), d.storage, codex.Credential{
					AccessToken:  "access-token",
					RefreshToken: "refresh-token",
					Email:        "user@example.com",
					AccountID:    "account-123",
					LastRefresh:  time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
					Expiry:       time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC),
				}))
			},
			assertOut: func(t *testing.T, text string) {
				if !strings.Contains(text, "authenticated") || !strings.Contains(text, "user@example.com") {
					t.Fatalf("expected authenticated account output, got %q", text)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newTestDeps()
			if tt.setup != nil {
				tt.setup(deps)
			}

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
				deps.localRegistry,
				deps.opts...,
			)

			ctx := context.Background()
			var (
				it  iterator.Iterator[component.Responsive]
				err error
			)
			if tt.name == "dream with no data path returns error" {
				assert.PanicsWithValue(t, "dream: DataPath is required", func() {
					it, err = sh.HandleCommand(ctx, tt.cmd, repl.NopProgressWriter())
				})
				return
			}
			it, err = sh.HandleCommand(ctx, tt.cmd, repl.NopProgressWriter())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			text := renderOutput(t, ctx, it)

			if tt.wantFloating > 0 {
				if deps.wm.floatingCalls != tt.wantFloating {
					t.Errorf("floating calls: got %d, want %d", deps.wm.floatingCalls, tt.wantFloating)
				}
				if text != "" {
					t.Errorf("expected empty output when floating window opens, got %q", text)
				}
				return
			}
			if tt.assertOut != nil {
				tt.assertOut(t, text)
			}
		})
	}
}

func TestComplete(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		args     []string
		setup    func(*testDeps)
		wantAny  []string // at least these should appear
		wantNone []string // none of these should appear
	}{
		{
			name:    "empty prefix returns all commands",
			cmd:     "",
			wantAny: commandNames,
		},
		{
			name:    "agent parent completes subcommands",
			cmd:     CommandName,
			args:    []string{""},
			wantAny: commandNames,
		},
		{
			name:    "agent parent forwards nested completion",
			cmd:     CommandName,
			args:    []string{"chats", ""},
			wantAny: []string{"clear", "compact", "export", "list", "log", "show"},
		},
		{
			name:     "prefix filters commands",
			cmd:      "mo",
			wantAny:  []string{"model", "models"},
			wantNone: []string{"help", "exit"},
		},
		{
			name: "model completes with model names and dialogue IDs",
			cmd:  "model",
			args: []string{""},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{ID: "sess-1"})
				d.store.set(dialoguemanager.Dialogue{ID: "sess-2"})
			},
			wantAny: []string{"gpt-4", "gpt-3.5", "sess-1", "sess-2"},
		},
		{
			name:    "chats completes subcommands",
			cmd:     "chats",
			args:    []string{""},
			wantAny: []string{"clear", "compact", "export", "list", "log", "show"},
		},
		{
			name:    "providers completes provider names",
			cmd:     "providers",
			args:    []string{""},
			wantAny: []string{"codex"},
		},
		{
			name:    "providers codex completes subcommands",
			cmd:     "providers",
			args:    []string{"codex", ""},
			wantAny: []string{"login", "status"},
		},
		{
			name:    "chats completes subcommands with prefix",
			cmd:     "chats",
			args:    []string{"c"},
			wantAny: []string{"clear", "compact"},
		},
		{
			name: "chats show completes with dialogue IDs",
			cmd:  "chats",
			args: []string{"show", ""},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{ID: "conv-1"})
			},
			wantAny: []string{"conv-1"},
		},
		{
			name: "chats compact completes with dialogue IDs",
			cmd:  "chats",
			args: []string{"compact", ""},
			setup: func(d *testDeps) {
				d.store.set(dialoguemanager.Dialogue{ID: "conv-1"})
			},
			wantAny: []string{"conv-1"},
		},
		{
			name:    "system-prompt completes with agent IDs",
			cmd:     "system-prompt",
			args:    []string{""},
			wantAny: []string{"default"},
		},
		{
			name: "no completions for second arg",
			cmd:  "model",
			args: []string{"a", "b"},
		},
		{
			name: "empty prefix includes skill names",
			cmd:  "",
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			wantAny: append(slices.Clone(commandNames), "debug", "lint", "refactor", "test-skill"),
		},
		{
			name: "prefix matches skill names",
			cmd:  "de",
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			wantAny:  []string{"debug"},
			wantNone: []string{"lint", "refactor", "test-skill"},
		},
		// --- effort completion tests ---
		{
			name:    "effort completes with levels",
			cmd:     "effort",
			args:    []string{""},
			wantAny: []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"},
		},
		// --- skills completion tests ---
		{
			name:    "skills completes subcommands",
			cmd:     "skills",
			args:    []string{""},
			wantAny: []string{"list", "show", "list-dirs", "add-dir", "remove-dir"},
		},
		{
			name:     "skills completes subcommands with prefix",
			cmd:      "skills",
			args:     []string{"li"},
			wantAny:  []string{"list", "list-dirs"},
			wantNone: []string{"show", "add-dir", "remove-dir"},
		},
		{
			name: "skills show completes skill names",
			cmd:  "skills",
			args: []string{"show", ""},
			setup: func(d *testDeps) {
				d.skillRegistry = skills.NewRegistry(osFileSystem{}, dirURI(""), []string{d.skillDir}, nil)
			},
			wantAny: []string{"test-skill"},
		},
		{
			name: "local delete completes cached model references",
			cmd:  "local",
			args: []string{"delete", "huggingface.co/foo"},
			setup: func(d *testDeps) {
				reg, err := llamacpp.NewRegistry(t.TempDir())
				require.NoError(t, err)
				seedLocalModel(t, reg, "huggingface.co", "foo/bar-GGUF", "latest", []byte("gguf-complete"))
				d.localRegistry = reg
			},
			wantAny: []string{"huggingface.co/foo/bar-GGUF:latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newTestDeps()
			if tt.setup != nil {
				tt.setup(deps)
			}

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
				deps.localRegistry,
				deps.opts...,
			)

			ctx := context.Background()
			it, err := sh.Complete(ctx, tt.cmd, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			results, err := iterator.ToSlice(ctx, it)
			if err != nil {
				t.Fatalf("consume iterator: %v", err)
			}

			for _, want := range tt.wantAny {
				found := false
				for _, r := range results {
					if r == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in completions %v", want, results)
				}
			}
			for _, notWant := range tt.wantNone {
				for _, r := range results {
					if r == notWant {
						t.Errorf("did not expect %q in completions %v", notWant, results)
					}
				}
			}
		})
	}
}

func TestHelp(t *testing.T) {
	deps := newTestDeps()
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
		deps.localRegistry,
		deps.opts...,
	)

	ctx := context.Background()
	it, err := sh.Help(ctx, []string{"chats"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := renderOutput(t, ctx, it)
	if !strings.Contains(text, "list") || !strings.Contains(text, "show <id>") {
		t.Fatalf("expected nested command name in help output, got %q", text)
	}
	if !strings.Contains(text, "show") || !strings.Contains(text, "export") {
		t.Fatalf("expected chats subcommands in help output, got %q", text)
	}
}

func TestHelpLocalDownload(t *testing.T) {
	deps := newTestDeps()
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
		deps.localRegistry,
		deps.opts...,
	)

	ctx := context.Background()
	it, err := sh.Help(ctx, []string{"local", "download"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := renderOutput(t, ctx, it)
	for _, want := range []string{
		"local download <reference>",
		"Synopsis",
		"Examples",
		"hf.co/unsloth/",
		"docker.io/library/myrepo:v1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in help output, got %q", want, text)
		}
	}
}

// --- test helpers ---

type testDeps struct {
	svc           llm.Service
	modelRegistry llmregistry.Registry
	defaultModel  string
	store         *memStore
	registry      *agent.Registry
	agentsConfig  *agent.Cfg
	cfg           config.Config
	opts          []Option
	wm            *stubWindowManager
	skillRegistry *skills.SkillRegistry
	workspaceRoot workspaceapi.URI
	fs            workspaceapi.FileSystem
	skillDir      string
	storage       storageapi.Service
	notifications browserapi.Notifications
	dataPath      string
	localRegistry *llamacpp.Registry
}

func withExecutor(exec workspaceapi.Executor) Option {
	return func(s *shell) { s.exec = exec }
}

func withLSP(lsp semanticapi.LSP) Option {
	return func(s *shell) { s.lsp = lsp }
}

func newTestDeps() *testDeps {
	// Create a temp skill dir with skills for tests that need them.
	skillDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		panic(err)
	}
	for _, s := range []struct{ name, desc, body string }{
		{"debug", "Debug issues step by step", "Debug body"},
		{"lint", "Run linters and fix warnings", "Lint body"},
		{"refactor", "Refactor code safely", "Refactor body"},
		{"test-skill", "A test skill", "Test skill body"},
	} {
		sub := filepath.Join(skillDir, s.name)
		_ = os.MkdirAll(sub, 0o755)
		content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s", s.name, s.desc, s.body)
		_ = os.WriteFile(filepath.Join(sub, "SKILL.md"), []byte(content), 0o644)
	}

	tools := []agent.Tool{
		stubTool{name: "read_file", desc: "Reads a file"},
		stubTool{name: "write_file", desc: "Writes a file"},
	}
	workspaceRoot := dirURI(skillDir) // reuse temp dir as workspace root
	// The shell requires a non-nil local llamacpp.Registry. We seed an
	// empty one rooted under the same temp skill dir so every test
	// gets a freshly-scoped cache; the individual setup closures
	// override this when they need to seed manifests.
	modelsRoot, err := os.MkdirTemp("", "llamacpp-models-*")
	if err != nil {
		panic(err)
	}
	localReg, err := llamacpp.NewRegistry(modelsRoot)
	if err != nil {
		panic(err)
	}
	return &testDeps{
		skillDir:      skillDir,
		workspaceRoot: workspaceRoot,
		fs:            &stubFS{},
		wm:            &stubWindowManager{},
		svc:           &stubService{response: "default summary"},
		skillRegistry: skills.NewRegistry(nopFileSystem{}, workspaceRoot, nil, nil),
		modelRegistry: func() llmregistry.Registry {
			r := llmregistry.NewStatic()
			r.Register(
				llmregistry.ModelEntry{Name: "gpt-4", Provider: "openai", ContextWindow: 128000},
				llmregistry.ModelEntry{Name: "gpt-3.5", Provider: "openai", ContextWindow: 16384},
			)
			return r
		}(),
		defaultModel: "gpt-4",
		store:        newMemStore(),
		registry:     agent.NewRegistry(tools...),
		agentsConfig: agent.NewConfig([]agent.Definition{
			{
				ID:           "default",
				Name:         "Default Agent",
				Model:        "gpt-4",
				SystemPrompt: "You are a helpful assistant",
				AllowAny:     true,
			},
		}),
		cfg: stubConfig{
			strings: map[string]string{
				"base_url":      "https://api.openai.com",
				"default_model": "gpt-4",
			},
			floats: map[string]float64{
				"temperature": 0.7,
			},
			configs: map[string]config.Config{
				"openai": stubConfig{
					strings: map[string]string{"api_key": "sk-secret-key-1234"},
				},
			},
		},
		storage:       storagestub.NewInMemoryService(),
		notifications: stubNotifications{},
		localRegistry: localReg,
	}
}

func renderOutput(t *testing.T, ctx context.Context, it iterator.Iterator[component.Responsive]) string {
	t.Helper()
	defer it.Close() //nolint:errcheck

	var buf strings.Builder
	for {
		r, ok := it.Next(ctx)
		if !ok {
			break
		}
		const width = 200
		h := r.Height(width)
		if h == 0 {
			h = 1
		}
		r.Resize(width, h)
		w := term.NewStringWriter(width, h)
		if err := w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}
		r.Draw(w)
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
		buf.WriteString(w.String())
	}
	if it.Err() != nil {
		t.Fatalf("iterator error: %v", it.Err())
	}
	return buf.String()
}

// --- stub tool ---

type stubTool struct {
	name string
	desc string
}

func (s stubTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        s.name,
			Description: s.desc,
		},
	}
}

func (s stubTool) Execute(context.Context, string) agent.ToolResult {
	return agent.ToolResult{}
}

func (s stubTool) Summary(string) string { return "" }

// --- in-memory store ---

type memStore struct {
	data map[string]dialoguemanager.Dialogue
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string]dialoguemanager.Dialogue)}
}

func (m *memStore) set(d dialoguemanager.Dialogue) {
	d.MessageCount = len(d.Messages)
	m.data[d.ID] = d
}

func (m *memStore) Health(context.Context) error { return nil }

func (m *memStore) Create(_ context.Context, d dialoguemanager.Dialogue) error {
	if _, ok := m.data[d.ID]; ok {
		return storageapi.ErrAlreadyExists
	}
	d.MessageCount = len(d.Messages)
	m.data[d.ID] = d
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (dialoguemanager.Dialogue, error) {
	d, ok := m.data[id]
	if !ok {
		return dialoguemanager.Dialogue{}, storageapi.ErrNotFound
	}
	return d, nil
}

func (m *memStore) Delete(_ context.Context, id string) error {
	delete(m.data, id)
	return nil
}

func (m *memStore) AppendMessages(
	_ context.Context, d dialoguemanager.Dialogue, msgs []llm.Message, _ llm.DialogueUsage,
) error {
	existing := m.data[d.ID]
	existing.Messages = append(existing.Messages, msgs...)
	existing.MessageCount = len(existing.Messages)
	m.data[d.ID] = existing
	return nil
}

func (m *memStore) List(context.Context) (iterator.Iterator[dialoguemanager.DialogueHeader], error) {
	var all []dialoguemanager.DialogueHeader
	for _, d := range m.data {
		all = append(all, d.Header())
	}
	return iterator.FromSlice(all), nil
}

func (m *memStore) ArchiveAndReplace(_ context.Context, p dialoguemanager.ArchiveAndReplaceParams) error {
	archived := p.Dialogue
	archived.ID = p.ArchivedDialogueID
	m.data[p.ArchivedDialogueID] = archived
	replaced := p.Dialogue
	replaced.Messages = p.Messages
	replaced.MessageCount = len(p.Messages)
	m.data[p.Dialogue.ID] = replaced
	return nil
}

// --- stub config ---

type stubConfig struct {
	strings map[string]string
	floats  map[string]float64
	ints    map[string]int
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

func (c stubConfig) GetBool(string) (bool, error) { return false, config.ErrNotFound }
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

// --- stub window manager ---

type stubWindowManager struct {
	floatingCalls int
}

func (s *stubWindowManager) Focus() (browserapi.Window, error) {
	return stubWindow(0), nil
}

func (s *stubWindowManager) Split(browserapi.Orientation, browserapi.Window, browserapi.Handler) (browserapi.Window, error) {
	return stubWindow(0), nil
}

func (s *stubWindowManager) Floating(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error) {
	s.floatingCalls++
	return stubWindow(1), nil
}

func (s *stubWindowManager) Bar(browserapi.BarConfig, tui.Handler) error { return nil }

func (s *stubWindowManager) Tab(workspaceapi.URI, rune, string, browserapi.Handler) (browserapi.Handler, error) {
	return nil, nil
}

func (s *stubWindowManager) SetWindowContent(browserapi.Window, browserapi.Handler) error {
	return nil
}

func (s *stubWindowManager) CloseWindow(browserapi.Window) error { return nil }

type stubWindow uint64

func (w stubWindow) WindowID() uint64 { return uint64(w) }

// --- stub MCP info ---

type stubMCPInfo struct {
	servers []*mcp.ServerInfo
}

func (s *stubMCPInfo) Servers() []*mcp.ServerInfo { return s.servers }

// --- stub storage ---

type stubStorage struct{}

func (stubStorage) Create(context.Context, string, any) error { return nil }
func (stubStorage) Set(context.Context, string, any) error    { return nil }
func (stubStorage) Update(context.Context, string, []storageapi.Update, ...storageapi.Precondition) error {
	return nil
}
func (stubStorage) Get(context.Context, string, any) error { return storageapi.ErrNotFound }
func (stubStorage) Delete(context.Context, string) error   { return nil }
func (stubStorage) List(context.Context, []storageapi.Filter) (storageapi.Iterator, error) {
	return nil, nil
}
func (stubStorage) Partition(string) (storageapi.Service, error) { return stubStorage{}, nil }
func (stubStorage) Close() error                                 { return nil }

type stubNotifications struct{}

func (stubNotifications) Notify(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "notif-1", nil
}

func (stubNotifications) NotifyOnce(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return stubNotifications{}.Notify(level, msg, args...)
}

func (stubNotifications) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

// --- stub filesystem ---

type stubFS struct{}

func (stubFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}

func (stubFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}

func (stubFS) Remove(path string) error                     { return os.Remove(path) }
func (stubFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (stubFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (stubFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

// --- nop filesystem (for nil dirs) ---

type nopFileSystem struct{}

func (nopFileSystem) URI(path string) (workspaceapi.URI, error) { return workspaceapi.URI{}, nil }
func (nopFileSystem) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, os.ErrNotExist
}
func (nopFileSystem) Remove(string) error                   { return nil }
func (nopFileSystem) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (nopFileSystem) ReadDir(string) ([]os.DirEntry, error) { return nil, nil }
func (nopFileSystem) MkdirAll(string, os.FileMode) error    { return nil }

// --- os filesystem (for real dirs) ---

type osFileSystem struct{}

func (osFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.CurrentUserHostURI(path)
}
func (osFileSystem) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}
func (osFileSystem) Remove(path string) error                     { return os.Remove(path) }
func (osFileSystem) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (osFileSystem) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (osFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

type testShellExec struct{}

func (testShellExec) Start(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
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

func (testShellExec) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (testShellExec) Close() error { return nil }

type stubTestLSP struct{}

func (s *stubTestLSP) Initialize(context.Context, semanticapi.InitializeParams) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}
func (s *stubTestLSP) Initialized(context.Context) error { return nil }
func (s *stubTestLSP) Shutdown(context.Context) error    { return nil }
func (s *stubTestLSP) Exit(context.Context) error        { return nil }
func (s *stubTestLSP) DidOpen(context.Context, semanticapi.DidOpenTextDocumentParams) error {
	return nil
}
func (s *stubTestLSP) DidChange(context.Context, semanticapi.DidChangeTextDocumentParams) error {
	return nil
}
func (s *stubTestLSP) DidClose(context.Context, semanticapi.DidCloseTextDocumentParams) error {
	return nil
}
func (s *stubTestLSP) DidSave(context.Context, semanticapi.DidSaveTextDocumentParams) error {
	return nil
}
func (s *stubTestLSP) Completion(context.Context, semanticapi.CompletionParams) (semanticapi.CompletionResult, error) {
	return semanticapi.CompletionResult{}, nil
}
func (s *stubTestLSP) Hover(context.Context, semanticapi.HoverParams) (*semanticapi.Hover, error) {
	return nil, nil
}
func (s *stubTestLSP) SignatureHelp(context.Context, semanticapi.SignatureHelpParams) (*semanticapi.SignatureHelp, error) {
	return nil, nil
}
func (s *stubTestLSP) Definition(context.Context, semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (s *stubTestLSP) Declaration(context.Context, semanticapi.DeclarationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (s *stubTestLSP) TypeDefinition(context.Context, semanticapi.TypeDefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (s *stubTestLSP) Implementation(context.Context, semanticapi.ImplementationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (s *stubTestLSP) References(context.Context, semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
	return nil, nil
}
func (s *stubTestLSP) DocumentHighlight(context.Context, semanticapi.DocumentHighlightParams) ([]semanticapi.DocumentHighlight, error) {
	return nil, nil
}
func (s *stubTestLSP) DocumentSymbol(context.Context, semanticapi.DocumentSymbolParams) (semanticapi.DocumentSymbolResult, error) {
	return semanticapi.DocumentSymbolResult{}, nil
}
func (s *stubTestLSP) CodeAction(context.Context, semanticapi.CodeActionParams) ([]semanticapi.CodeActionResult, error) {
	return nil, nil
}
func (s *stubTestLSP) CodeLens(context.Context, semanticapi.CodeLensParams) ([]semanticapi.CodeLens, error) {
	return nil, nil
}
func (s *stubTestLSP) Formatting(context.Context, semanticapi.DocumentFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) RangeFormatting(context.Context, semanticapi.DocumentRangeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) Rename(context.Context, semanticapi.RenameParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) PrepareRename(context.Context, semanticapi.PrepareRenameParams) (*semanticapi.PrepareRenameResult, error) {
	return nil, nil
}
func (s *stubTestLSP) FoldingRange(context.Context, semanticapi.FoldingRangeParams) ([]semanticapi.FoldingRange, error) {
	return nil, nil
}
func (s *stubTestLSP) SelectionRange(context.Context, semanticapi.SelectionRangeParams) ([]semanticapi.SelectionRange, error) {
	return nil, nil
}
func (s *stubTestLSP) SemanticTokensFull(context.Context, semanticapi.SemanticTokensParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}
func (s *stubTestLSP) SemanticTokensRange(context.Context, semanticapi.SemanticTokensRangeParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}
func (s *stubTestLSP) Diagnostic(context.Context, semanticapi.DocumentDiagnosticParams) (semanticapi.DocumentDiagnosticReport, error) {
	return semanticapi.DocumentDiagnosticReport{}, nil
}
func (s *stubTestLSP) WorkspaceDiagnostic(context.Context, semanticapi.WorkspaceDiagnosticParams) (semanticapi.WorkspaceDiagnosticReport, error) {
	return semanticapi.WorkspaceDiagnosticReport{}, nil
}
func (s *stubTestLSP) WorkspaceSymbol(context.Context, semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
	return nil, nil
}
func (s *stubTestLSP) ExecuteCommand(context.Context, semanticapi.ExecuteCommandParams) (string, error) {
	return "", nil
}
func (s *stubTestLSP) PrepareCallHierarchy(context.Context, semanticapi.CallHierarchyPrepareParams) ([]semanticapi.CallHierarchyItem, error) {
	return nil, nil
}
func (s *stubTestLSP) CallHierarchyIncomingCalls(context.Context, semanticapi.CallHierarchyIncomingCallsParams) ([]semanticapi.CallHierarchyIncomingCall, error) {
	return nil, nil
}
func (s *stubTestLSP) CallHierarchyOutgoingCalls(context.Context, semanticapi.CallHierarchyOutgoingCallsParams) ([]semanticapi.CallHierarchyOutgoingCall, error) {
	return nil, nil
}
func (s *stubTestLSP) CompletionResolve(context.Context, semanticapi.CompletionItem) (semanticapi.CompletionItem, error) {
	return semanticapi.CompletionItem{}, nil
}
func (s *stubTestLSP) CodeLensResolve(context.Context, semanticapi.CodeLens) (semanticapi.CodeLens, error) {
	return semanticapi.CodeLens{}, nil
}
func (s *stubTestLSP) DocumentColor(context.Context, semanticapi.DocumentColorParams) ([]semanticapi.ColorInformation, error) {
	return nil, nil
}
func (s *stubTestLSP) ColorPresentation(context.Context, semanticapi.ColorPresentationParams) ([]semanticapi.ColorPresentation, error) {
	return nil, nil
}
func (s *stubTestLSP) DocumentLink(context.Context, semanticapi.DocumentLinkParams) ([]semanticapi.DocumentLink, error) {
	return nil, nil
}
func (s *stubTestLSP) DocumentLinkResolve(context.Context, semanticapi.DocumentLink) (semanticapi.DocumentLink, error) {
	return semanticapi.DocumentLink{}, nil
}
func (s *stubTestLSP) OnTypeFormatting(context.Context, semanticapi.DocumentOnTypeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) LinkedEditingRange(context.Context, semanticapi.LinkedEditingRangeParams) (*semanticapi.LinkedEditingRanges, error) {
	return nil, nil
}
func (s *stubTestLSP) Moniker(context.Context, semanticapi.MonikerParams) ([]semanticapi.Moniker, error) {
	return nil, nil
}
func (s *stubTestLSP) WillSaveWaitUntil(context.Context, semanticapi.WillSaveTextDocumentParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) SemanticTokensFullDelta(context.Context, semanticapi.SemanticTokensDeltaParams) (*semanticapi.SemanticTokensDelta, error) {
	return nil, nil
}
func (s *stubTestLSP) PrepareTypeHierarchy(context.Context, semanticapi.TypeHierarchyPrepareParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (s *stubTestLSP) TypeHierarchySupertypes(context.Context, semanticapi.TypeHierarchySupertypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (s *stubTestLSP) TypeHierarchySubtypes(context.Context, semanticapi.TypeHierarchySubtypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (s *stubTestLSP) InlayHint(context.Context, semanticapi.InlayHintParams) ([]semanticapi.InlayHint, error) {
	return nil, nil
}
func (s *stubTestLSP) InlayHintResolve(context.Context, semanticapi.InlayHint) (semanticapi.InlayHint, error) {
	return semanticapi.InlayHint{}, nil
}
func (s *stubTestLSP) InlineValue(context.Context, semanticapi.InlineValueParams) ([]semanticapi.InlineValue, error) {
	return nil, nil
}
func (s *stubTestLSP) WillCreateFiles(context.Context, semanticapi.CreateFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) WillRenameFiles(context.Context, semanticapi.RenameFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) WillDeleteFiles(context.Context, semanticapi.DeleteFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (s *stubTestLSP) WillSave(context.Context, semanticapi.WillSaveTextDocumentParams) error {
	return nil
}
func (s *stubTestLSP) DidChangeConfiguration(context.Context, semanticapi.DidChangeConfigurationParams) error {
	return nil
}
func (s *stubTestLSP) DidChangeWatchedFiles(context.Context, semanticapi.DidChangeWatchedFilesParams) error {
	return nil
}
func (s *stubTestLSP) DidChangeWorkspaceFolders(context.Context, semanticapi.DidChangeWorkspaceFoldersParams) error {
	return nil
}
func (s *stubTestLSP) WorkDoneProgressCancel(context.Context, semanticapi.WorkDoneProgressCancelParams) error {
	return nil
}
func (s *stubTestLSP) SetTrace(context.Context, semanticapi.SetTraceParams) error { return nil }
func (s *stubTestLSP) DidCreateFiles(context.Context, semanticapi.CreateFilesParams) error {
	return nil
}
func (s *stubTestLSP) DidRenameFiles(context.Context, semanticapi.RenameFilesParams) error {
	return nil
}
func (s *stubTestLSP) DidDeleteFiles(context.Context, semanticapi.DeleteFilesParams) error {
	return nil
}

var _ semanticapi.LSP = (*stubTestLSP)(nil)

func TestFormatHistoryMarkdown(t *testing.T) {
	tests := []struct {
		name          string
		msgs          []llm.Message
		includeSystem bool
		want          string
	}{
		{
			name: "empty messages returns empty string",
			msgs: nil,
			want: "",
		},
		{
			name:          "system message as blockquote",
			includeSystem: true,
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "You are helpful."},
			},
			want: "> **system:** You are helpful.",
		},
		{
			name: "system message omitted by default",
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "You are helpful."},
				{Role: llm.RoleUser, Content: "Hi"},
			},
			want: "# User\n\nHi",
		},
		{
			name: "user message with H1 header",
			msgs: []llm.Message{
				{Role: llm.RoleUser, Content: "hello world"},
			},
			want: "# User\n\nhello world",
		},
		{
			name: "assistant message with H1 header",
			msgs: []llm.Message{
				{Role: llm.RoleAssistant, Content: "I can help."},
			},
			want: "# Assistant\n\nI can help.",
		},
		{
			name: "assistant with reasoning content",
			msgs: []llm.Message{
				{Role: llm.RoleAssistant, Content: "The answer is 42.", ReasoningContent: "Let me think..."},
			},
			want: "# Assistant\n\n> Let me think...\n\nThe answer is 42.",
		},
		{
			name: "assistant with tool calls in metadata",
			msgs: []llm.Message{
				{
					Role:    llm.RoleAssistant,
					Content: "Let me read that file.",
					ToolCalls: []llm.ToolCall{
						{Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"foo.go"}`}},
					},
				},
			},
			want: "# Assistant\n\nLet me read that file.\n\n**Tool Call:** read_file\n```\n{\"path\":\"foo.go\"}\n```",
		},
		{
			name: "tool result with fenced code block",
			msgs: []llm.Message{
				{Role: llm.RoleTool, Name: "read_file", Content: "package main"},
			},
			want: "**Tool Result** (read_file):\n```\npackage main\n```",
		},
		{
			name:          "mixed conversation with system included",
			includeSystem: true,
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "Be helpful."},
				{Role: llm.RoleUser, Content: "Hi"},
				{Role: llm.RoleAssistant, Content: "Hello!"},
			},
			want: "> **system:** Be helpful.\n\n---\n\n# User\n\nHi\n\n---\n\n# Assistant\n\nHello!",
		},
		{
			name:          "mixed conversation with system excluded",
			includeSystem: false,
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "Be helpful."},
				{Role: llm.RoleUser, Content: "Hi"},
				{Role: llm.RoleAssistant, Content: "Hello!"},
			},
			want: "# User\n\nHi\n\n---\n\n# Assistant\n\nHello!",
		},
		{
			name: "assistant with only tool calls and no content",
			msgs: []llm.Message{
				{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{Function: llm.FunctionCall{Name: "search", Arguments: `{"q":"test"}`}},
					},
				},
			},
			want: "# Assistant\n\n\n**Tool Call:** search\n```\n{\"q\":\"test\"}\n```",
		},
		{
			name: "tool result resolves name from preceding tool call",
			msgs: []llm.Message{
				{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x.go"}`}},
					},
				},
				{Role: llm.RoleTool, ToolCallID: "call_1", Content: "package x"},
			},
			want: "# Assistant\n\n\n**Tool Call:** read_file\n```\n{\"path\":\"x.go\"}\n```\n\n---\n\n**Tool Result** (read_file):\n```\npackage x\n```",
		},
		{
			name:          "only system messages with include returns content",
			includeSystem: true,
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "system only"},
			},
			want: "> **system:** system only",
		},
		{
			name: "only system messages without include returns empty",
			msgs: []llm.Message{
				{Role: llm.RoleSystem, Content: "system only"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHistoryMarkdown(tt.msgs, tt.includeSystem)
			if got != tt.want {
				t.Errorf("formatHistoryMarkdown:\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// --- stub LLM service ---

type stubService struct {
	response string
	err      error
}

func (s *stubService) CreateCompletion(
	_ context.Context, _ llm.Request,
) (iterator.Iterator[llm.Event], error) {
	if s.err != nil {
		return nil, s.err
	}
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: s.response,
	}
	events := []llm.Event{
		{Type: llm.EventTextDelta, Text: s.response},
		{Type: llm.EventStreamDone, DoneData: &llm.DoneData{Message: msg, FinishReason: "stop"}},
	}
	return iterator.FromSlice(events), nil
}

func (s *stubService) CountTokens([]llm.Message) (int, error) {
	return 0, nil
}

func (s *stubService) ContextWindow() int {
	return math.MaxInt
}

func TestExportAuditJSONLContent(t *testing.T) {
	auditBackend := storagestub.NewInMemoryService()
	auditStore := llm.NewAuditStore(auditBackend)
	ctx := context.Background()

	auditStore.Append(ctx, "d1", llm.AuditEntry{
		EstimatedTokens: 100,
		ContextWindow:   4096,
		Tools:           2,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
		Response:     &llm.Message{Role: llm.RoleAssistant, Content: "hi there"},
		Usage:        llm.Usage{TokensSent: 50, TokensReceived: 10},
		FinishReason: llm.FinishReasonStop,
	})
	auditStore.Append(ctx, "d1", llm.AuditEntry{
		EstimatedTokens: 200,
		ContextWindow:   4096,
		Tools:           3,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "hi there"},
			{Role: llm.RoleUser, Content: "how are you"},
		},
		Response:     &llm.Message{Role: llm.RoleAssistant, Content: "I'm fine"},
		Usage:        llm.Usage{TokensSent: 120, TokensReceived: 20, TokensReasoned: 5},
		FinishReason: llm.FinishReasonStop,
	})

	deps := newTestDeps()
	deps.opts = append(deps.opts, WithAuditStore(auditStore))
	sh := New(
		deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
		deps.store, deps.registry, deps.agentsConfig, deps.cfg,
		deps.skillRegistry, deps.workspaceRoot, deps.fs,
		deps.storage, nil, nil, nil,
		deps.notifications,
		deps.dataPath,
		deps.localRegistry,
		deps.opts...,
	)

	it, err := sh.HandleCommand(ctx, repl.Command{
		Name: "chats",
		Args: []string{"export", "--audit", "d1"},
	}, repl.NopProgressWriter())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := renderOutput(t, ctx, it)

	// Extract the file path from the output.
	const prefix = "Exported 2 audit entries to "
	if !strings.Contains(text, prefix) {
		t.Fatalf("unexpected output: %q", text)
	}
	// The path is between backticks in the markdown output, but rendered
	// as plain text. Find the temp file path (starts with /tmp or similar).
	idx := strings.Index(text, "/")
	if idx < 0 {
		t.Fatalf("no file path found in output: %q", text)
	}
	path := strings.TrimSpace(text[idx:])

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}

	// Decode first entry.
	var entry1 auditExportEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry1); err != nil {
		t.Fatalf("decode line 1: %v", err)
	}
	if entry1.DialogueID != "d1" {
		t.Errorf("entry1.DialogueID = %q, want %q", entry1.DialogueID, "d1")
	}
	if entry1.Turn != 1 {
		t.Errorf("entry1.Turn = %d, want 1", entry1.Turn)
	}
	if entry1.EstimatedTokens != 100 {
		t.Errorf("entry1.EstimatedTokens = %d, want 100", entry1.EstimatedTokens)
	}
	if entry1.Usage.TokensSent != 50 {
		t.Errorf("entry1.Usage.TokensSent = %d, want 50", entry1.Usage.TokensSent)
	}
	if len(entry1.Messages) != 1 {
		t.Errorf("entry1.Messages len = %d, want 1", len(entry1.Messages))
	}
	if entry1.Response == nil || entry1.Response.Content != "hi there" {
		t.Errorf("entry1.Response.Content = %v, want %q", entry1.Response, "hi there")
	}

	// Decode second entry.
	var entry2 auditExportEntry
	if err := json.Unmarshal([]byte(lines[1]), &entry2); err != nil {
		t.Fatalf("decode line 2: %v", err)
	}
	if entry2.Turn != 2 {
		t.Errorf("entry2.Turn = %d, want 2", entry2.Turn)
	}
	if entry2.EstimatedTokens != 200 {
		t.Errorf("entry2.EstimatedTokens = %d, want 200", entry2.EstimatedTokens)
	}
	if entry2.Usage.TokensReasoned != 5 {
		t.Errorf("entry2.Usage.TokensReasoned = %d, want 5", entry2.Usage.TokensReasoned)
	}
	if len(entry2.Messages) != 3 {
		t.Errorf("entry2.Messages len = %d, want 3", len(entry2.Messages))
	}
}

func TestExportConversationContent(t *testing.T) {
	ctx := context.Background()

	deps := newTestDeps()
	_ = deps.store.Create(ctx, dialoguemanager.Dialogue{
		ID: "d1",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "you are helpful"},
			{Role: llm.RoleUser, Content: "hello"},
			{Role: llm.RoleAssistant, Content: "hi there"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
				{ID: "tc1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x.go"}`}},
			}},
			{Role: llm.RoleTool, ToolCallID: "tc1", Content: "package x"},
			{Role: llm.RoleAssistant, Content: "here is the file"},
			{Role: llm.RoleUser, Content: "thanks"},
			{Role: llm.RoleAssistant, Content: "you're welcome"},
		},
	})

	sh := New(
		deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
		deps.store, deps.registry, deps.agentsConfig, deps.cfg,
		deps.skillRegistry, deps.workspaceRoot, deps.fs,
		deps.storage, nil, nil, nil,
		deps.notifications,
		deps.dataPath,
		deps.localRegistry,
		deps.opts...,
	)

	it, err := sh.HandleCommand(ctx, repl.Command{
		Name: "chats",
		Args: []string{"export", "d1"},
	}, repl.NopProgressWriter())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := renderOutput(t, ctx, it)

	idx := strings.Index(text, "/")
	if idx < 0 {
		t.Fatalf("no file path found in output: %q", text)
	}
	path := strings.TrimSpace(text[idx:])

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	got := string(data)

	// Should contain user and assistant text messages only.
	if !strings.Contains(got, "## User\n\nhello\n") {
		t.Errorf("expected first user message, got:\n%s", got)
	}
	if !strings.Contains(got, "## Assistant\n\nhi there\n") {
		t.Errorf("expected first assistant message, got:\n%s", got)
	}
	if !strings.Contains(got, "## Assistant\n\nhere is the file\n") {
		t.Errorf("expected assistant text after tool call, got:\n%s", got)
	}
	if !strings.Contains(got, "## User\n\nthanks\n") {
		t.Errorf("expected second user message, got:\n%s", got)
	}
	if !strings.Contains(got, "## Assistant\n\nyou're welcome\n") {
		t.Errorf("expected final assistant message, got:\n%s", got)
	}

	// Should NOT contain system, tool, or tool call content.
	if strings.Contains(got, "you are helpful") {
		t.Error("system message should not be exported")
	}
	if strings.Contains(got, "read_file") {
		t.Error("tool call should not be exported")
	}
	if strings.Contains(got, "package x") {
		t.Error("tool result should not be exported")
	}
}

func TestForkDialogueStore(t *testing.T) {
	t.Run("fork includes trailing tool results", func(t *testing.T) {
		ctx := context.Background()
		deps := newTestDeps()
		_ = deps.store.Create(ctx, dialoguemanager.Dialogue{
			ID:           "conv-1",
			AgentID:      "default",
			Model:        "gpt-4",
			WorkspaceURI: "file:///workspace",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "You are helpful"},
				{Role: llm.RoleUser, Content: "hello"},
				{Role: llm.RoleAssistant, Content: "hi there"},
				{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{
					{ID: "tc1", Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x.go"}`}},
				}},
				{Role: llm.RoleTool, ToolCallID: "tc1", Content: "package x"},
				{Role: llm.RoleUser, Content: "thanks"},
			},
		})

		sh := New(
			deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
			deps.store, deps.registry, deps.agentsConfig, deps.cfg,
			deps.skillRegistry, deps.workspaceRoot, deps.fs,
			deps.storage, nil, nil, nil,
			deps.notifications,
			deps.dataPath,
			deps.localRegistry,
			deps.opts...,
		).(*shell)
		d, err := deps.store.Get(ctx, "conv-1")
		if err != nil {
			t.Fatalf("get dialogue: %v", err)
		}

		// Fork at the assistant message with tool calls (index 3).
		// The trailing tool result (index 4) should be included.
		forkedID, err := sh.forkDialogue(ctx, d, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if forkedID != "conv-1-fork" {
			t.Fatalf("expected generated fork ID, got %q", forkedID)
		}

		forked, err := deps.store.Get(ctx, forkedID)
		if err != nil {
			t.Fatalf("get forked dialogue: %v", err)
		}
		// Messages: system, user, assistant, assistant+tool_calls, tool_result = 5
		if len(forked.Messages) != 5 {
			t.Fatalf("expected 5 messages in forked dialogue, got %d", len(forked.Messages))
		}
		if forked.AgentID != "default" {
			t.Errorf("forked AgentID = %q, want %q", forked.AgentID, "default")
		}
		if forked.Model != "gpt-4" {
			t.Errorf("forked Model = %q, want %q", forked.Model, "gpt-4")
		}
		if forked.WorkspaceURI != "file:///workspace" {
			t.Errorf("forked WorkspaceURI = %q, want %q", forked.WorkspaceURI, "file:///workspace")
		}
	})

	t.Run("fork at user message without trailing tool results", func(t *testing.T) {
		ctx := context.Background()
		deps := newTestDeps()
		_ = deps.store.Create(ctx, dialoguemanager.Dialogue{
			ID:           "conv-2",
			AgentID:      "default",
			Model:        "gpt-4",
			WorkspaceURI: "file:///workspace-two",
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "You are helpful"},
				{Role: llm.RoleUser, Content: "hello"},
				{Role: llm.RoleAssistant, Content: "hi there"},
			},
		})

		sh := New(
			deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
			deps.store, deps.registry, deps.agentsConfig, deps.cfg,
			deps.skillRegistry, deps.workspaceRoot, deps.fs,
			deps.storage, nil, nil, nil,
			deps.notifications,
			deps.dataPath,
			deps.localRegistry,
			deps.opts...,
		).(*shell)
		d, err := deps.store.Get(ctx, "conv-2")
		if err != nil {
			t.Fatalf("get dialogue: %v", err)
		}

		forkedID, err := sh.forkDialogue(ctx, d, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		forked, err := deps.store.Get(ctx, forkedID)
		if err != nil {
			t.Fatalf("get forked dialogue: %v", err)
		}
		// Messages: system, user = 2
		if len(forked.Messages) != 2 {
			t.Fatalf("expected 2 messages in forked dialogue, got %d", len(forked.Messages))
		}
		if forked.WorkspaceURI != "file:///workspace-two" {
			t.Errorf("forked WorkspaceURI = %q, want %q", forked.WorkspaceURI, "file:///workspace-two")
		}
	})

	t.Run("fork IDs use numbered suffixes", func(t *testing.T) {
		ctx := context.Background()
		deps := newTestDeps()
		_ = deps.store.Create(ctx, dialoguemanager.Dialogue{
			ID:      "conv-3",
			AgentID: "default",
			Model:   "gpt-4",
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "hello"},
			},
		})
		deps.store.set(dialoguemanager.Dialogue{ID: "conv-3-fork"})
		deps.store.set(dialoguemanager.Dialogue{ID: "conv-3-fork-2"})

		sh := New(
			deps.wm, deps.svc, deps.modelRegistry, deps.defaultModel,
			deps.store, deps.registry, deps.agentsConfig, deps.cfg,
			deps.skillRegistry, deps.workspaceRoot, deps.fs,
			deps.storage, nil, nil, nil,
			deps.notifications,
			deps.dataPath,
			deps.localRegistry,
			deps.opts...,
		).(*shell)
		d, err := deps.store.Get(ctx, "conv-3")
		if err != nil {
			t.Fatalf("get dialogue: %v", err)
		}

		forkedID, err := sh.forkDialogue(ctx, d, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if forkedID != "conv-3-fork-3" {
			t.Fatalf("forkedID = %q, want %q", forkedID, "conv-3-fork-3")
		}
	})
}

func TestForkCandidatesUsesIcons(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "ignore me"},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "hi there"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x.go"}`},
		}}},
	}

	got := forkCandidates(messages)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got))
	}

	if got[0].label != " hello" {
		t.Fatalf("user candidate label = %q, want %q", got[0].label, " hello")
	}
	if got[1].label != "󰚩 hi there" {
		t.Fatalf("assistant candidate label = %q, want %q", got[1].label, "󰚩 hi there")
	}
}

func TestNewForkPickerHandlerUsesResponsiveStrings(t *testing.T) {
	candidates := []forkCandidate{{label: " hello", preview: "hello"}}
	h := newForkPickerHandler(candidates, nil, nil)
	node, ok := h.list.Front()
	if !ok {
		t.Fatal("expected picker row")
	}
	if _, ok := node.Value().(*component.ResponsiveString); !ok {
		t.Fatalf("picker row type = %T, want *component.ResponsiveString", node.Value())
	}
}

func TestForkPreviewMarkdown(t *testing.T) {
	t.Run("preserves full markdown content", func(t *testing.T) {
		msg := llm.Message{Role: llm.RoleAssistant, Content: "# Title\n\nSome **bold** text"}
		got := forkPreviewMarkdown(msg)
		want := "# Title\n\nSome **bold** text"
		if got != want {
			t.Fatalf("forkPreviewMarkdown() = %q, want %q", got, want)
		}
	})

	t.Run("falls back to tool call markdown when content is empty", func(t *testing.T) {
		msg := llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			Function: llm.FunctionCall{Name: "read_file", Arguments: `{"path":"x.go"}`},
		}}}
		got := forkPreviewMarkdown(msg)
		if !strings.Contains(got, "**Tool Call:** read_file") {
			t.Fatalf("expected tool call markdown, got %q", got)
		}
		if !strings.Contains(got, `{"path":"x.go"}`) {
			t.Fatalf("expected tool call args in markdown, got %q", got)
		}
	})
}

func TestForkPickerDimensions(t *testing.T) {
	t.Run("uses preview height and list count", func(t *testing.T) {
		width, height := forkPickerDimensions(3, 20, 30)
		if width != forkMinWidth {
			t.Fatalf("width = %d, want %d", width, forkMinWidth)
		}
		wantHeight := forkPreviewHeight + forkSeparatorHeight + 3
		if height != wantHeight {
			t.Fatalf("height = %d, want %d", height, wantHeight)
		}
	})

	t.Run("caps list height and keeps empty list visible", func(t *testing.T) {
		width, height := forkPickerDimensions(99, 120, 40)
		if width != 120 {
			t.Fatalf("width = %d, want %d", width, 120)
		}
		wantHeight := forkPreviewHeight + forkSeparatorHeight + forkMaxListHeight
		if height != wantHeight {
			t.Fatalf("height = %d, want %d", height, wantHeight)
		}

		_, emptyHeight := forkPickerDimensions(0, 10, 10)
		wantEmptyHeight := forkPreviewHeight + forkSeparatorHeight + 1
		if emptyHeight != wantEmptyHeight {
			t.Fatalf("empty height = %d, want %d", emptyHeight, wantEmptyHeight)
		}
	})
}
