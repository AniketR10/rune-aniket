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
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/workspace"
)

// langPkgManager returns the tree-sitter fixtures for the requested
// language only, so a multi-language workspace loads the correct parser
// per file extension.
type langPkgManager struct {
	wd string
}

func (m langPkgManager) LibDir(_ context.Context, langID string) (iterator.Iterator[string], error) {
	dir := filepath.Join(m.wd, langID)
	if _, err := os.Stat(dir); err != nil {
		return iterator.FromSlice([]string(nil)), nil
	}
	return iterator.FromSlice([]string{
		filepath.Join(dir, "tree-sitter.so"),
		filepath.Join(dir, "locals.scm"),
		filepath.Join(dir, "highlights.scm"),
		filepath.Join(dir, "indents.scm"),
		filepath.Join(dir, "folds.scm"),
	}), nil
}

const goGeometryDef = `package geometry

func Area() int {
	return 0
}
`

const goGeometryUse = `package main

import "example/geometry"

func main() {
	_ = geometry.Area()
}
`

const pyShapesDef = `def perimeter():
    return 0
`

const pyShapesUse = `import shapes

shapes.perimeter()
`

func newResolveParser(t *testing.T, files map[string]string) syntaxapi.Parser {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	uri, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	for name, content := range files {
		createFile(t, scheme, name, content)
	}
	return syntax.NewParser(scheme, langPkgManager{wd: wd}, uri)
}

func resolveAll(t *testing.T, parser syntaxapi.Parser, name string) ([]syntaxapi.Match, error) {
	t.Helper()
	it, err := parser.ResolveSymbol(context.Background(), name, nil)
	require.NoError(t, err)
	return iterator.ToSlice(context.Background(), it)
}

func TestParserResolveSymbolLanguageDetection(t *testing.T) {
	t.Run("go only resolves go and skips python", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"geometry/area.go": goGeometryDef,
			"main.go":          goGeometryUse,
		})

		matches, err := resolveAll(t, parser, "geometry.Area")
		require.NoError(t, err)
		assert.NotEmpty(t, matches, "Go symbol should resolve in a Go-only workspace")

		_, err = resolveAll(t, parser, "shapes.perimeter")
		require.Error(t, err, "Python symbol must not resolve in a Go-only workspace")
	})

	t.Run("python only resolves python and skips go", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"shapes.py": pyShapesDef,
			"main.py":   pyShapesUse,
		})

		matches, err := resolveAll(t, parser, "shapes.perimeter")
		require.NoError(t, err)
		assert.NotEmpty(t, matches, "Python symbol should resolve in a Python-only workspace")

		_, err = resolveAll(t, parser, "geometry.Area")
		require.Error(t, err, "Go symbol must not resolve in a Python-only workspace")
	})

	t.Run("mixed workspace resolves both languages", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"geometry/area.go": goGeometryDef,
			"main.go":          goGeometryUse,
			"shapes.py":        pyShapesDef,
			"app.py":           pyShapesUse,
		})

		goMatches, err := resolveAll(t, parser, "geometry.Area")
		require.NoError(t, err)
		assert.NotEmpty(t, goMatches)

		pyMatches, err := resolveAll(t, parser, "shapes.perimeter")
		require.NoError(t, err)
		assert.NotEmpty(t, pyMatches)
	})

	t.Run("name without dot returns ErrNoDot", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{"main.go": goGeometryUse})
		_, err := parser.ResolveSymbol(context.Background(), "NoDot", nil)
		assert.ErrorIs(t, err, syntaxapi.ErrNoDot)
	})

	t.Run("absent language yields not found", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{"main.go": goGeometryUse})
		_, err := resolveAll(t, parser, "shapes.perimeter")
		require.Error(t, err)
		assert.False(t, errors.Is(err, syntaxapi.ErrNoDot))
	})
}

func listReferencedAll(t *testing.T, parser syntaxapi.Parser) map[string]bool {
	t.Helper()
	it, err := parser.ListReferencedSymbols(context.Background())
	require.NoError(t, err)
	names, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.NoError(t, it.Err())
	got := make(map[string]bool, len(names))
	for _, n := range names {
		got[n] = true
	}
	return got
}

func TestParserListReferencedSymbolsLanguageDetection(t *testing.T) {
	t.Run("go only lists go refs and not python", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"geometry/area.go": goGeometryDef,
			"main.go":          goGeometryUse,
		})

		got := listReferencedAll(t, parser)
		assert.True(t, got["geometry.Area"], "Go reference should be listed")
		assert.False(t, got["shapes.perimeter"], "Python ref must not appear in a Go-only workspace")
	})

	t.Run("python only lists python refs and not go", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"shapes.py": pyShapesDef,
			"main.py":   pyShapesUse,
		})

		got := listReferencedAll(t, parser)
		assert.True(t, got["shapes.perimeter"], "Python reference should be listed")
		assert.False(t, got["geometry.Area"], "Go ref must not appear in a Python-only workspace")
	})

	t.Run("mixed workspace lists both languages", func(t *testing.T) {
		parser := newResolveParser(t, map[string]string{
			"geometry/area.go": goGeometryDef,
			"main.go":          goGeometryUse,
			"shapes.py":        pyShapesDef,
			"app.py":           pyShapesUse,
		})

		got := listReferencedAll(t, parser)
		assert.True(t, got["geometry.Area"], "Go reference should be listed")
		assert.True(t, got["shapes.perimeter"], "Python reference should be listed")
	})
}
