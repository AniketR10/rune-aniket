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
	"unstable.build/go-tui/handler/locationpicker"
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
