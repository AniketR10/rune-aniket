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

package headless

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
)

// session holds everything a headless run needs after bootstrap.
type session struct {
	agent    *agent.Agent
	registry *agent.Registry
	model    llmapi.ModelEntry
	cwd      workspaceapi.URI
	// cleanup removes the run-scoped dialogue directory.
	cleanup func()
}

// bootstrap builds the agent graph from host capabilities.
//
// Everything the interactive extension installs for the sake of a user
// interface — terminal, notifications, editor, window manager,
// interrupter, command registration, MCP, hooks, skills, web_fetch and
// subagents — is deliberately absent so a headless run has no side
// effects on the live session and behaves deterministically.
func bootstrap(
	ctx context.Context, w *extensionapi.Workspace, opts Options, runID string,
) (*session, error) {
	fs := w.FileSystem(ctx)
	executor := w.Executor(ctx)
	lsp := w.LSP(ctx)
	parser := w.Parser(ctx)
	storage := w.Storage(ctx)
	llmSvc := w.LLM(ctx)
	dataDir := w.DataDir(ctx)

	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	cwd, err := fs.URI(wd)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory %q: %w", wd, err)
	}

	model, err := llmarg.Resolve(ctx, llmSvc, opts.Model)
	if err != nil {
		return nil, fmt.Errorf("resolve model %q: %w", opts.Model, err)
	}

	// The host strips the `extensions` key before serving its config
	// over RPC, so the extension's own config block is unreachable
	// here; a nil snapshot keeps apply_patch able to edit
	// .rune/config.yaml while resolving reads through the overlay only.
	cfg := configedit.NewConfig(fs, cwd, nil)
	tools, tracker := agentools.DefaultTools(
		fs, executor, cwd, lsp, agentools.Config{}, cfg)
	tools = append(tools, agentools.LSPTools(lsp, fs, parser, cwd, tracker)...)
	tools = append(tools, agentools.SyntaxTools(parser, fs, cwd, tracker)...)
	registry := agent.NewRegistry(tools...)

	systemPrompt := agent.DefaultSystemPrompt(cwd)
	agentsFiles := agent.DiscoverAgentsFiles(fs, cwd.Path(), agent.DefaultAgentsFile)

	sessionsDir := filepath.Join(dataDir, "headless", runID)
	store := dialoguemanager.NewStore(storage, sessionsDir)

	ag := agent.NewAgent(llmSvc, registry,
		skills.NewRegistry(fs, cwd, nil, nil), store, agent.NoMemory(),
		agent.Config{
			SystemPrompt:        systemPrompt,
			ProjectInstructions: agent.LoadAgentsFiles(fs, agentsFiles),
			Model:               model,
			Workspace:           cwd,
			AgentID:             ExtensionID,
			SessionKey:          runID,
		})
	ag.SetEffort(llmapi.ReasoningEffort(opts.Effort))

	return &session{
		agent:    ag,
		registry: registry,
		model:    model,
		cwd:      cwd,
		cleanup: func() {
			if err := os.RemoveAll(sessionsDir); err != nil {
				slog.Warn("remove headless session directory",
					"dir", sessionsDir, "error", err)
			}
		},
	}, nil
}
