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

package syntaxtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/workspace"
)

// TestResolveRustModuleE2E drives the public ResolveSymbol API end to end
// against a real on-disk Rust crate served by a real FileScheme and parsed
// by a real tree-sitter parser, so the whole pipeline (language detection,
// reference queries, definition queries, FileScheme I/O) is exercised
// exactly as it runs in production.
func TestResolveRustModuleE2E(t *testing.T) {
	root := rustModuleWorkspace(t)
	parser := newFileSchemeParser(t, root)

	src := func(rel string) string {
		return "file://" + filepath.Join(root, "src", rel)
	}
	mainRS := src("main.rs")
	geometryRS := src("geometry.rs")
	shapesRS := src("shapes.rs")

	tests := []struct {
		name string
		// symbol uses a dot separator; ResolveSymbol splits on the first
		// dot while Rust source uses `module::Symbol`.
		symbol          string
		wantURIs        []string
		wantErr         bool
		wantErrNotNoDot bool
	}{
		{
			// geometry::area is referenced from main.rs (both as a path
			// call and via the `use` alias); the reference phase surfaces
			// the call site rather than the definition.
			name:     "qualified function reference resolves to call site",
			symbol:   "geometry.area",
			wantURIs: []string{mainRS},
		},
		{
			// circumference is defined in geometry.rs and referenced from
			// shapes.rs as geometry::circumference.
			name:     "qualified reference across modules resolves to caller",
			symbol:   "geometry.circumference",
			wantURIs: []string{shapesRS},
		},
		{
			// Point is never referenced qualified, so the definitions
			// phase surfaces its declaration via the module file stem.
			name:     "definition-only type resolves via module stem",
			symbol:   "geometry.Point",
			wantURIs: []string{geometryRS},
		},
		{
			// Circle::new is referenced from main.rs via shapes::Circle.
			name:     "scoped type reference resolves to call site",
			symbol:   "shapes.Circle",
			wantURIs: []string{mainRS},
		},
		{
			// Rectangle is declared but never referenced; definitions
			// phase surfaces shapes.rs.
			name:     "unreferenced type resolves to its module file",
			symbol:   "shapes.Rectangle",
			wantURIs: []string{shapesRS},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := resolveAll(t, parser, tt.symbol)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrNotNoDot {
					assert.False(t, errors.Is(err, syntaxapi.ErrNoDot))
				}
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, matches)
			assert.ElementsMatchf(t, tt.wantURIs, matchURIs(matches),
				"resolved URIs mismatch; matches=%+v", matches)
		})
	}
}

func TestResolveRustModuleNotFoundE2E(t *testing.T) {
	root := rustModuleWorkspace(t)
	parser := newFileSchemeParser(t, root)

	t.Run("missing symbol yields not found", func(t *testing.T) {
		_, err := resolveAll(t, parser, "geometry.DoesNotExist")
		require.Error(t, err)
		assert.False(t, errors.Is(err, syntaxapi.ErrNoDot))
	})

	t.Run("name without dot returns ErrNoDot", func(t *testing.T) {
		_, err := parser.ResolveSymbol(context.Background(), "NoDot", nil)
		assert.ErrorIs(t, err, syntaxapi.ErrNoDot)
	})
}

// rustModuleWorkspace copies the on-disk Rust crate fixture into a fresh
// temp dir so the FileScheme indexes a real, writable workspace.
func rustModuleWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, "testdata_rust_module", root)
	return root
}

// newFileSchemeParser builds a parser backed by a real FileScheme rooted at
// root and the on-disk Rust tree-sitter assets under ./rust.
func newFileSchemeParser(t *testing.T, root string) syntaxapi.Parser {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	uri, err := workspaceapi.ParseURI("file://" + root)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	return syntax.NewParser(scheme, langPkgManager{wd: wd}, uri)
}

func matchURIs(matches []syntaxapi.Match) []string {
	uris := make([]string, len(matches))
	for i, m := range matches {
		uris[i] = m.URI
	}
	return uris
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			require.NoError(t, os.MkdirAll(dstPath, 0o755))
			copyTree(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(dstPath, data, 0o644))
	}
}
