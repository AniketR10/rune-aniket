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
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
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
	// Workspace seeds the shell interpreter's working directory.
	// Without it the interpreter inherits the process working
	// directory, which under a macOS .app launch is the bundle.
	Workspace workspaceapi.URI
	// Modal reports whether the editor backing the input line is
	// modal (vi). Only when true does ModalStartInsert take effect; a
	// modeless editor has no normal mode to switch out of.
	Modal bool
	// ModalStartInsert opens the input line in insert mode at
	// construction when Modal is true, sparing the user from pressing
	// `i` before typing into a fresh shell prompt.
	ModalStartInsert bool
	// DisableShellInterpreter, when non-nil, runs the shell as a pure
	// command registry with no mvdan/sh parsing and no PATH executable
	// fallback. Lines whose first token does not match a registered
	// command are dispatched to this handler instead of surfacing
	// repl.ErrNotFound. Use it for REPL surfaces (e.g. a language REPL)
	// where every input line is a fragment of the hosted language
	// rather than a discrete command — the handler receives the whole
	// line reconstructed from cmd.Name and cmd.Args.
	DisableShellInterpreter repl.CommandHandler

	// ClearHook, when set, runs after the shell clears its screen
	// (<c-l>). A host that keeps state mirrored on the screen (e.g. a
	// REPL accumulating a program) uses it to reset that state so it
	// stays in sync with the now-blank view.
	ClearHook func()
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
