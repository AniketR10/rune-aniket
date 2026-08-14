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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// newTestPickCmd wires a listPickCmd over the given entries against
// recording mocks.
func newTestPickCmd(entries []pickEntry) (
	*listPickCmd, *fakeWM, *mockEditor, *mockResourceOpener, *mockNotifications,
) {
	editor := newMockEditor()
	opener := newMockResourceOpener(editor)
	wm := &fakeWM{}
	notify := &mockNotifications{}
	cmd := &listPickCmd{
		pickDeps: pickDeps{
			editor: editor, wm: wm, opener: opener,
			notify: notify, fs: realFS{root: "/ws"},
		},
		produce: func(context.Context, textapi.Command) ([]pickEntry, error) {
			return entries, nil
		},
		emptyMsg: "nothing here",
	}
	return cmd, wm, editor, opener, notify
}

func pickEntryAt(name string, line uint32) pickEntry {
	return pickEntry{
		loc: semanticapi.Location{
			URI: "file:///ws/src/" + name,
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: line},
				End:   semanticapi.Position{Line: line, Character: 3},
			},
		},
		display: name,
	}
}

// The picker's deferred highlight pass must wake the IDE event loop;
// without an interrupt the preview highlights render only on the next
// input event.
func TestPickerViewTickInterrupts(t *testing.T) {
	ir := &recordingInterrupter{}
	v := &pickerView{ch: make(chan int, 1), interrupt: ir}
	ran := false
	v.tick(func() { ran = true })
	require.True(t, ran, "the deferred work runs")
	assert.Equal(t, 1, ir.interrupts(), "the tick requests a redraw")
}

func TestListPickCmdEmptyNotifies(t *testing.T) {
	cmd, wm, _, opener, notify := newTestPickCmd(nil)
	require.NoError(t, cmd.HandleCommand(t.Context(), rustCmd("diagnostics", newTestURI(t), nil)))
	assert.True(t, notify.hasMessage("nothing here"), "an empty result is reported")
	assert.Empty(t, opener.openedURIs(), "nothing is opened")
	wm.mu.Lock()
	defer wm.mu.Unlock()
	assert.Nil(t, wm.floating, "no picker is floated")
}

func TestListPickCmdSingleEntryJumps(t *testing.T) {
	entry := pickEntryAt("main.rs", 4)
	cmd, wm, editor, opener, _ := newTestPickCmd([]pickEntry{entry})
	require.NoError(t, cmd.HandleCommand(t.Context(), rustCmd("diagnostics", newTestURI(t), nil)))

	opened := opener.openedURIs()
	require.Len(t, opened, 1, "the lone entry is opened without a picker")
	assert.Contains(t, opened[0], "src/main.rs")
	h, err := editor.Editor(parseTestURI(t, opened[0]))
	require.NoError(t, err)
	assert.Equal(t, 4, editor.lastCursor(h).Y, "the cursor lands on the entry's line")
	wm.mu.Lock()
	defer wm.mu.Unlock()
	assert.Nil(t, wm.floating, "no picker is floated for a single entry")
}

func TestListPickCmdSelectionJumps(t *testing.T) {
	entries := []pickEntry{
		pickEntryAt("main.rs", 1),
		pickEntryAt("lib.rs", 7),
	}
	cmd, wm, editor, opener, _ := newTestPickCmd(entries)

	done := make(chan error, 1)
	go func() {
		done <- cmd.HandleCommand(context.Background(), rustCmd("diagnostics", newTestURI(t), nil))
	}()

	view := awaitPickerView(t, wm)
	displays := make([]string, 0, len(view.Entries()))
	for _, e := range view.Entries() {
		displays = append(displays, e.Display)
	}
	assert.Equal(t, []string{"main.rs", "lib.rs"}, displays)

	view.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	view.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the jump")
	}

	opened := opener.openedURIs()
	require.Len(t, opened, 1)
	assert.Contains(t, opened[0], "src/lib.rs", "the second entry was selected")
	h, err := editor.Editor(parseTestURI(t, opened[0]))
	require.NoError(t, err)
	assert.Equal(t, 7, editor.lastCursor(h).Y, "the cursor lands on the entry's line")
}

// Dismissing the picker without a choice unblocks the command instead of
// leaving its goroutine waiting on a selection that never arrives.
func TestListPickCmdCancelUnblocks(t *testing.T) {
	cmd, wm, _, opener, _ := newTestPickCmd([]pickEntry{
		pickEntryAt("main.rs", 1),
		pickEntryAt("lib.rs", 7),
	})

	done := make(chan error, 1)
	go func() {
		done <- cmd.HandleCommand(context.Background(), rustCmd("diagnostics", newTestURI(t), nil))
	}()

	require.NoError(t, awaitPickerView(t, wm).Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the cancelled command")
	}
	assert.Empty(t, opener.openedURIs(), "cancelling opens nothing")
}

// The jump must land in the window that was focused when the command
// was invoked, not in the picker's floating window: in production the
// float holds the focus while it is up, so resolving the target window
// after the selection replaces the picker window with the opened file.
func TestListPickCmdJumpTargetsInvokeWindow(t *testing.T) {
	entries := []pickEntry{
		pickEntryAt("main.rs", 1),
		pickEntryAt("lib.rs", 7),
	}
	cmd, wm, _, _, _ := newTestPickCmd(entries)

	done := make(chan error, 1)
	go func() {
		done <- cmd.HandleCommand(context.Background(), rustCmd("diagnostics", newTestURI(t), nil))
	}()

	view := awaitPickerView(t, wm)
	view.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the jump")
	}

	win, content := wm.lastContent()
	require.NotNil(t, content, "the jump sets window content")
	assert.Equal(t, uint64(editorWinID), win.WindowID(),
		"the jump must target the invoking editor window, not the floating picker")
}

func awaitPickerView(t *testing.T, wm *fakeWM) *pickerView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		wm.mu.Lock()
		view, ok := wm.floating.(*pickerView)
		wm.mu.Unlock()
		if ok {
			return view
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the location picker")
	return nil
}

// requestLSP answers a single ExecuteRequest method with a canned
// result.
type requestLSP struct {
	noopLSP
	method string
	result json.RawMessage
}

func (l *requestLSP) Initialize(
	_ context.Context, _ semanticapi.InitializeParams,
) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}

func (l *requestLSP) ExecuteRequest(
	_ context.Context, p semanticapi.ExecuteRequestParams,
) (json.RawMessage, error) {
	if p.Method != l.method {
		return nil, fmt.Errorf("unexpected method %s", p.Method)
	}
	return l.result, nil
}

// rust-analyzer reports each crate's path as a file:// URL of the crate
// root directory (older servers send a plain path). The producer must
// not mangle the URL and should point at the crate manifest when it
// exists, so the picker previews and opens a real file.
func TestDependenciesPickResolvesCratePaths(t *testing.T) {
	tests := []struct {
		name    string
		crate   map[string]string
		fs      func(*fakeFS)
		wantURI string
	}{
		{
			name:  "url path resolves to the crate manifest",
			crate: map[string]string{"name": "serde", "version": "1.0.0", "path": "file:///dep/serde"},
			fs: func(f *fakeFS) {
				f.addDir("/dep/serde").addFile("/dep/serde/Cargo.toml")
			},
			wantURI: "file:///dep/serde/Cargo.toml",
		},
		{
			name:  "plain path resolves to the crate manifest",
			crate: map[string]string{"name": "libc", "path": "/dep/libc"},
			fs: func(f *fakeFS) {
				f.addDir("/dep/libc").addFile("/dep/libc/Cargo.toml")
			},
			wantURI: "file:///dep/libc/Cargo.toml",
		},
		{
			name:    "directory without a manifest is kept as is",
			crate:   map[string]string{"name": "core", "path": "file:///dep/core"},
			fs:      func(f *fakeFS) { f.addDir("/dep/core") },
			wantURI: "file:///dep/core",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(map[string]any{"crates": []any{tt.crate}})
			require.NoError(t, err)
			lsp := &requestLSP{method: "rust-analyzer/fetchDependencyList", result: result}
			fs := newFakeFS()
			tt.fs(fs)

			entries, err := dependenciesPick(lsp, fs)(t.Context(), textapi.Command{})
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, tt.wantURI, entries[0].loc.URI)
			wantDisplay := tt.crate["name"]
			if v := tt.crate["version"]; v != "" {
				wantDisplay += " " + v
			}
			assert.Equal(t, wantDisplay, entries[0].display)
		})
	}
}
