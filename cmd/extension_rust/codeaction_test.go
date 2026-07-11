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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// stubResource implements textapi.Handler so a code-action command sees a
// non-nil focused resource.
type stubResource struct {
	uri workspaceapi.URI
}

func (s *stubResource) Handle(_ term.Event) (bool, bool) { return false, false }
func (s *stubResource) Draw(_ term.Writer)               {}
func (s *stubResource) Resize(_, _ int)                  {}
func (s *stubResource) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (s *stubResource) Selection() (string, bool)  { return "", false }
func (s *stubResource) Close() error               { return nil }
func (s *stubResource) Resource() workspaceapi.URI { return s.uri }

var _ textapi.Handler = (*stubResource)(nil)

// actionLSP is a semanticapi.LSP whose CodeAction returns a scripted set
// of results and whose ExecuteCommand records the commands it receives.
type actionLSP struct {
	noopLSP
	params   semanticapi.CodeActionParams
	results  []semanticapi.CodeActionResult
	executed []string
}

func (l *actionLSP) Initialize(
	_ context.Context, _ semanticapi.InitializeParams,
) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}

func (l *actionLSP) CodeAction(
	_ context.Context, params semanticapi.CodeActionParams,
) ([]semanticapi.CodeActionResult, error) {
	l.params = params
	return l.results, nil
}

func (l *actionLSP) ExecuteCommand(
	_ context.Context, params semanticapi.ExecuteCommandParams,
) (string, error) {
	l.executed = append(l.executed, params.Command)
	return "", nil
}

func action(title string, kind semanticapi.CodeActionKind, cmd *semanticapi.Command) semanticapi.CodeActionResult {
	a := &semanticapi.CodeAction{Title: title, Kind: kind, Command: cmd}
	return semanticapi.CodeActionResult{CodeAction: a}
}

func cmdFor(uri workspaceapi.URI, name string, args []string) textapi.Command {
	return textapi.Command{
		Name:     name,
		Args:     args,
		URI:      uri,
		Resource: &stubResource{uri: uri},
	}
}

func newTestURI(t *testing.T) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI("file:///ws/src/main.rs")
	require.NoError(t, err)
	return uri
}

// waitForPicker polls the window manager until a code-action picker has
// been shown, failing the test if it never appears.
func waitForPicker(t *testing.T, wm *fakeWM) *codeActionPicker {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		wm.mu.Lock()
		f := wm.floating
		wm.mu.Unlock()
		if p, ok := f.(*codeActionPicker); ok {
			return p
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for code-action picker")
	return nil
}

func TestCodeActionRequiresOpenFile(t *testing.T) {
	lsp := &actionLSP{}
	notify := newFakeNotifications()
	h := codeActionHandler(lsp, &fakeEditor{}, notify, &fakeWM{}, lspcmd.NewSelectionTracker(),
		"refactor.rewrite", "hint")
	err := h.HandleCommand(context.Background(), textapi.Command{Name: "rewrite"})
	require.Error(t, err)
}

func TestCodeActionNoActionNotifiesHint(t *testing.T) {
	uri := newTestURI(t)
	lsp := &actionLSP{}
	notify := newFakeNotifications()
	h := codeActionHandler(lsp, &fakeEditor{}, notify, &fakeWM{}, lspcmd.NewSelectionTracker(),
		"refactor.rewrite", "no rewrite here")
	require.NoError(t, h.HandleCommand(context.Background(), cmdFor(uri, "rewrite", nil)))
	assert.Contains(t, notify.notifMessages(), "no rewrite here")
}

// The broad-kind subcommands must pass their kind as the Only filter and
// drop results whose kind does not share the requested prefix.
func TestCodeActionFiltersByKind(t *testing.T) {
	uri := newTestURI(t)
	lsp := &actionLSP{results: []semanticapi.CodeActionResult{
		action("Extract into variable", "refactor.extract", &semanticapi.Command{Command: "extract"}),
		action("Invert if", "refactor.rewrite", &semanticapi.Command{Command: "rewrite"}),
	}}
	notify := newFakeNotifications()
	h := codeActionHandler(lsp, &fakeEditor{}, notify, &fakeWM{}, lspcmd.NewSelectionTracker(),
		"refactor.extract", "hint")
	require.NoError(t, h.HandleCommand(context.Background(), cmdFor(uri, "extract", nil)))

	require.Equal(t, []semanticapi.CodeActionKind{"refactor.extract"}, lsp.params.Context.Only)
	// Only the single matching action ran, applied directly without a picker.
	assert.Equal(t, []string{"extract"}, lsp.executed)
}

// The list subcommand requests all kinds (nil Only) and keeps every
// returned action regardless of kind.
func TestCodeActionListRequestsAllKinds(t *testing.T) {
	uri := newTestURI(t)
	lsp := &actionLSP{results: []semanticapi.CodeActionResult{
		action("Extract into variable", "refactor.extract", nil),
		action("Invert if", "refactor.rewrite", nil),
	}}
	notify := newFakeNotifications()
	wm := &fakeWM{}
	h := codeActionHandler(lsp, &fakeEditor{}, notify, wm, lspcmd.NewSelectionTracker(), "", "hint")

	// Two applicable actions means the picker is shown and HandleCommand
	// blocks on the user's choice. Run it in the background, wait for the
	// picker, then cancel to unblock without selecting anything.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.HandleCommand(ctx, cmdFor(uri, "list", nil)) }()

	picker := waitForPicker(t, wm)
	assert.Len(t, picker.actions, 2)
	assert.Nil(t, lsp.params.Context.Only)

	cancel()
	require.Error(t, <-done)
}

func TestCodeActionExecutesCommand(t *testing.T) {
	uri := newTestURI(t)
	lsp := &actionLSP{results: []semanticapi.CodeActionResult{
		action("Organize imports", "source.organizeImports",
			&semanticapi.Command{Command: "rust-analyzer.organizeImports"}),
	}}
	notify := newFakeNotifications()
	h := codeActionHandler(lsp, &fakeEditor{}, notify, &fakeWM{}, lspcmd.NewSelectionTracker(),
		"source.organizeImports", "hint")
	require.NoError(t, h.HandleCommand(context.Background(), cmdFor(uri, "organize-imports", nil)))
	assert.Equal(t, []string{"rust-analyzer.organizeImports"}, lsp.executed)
}
