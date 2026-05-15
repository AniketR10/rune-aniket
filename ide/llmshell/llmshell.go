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


// Package llmshell exposes the `models` REPL command tree on the rune
// (IDE) side. The shell wraps an llmrouter.Router and surfaces two
// sub-commands:
//
//   - models providers — inspect provider authentication (codex login/status today)
//   - models local — manage the local llama.cpp model cache
//
// The agent loop continues to expose its own `agent` shell from
// rune-agent until migration; this shell is intentionally narrower.
package llmshell

import (
	"context"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/llm/llamacpp"
	"unstable.build/go-tui/llm/llmrouter"
)

// CommandName is the top-level REPL command exposed by this shell.
const CommandName = "models"

var commandManual = textapi.CommandManual{
	Name:     CommandName,
	Summary:  "Inspect and manage LLM providers and local models.",
	Synopsis: "<command> [args]",
	Commands: []textapi.CommandManual{
		{
			Name:     "providers",
			Summary:  "Inspect and manage provider authentication.",
			Synopsis: "<codex> <login|status>",
			Commands: []textapi.CommandManual{
				{
					Name:     "codex",
					Summary:  "Manage Codex provider authentication.",
					Synopsis: "<login|status>",
					Commands: []textapi.CommandManual{
						{Name: "login", Summary: "Authenticate with Codex."},
						{Name: "status", Summary: "Show Codex authentication status."},
					},
				},
			},
		},
		{
			Name:     "local",
			Summary:  "Manage locally cached GGUF models.",
			Synopsis: "<list|download|delete> [args]",
			Commands: []textapi.CommandManual{
				{Name: "list", Summary: "List GGUF models in the local cache."},
				{
					Name:     "download",
					Summary:  "Download a GGUF model from an OCI registry.",
					Synopsis: "<host/>owner/repo[:tag|@digest]",
				},
				{Name: "delete", Summary: "Delete a locally cached GGUF model.", Synopsis: "<reference>"},
			},
		},
	},
}

// Manual returns the parent REPL command manual.
func Manual() textapi.CommandManual { return commandManual }

// Config configures a Handler. All fields are mandatory: the router
// is the dispatcher used by the providers subtree to advertise the
// installed clients, the local registry backs the `local` subtree,
// and storage backs codex auth state.
type Config struct {
	// Router is the LLM router from which available models are read.
	Router *llmrouter.Router
	// LocalRegistry is the llama.cpp cache registry that the local
	// subcommand operates on.
	LocalRegistry *llamacpp.Registry
	// Storage is the persistent storage service used by the codex
	// provider for auth state.
	Storage storageapi.Service
}

// Handler is the parent dispatcher for the `models` command tree.
type Handler struct {
	router        *llmrouter.Router
	localRegistry *llamacpp.Registry
	storage       storageapi.Service

	providers *providersHandler
	local     *localHandler
}

// New returns a Handler configured with cfg. It panics if any
// dependency is nil — the rune-side wiring constructs every collaborator
// at workspace boot, so a missing one indicates a programming error.
func New(cfg Config) *Handler {
	if cfg.Router == nil {
		panic("llmshell: Config.Router must not be nil")
	}
	if cfg.LocalRegistry == nil {
		panic("llmshell: Config.LocalRegistry must not be nil")
	}
	if cfg.Storage == nil {
		panic("llmshell: Config.Storage must not be nil")
	}
	return &Handler{
		router:        cfg.Router,
		localRegistry: cfg.LocalRegistry,
		storage:       cfg.Storage,
		providers:     newProvidersHandler(cfg.Storage),
		local:         newLocalHandler(cfg.LocalRegistry),
	}
}

// HandleCommand satisfies repl.CommandHandler. The parent shell splits
// the first arg and routes to the matching sub-handler.
func (h *Handler) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	if len(cmd.Args) == 0 {
		return markdownOutput(usageMarkdown(commandManual)), nil
	}
	sub := cmd.Args[0]
	rest := repl.Command{Name: cmd.Name + " " + sub, Args: cmd.Args[1:]}
	switch sub {
	case "providers":
		return h.providers.HandleCommand(ctx, rest, pw)
	case "local":
		return h.local.HandleCommand(ctx, rest, pw)
	case "help":
		return markdownOutput(usageMarkdown(commandManual)), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", sub)
	}
}

// Complete satisfies repl.CommandHandler.
func (h *Handler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) <= 1 {
		filter := ""
		if len(args) == 1 {
			filter = args[0]
		}
		return iterator.FromSlice(filterNames([]string{"providers", "local", "help"}, filter)), nil
	}
	switch args[0] {
	case "providers":
		return h.providers.Complete(ctx, cmd, args[1:])
	case "local":
		return h.local.Complete(ctx, cmd, args[1:])
	}
	return iterator.FromSlice[string](nil), nil
}

// Help satisfies textapi.REPLHandler. The shell renders the manual
// for the `models` tree (or a subcommand when args points at one).
func (h *Handler) Help(
	_ context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	man := commandManual
	for _, a := range args {
		sub, ok := findSubcommand(man, a)
		if !ok {
			break
		}
		man = sub
	}
	return markdownOutput(usageMarkdown(man)), nil
}

// findSubcommand looks up a child of man whose Name matches name.
func findSubcommand(man textapi.CommandManual, name string) (textapi.CommandManual, bool) {
	for _, c := range man.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return textapi.CommandManual{}, false
}

// markdownOutput wraps a markdown string into a single-shot iterator
// suitable for returning from HandleCommand. Falls back to a plain
// responsive string when the markdown parser rejects the content.
func markdownOutput(content string) iterator.Iterator[component.Responsive] {
	md, err := markdown.New(content)
	if err != nil {
		r := component.NewResponsiveString(content, component.StringResponsiveConfig{})
		return iterator.FromSlice([]component.Responsive{r})
	}
	return iterator.FromSlice([]component.Responsive{md})
}

// markdownResponsive returns a single Responsive value wrapping the
// given markdown content.
func markdownResponsive(content string) component.Responsive {
	it := markdownOutput(content)
	v, _ := it.Next(context.Background())
	return v
}

func filterNames(names []string, prefix string) []string {
	if prefix == "" {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

func usageMarkdown(man textapi.CommandManual) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## `%s`\n\n", man.Name)
	if man.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", man.Summary)
	}
	if man.Synopsis != "" {
		fmt.Fprintf(&b, "**Usage**: `%s %s`\n\n", man.Name, man.Synopsis)
	}
	if len(man.Commands) > 0 {
		b.WriteString("### Commands\n\n")
		for _, c := range man.Commands {
			fmt.Fprintf(&b, "- `%s` — %s\n", c.Name, c.Summary)
		}
	}
	return b.String()
}
