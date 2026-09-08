// Copyright (C) 2017-2026 Unstable Build, LLC
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
	"maps"
	"sort"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/ide/idelsp/lspcmd"
)

// actionCmdName is the command-prompt command that exposes
// rust-analyzer's assists (code actions). It shares the `rust` word with
// the rustup REPL command, but the two live in separate registries: this
// one is dispatched from the command prompt, the REPL one from the
// console.
const actionCmdName = "rust"

// newRustActionHandler builds the `rust` command-prompt handler that
// surfaces rust-analyzer's assists as subcommands. Each subcommand
// requests code actions of a broad kind (or all kinds for `list`) at the
// cursor or selection and applies the chosen one. The selection tracker
// is subscribed by the caller and shared so the handler sees the live
// selection range.
func newRustActionHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, notify browserapi.Notifications,
	opener browserapi.ResourceOpener, sel *lspcmd.SelectionTracker,
	exec workspaceapi.Executor, fs workspaceapi.FileSystem,
	parser syntaxapi.Parser, interrupt term.Interrupter,
	cwd string, experimental, memoryUsage bool,
) (textapi.CommandManual, textapi.CommandHandler) {
	edit := editDeps{lsp: lsp, editor: editor, opener: opener, notify: notify, sel: sel}
	picks := pickDeps{
		editor: editor, wm: wm, opener: opener, notify: notify,
		fs: fs, parser: parser, interrupt: interrupt,
	}
	handlers := map[string]textapi.CommandHandler{
		// Browse every assist applicable at the cursor or selection.
		"list": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "", false,
			"No assists available at the cursor. Place the cursor on a symbol or select an expression"),

		// Broad assist families. rust-analyzer tags each assist with one
		// of these kinds; the picker then narrows to the specific assist.
		"extract": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "refactor.extract", false,
			"No extract assist here. Select an expression, statements, a module, or a type to extract"),
		"inline": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "refactor.inline", false,
			"No inline assist here. Place the cursor on a call, local, type alias, or macro to inline"),
		"rewrite": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "refactor.rewrite", false,
			"No rewrite assist here. Place the cursor on a construct rust-analyzer can rewrite in place"),
		"refactor": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "refactor", false,
			"No refactoring available at the cursor"),
		"quickfix": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "quickfix", false,
			"No quick fix available. Place the cursor on a diagnostic to see its fixes"),

		// Whole-file source actions.
		"organize-imports": lspcmd.CodeActionHandler(lsp, editor, notify, wm, sel, "source.organizeImports", true,
			"Imports are already organized"),

		// View/status commands render server output in a floating,
		// scrollable read-only viewer.
		"status":      stringViewCmd(lsp, wm, notify, parser, interrupt, analyzerStatusView(lsp), "rust-analyzer reported no status"),
		"syntax-tree": stringViewCmd(lsp, wm, notify, parser, interrupt, docTextView(lsp, "rust-analyzer/viewSyntaxTree"), "No syntax tree for this file"),
		// HIR and MIR are rendered by rust-analyzer as Rust-like source,
		// so they highlight as Rust.
		"hir":          stringViewCmd(lsp, wm, notify, parser, interrupt, posTextView(lsp, "rust-analyzer/viewHir"), "No HIR at the cursor").withLang("rust"),
		"mir":          stringViewCmd(lsp, wm, notify, parser, interrupt, posTextView(lsp, "rust-analyzer/viewMir"), "No MIR at the cursor").withLang("rust"),
		"interpret":    stringViewCmd(lsp, wm, notify, parser, interrupt, posTextView(lsp, "rust-analyzer/interpretFunction"), "Place the cursor on a const-evaluable function to interpret"),
		"file-text":    stringViewCmd(lsp, wm, notify, parser, interrupt, fileTextView(lsp), "No file text available").withLang("rust"),
		"item-tree":    stringViewCmd(lsp, wm, notify, parser, interrupt, docTextView(lsp, "rust-analyzer/viewItemTree"), "No item tree for this file"),
		"expand-macro": stringViewCmd(lsp, wm, notify, parser, interrupt, expandMacroView(lsp), "Place the cursor on a macro invocation to expand it").withLang("rust"),
		"crate-graph": stringViewCmd(lsp, wm, notify, parser, interrupt, noParamsView(lsp, "rust-analyzer/viewCrateGraph", struct {
			Full bool `json:"full"`
		}{Full: false}), "No crate graph available").withLang("dot"),
		"dependencies": &listPickCmd{
			pickDeps: picks, produce: dependenciesPick(lsp, fs),
			emptyMsg: "No dependencies reported",
		},
		"related-tests": &listPickCmd{
			pickDeps: picks, produce: relatedTestsPick(lsp),
			emptyMsg: "No related tests for the symbol at the cursor",
		},
		"recursive-memory-layout": stringViewCmd(lsp, wm, notify, parser, interrupt, recursiveMemoryLayoutView(lsp), "No memory layout for the type at the cursor"),
		"failed-obligations":      stringViewCmd(lsp, wm, notify, parser, interrupt, failedObligationsView(lsp), "No failed trait obligations at the cursor"),
		"diagnostics": &listPickCmd{
			pickDeps: picks, produce: diagnosticsPick(lsp),
			emptyMsg: "No diagnostics for this file",
		},

		// Workspace/flycheck commands.
		"reload-workspace":    &wsRequestCmd{lsp: lsp, notify: notify, method: "rust-analyzer/reloadWorkspace", done: "Reloaded the Cargo workspace"},
		"rebuild-proc-macros": &wsRequestCmd{lsp: lsp, notify: notify, method: "rust-analyzer/rebuildProcMacros", done: "Rebuilt proc macros"},
		"run-flycheck":        &flycheckCmd{lsp: lsp, notify: notify, kind: flycheckRun, done: "Started flycheck"},
		"clear-flycheck":      &flycheckCmd{lsp: lsp, notify: notify, kind: flycheckClear, done: "Cleared flycheck diagnostics"},
		"cancel-flycheck":     &flycheckCmd{lsp: lsp, notify: notify, kind: flycheckCancel, done: "Cancelled flycheck"},
	}

	manual := textapi.CommandManual{
		Name:     actionCmdName,
		Summary:  "Rust assists (code actions) powered by rust-analyzer",
		Synopsis: "<subcommand> [<args>...]",
		Commands: []textapi.CommandManual{
			{Name: "list", Summary: "List every assist applicable at the cursor or selection and apply the chosen one"},
			{Name: "extract", Summary: "Extract the selection into a variable, constant, function, module, or type"},
			{Name: "inline", Summary: "Inline the call, local variable, type alias, or macro at the cursor"},
			{Name: "rewrite", Summary: "Rewrite the construct at the cursor in place (e.g. convert, invert, or reorder)"},
			{Name: "refactor", Summary: "Show every refactoring assist applicable at the cursor or selection"},
			{Name: "quickfix", Summary: "Apply a quick fix for the diagnostic at the cursor"},
			{Name: "organize-imports", Summary: "Merge and sort use declarations for the current file"},
			{Name: "status", Summary: "Show rust-analyzer's analysis status for the current crate"},
			{Name: "syntax-tree", Summary: "Show the syntax tree of the current file"},
			{Name: "hir", Summary: "Show the HIR of the item at the cursor"},
			{Name: "mir", Summary: "Show the MIR of the function at the cursor"},
			{Name: "interpret", Summary: "Const-evaluate the function at the cursor and show the result"},
			{Name: "file-text", Summary: "Show rust-analyzer's view of the current file text"},
			{Name: "item-tree", Summary: "Show the item tree of the current file"},
			{Name: "expand-macro", Summary: "Expand the macro invocation at the cursor"},
			{Name: "crate-graph", Summary: "Show the crate dependency graph (DOT)"},
			{Name: "dependencies", Summary: "List the crates in the workspace dependency graph and open one"},
			{Name: "related-tests", Summary: "List the tests related to the symbol at the cursor and jump to one"},
			{Name: "recursive-memory-layout", Summary: "Show the recursive memory layout of the type at the cursor"},
			{Name: "failed-obligations", Summary: "Show failed trait obligations at the cursor"},
			{Name: "diagnostics", Summary: "List rust-analyzer's diagnostics for the current file and jump to one"},
			{Name: "reload-workspace", Summary: "Reload the Cargo workspace"},
			{Name: "rebuild-proc-macros", Summary: "Rebuild the workspace proc macros"},
			{Name: "run-flycheck", Summary: "Run flycheck (cargo check/clippy) for the current file"},
			{Name: "clear-flycheck", Summary: "Clear flycheck diagnostics"},
			{Name: "cancel-flycheck", Summary: "Cancel a running flycheck"},
		},
	}

	// The experimental/* extensions are opt-in: rust-analyzer answers them
	// unconditionally, but they are less stable than the rust-analyzer/*
	// views, so they register only when the extension's `experimental`
	// config flag is set.
	if experimental {
		runner := &runCmd{lsp: lsp, exec: exec, wm: wm, notify: notify, parser: parser, cwd: cwd}
		experimentalHandlers := map[string]textapi.CommandHandler{
			"parent-module":   &navCmd{lsp: lsp, editor: editor, wm: wm, opener: opener, notify: notify, method: "experimental/parentModule", posArg: true, notFound: "No parent module for this file"},
			"child-modules":   &navCmd{lsp: lsp, editor: editor, wm: wm, opener: opener, notify: notify, method: "experimental/childModules", posArg: true, notFound: "No child modules for this file"},
			"open-cargo-toml": &navCmd{lsp: lsp, editor: editor, wm: wm, opener: opener, notify: notify, method: "experimental/openCargoToml", posArg: false, notFound: "No Cargo.toml found for this file"},
			"external-docs":   &externalDocsCmd{lsp: lsp, notify: notify},

			"join-lines":     &joinLinesCmd{editDeps: edit},
			"matching-brace": &matchingBraceCmd{editDeps: edit},
			"on-enter":       &snippetEditCmd{editDeps: edit, method: "experimental/onEnter", emptyMsg: "Nothing to insert at the cursor"},
			"move-item-up":   &snippetEditCmd{editDeps: edit, method: "experimental/moveItem", direction: "Up", emptyMsg: "Cannot move the item up"},
			"move-item-down": &snippetEditCmd{editDeps: edit, method: "experimental/moveItem", direction: "Down", emptyMsg: "Cannot move the item down"},
			"ssr":            &ssrCmd{editDeps: edit},

			"runnables": &listPickCmd{
				pickDeps: picks, produce: runnablesPick(lsp),
				emptyMsg: "No runnables at the cursor",
			},
			"run":     runner,
			"type":    &hoverRangeCmd{lsp: lsp, wm: wm, notify: notify, parser: parser, sel: sel},
			"symbols": &workspaceSymbolCmd{pickDeps: picks, lsp: lsp},
			"hover":   &hoverCmd{pickDeps: picks, lsp: lsp, runner: runner},
			"eval-predicate": stringViewCmd(lsp, wm, notify, parser, interrupt, evalPredicateView(lsp),
				"rust-analyzer returned no predicate evaluation"),
		}
		maps.Copy(handlers, experimentalHandlers)
		manual.Commands = append(manual.Commands,
			textapi.CommandManual{Name: "parent-module", Summary: "Go to the parent module of the current file"},
			textapi.CommandManual{Name: "child-modules", Summary: "Go to the child module(s) declared at the cursor"},
			textapi.CommandManual{Name: "open-cargo-toml", Summary: "Open the Cargo.toml that owns the current file"},
			textapi.CommandManual{Name: "external-docs", Summary: "Show the documentation URL for the symbol at the cursor"},
			textapi.CommandManual{Name: "join-lines", Summary: "Join the selected lines (or the cursor line) into one"},
			textapi.CommandManual{Name: "matching-brace", Summary: "Move the cursor to the brace matching the one at the cursor"},
			textapi.CommandManual{Name: "on-enter", Summary: "Apply rust-analyzer's smart-enter edit at the cursor"},
			textapi.CommandManual{Name: "move-item-up", Summary: "Move the item at the cursor up"},
			textapi.CommandManual{Name: "move-item-down", Summary: "Move the item at the cursor down"},
			textapi.CommandManual{Name: "ssr", Summary: "Run a structural search and replace query (pattern ==>> replacement)"},
			textapi.CommandManual{Name: "runnables", Summary: "List the runnable cargo targets at the cursor and jump to one"},
			textapi.CommandManual{Name: "run", Summary: "Run the cargo target at the cursor (pick one when several apply)"},
			textapi.CommandManual{Name: "type", Summary: "Show the type of the current selection"},
			textapi.CommandManual{Name: "symbols", Summary: "Search workspace symbols (types-only, workspace or with dependencies)"},
			textapi.CommandManual{Name: "hover", Summary: "Show hover docs as markdown and pick a hover action (run/debug, go to impl/type)"},
			textapi.CommandManual{Name: "eval-predicate", Summary: "Evaluate a trait predicate at the cursor (e.g. rust eval-predicate T: Clone)"},
		)
	}

	// memoryUsage is a tool for developing rust-analyzer itself: the
	// release build we ship rejects rust-analyzer/memoryUsage, which is
	// only answered by a memory-profiling build the user points lsp_path
	// at.
	if memoryUsage {
		handlers["memory-usage"] = stringViewCmd(lsp, wm, notify, parser, interrupt,
			noParamsView(lsp, "rust-analyzer/memoryUsage", nil), "rust-analyzer reported no memory usage")
		manual.Commands = append(manual.Commands, textapi.CommandManual{
			Name:    "memory-usage",
			Summary: "Show rust-analyzer's memory usage (needs a memory-profiling rust-analyzer build)",
		})
	}

	return manual, &rustActionRouter{handlers: handlers, editor: editor}
}

var _ textapi.CommandHandler = (*rustActionRouter)(nil)

type rustActionRouter struct {
	handlers map[string]textapi.CommandHandler
	editor   textapi.Editor
}

func (r *rustActionRouter) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Name != actionCmdName {
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
	if len(cmd.Args) == 0 {
		return fmt.Errorf("missing subcommand")
	}
	cmd.Name = cmd.Args[0]
	cmd.Args = cmd.Args[1:]
	h, ok := r.handlers[cmd.Name]
	if !ok {
		return fmt.Errorf("unknown rust subcommand: %s", cmd.Name)
	}
	r.refreshCursor(&cmd)
	return h.HandleCommand(ctx, cmd)
}

// refreshCursor replaces a zero cursor snapshot with the live editor
// cursor. Dispatch paths that cannot capture a cursor (a non-text
// window in focus, a stale tab handler) deliver {0,0} alongside a
// valid resource; trusting it would run every position-based
// subcommand against the top of the file and apply the resulting
// edits there. When the cursor genuinely sits at 0,0 the live lookup
// returns the same position, so the refresh is a no-op.
func (r *rustActionRouter) refreshCursor(cmd *textapi.Command) {
	if cmd.Resource == nil || cmd.Cursor.Content != (term.Coordinates{}) {
		return
	}
	if live, err := r.editor.Cursor(cmd.Resource); err == nil {
		cmd.Cursor.Content = live
	}
}

func (r *rustActionRouter) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if cmd != actionCmdName {
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
