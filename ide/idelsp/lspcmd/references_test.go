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

package lspcmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/handler/locationpicker"
)

var _ textapi.CommandHandler = (*referencesHandler)(nil)

// waitFloating waits for the async cursor path to float a picker.
func waitFloating(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the async LSP result")
	}
}

func TestReferencesEnrichedDisplay(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	locs := []semanticapi.Location{
		{
			URI: "file:///project/a.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 5},
				End:   semanticapi.Position{Line: 0, Character: 8},
			},
		},
		{
			URI: "file:///project/b.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 3, Character: 0},
				End:   semanticapi.Position{Line: 3, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	done := make(chan struct{}, 1)
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			select {
			case done <- struct{}{}:
			default:
			}
			return nil, nil
		},
	}
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		rootURI, syncTick, nil, DefaultReferencesConfig(), nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///project/a.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 5, Y: 0}

	err = h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitFloating(t, done)
	require.NotNil(t, fh)

	lh := fh.(*locationpicker.Picker)
	assert.Equal(t, "a.go:1", lh.Entries()[0].Display)
}

func TestReferencesRelativePaths(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)

	locs := []semanticapi.Location{
		{
			URI: "file:///workspace/src/pkg/handler.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 10, Character: 5},
				End:   semanticapi.Position{Line: 10, Character: 11},
			},
		},
		{
			URI: "file:///workspace/cmd/main.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 3, Character: 0},
				End:   semanticapi.Position{Line: 3, Character: 6},
			},
		},
		{
			URI: "file:///other/lib/ext.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 0, Character: 0},
				End:   semanticapi.Position{Line: 0, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	done := make(chan struct{}, 1)
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			select {
			case done <- struct{}{}:
			default:
			}
			return nil, nil
		},
	}
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		rootURI, syncTick, nil, ReferencesConfig{ListConfig: locationpicker.DefaultConfig()}, nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///workspace/src/pkg/handler.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 5, Y: 10}

	err = h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitFloating(t, done)
	require.NotNil(t, fh)

	lh := fh.(*locationpicker.Picker)
	require.Len(t, lh.Entries(), 3)
	assert.Equal(t, "src/pkg/handler.go:11", lh.Entries()[0].Display, "workspace nested path should be relative")
	assert.Equal(t, "cmd/main.go:4", lh.Entries()[1].Display, "workspace path should be relative")
	assert.Equal(t, "/other/lib/ext.go:1", lh.Entries()[2].Display, "out-of-workspace path should be absolute")
}

func TestReferencesZeroRootURI(t *testing.T) {
	locs := []semanticapi.Location{
		{
			URI: "file:///workspace/pkg/foo.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 5, Character: 0},
				End:   semanticapi.Position{Line: 5, Character: 3},
			},
		},
		{
			URI: "file:///workspace/pkg/bar.go",
			Range: semanticapi.Range{
				Start: semanticapi.Position{Line: 10, Character: 0},
				End:   semanticapi.Position{Line: 10, Character: 3},
			},
		},
	}
	lsp := &mockLSP{
		referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
			return locs, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	done := make(chan struct{}, 1)
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			select {
			case done <- struct{}{}:
			default:
			}
			return nil, nil
		},
	}
	// Zero RootURI (not set) — paths should fall back to absolute.
	h := ReferencesHandler(
		lsp, editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		workspaceapi.URI{}, syncTick, nil, DefaultReferencesConfig(), nil,
	)

	uri, _ := workspaceapi.ParseURI("file:///workspace/pkg/foo.go")
	cmd := textapi.Command{Name: "references", URI: uri, Resource: &mockHandler{uri: uri}}
	cmd.Cursor.Content = term.Coordinates{X: 0, Y: 5}

	err := h.HandleCommand(context.Background(), cmd)
	require.NoError(t, err)
	waitFloating(t, done)
	require.NotNil(t, fh)

	lh := fh.(*locationpicker.Picker)
	require.Len(t, lh.Entries(), 2)
	assert.Equal(t, "/workspace/pkg/foo.go:6", lh.Entries()[0].Display, "zero RootURI should produce absolute path")
	assert.Equal(t, "/workspace/pkg/bar.go:11", lh.Entries()[1].Display, "zero RootURI should produce absolute path")
}

func TestReferencesHandler(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	tests := []struct {
		name         string
		locs         []semanticapi.Location
		nilResource  bool
		wantErr      bool
		wantFloat    bool
		wantNavigate bool
		wantEntries  int
	}{
		{
			name: "single reference navigates directly",
			locs: []semanticapi.Location{
				{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 10, Character: 5},
						End:   semanticapi.Position{Line: 10, Character: 8},
					},
				},
			},
			wantNavigate: true,
		},
		{
			name: "multiple references",
			locs: []semanticapi.Location{
				{
					URI: "file:///project/a.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 10, Character: 5},
						End:   semanticapi.Position{Line: 10, Character: 8},
					},
				},
				{
					URI: "file:///project/b.go",
					Range: semanticapi.Range{
						Start: semanticapi.Position{Line: 20, Character: 0},
						End:   semanticapi.Position{Line: 20, Character: 3},
					},
				},
			},
			wantFloat:   true,
			wantEntries: 2,
		},
		{name: "zero references"},
		{name: "nil resource", nilResource: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := &mockLSP{
				referencesFn: func(_ context.Context, _ semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
					return tt.locs, nil
				},
			}
			done := make(chan struct{}, 1)
			signal := func() {
				select {
				case done <- struct{}{}:
				default:
				}
			}
			var navigated bool
			editor := &mockEditor{
				editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
					return &mockHandler{uri: u}, nil
				},
				setCursorFn: func(_ textapi.Handler, _ term.Coordinates) error {
					navigated = true
					signal()
					return nil
				},
			}
			var fh browserapi.Floating
			wm := &mockWindowManager{
				floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
					fh = h
					signal()
					return nil, nil
				},
			}
			notify := &recordingNotifications{}
			h := ReferencesHandler(
				lsp, editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
				rootURI, syncTick, nil, DefaultReferencesConfig(), nil,
			)

			uri, _ := workspaceapi.ParseURI("file:///project/a.go")
			cmd := textapi.Command{Name: "references", URI: uri}
			if !tt.nilResource {
				cmd.Resource = &mockHandler{uri: uri}
			}
			cmd.Cursor.Content = term.Coordinates{X: 5, Y: 10}

			err := h.HandleCommand(context.Background(), cmd)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantFloat || tt.wantNavigate {
				waitFloating(t, done)
			} else {
				// Zero references complete silently; synchronise on the
				// cursor-path progress notification finishing.
				require.Eventually(t, func() bool {
					_, updates := notify.snapshot()
					for _, u := range updates {
						if u.step == 1 && u.total == 1 {
							return true
						}
					}
					return false
				}, 5*time.Second, 5*time.Millisecond,
					"cursor path must complete its progress notification")
			}
			assert.Equal(t, tt.wantFloat, fh != nil)
			assert.Equal(t, tt.wantNavigate, navigated)
			if tt.wantEntries > 0 {
				lh := fh.(*locationpicker.Picker)
				assert.Equal(t, tt.wantEntries, len(lh.Entries()))
			}
		})
	}
}
