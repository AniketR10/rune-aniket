// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/ide/idelsp/lspcmd"
)

// zigActionCmdName is the command-prompt command that exposes zls's code
// actions. It shares the `zig` word with the toolchain REPL command, but
// the two live in separate registries: this one is dispatched from the
// command prompt, the REPL one from the console.
const zigActionCmdName = "zig"

// zls answers no code action at all while a file fails to parse, and
// never answers for .zon files, so every hint names the syntax-error
// case rather than claiming the file is already clean.
const zigSyntaxHint = "zls reports none while the file has a syntax error, " +
	"and never for .zon files"

// newZigActionHandler builds the `zig` command-prompt handler that
// surfaces zls's code actions as subcommands. The selection tracker is
// subscribed by the caller and shared so the handler sees the live
// selection range.
func newZigActionHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, notify browserapi.Notifications,
	sel *lspcmd.SelectionTracker,
) (textapi.CommandManual, textapi.CommandHandler) {
	handlers := map[string]textapi.CommandHandler{
		"list": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "", false,
			"No code actions available. "+zigSyntaxHint),

		// zls builds its quickfixes from the whole ast-check error
		// bundle, ignoring the requested range, so these are file-wide
		// rather than cursor-scoped.
		"quickfix": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "quickfix", false,
			"No quick fixes available. "+zigSyntaxHint),

		// The only zls refactoring converts a string literal to or from
		// its multiline form, and it is the one action that honors the
		// requested range.
		"refactor": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "refactor", false,
			"No refactoring at the cursor. Place the cursor inside a string "+
				"literal to convert it to or from a multiline literal"),

		"organize-imports": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "source.organizeImports", true,
			"The @import declarations are already organized. "+zigSyntaxHint),
		"fix-all": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "source.fixAll", true,
			"No automatic fixes available. "+zigSyntaxHint),
	}

	manual := textapi.CommandManual{
		Name:     zigActionCmdName,
		Summary:  "Zig code actions powered by zls",
		Synopsis: "<subcommand> [<args>...]",
		Commands: []textapi.CommandManual{
			{Name: "list", Summary: "List every code action zls offers for the current file and apply the chosen one"},
			{Name: "quickfix", Summary: "Apply a quick fix for a compile error in the current file"},
			{Name: "refactor", Summary: "Convert the string literal at the cursor to or from its multiline form"},
			{Name: "organize-imports", Summary: "Sort and deduplicate the @import declarations of the current file"},
			{Name: "fix-all", Summary: "Apply every quick fix zls offers for the current file at once"},
		},
	}

	return manual, &zigActionRouter{handlers: handlers, editor: editor}
}

var _ textapi.CommandHandler = (*zigActionRouter)(nil)

type zigActionRouter struct {
	handlers map[string]textapi.CommandHandler
	editor   textapi.Editor
}

func (r *zigActionRouter) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Name != zigActionCmdName {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	if len(cmd.Args) == 0 {
		return fmt.Errorf("missing subcommand")
	}
	cmd.Name = cmd.Args[0]
	cmd.Args = cmd.Args[1:]
	h, ok := r.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown zig subcommand: %s", cmd.Name)
	}
	r.refreshCursor(&cmd)
	return h.HandleCommand(ctx, cmd)
}

// refreshCursor replaces a zero cursor snapshot with the live editor
// cursor. Dispatch paths that cannot capture a cursor (a non-text
// window in focus, a stale tab handler) deliver {0,0} alongside a valid
// resource; trusting it would run the cursor-scoped subcommands against
// the top of the file and apply the resulting edits there. When the
// cursor genuinely sits at 0,0 the live lookup returns the same
// position, so the refresh is a no-op.
func (r *zigActionRouter) refreshCursor(cmd *textapi.Command) {
	if cmd.Resource == nil || cmd.Cursor.Content != (term.Coordinates{}) {
		return
	}
	if live, err := r.editor.Cursor(cmd.Resource); err == nil {
		cmd.Cursor.Content = live
	}
}

func (r *zigActionRouter) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != zigActionCmdName {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	cmd = args[0]
	args = args[1:]
	if h, ok := r.handlers[cmd]; ok {
		return h.Complete(ctx, cmd, args)
	}
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return iterator.FromSlice(names), nil
}
