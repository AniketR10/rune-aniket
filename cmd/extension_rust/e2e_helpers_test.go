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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idelsp"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// findRustAnalyzer locates the rust-analyzer binary or skips the test.
// It mirrors resolveRustAnalyzer's search order: PATH first, then the
// bundled and cargo-installed locations.
func findRustAnalyzer(t *testing.T) string {
	t.Helper()
	if bin, err := exec.LookPath("rust-analyzer"); err == nil {
		return bin
	}
	for _, p := range []string{
		filepath.Join(os.Getenv("HOME"), ".rune", "bin", "rust-analyzer"),
		filepath.Join(os.Getenv("HOME"), ".cargo", "bin", "rust-analyzer"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("rust-analyzer not found, skipping e2e test")
	return ""
}

// setupCargoWorkspace copies the testdata Cargo project into a fresh temp
// directory so each test gets an isolated, writable workspace, and
// returns its path.
func setupCargoWorkspace(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rust-ext-e2e-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	src := "testdata/e2e"
	require.NoError(t, filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	}))
	return dir
}

// rustEnvE2E holds the initialized rust-analyzer manager and file URIs.
type rustEnvE2E struct {
	mgr      *idelsp.Manager
	cb       *raCallback
	dir      string
	fileURIs map[string]string
}

func (e *rustEnvE2E) capturedEdits() []semanticapi.ApplyWorkspaceEditParams {
	e.cb.mu.Lock()
	defer e.cb.mu.Unlock()
	return append([]semanticapi.ApplyWorkspaceEditParams{}, e.cb.appliedEdits...)
}

// initRustAnalyzer creates an idelsp.Manager, initializes rust-analyzer
// with the extension's rustInitializeParams, opens the given files, and
// waits for the server to finish loading. It captures workspace/applyEdit
// and mirrors applied edits back as editor events so the server's overlay
// stays in sync, matching the real IDE.
func initRustAnalyzer(t *testing.T, raBin string, openFiles []string) *rustEnvE2E {
	t.Helper()
	dir := setupCargoWorkspace(t)
	rootURI := "file://" + dir

	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme := newLocalScheme(dir)

	ready := make(chan struct{})
	var once sync.Once
	cb := &raCallback{
		onProgress: readyOnCachePrimed(&once, ready),
	}
	cfg := idelsp.Config{MaxRetries: 1, Callback: cb, WorkDoneProgress: true}

	mgr := idelsp.New(uri, scheme, scheme, &stubPkgManager{bin: raBin}, nil, nil, cfg)

	ctx := context.Background()
	params, err := rustInitializeParams(rootURI, raBin, sysrootFor(ctx), true)
	require.NoError(t, err)

	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)

	fileURIs := make(map[string]string)
	for _, name := range openFiles {
		path := filepath.Join(dir, name)
		fileURI := "file://" + path
		fileURIs[name] = fileURI

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		trimmed := strings.TrimSuffix(string(content), "\n")
		wsURI, err := workspaceapi.ParseURI(fileURI)
		require.NoError(t, err)
		mgr.Handle(ctx, textapi.Event{
			Type:    textapi.EventTypeOpen,
			URI:     wsURI,
			Content: trimmed,
		})
	}

	env := &rustEnvE2E{mgr: mgr, cb: cb, dir: dir, fileURIs: fileURIs}
	waitRustAnalyzerReady(t, ready, mgr, env)
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })

	env.simulateEditorEvents()
	return env
}

// simulateEditorEvents mirrors workspace/applyEdit changes back into the
// manager as EventTypeEdit so rust-analyzer's overlay tracks command-driven
// edits, as the real editor would.
func (e *rustEnvE2E) simulateEditorEvents() {
	e.cb.mu.Lock()
	e.cb.onApplyEdit = func(params semanticapi.ApplyWorkspaceEditParams) {
		for _, dc := range params.Edit.DocumentChanges {
			if dc.TextDocumentEdit == nil {
				continue
			}
			wsURI, err := workspaceapi.ParseURI(dc.TextDocumentEdit.TextDocument.URI)
			if err != nil {
				continue
			}
			for _, te := range dc.TextDocumentEdit.Edits {
				e.mgr.Handle(context.Background(), textapi.Event{
					Type:    textapi.EventTypeEdit,
					URI:     wsURI,
					Content: te.NewText,
					Start:   term.Coordinates{X: int(te.Range.Start.Character), Y: int(te.Range.Start.Line)},
					End:     term.Coordinates{X: int(te.Range.End.Character), Y: int(te.Range.End.Line)},
				})
			}
		}
		for fileURI, edits := range params.Edit.Changes {
			wsURI, err := workspaceapi.ParseURI(fileURI)
			if err != nil {
				continue
			}
			for _, te := range edits {
				e.mgr.Handle(context.Background(), textapi.Event{
					Type:    textapi.EventTypeEdit,
					URI:     wsURI,
					Content: te.NewText,
					Start:   term.Coordinates{X: int(te.Range.Start.Character), Y: int(te.Range.Start.Line)},
					End:     term.Coordinates{X: int(te.Range.End.Character), Y: int(te.Range.End.Line)},
				})
			}
		}
	}
	e.cb.mu.Unlock()
}

// sysrootFor resolves the toolchain sysroot so rust-analyzer can find the
// standard library; an empty result lets the server fall back to its own
// discovery.
func sysrootFor(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "rustc", "--print", "sysroot").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// newTestActionHandler builds the rust action command handler wired to
// the env's manager, a mock editor, and mock notifications, mirroring the
// production wiring in extendWorkspaceWith.
func newTestActionHandler(
	t *testing.T, env *rustEnvE2E,
) (textapi.CommandHandler, *mockEditor, *mockNotifications) {
	t.Helper()
	handler, me, mn, _ := newTestActionHandlerOpener(t, env)
	return handler, me, mn
}

// newTestActionHandlerOpener is newTestActionHandler that also returns the
// mock resource opener, for navigation-command tests that assert which
// files were opened.
func newTestActionHandlerOpener(
	t *testing.T, env *rustEnvE2E,
) (textapi.CommandHandler, *mockEditor, *mockNotifications, *mockResourceOpener) {
	t.Helper()
	me := newMockEditor()
	me.onCellEdit = func(uri workspaceapi.URI, start, end term.Coordinates, text string) {
		env.mgr.Handle(context.Background(), textapi.Event{
			Type:    textapi.EventTypeEdit,
			URI:     uri,
			Content: text,
			Start:   start,
			End:     end,
		})
	}
	mn := &mockNotifications{}
	sel := lspcmd.NewSelectionTracker()
	require.NoError(t, me.SubscribeEvents(
		[]textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}, sel))
	opener := newMockResourceOpener(me)
	_, handler := newRustActionHandler(env.mgr, me, &fakeWM{}, mn, opener, sel, true)
	return handler, me, mn, opener
}

// rustCmd builds a textapi.Command for the `rust` action command.
func rustCmd(sub string, uri workspaceapi.URI, resource textapi.Handler) textapi.Command {
	return textapi.Command{
		Name:     actionCmdName,
		Args:     []string{sub},
		URI:      uri,
		Resource: resource,
	}
}

// rustCmdAt is rustCmd with the cursor positioned at (line, char).
func rustCmdAt(
	sub string, uri workspaceapi.URI, resource textapi.Handler, line, char int,
) textapi.Command {
	cmd := rustCmd(sub, uri, resource)
	cmd.Cursor.Content = term.Coordinates{X: char, Y: line}
	return cmd
}

// collectEditText concatenates the NewText of every recorded editor edit.
func collectEditText(edits []mockEdit) string {
	var b strings.Builder
	for _, e := range edits {
		b.WriteString(e.NewText)
	}
	return b.String()
}

// capturedEditText concatenates the NewText of every workspace/applyEdit change.
func capturedEditText(captured []semanticapi.ApplyWorkspaceEditParams) string {
	var b strings.Builder
	for _, ae := range captured {
		for _, edits := range ae.Edit.Changes {
			for _, e := range edits {
				b.WriteString(e.NewText)
			}
		}
		for _, dc := range ae.Edit.DocumentChanges {
			if dc.TextDocumentEdit != nil {
				for _, e := range dc.TextDocumentEdit.Edits {
					b.WriteString(e.NewText)
				}
			}
		}
	}
	return b.String()
}

func parseTestURI(t *testing.T, fileURI string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(fileURI)
	require.NoError(t, err)
	return u
}

// readyOnCachePrimed closes ready when rust-analyzer's cachePriming
// progress reports "end". rust-analyzer only computes assists once the
// crate is indexed, and it emits several earlier progress sequences
// (Fetching, Building CrateGraph, Roots Scanned) whose "end" fires before
// the project is ready, so the token must be matched explicitly.
func readyOnCachePrimed(once *sync.Once, ready chan struct{}) func(semanticapi.ProgressParams) {
	return func(p semanticapi.ProgressParams) {
		if p.Token.StringValue != "rustAnalyzer/cachePriming" {
			return
		}
		var v struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(p.Value, &v) != nil {
			return
		}
		if v.Kind == "end" {
			once.Do(func() { close(ready) })
		}
	}
}

// waitRustAnalyzerReady blocks until rust-analyzer has primed its cache,
// which is when assists first become available. It waits for the
// cachePriming progress "end" (fed through the callback), then confirms
// the server actually answers assists at a known extractable position to
// absorb the brief window between the end signal and the first successful
// codeAction.
func waitRustAnalyzerReady(
	t *testing.T, ready <-chan struct{}, mgr *idelsp.Manager, env *rustEnvE2E,
) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(120 * time.Second):
		t.Fatal("rust-analyzer did not finish cache priming")
	}

	var fileURI string
	for _, u := range env.fileURIs {
		fileURI = u
		break
	}
	if fileURI == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		results, err := mgr.CodeAction(ctx, semanticapi.CodeActionParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 1, Character: 4},
				End:   semanticapi.Position{Line: 1, Character: 4},
			},
			Context: semanticapi.CodeActionContext{
				Diagnostics: []semanticapi.Diagnostic{},
				TriggerKind: semanticapi.CodeActionTriggerKindInvoked,
			},
		})
		if err == nil && len(results) > 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// stubPkgManager implements idelsp.PkgManager, resolving the language
// server directly to the discovered binary.
type stubPkgManager struct {
	bin string
}

func (p *stubPkgManager) LibDir(
	_ context.Context, _ string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice([]string{p.bin}), nil
}
