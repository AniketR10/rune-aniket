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

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// TestE2E drives the `rust` command handler against a real rust-analyzer
// serving a real Cargo project under testdata/e2e. Each subtest exercises
// one command end to end: it opens a file, positions the cursor (or a
// selection), runs the command through the real handler, and asserts the
// server produced the expected edit. The suite skips when rust-analyzer is
// not installed.
func TestE2E(t *testing.T) {
	t.Parallel()
	raBin := findRustAnalyzer(t)

	t.Run("ExtractVariable", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Select the expression `(1 + 2)` on line 1 (0-based): columns
		// 17..24 of `    let result = (1 + 2) * 4;`. `rust extract` offers
		// variable/constant/static/function, so pick "Extract into variable".
		me.fireEvent(t.Context(), textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 17, Y: 1},
			End:   term.Coordinates{X: 24, Y: 1},
		})

		cmd := rustCmdAt("extract", uri, resource, 1, 17)
		runAction(t, handler, cmd, "Extract into variable")

		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "expected an extract-variable edit")
		assert.Contains(t, combined, "var_name", "extract into variable introduces var_name")
		assert.NotContains(t, combined, "${", "snippet placeholders must not leak into the buffer")
		assert.NotContains(t, combined, "$0", "snippet tab stops must not leak into the buffer")
	})

	t.Run("InlineLocalVariable", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/inline.rs"})
		handler, me, _ := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/inline.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the `x` usage in `    x * 4` (line 2, col 4).
		cmd := rustCmdAt("inline", uri, resource, 2, 4)
		runAction(t, handler, cmd, "Inline variable")

		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "expected an inline edit")
		assert.Contains(t, combined, "1 + 2", "inlining x should substitute its initializer")
	})

	t.Run("Rewrite", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/rewrite.rs"})
		handler, me, _ := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/rewrite.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the `>` operator in `    a > b` (line 1, col 6);
		// "Flip comparison" swaps the operands and inverts the operator.
		cmd := rustCmdAt("rewrite", uri, resource, 1, 6)
		runAction(t, handler, cmd, "Flip comparison")

		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "expected a rewrite edit")
		assert.Contains(t, combined, "<", "flip comparison turns > into <")
	})

	t.Run("Refactor", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/refactor.rs"})
		handler, me, _ := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/refactor.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the `if` keyword (line 1, col 4). The refactor family
		// includes "Convert to guarded return", which we pick.
		cmd := rustCmdAt("refactor", uri, resource, 1, 4)
		runAction(t, handler, cmd, "Invert if")

		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "expected a refactor edit from the picked assist")
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/list.rs"})
		handler, me, _ := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/list.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Select `1 + 2 + 3` (line 1) so multiple assists apply; `list`
		// surfaces them all and we pick "Extract into variable".
		me.fireEvent(t.Context(), textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 16, Y: 1},
			End:   term.Coordinates{X: 25, Y: 1},
		})

		cmd := rustCmdAt("list", uri, resource, 1, 16)
		runAction(t, handler, cmd, "Extract into variable")

		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "expected an edit from the picked assist in list mode")
		assert.Contains(t, combined, "var_name")
	})

	t.Run("OrganizeImports", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/main.rs"})
		handler, me, mn := newTestActionHandler(t, env)

		uri := parseTestURI(t, env.fileURIs["src/main.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		cmd := rustCmdAt("organize-imports", uri, resource, 0, 0)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))

		// organize-imports either edits the buffer directly, applies via
		// workspace/applyEdit, or reports nothing to do — all valid. When
		// it edits, the two separate use lines merge into a braced group.
		edits := me.editsFor(resource)
		captured := env.capturedEdits()
		if len(edits) == 0 && len(captured) == 0 {
			assert.True(t, mn.hasMessage("Imports are already organized"))
			return
		}
		combined := collectEditText(edits) + capturedEditText(captured)
		assert.Contains(t, combined, "{", "merged imports should use a braced group")
	})

	t.Run("FileText", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("file-text", uri, resource, 0, 0))
		assert.Contains(t, text, "pub fn compute", "file-text should echo the buffer")
	})

	t.Run("SyntaxTree", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("syntax-tree", uri, resource, 0, 0))
		assert.Contains(t, text, "FN", "syntax tree should contain a function node")
	})

	t.Run("Hir", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor inside compute's body.
		text := runViewer(t, handler, rustCmdAt("hir", uri, resource, 1, 8))
		assert.NotEmpty(t, text, "hir should return the lowered body")
	})

	t.Run("Mir", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("mir", uri, resource, 0, 7))
		assert.NotEmpty(t, text, "mir should return the lowered function")
	})

	t.Run("ItemTree", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("item-tree", uri, resource, 0, 0))
		assert.Contains(t, text, "compute", "item tree should list the function")
	})

	t.Run("ExpandMacro", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/macros.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/macros.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the answer!() invocation (line 7, col 4).
		text := runViewer(t, handler, rustCmdAt("expand-macro", uri, resource, 7, 4))
		assert.Contains(t, text, "42", "expanding answer!() yields its body")
	})

	t.Run("Status", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("status", uri, resource, 0, 0))
		assert.NotEmpty(t, text, "analyzerStatus should report a status string")
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// memoryUsage is only answered by memory-profiling builds of
		// rust-analyzer; other builds reject it. Accept either outcome so
		// the command wiring is still exercised without requiring that
		// build.
		err := handler.HandleCommand(t.Context(), rustCmdAt("memory-usage", uri, resource, 0, 0))
		if err != nil {
			assert.Contains(t, err.Error(), "Memory profiling is not enabled")
		}
	})

	t.Run("CrateGraph", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("crate-graph", uri, resource, 0, 0))
		assert.Contains(t, text, "digraph", "crate graph is emitted as DOT")
	})

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		text := runViewer(t, handler, rustCmdAt("dependencies", uri, resource, 0, 0))
		assert.Contains(t, text, "std", "the dependency list includes std")
	})

	t.Run("ParentModule", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		cmd := rustCmdAt("parent-module", uri, resource, 0, 0)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		opened := opener.openedURIs()
		require.NotEmpty(t, opened, "parent-module should open the declaring file")
		assert.Contains(t, opened[0], "lib.rs", "extract_var's parent is declared in lib.rs")
	})

	t.Run("OpenCargoToml", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		cmd := rustCmdAt("open-cargo-toml", uri, resource, 0, 0)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		opened := opener.openedURIs()
		require.NotEmpty(t, opened, "open-cargo-toml should open a Cargo.toml")
		assert.Contains(t, opened[0], "Cargo.toml")
	})

	t.Run("ExternalDocs", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/main.rs"})
		handler, me, mn := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/main.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// The cursor must be on a symbol *usage*, not its `use` import:
		// rust-analyzer's externalDocs returns {web:null, local:null} for the
		// import path but a docs.rs/doc.rust-lang.org URL for the HashMap in
		// `HashMap::new()` on line 4. With localDocs advertised the response
		// is the {web, local} object and parseExternalDocs prefers web.
		cmd := rustCmdAt("external-docs", uri, resource, 4, 20)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		assert.True(t, mn.hasMessage("https://doc.rust-lang.org"),
			"external-docs reports the HashMap documentation URL; got %v", mn.messages)
	})

	t.Run("JoinLines", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/join.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/join.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Select the multi-line add(...) call arguments (lines 1..3).
		me.fireEvent(t.Context(), textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 14, Y: 1},
			End:   term.Coordinates{X: 6, Y: 3},
		})
		cmd := rustCmdAt("join-lines", uri, resource, 1, 14)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "join-lines should produce an edit")
		assert.NotContains(t, combined, "\n", "joined arguments collapse onto one line")
	})

	t.Run("Ssr", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/ssr.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/ssr.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		cmd := rustCmd("ssr", uri, resource)
		cmd.Args = []string{"ssr", "foo($a) ==>> bar($a)"}
		cmd.Cursor.Content = term.Coordinates{X: 4, Y: 1}
		// The router strips the leading subcommand name from Args.
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "ssr should produce an edit")
		assert.Contains(t, combined, "bar", "ssr rewrites foo(...) into bar(...)")
	})

	t.Run("MoveItemDown", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/moveitem.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/moveitem.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on first()'s signature (line 0).
		cmd := rustCmdAt("move-item-down", uri, resource, 0, 3)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		combined := allEdits(me, resource, env)
		require.NotEmpty(t, combined, "move-item-down should produce an edit")
		assert.NotContains(t, combined, "$0", "snippet tab stops must not leak into the buffer")
		assert.NotContains(t, combined, "${", "snippet placeholders must not leak into the buffer")
	})

	t.Run("OnEnter", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/onenter.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/onenter.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor at the end of the `/// first line` doc comment (line 0).
		cmd := rustCmdAt("on-enter", uri, resource, 0, 14)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		combined := allEdits(me, resource, env)
		if combined == "" {
			return // some rust-analyzer builds decline onEnter here.
		}
		assert.NotContains(t, combined, "$0", "snippet tab stops must not leak into the buffer")
	})

	t.Run("MatchingBrace", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/brace.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/brace.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the opening paren in `(1 + 2)` (line 1, col 12).
		cmd := rustCmdAt("matching-brace", uri, resource, 1, 12)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		got := me.lastCursor(resource)
		assert.Greater(t, got.X, 12, "cursor should move to the matching close paren")
	})

	t.Run("Flycheck", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		for _, sub := range []string{"run-flycheck", "clear-flycheck", "cancel-flycheck"} {
			require.NoError(t, handler.HandleCommand(t.Context(), rustCmdAt(sub, uri, resource, 0, 0)),
				"%s must not error", sub)
		}
	})

	t.Run("ReloadWorkspace", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, mn := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		require.NoError(t, handler.HandleCommand(t.Context(), rustCmdAt("reload-workspace", uri, resource, 0, 0)))
		assert.True(t, mn.hasMessage("Reloaded"), "reload-workspace reports completion")
	})

	// The runnables, related-tests, interpret, recursive-memory-layout and
	// failed-obligations subcommands are best-effort: some rust-analyzer
	// builds answer them and some decline. This asserts only that the
	// command wiring encodes valid params (no serialization error), so a
	// declining server surfaces as an empty result rather than a crash.
	t.Run("BestEffortViewers", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/refactor.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/refactor.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		for _, sub := range []string{
			"runnables", "related-tests", "interpret",
			"recursive-memory-layout", "failed-obligations",
		} {
			err := handler.HandleCommand(t.Context(), rustCmdAt(sub, uri, resource, 0, 7))
			if err != nil {
				assert.NotContains(t, err.Error(), "Failed to deserialize",
					"%s must send valid params", sub)
			}
		}
	})

	t.Run("ChildModules", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/lib.rs"})
		handler, me, _, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/lib.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// The cursor must sit at the crate root outside any `mod` item so
		// rust-analyzer's child_modules takes the file-to-module-defs branch
		// and returns the crate's module declarations. A cursor on a `mod x;`
		// declaration instead descends into that module; see
		// rust-analyzer/crates/ide/src/child_modules.rs. lib.rs ends with a
		// dedicated comment anchor line (line 14, 0-based) that stays outside
		// every module node regardless of how many modules precede it.
		// childModules returns the declaration locations, all in lib.rs, and
		// the command opens the first one.
		cmd := rustCmdAt("child-modules", uri, resource, 14, 0)
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		opened := opener.openedURIs()
		require.NotEmpty(t, opened, "child-modules must open a declaration location")
		assert.Contains(t, opened[0], "lib.rs",
			"child-modules opens the module declaration in the crate root")
		got := me.lastCursor(resource)
		assert.Equal(t, 0, got.Y, "cursor lands on the first module declaration (line 0)")
		assert.Equal(t, 8, got.X, "cursor lands on the module name, past `pub mod `")
	})

	t.Run("Symbols", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/types.rs"})
		handler, me, _, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/types.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// `Widget` names exactly one workspace symbol, so the command takes
		// the single-hit path and opens its definition in types.rs. The
		// searchScope/searchKind filters are sent and accepted by the server
		// (see TestE2E/SymbolsScopeKindAccepted) even though this query does
		// not exercise their narrowing.
		cmd := rustCmd("symbols", uri, resource)
		cmd.Args = []string{"symbols", "Widget"}
		require.NoError(t, handler.HandleCommand(t.Context(), cmd))
		opened := opener.openedURIs()
		require.Len(t, opened, 1, "exactly one symbol is named Widget")
		assert.Contains(t, opened[0], "types.rs")
		got := me.lastCursor(resource)
		assert.Equal(t, 0, got.Y, "cursor lands on the Widget declaration line")
		assert.Equal(t, 11, got.X, "cursor lands on the struct name")
	})

	// The `deps` argument switches searchScope to workspaceAndDependencies.
	// This asserts the raw workspace/symbol request with the experimental
	// searchScope/searchKind fields is accepted by the real server (no
	// deserialize error) and still resolves the workspace symbol.
	t.Run("SymbolsScopeKindAccepted", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/types.rs"})
		handler, me, mn, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/types.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		cmd := rustCmd("symbols", uri, resource)
		cmd.Args = []string{"symbols", "Widget", "deps"}
		require.NoError(t, handler.HandleCommand(t.Context(), cmd),
			"workspace/symbol with searchScope/searchKind must be accepted")
		assert.False(t, mn.hasMessage("No matching"), "Widget resolves with dependency scope")
		assert.NotEmpty(t, opener.openedURIs())
	})

	t.Run("TypeOfSelection", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/extract_var.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/extract_var.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Select `(1 + 2)` on line 1 (cols 17..24); hoverRange reports i32.
		me.fireEvent(t.Context(), textapi.Event{
			Type:  textapi.EventTypeSelection,
			URI:   uri,
			Start: term.Coordinates{X: 17, Y: 1},
			End:   term.Coordinates{X: 24, Y: 1},
		})
		text := runViewer(t, handler, rustCmdAt("type", uri, resource, 1, 17))
		assert.Contains(t, text, "i32", "the type of (1 + 2) is i32")
	})

	// `rust hover` renders the hover documentation as markdown in a floating
	// window. Hovering the `make` fn returns non-empty docs, so a floating
	// markdown handler (not a picker) is shown even before any action pick.
	t.Run("HoverMarkdown", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/types.rs"})
		handler, me, _, _ := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/types.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the `make` fn name (line 4, col 7). Hover has actions
		// here (Go to type Widget), so drive the picker to completion.
		titles := runHoverAction(t, handler, rustCmdAt("hover", uri, resource, 4, 7), "")
		require.NotEmpty(t, titles, "hover over `make` returns actions")
	})

	// The `make` fn's return type is `Widget`, so rust-analyzer attaches a
	// "Go to type" action (rust-analyzer.gotoLocation). Selecting it must
	// navigate to Widget's definition in types.rs. Actions only come back
	// because initialize.go advertises experimental.hoverActions plus the
	// client command list; without them the picker would be empty.
	t.Run("HoverGotoTypeAction", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/types.rs"})
		handler, me, _, opener := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/types.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		titles := runHoverAction(t, handler, rustCmdAt("hover", uri, resource, 4, 7), "Widget")
		require.Contains(t, strings.Join(titles, "|"), "Widget",
			"hover over `make` offers a Go to type Widget action; got %v", titles)
		opened := opener.openedURIs()
		require.NotEmpty(t, opened, "gotoLocation opens the type definition")
		assert.Contains(t, opened[0], "types.rs", "Widget is defined in types.rs")
	})

	// Hovering a type (the `Widget` struct) attaches a
	// "N implementations"/references action (rust-analyzer.showReferences).
	// This confirms the showReferences action path is populated by the real
	// server, which again depends on the advertised client command list.
	t.Run("HoverImplementationsAction", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/types.rs"})
		handler, me, _, _ := newTestActionHandlerOpener(t, env)
		uri := parseTestURI(t, env.fileURIs["src/types.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on the `Widget` struct name (line 0, col 11).
		titles := runHoverAction(t, handler, rustCmdAt("hover", uri, resource, 0, 11), "")
		joined := strings.ToLower(strings.Join(titles, "|"))
		assert.Contains(t, joined, "implementation",
			"hover over the Widget struct offers an implementations action; got %v", titles)
	})

	// eval-predicate evaluates a where-clause predicate in the type
	// environment at the cursor. `Marked: Marker` holds because predicate.rs
	// has `impl Marker for Marked`. The cursor must be inside a function body
	// so rust-analyzer has a type environment to evaluate against.
	t.Run("EvalPredicate", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/predicate.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/predicate.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor inside eval_here's body (line 7, `    let _x = 1;`).
		cmd := rustCmd("eval-predicate", uri, resource)
		cmd.Args = []string{"eval-predicate", "Marked:", "Marker"}
		cmd.Cursor.Content = term.Coordinates{X: 8, Y: 7}
		text := runViewer(t, handler, cmd)
		assert.Contains(t, strings.ToLower(text), "holds",
			"Marked: Marker holds because impl Marker for Marked exists; got %q", text)
	})

	// diagnostics pulls rust-analyzer's diagnostics for the current file.
	// diag.rs assigns a &str to a u32 binding, so rust-analyzer reports a
	// mismatched-types error. Diagnostics are computed asynchronously after
	// the file opens, so poll until the error shows up.
	t.Run("Diagnostics", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/diagbin.rs"})
		handler, me, _ := newTestActionHandler(t, env)
		uri := parseTestURI(t, env.fileURIs["src/diagbin.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		var text string
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			text = runViewerOrEmpty(t, handler, rustCmdAt("diagnostics", uri, resource, 1, 0))
			if strings.Contains(text, "E0308") {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		// rust-analyzer's own type-mismatch diagnostic for `let _x: u32 = "..."`
		// carries code E0308 and message "expected u32, found &'static str".
		assert.Contains(t, text, "E0308",
			"diagnostics should report the type-mismatch error code; got %q", text)
		assert.Contains(t, strings.ToLower(text), "expected u32",
			"diagnostics should include the diagnostic message; got %q", text)
	})

	// run fetches the runnable at the cursor and hands the reconstructed
	// command to the workspace executor. The command reconstruction is
	// validated in TestRunnableCommand*; this asserts the end-to-end path
	// against the real rust-analyzer runnable, capturing the command instead
	// of spawning a real (slow, network-bound) cargo build.
	t.Run("Run", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/main.rs"})
		capture := &captureExecutor{fakeStdout: "1 1\n"}
		handler, me, _ := newTestActionHandlerExec(t, env, capture)
		uri := parseTestURI(t, env.fileURIs["src/main.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on `fn main` (line 3, col 3) yields several runnables (run,
		// check, test, ...), so a picker appears; select "run e2e".
		text := runRunAction(t, handler, rustCmdAt("run", uri, resource, 3, 3), "run e2e")

		cmd, ok := capture.lastCmd()
		require.True(t, ok, "run should hand a command to the executor")
		assert.Equal(t, "cargo", cmd.Path, "cargo runnable runs the cargo binary")
		assert.Contains(t, cmd.Args, "run", "the e2e main runnable is a cargo run")
		assert.Contains(t, cmd.Args, "e2e", "the runnable targets the e2e binary")
		assert.Contains(t, text, "1 1", "the viewer shows the captured program output; got %q", text)
	})

	// The hover Run action (rust-analyzer.runSingle) executes the runnable it
	// carries, not just a notification. Hovering `fn main`, then selecting the
	// Run action, hands a cargo run command to the executor.
	t.Run("HoverRunAction", func(t *testing.T) {
		t.Parallel()
		env := initRustAnalyzer(t, raBin, []string{"src/main.rs"})
		capture := &captureExecutor{fakeStdout: "1 1\n"}
		handler, me, _ := newTestActionHandlerExec(t, env, capture)
		uri := parseTestURI(t, env.fileURIs["src/main.rs"])
		resource := &stubResource{uri: uri}
		me.Register(resource)

		// Cursor on `fn main` (line 3, col 3); select the Run action.
		titles := runHoverAction(t, handler, rustCmdAt("hover", uri, resource, 3, 3), "Run")
		require.NotEmpty(t, titles, "hover over fn main offers Run/Debug actions")
		cmd, ok := capture.lastCmd()
		require.True(t, ok, "the Run hover action executes the runnable")
		assert.Equal(t, "cargo", cmd.Path)
		assert.Contains(t, cmd.Args, "run")
	})
}

// runCommand runs a code-action command and applies the assist whose
// title is wantTitle. rust-analyzer usually returns several assists for a
// broad kind, so the command shows the picker; HandleCommand then blocks
// on the user's choice. This runs it in the background, drives the picker
// to the matching entry, and confirms it. When only one assist applies no
// picker appears and HandleCommand returns on its own.
func runAction(
	t *testing.T, handler textapi.CommandHandler, cmd textapi.Command, wantTitle string,
) {
	t.Helper()
	wm := handlerWM(t, handler)

	done := make(chan error, 1)
	go func() { done <- handler.HandleCommand(context.Background(), cmd) }()

	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case err := <-done:
			// Single-action path: applied without a picker.
			require.NoError(t, err)
			return
		default:
		}
		wm.mu.Lock()
		f := wm.floating
		wm.mu.Unlock()
		if picker, ok := f.(*codeActionPicker); ok {
			selectPickerTitle(t, picker, wantTitle)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for code-action picker for %q", wantTitle)
		}
		time.Sleep(20 * time.Millisecond)
	}

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out applying picked assist %q", wantTitle)
	}
}

// selectPickerTitle navigates the picker to the action titled wantTitle
// and confirms it with Enter, failing if no such action is offered.
func selectPickerTitle(t *testing.T, picker *codeActionPicker, wantTitle string) {
	t.Helper()
	idx := -1
	for i, a := range picker.actions {
		if a.Title == wantTitle {
			idx = i
			break
		}
	}
	if idx < 0 {
		titles := make([]string, len(picker.actions))
		for i, a := range picker.actions {
			titles[i] = a.Title
		}
		t.Fatalf("assist %q not offered; got %v", wantTitle, titles)
	}
	for i := 0; i < idx; i++ {
		picker.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	}
	picker.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
}

// allEdits concatenates every edit the command produced, whether applied
// directly to the editor buffer or via workspace/applyEdit.
func allEdits(me *mockEditor, resource textapi.Handler, env *rustEnvE2E) string {
	return collectEditText(me.editsFor(resource)) + capturedEditText(env.capturedEdits())
}

// handlerWM extracts the fakeWM the handler's code-action commands were
// wired with, so the picker it shows can be driven.
func handlerWM(t *testing.T, handler textapi.CommandHandler) *fakeWM {
	t.Helper()
	router, ok := handler.(*rustActionRouter)
	require.True(t, ok, "handler must be a rustActionRouter")
	for _, h := range router.handlers {
		if ca, ok := h.(*codeActionCmd); ok {
			wm, ok := ca.wm.(*fakeWM)
			require.True(t, ok, "code action command must use a fakeWM")
			return wm
		}
	}
	t.Fatal("no code-action command found in router")
	return nil
}

// runViewer runs a viewer subcommand and returns the text of the floating
// textView it displays, failing if no viewer appears.
func runViewer(t *testing.T, handler textapi.CommandHandler, cmd textapi.Command) string {
	t.Helper()
	wm := handlerWM(t, handler)
	require.NoError(t, handler.HandleCommand(context.Background(), cmd))
	wm.mu.Lock()
	f := wm.floating
	wm.mu.Unlock()
	view, ok := f.(*textView)
	require.True(t, ok, "expected a textView to be shown")
	return view.text
}

// runViewerOrEmpty runs a viewer subcommand and returns the floating
// textView's text, or "" when the command showed no viewer (e.g. an empty
// result reported via a notification). It clears any previously recorded
// floating handler first so repeated polling calls do not observe a stale
// viewer from an earlier iteration.
func runViewerOrEmpty(t *testing.T, handler textapi.CommandHandler, cmd textapi.Command) string {
	t.Helper()
	wm := handlerWM(t, handler)
	wm.mu.Lock()
	wm.floating = nil
	wm.mu.Unlock()
	require.NoError(t, handler.HandleCommand(context.Background(), cmd))
	wm.mu.Lock()
	f := wm.floating
	wm.mu.Unlock()
	if view, ok := f.(*textView); ok {
		return view.text
	}
	return ""
}

// runHoverAction runs a `rust hover` command, waits for the hover-action
// picker, records its labels, selects the entry whose label contains want
// (or the first entry when want is empty), and drives the command to
// completion. It returns the picker labels so tests can assert which
// actions the real server attached. A hover with no actions returns nil
// once HandleCommand completes on its own.
func runHoverAction(
	t *testing.T, handler textapi.CommandHandler, cmd textapi.Command, want string,
) []string {
	t.Helper()
	wm := handlerWM(t, handler)

	deadline := time.Now().Add(30 * time.Second)
	done := make(chan error, 1)
	go func() {
		for {
			err := handler.HandleCommand(context.Background(), cmd)
			// rust-analyzer answers ContentModified while it is
			// (re)indexing under load; the hover is retryable until
			// the server settles.
			if err != nil && strings.Contains(err.Error(), "content modified") &&
				time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			done <- err
			return
		}
	}()

	for {
		select {
		case err := <-done:
			// No actions: the markdown window is shown and the command
			// returns without a picker.
			require.NoError(t, err)
			return nil
		default:
		}
		wm.mu.Lock()
		f := wm.floating
		wm.mu.Unlock()
		if picker, ok := f.(*listPicker); ok {
			labels := append([]string(nil), picker.labels...)
			selectListPicker(picker, want)
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(20 * time.Second):
				t.Fatalf("timed out completing hover action for %q", want)
			}
			return labels
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for hover-action picker for %q", want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// selectListPicker navigates the picker to the first label containing want
// (or the first label when want is empty) and confirms it with Enter.
func selectListPicker(picker *listPicker, want string) {
	idx := 0
	if want != "" {
		for i, label := range picker.labels {
			if strings.Contains(label, want) {
				idx = i
				break
			}
		}
	}
	for i := 0; i < idx; i++ {
		picker.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	}
	picker.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
}

// runRunAction runs a `rust run` command, waits for the runnables picker,
// selects the entry whose label contains want, drives the command to
// completion, and returns the text of the output viewer it then shows.
func runRunAction(
	t *testing.T, handler textapi.CommandHandler, cmd textapi.Command, want string,
) string {
	t.Helper()
	wm := handlerWM(t, handler)
	wm.mu.Lock()
	wm.floating = nil
	wm.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- handler.HandleCommand(context.Background(), cmd) }()

	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case err := <-done:
			require.NoError(t, err)
			return viewerText(wm)
		default:
		}
		wm.mu.Lock()
		picker, isPicker := wm.floating.(*listPicker)
		wm.mu.Unlock()
		if isPicker {
			selectListPicker(picker, want)
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(20 * time.Second):
				t.Fatalf("timed out completing run action for %q", want)
			}
			return viewerText(wm)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for runnables picker for %q", want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// viewerText returns the text of the floating textView currently recorded on
// wm, or "" when the latest floating handler is not a textView.
func viewerText(wm *fakeWM) string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if view, ok := wm.floating.(*textView); ok {
		return view.text
	}
	return ""
}
