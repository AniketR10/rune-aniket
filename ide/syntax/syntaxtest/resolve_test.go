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
	"sync"
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

type Point struct {
	X int
	Y int
}

func (p Point) Norm() int {
	return p.X*p.X + p.Y*p.Y
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

// recordingPkgManager wraps a PkgManager and records every language for
// which a grammar (LibDir) was requested, so tests can assert which
// languages the workspace walk attempted to load parsers for.
type recordingPkgManager struct {
	inner syntax.PkgManager
	mu    sync.Mutex
	seen  map[string]bool
}

func (m *recordingPkgManager) LibDir(
	ctx context.Context, langID string,
) (iterator.Iterator[string], error) {
	m.mu.Lock()
	if m.seen == nil {
		m.seen = make(map[string]bool)
	}
	m.seen[langID] = true
	m.mu.Unlock()
	return m.inner.LibDir(ctx, langID)
}

func (m *recordingPkgManager) requested(langID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seen[langID]
}

func newRecordingResolveParser(
	t *testing.T, files map[string]string,
) (syntaxapi.Parser, *recordingPkgManager) {
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
	pkg := &recordingPkgManager{inner: langPkgManager{wd: wd}}
	return syntax.NewParser(scheme, pkg, uri), pkg
}

func resolveAll(t *testing.T, parser syntaxapi.Parser, name string) ([]syntaxapi.Match, error) {
	t.Helper()
	it, err := parser.ResolveSymbol(context.Background(), name, nil)
	require.NoError(t, err)
	return iterator.ToSlice(context.Background(), it)
}

type recordingProgress struct {
	mu     sync.Mutex
	events []progressEvent
}

type progressEvent struct {
	step, total int64
}

func (r *recordingProgress) Report(_ string, _ int, step, total int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, progressEvent{step: step, total: total})
}

// TestParserResolveSymbolProgressMonotonic resolves an unresolvable name in a
// mixed-language workspace so symbolresolve.Resolve runs every detected spec.
// The aggregated progress forwarded to the caller must be monotonic — step and
// total never decrease and step never exceeds total — even though each spec
// restarts its own per-phase reporting.
func TestParserResolveSymbolProgressMonotonic(t *testing.T) {
	parser := newResolveParser(t, map[string]string{
		"geometry/area.go": goGeometryDef,
		"main.go":          goGeometryUse,
		"shapes.py":        pyShapesDef,
		"app.py":           pyShapesUse,
	})

	rec := &recordingProgress{}
	it, err := parser.ResolveSymbol(
		context.Background(), "missingpkg.DoesNotExistAnywhere", rec,
	)
	require.NoError(t, err)
	_, err = iterator.ToSlice(context.Background(), it)
	require.Error(t, err)

	rec.mu.Lock()
	events := append([]progressEvent(nil), rec.events...)
	rec.mu.Unlock()
	require.GreaterOrEqual(t, len(events), 2,
		"multiple specs should each report progress")

	var prevStep, prevTotal int64
	for i, e := range events {
		assert.GreaterOrEqualf(t, e.step, prevStep, "step non-decreasing at %d: %+v", i, events)
		assert.LessOrEqualf(t, e.step, e.total, "step <= total at %d: %+v", i, events)
		assert.GreaterOrEqualf(t, e.total, prevTotal, "total non-decreasing at %d: %+v", i, events)
		prevStep, prevTotal = e.step, e.total
	}
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

// TestParserResolveSymbolScopesDefinitionWalk reproduces the
// definitions-phase hang: resolving an unresolvable method against a
// non-Go spec (no package clause) must scope the workspace walk to the
// spec's own language and never load grammars for unrelated languages
// present in the workspace.
func TestParserResolveSymbolScopesDefinitionWalk(t *testing.T) {
	parser, pkg := newRecordingResolveParser(t, map[string]string{
		"shapes.py":   pyShapesDef,
		"app.py":      pyShapesUse,
		"config.yaml": "name: value\n",
	})

	// A method on a Python module type resolves to nothing via references,
	// forcing the Python spec's definitions phase to run.
	matches, err := resolveAll(t, parser, "shapes.Shape.perimeter")
	require.Error(t, err, "unresolvable method must not resolve")
	assert.Empty(t, matches)

	assert.True(t, pkg.requested("python"),
		"resolving a python symbol must load the python grammar")
	assert.False(t, pkg.requested("yaml"),
		"definitions phase must not load grammars for unrelated languages")
}

// TestParserResolveSymbolMethod resolves a pkg.Type.Method name through
// the real spec engine and asserts it lands on the method declaration
// without loading grammars for unrelated languages in the workspace.
func TestParserResolveSymbolMethod(t *testing.T) {
	parser, pkg := newRecordingResolveParser(t, map[string]string{
		"geometry/area.go": goGeometryDef,
		"main.go":          goGeometryUse,
		"config.yaml":      "name: value\n",
	})

	matches, err := resolveAll(t, parser, "geometry.Point.Norm")
	require.NoError(t, err)
	require.Len(t, matches, 1)

	m := matches[0]
	assert.Contains(t, m.URI, "geometry/area.go")
	// goGeometryDef declares Norm on line 12 (1-based); the capture
	// lands on the method name, which begins after "func (p Point) ".
	assert.Equal(t, 11, m.Pos.Y, "method name should be on the Norm line")
	assert.Equal(t, len("func (p Point) "), m.Pos.X)

	assert.True(t, pkg.requested("go"),
		"resolving a go method must load the go grammar")
	assert.False(t, pkg.requested("yaml"),
		"method definitions phase must not load grammars for unrelated languages")
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
		assert.True(t, got["geometry.Point.Norm"],
			"Go method definition should be listed")
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
		assert.True(t, got["geometry.Point.Norm"],
			"Go method definition should be listed")
		assert.True(t, got["shapes.perimeter"], "Python reference should be listed")
	})
}
