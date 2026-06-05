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

package ideshell

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/command"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/term/sh"
)

// Config configures optional dependencies of the IDE shell handler.
type Config struct {
	// Storage is used by both the inner repl history persistence and
	// by the reverse-history search overlay. May be nil in tests.
	Storage storageapi.Service
	// HistoryDocumentID is the storage key under which the shell
	// history is persisted. Required if Storage is set.
	HistoryDocumentID string
	// MaxHistory caps the number of persisted history entries.
	MaxHistory int
	// Prompt is the inputbox prompt prefix shown both in the
	// regular shell view and on the search overlay's input bar.
	// Defaults to "> ".
	Prompt string
	// EditModeKey toggles a modal text-editor mode where the shell
	// input can be freely edited without triggering completion or
	// history-search overlays. Pressing it again, <enter>, <tab>,
	// or <ctrl-c> exits the mode and the edited buffer replaces
	// the inner inputbox text. Zero value disables modal edit.
	EditModeKey term.KeyComb
	// Editor drives the shell's modal edit mode (see EditModeKey).
	// Required when EditModeKey is non-zero.
	Editor command.Editor
}

// New creates an IDE shell Handler wired with a CommandRegistry, sh
// layer, and the built-in help command. The returned Handler wraps an
// SDK repl.Handler and adds an interactive reverse-history search
// overlay. The returned registry can be used to register additional
// commands.
func New(
	scheduleNextTick func(func()) bool,
	interrupter term.Interrupter,
	cfg Config,
	opts ...repl.Option,
) (*Handler, *CommandRegistry) {
	r := NewRegistry()
	registerBaseCommands(r)
	prompt := cfg.Prompt
	if prompt == "" {
		prompt = defaultPrompt
	}
	if cfg.Storage != nil && cfg.HistoryDocumentID != "" {
		opts = append(opts,
			repl.WithStorage(cfg.HistoryDocumentID, cfg.Storage),
		)
	}
	if cfg.MaxHistory > 0 {
		opts = append(opts, repl.WithMaxHistory(cfg.MaxHistory))
	}
	opts = append(opts, repl.WithPrompt(prompt))
	shim := &completionShim{underlying: sh.New(r)}
	inner := repl.New(shim, scheduleNextTick, interrupter, opts...)
	list := search.NewList(search.ListConfig{
		Algo:            search.FuzzyMatch,
		Interrupter:     interrupter,
		SyncSearch:      true,
		BottomSearchBar: true,
	})
	h := &Handler{
		inner:      inner,
		storage:    cfg.Storage,
		historyKey: cfg.HistoryDocumentID,
		maxHistory: cfg.MaxHistory,
		list:       list,
		shim:       shim,
		prompt:     prompt,
	}
	h.mouseDelegate = newMouseDelegate(&h.grid, func(ev term.Event) {
		_, _ = h.inner.Handle(ev)
	})
	h.mouse = mouse.New(h.mouseDelegate)
	if cfg.EditModeKey != (term.KeyComb{}) {
		if cfg.Editor == nil {
			panic("ideshell.Config.Editor is required when EditModeKey is set")
		}
		h.editSession = command.NewEditSession(cfg.Editor, cfg.EditModeKey)
		h.editKey = cfg.EditModeKey
	}
	return h, r
}

const defaultPrompt = "> "

func registerBaseCommands(r *CommandRegistry) {
	r.Register("help", "Show available commands", &helpHandler{r: r})
}

type helpHandler struct {
	r *CommandRegistry
}

func (h *helpHandler) HandleCommand(
	ctx context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	return h.r.Help(ctx, cmd.Args)
}

func (h *helpHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	return h.r.Complete(ctx, args[len(args)-1], nil)
}

func (h *helpHandler) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return toLines(
		"Show available commands or help for a specific command",
	), nil
}

func toLines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = toResponsive(s)
	}
	return iterator.FromSlice(out)
}

func toResponsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}
