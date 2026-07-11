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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

func newTestActionRouter(lsp semanticapi.LSP, notify *fakeNotifications) textapi.CommandHandler {
	_, h := newRustActionHandler(lsp, &fakeEditor{}, &fakeWM{}, notify, nil,
		lspcmd.NewSelectionTracker(), newFakeExecutor(), "/ws", true)
	return h
}

func TestRustActionManualListsSubcommands(t *testing.T) {
	manual, _ := newRustActionHandler(&actionLSP{}, &fakeEditor{}, &fakeWM{},
		newFakeNotifications(), nil, lspcmd.NewSelectionTracker(), newFakeExecutor(), "/ws", true)
	assert.Equal(t, actionCmdName, manual.Name)
	names := make(map[string]bool)
	for _, c := range manual.Commands {
		names[c.Name] = true
	}
	for _, want := range []string{"list", "extract", "inline", "rewrite", "refactor", "quickfix", "organize-imports"} {
		assert.True(t, names[want], "manual missing subcommand %q", want)
	}
}

// The experimental/* subcommands register only when the experimental flag
// is set; the always-on assist and view commands register either way.
func TestRustActionExperimentalGating(t *testing.T) {
	experimentalOnly := []string{
		"parent-module", "child-modules", "open-cargo-toml", "external-docs",
		"join-lines", "matching-brace", "on-enter", "move-item-up", "move-item-down",
		"ssr", "runnables", "run", "type", "symbols", "hover", "eval-predicate",
	}
	alwaysOn := []string{"list", "extract", "status", "hir", "expand-macro", "reload-workspace"}

	manualNames := func(experimental bool) map[string]bool {
		manual, h := newRustActionHandler(&actionLSP{}, &fakeEditor{}, &fakeWM{},
			newFakeNotifications(), nil, lspcmd.NewSelectionTracker(),
			newFakeExecutor(), "/ws", experimental)
		names := make(map[string]bool)
		for _, c := range manual.Commands {
			names[c.Name] = true
		}
		router := h.(*rustActionRouter)
		for name := range router.handlers {
			names[name] = true
		}
		return names
	}

	off := manualNames(false)
	for _, name := range experimentalOnly {
		assert.False(t, off[name], "%q must not register without the experimental flag", name)
	}
	for _, name := range alwaysOn {
		assert.True(t, off[name], "%q must register regardless of the experimental flag", name)
	}

	on := manualNames(true)
	for _, name := range append(append([]string{}, experimentalOnly...), alwaysOn...) {
		assert.True(t, on[name], "%q must register with the experimental flag", name)
	}
}

func TestRustActionRouterUnknownCommand(t *testing.T) {
	h := newTestActionRouter(&actionLSP{}, newFakeNotifications())
	err := h.HandleCommand(context.Background(), textapi.Command{Name: "notrust"})
	require.Error(t, err)
}

func TestRustActionRouterMissingSubcommand(t *testing.T) {
	h := newTestActionRouter(&actionLSP{}, newFakeNotifications())
	err := h.HandleCommand(context.Background(), textapi.Command{Name: actionCmdName})
	require.Error(t, err)
}

func TestRustActionRouterUnknownSubcommand(t *testing.T) {
	h := newTestActionRouter(&actionLSP{}, newFakeNotifications())
	err := h.HandleCommand(context.Background(),
		textapi.Command{Name: actionCmdName, Args: []string{"nope"}})
	require.ErrorContains(t, err, "unknown rust subcommand")
}

// A known subcommand dispatches to its code-action handler, which
// forwards the request to the LSP with the mapped kind.
func TestRustActionRouterDispatchesToKind(t *testing.T) {
	uri := newTestURI(t)
	lsp := &actionLSP{}
	h := newTestActionRouter(lsp, newFakeNotifications())
	cmd := cmdFor(uri, actionCmdName, []string{"extract"})
	require.NoError(t, h.HandleCommand(context.Background(), cmd))
	require.Equal(t, []semanticapi.CodeActionKind{"refactor.extract"}, lsp.params.Context.Only)
}

func TestRustActionRouterCompletesSubcommands(t *testing.T) {
	h := newTestActionRouter(&actionLSP{}, newFakeNotifications())
	it, err := h.Complete(context.Background(), actionCmdName, []string{"ex"})
	require.NoError(t, err)
	got, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	assert.Contains(t, got, "extract")
	// Completion of the subcommand list is sorted.
	for i := 1; i < len(got); i++ {
		assert.LessOrEqual(t, got[i-1], got[i])
	}
}
