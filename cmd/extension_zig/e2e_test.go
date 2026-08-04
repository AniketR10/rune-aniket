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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// TestE2E_ZlsBringUp drives the extension's zlsInitializeParams against
// a real zls serving the testdata project: hover answers about the
// fixture symbol and definition resolves across files, proving the
// initialization options (command, zig_exe_path) and the workspace
// folder registration reach the server intact.
func TestE2E_ZlsBringUp(t *testing.T) {
	t.Parallel()
	zlsBin := findZlsBin(t)
	zigBin := findZigBin(t)

	// Build-on-save is covered by TestE2E_BuildOnSaveDiagnostics; disable
	// it here so this instance never spawns a zig build it does not need.
	disabled := false
	env := initZls(t, zlsBin, zigBin, []string{"src/main.zig", "src/lib.zig"},
		buildOnSaveOptions{Enable: &disabled})
	ctx := context.Background()
	mainURI := env.fileURIs["src/main.zig"]
	pos := locateInFile(t, filepath.Join(env.dir, "src", "main.zig"), "lib.add(", "add(")

	hover, err := env.mgr.Hover(ctx, semanticapi.HoverParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Position:     pos,
	})
	require.NoError(t, err)
	require.NotNil(t, hover, "hover over lib.add must answer")
	assert.Contains(t, hover.Contents.Value, "add",
		"hover should describe the add function")

	res, err := env.mgr.Definition(ctx, semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Position:     pos,
	})
	require.NoError(t, err)
	var target string
	switch {
	case res.Location != nil:
		target = res.Location.URI
	case len(res.Locations) > 0:
		target = res.Locations[0].URI
	case len(res.LocationLinks) > 0:
		target = res.LocationLinks[0].TargetURI
	}
	assert.Contains(t, target, "lib.zig",
		"definition of lib.add must resolve into lib.zig")

	// workspace/symbol only searches workspaces registered from the
	// initialize params' workspaceFolders; zls ignores rootUri alone and
	// would answer [] forever if the folder were dropped. zls indexes the
	// workspace asynchronously after initialize, so poll.
	require.Eventually(t, func() bool {
		syms, err := env.mgr.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{
			Query: "add",
		})
		if err != nil {
			return false
		}
		for _, s := range syms {
			if s.Name == "add" {
				return true
			}
		}
		return false
	}, 30*time.Second, 300*time.Millisecond,
		"workspace/symbol must find the fixture symbol, proving the "+
			"workspaceFolders registration reached zls")

	// zls only pushes ast-check diagnostics when the client advertises
	// textDocument.publishDiagnostics (its build-on-save publish path is
	// not gated, so that test cannot cover this). Build-on-save is
	// disabled here, making the capability-gated path the only possible
	// source of the diagnostic below.
	orig, err := os.ReadFile(filepath.Join(env.dir, "src", "main.zig"))
	require.NoError(t, err)
	require.NoError(t, env.mgr.DidChange(ctx, semanticapi.DidChangeTextDocumentParams{
		TextDocument: semanticapi.VersionedTextDocumentIdentifier{
			URI: mainURI, Version: 2,
		},
		ContentChanges: []semanticapi.TextDocumentContentChangeEvent{
			{Text: string(orig) + "\nconst broken = ;\n"},
		},
	}))
	require.Eventually(t, func() bool {
		diags, ok := env.cb.diagnosticsFor(mainURI)
		if !ok {
			return false
		}
		for _, d := range diags {
			if d.Severity == semanticapi.DiagnosticSeverityError {
				return true
			}
		}
		return false
	}, 30*time.Second, 300*time.Millisecond,
		"expected zls to push an ast-check diagnostic, proving the "+
			"publishDiagnostics capability reached zls")
}

// TestE2E_BuildOnSaveDiagnostics proves the build-on-save pipeline end
// to end: the fixture's build.zig declares a "check" step (so zls
// auto-enables build-on-save without any explicit config), and a saved
// type error at a cross-file call site — invisible to ast-check, which
// has no cross-file semantics — comes back as a pushed diagnostic
// produced by the real `zig build` check run.
func TestE2E_BuildOnSaveDiagnostics(t *testing.T) {
	t.Parallel()
	zlsBin := findZlsBin(t)
	zigBin := findZigBin(t)

	env := initZls(t, zlsBin, zigBin, []string{"src/main.zig"}, buildOnSaveOptions{})
	ctx := context.Background()
	mainURI := env.fileURIs["src/main.zig"]
	mainPath := filepath.Join(env.dir, "src", "main.zig")

	orig, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	broken := strings.Replace(string(orig), `lib.add(2, 3)`, `lib.add(2, "three")`, 1)
	require.NotEqual(t, string(orig), broken, "fixture must contain the call to break")

	// Build-on-save compiles from disk, so persist the broken content
	// before mirroring it into the server's overlay and saving.
	require.NoError(t, os.WriteFile(mainPath, []byte(broken), 0o644))
	require.NoError(t, env.mgr.DidChange(ctx, semanticapi.DidChangeTextDocumentParams{
		TextDocument: semanticapi.VersionedTextDocumentIdentifier{
			URI: mainURI, Version: 2,
		},
		ContentChanges: []semanticapi.TextDocumentContentChangeEvent{
			{Text: broken},
		},
	}))
	require.NoError(t, env.mgr.DidSave(ctx, semanticapi.DidSaveTextDocumentParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: mainURI},
		Text:         broken,
	}))

	// The first build-on-save run compiles the build runner, so give it
	// a generous window.
	require.Eventually(t, func() bool {
		diags, ok := env.cb.diagnosticsFor(mainURI)
		if !ok {
			return false
		}
		for _, d := range diags {
			if d.Severity == semanticapi.DiagnosticSeverityError &&
				strings.Contains(d.Message, "expected type") {
				return true
			}
		}
		return false
	}, 120*time.Second, 500*time.Millisecond,
		"expected zls build-on-save to push the cross-file type error")
}
