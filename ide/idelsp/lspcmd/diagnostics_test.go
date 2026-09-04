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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/handler/locationpicker"
)

var _ textapi.CommandHandler = (*diagnosticsHandler)(nil)

// mockDiagnosticsSource is a minimal DiagnosticsSource for tests.
type mockDiagnosticsSource struct {
	diags map[string][]semanticapi.Diagnostic
}

func (m *mockDiagnosticsSource) Diagnostics() map[string][]semanticapi.Diagnostic {
	return m.diags
}

func TestDiagnosticsHandler(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)

	tests := []struct {
		name        string
		source      DiagnosticsSource
		wantFloat   bool
		wantNotify  bool
		wantEntries []string
	}{
		{
			name:       "nil source notifies",
			source:     nil,
			wantNotify: true,
		},
		{
			name: "empty diagnostics notifies",
			source: &mockDiagnosticsSource{
				diags: map[string][]semanticapi.Diagnostic{},
			},
			wantNotify: true,
		},
		{
			name: "single diagnostic shows picker",
			source: &mockDiagnosticsSource{
				diags: map[string][]semanticapi.Diagnostic{
					"file:///project/a.go": {
						{
							Range: semanticapi.Range{
								Start: semanticapi.Position{Line: 4, Character: 0},
								End:   semanticapi.Position{Line: 4, Character: 5},
							},
							Severity: semanticapi.DiagnosticSeverityError,
							Message:  "boom",
						},
					},
				},
			},
			wantFloat: true,
			wantEntries: []string{
				"a.go:5 [E] boom",
			},
		},
		{
			name: "multiple diagnostics sorted by severity",
			source: &mockDiagnosticsSource{
				diags: map[string][]semanticapi.Diagnostic{
					"file:///project/a.go": {
						{
							Range: semanticapi.Range{
								Start: semanticapi.Position{Line: 0, Character: 0},
								End:   semanticapi.Position{Line: 0, Character: 1},
							},
							Severity: semanticapi.DiagnosticSeverityHint,
							Message:  "hint a",
						},
						{
							Range: semanticapi.Range{
								Start: semanticapi.Position{Line: 9, Character: 0},
								End:   semanticapi.Position{Line: 9, Character: 1},
							},
							Severity: semanticapi.DiagnosticSeverityError,
							Message:  "err a",
						},
					},
					"file:///project/b.go": {
						{
							Range: semanticapi.Range{
								Start: semanticapi.Position{Line: 2, Character: 0},
								End:   semanticapi.Position{Line: 2, Character: 1},
							},
							Severity: semanticapi.DiagnosticSeverityWarning,
							Message:  "warn b",
						},
					},
				},
			},
			wantFloat: true,
			wantEntries: []string{
				"a.go:10 [E] err a",
				"b.go:3 [W] warn b",
				"a.go:1 [H] hint a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := &mockEditor{
				editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
					return &mockHandler{uri: u}, nil
				},
			}
			var fh browserapi.Floating
			wm := &mockWindowManager{
				floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
					fh = h
					return nil, nil
				},
			}
			var notified bool
			notify := &mockNotifications{
				notifyFn: func(_ browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
					notified = true
					return "", nil
				},
			}
			h := DiagnosticsHandler(
				editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
				rootURI, syncTick, nil, tt.source, DefaultDiagnosticsConfig(), nil,
			)
			err := h.HandleCommand(context.Background(), textapi.Command{Name: "diagnostics"})
			require.NoError(t, err)
			assert.Equal(t, tt.wantFloat, fh != nil, "want floating window")
			assert.Equal(t, tt.wantNotify, notified, "want notification")
			if tt.wantFloat {
				picker := fh.(*locationpicker.Picker)
				got := make([]string, len(picker.Entries()))
				for i, e := range picker.Entries() {
					got[i] = e.Display
				}
				assert.Equal(t, tt.wantEntries, got)
			}
		})
	}
}

func TestDiagnosticsHandler_RelativePaths(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)

	source := &mockDiagnosticsSource{
		diags: map[string][]semanticapi.Diagnostic{
			"file:///workspace/src/a.go": {{
				Range: semanticapi.Range{
					Start: semanticapi.Position{Line: 1, Character: 0},
					End:   semanticapi.Position{Line: 1, Character: 1},
				},
				Severity: semanticapi.DiagnosticSeverityWarning,
				Message:  "w",
			}},
			"file:///other/x.go": {{
				Range: semanticapi.Range{
					Start: semanticapi.Position{Line: 0, Character: 0},
					End:   semanticapi.Position{Line: 0, Character: 1},
				},
				Severity: semanticapi.DiagnosticSeverityError,
				Message:  "e",
			}},
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	var fh browserapi.Floating
	wm := &mockWindowManager{
		floatingFn: func(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
			fh = h
			return nil, nil
		},
	}
	h := DiagnosticsHandler(
		editor, wm, &mockResourceOpener{}, &mockNotifications{}, &mockFileSystem{},
		rootURI, syncTick, nil, source, DefaultDiagnosticsConfig(), nil,
	)
	err = h.HandleCommand(context.Background(), textapi.Command{Name: "diagnostics"})
	require.NoError(t, err)
	require.NotNil(t, fh)

	picker := fh.(*locationpicker.Picker)
	require.Len(t, picker.Entries(), 2)
	assert.Equal(t, "/other/x.go:1 [E] e", picker.Entries()[0].Display)
	assert.Equal(t, "src/a.go:2 [W] w", picker.Entries()[1].Display)
}
