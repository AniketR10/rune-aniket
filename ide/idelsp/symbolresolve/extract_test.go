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

package symbolresolve_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
)

func TestSpecForFile(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		path string
		want *symbolresolve.Spec
	}{
		{"a.go", symbolresolve.Go},
		{"pkg/deep/a.go", symbolresolve.Go},
		{"a.py", symbolresolve.Python},
		{"a.pyi", symbolresolve.Python},
		{"a.rs", symbolresolve.Rust},
		{"a.txt", nil},
		{"go", nil},
	} {
		assert.Equal(t, tc.want, symbolresolve.SpecForFile(tc.path), tc.path)
	}
}

// occurrence keys one extracted symbol by qualified name and kind.
type occurrence struct {
	name string
	kind symbolresolve.SymbolKind
}

func extractFileT(
	t *testing.T, env *resolveEnv, spec *symbolresolve.Spec, rel string,
) (symbolresolve.FileExtraction, map[occurrence]int) {
	t.Helper()
	uri, err := workspaceapi.ParseURI(env.fileURI(rel))
	require.NoError(t, err)
	q := env.parser.NewQuerySession()
	defer func() { require.NoError(t, q.Close()) }()
	ext, err := symbolresolve.ExtractFile(
		context.Background(), q, spec, env.qc, uri)
	require.NoError(t, err)
	occs := make(map[occurrence]int, len(ext.Symbols))
	for _, s := range ext.Symbols {
		occs[occurrence{s.Name, s.Kind}]++
	}
	return ext, occs
}

func TestExtractFileGo(t *testing.T) {
	t.Parallel()

	env := setupResolveEnv(t)

	t.Run("refs and imports from main.go", func(t *testing.T) {
		ext, occs := extractFileT(t, env, symbolresolve.Go, "main.go")

		for _, want := range []occurrence{
			{"mylib.MyType", symbolresolve.SymbolRef},
			{"iter.Iterator", symbolresolve.SymbolRef},
			{"nested.Repeated", symbolresolve.SymbolRef},
			{"iter.Reduce", symbolresolve.SymbolRef},
			{"mylib.MyFunc", symbolresolve.SymbolRef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
		// m.String(): the selector query requires the operand to be an
		// imported alias, so local variables never produce references.
		assert.Zero(t, occs[occurrence{"m.String", symbolresolve.SymbolRef}])
		// main and useTypes are unexported, so main.go has no defs.
		for o := range occs {
			assert.Equalf(t, symbolresolve.SymbolRef, o.kind,
				"unexpected non-ref %+v", o)
		}

		assert.Equal(t, map[string]string{
			"context": "context",
			"fmt":     "fmt",
			"iter":    "example.com/resolvetest/iter",
			"mylib":   "example.com/resolvetest/mylib",
			"nested":  "example.com/resolvetest/nested/nested",
		}, ext.Imports)
	})

	t.Run("defs and methods from mylib.go", func(t *testing.T) {
		ext, occs := extractFileT(t, env, symbolresolve.Go, "mylib/mylib.go")

		assert.Equal(t, map[occurrence]int{
			{"mylib.MyType", symbolresolve.SymbolDef}:              1,
			{"mylib.MyFunc", symbolresolve.SymbolDef}:              1,
			{"mylib.New", symbolresolve.SymbolDef}:                 1,
			{"mylib.MyType.String", symbolresolve.SymbolMethodDef}: 1,
			{"mylib.MyType.Set", symbolresolve.SymbolMethodDef}:    1,
		}, occs, "unexportedHelper must be excluded")
		assert.Empty(t, ext.Imports)
	})

	t.Run("blank import contributes no symbols", func(t *testing.T) {
		ext, occs := extractFileT(t, env, symbolresolve.Go, "blankimport.go")
		assert.Empty(t, occs)
		assert.NotContains(t, ext.Imports, "_")
	})

	t.Run("dot-imported references are invisible", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Go, "dotimport.go")
		assert.Zero(t, occs[occurrence{"mylib.MyFunc", symbolresolve.SymbolRef}])
		assert.NotZero(t, occs[occurrence{"main.DotUser", symbolresolve.SymbolDef}])
	})
}

func TestExtractFilePython(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	t.Run("attribute refs from main.py", func(t *testing.T) {
		ext, occs := extractFileT(t, env, symbolresolve.Python, "main.py")

		for _, want := range []occurrence{
			{"geometry.area", symbolresolve.SymbolRef},
			{"geometry.Shape", symbolresolve.SymbolRef},
			{"requests.get", symbolresolve.SymbolRef},
			{"service.Service", symbolresolve.SymbolRef},
			{"text.slugify", symbolresolve.SymbolRef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
		// The module qualifier is the file stem.
		assert.NotZero(t, occs[occurrence{"main.main", symbolresolve.SymbolDef}])
		// Python has no import queries.
		assert.Empty(t, ext.Imports)
	})

	t.Run("defs and methods from geometry.py", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Python, "geometry.py")

		// Python has no visibility filter: underscore names are kept.
		for _, want := range []occurrence{
			{"geometry.area", symbolresolve.SymbolDef},
			{"geometry.perimeter", symbolresolve.SymbolDef},
			{"geometry.Shape", symbolresolve.SymbolDef},
			{"geometry._Internal", symbolresolve.SymbolDef},
			{"geometry._make_internal", symbolresolve.SymbolDef},
			{"geometry.Shape.__init__", symbolresolve.SymbolMethodDef},
			{"geometry.Shape.area", symbolresolve.SymbolMethodDef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
	})
}

func TestExtractFilePythonPackage(t *testing.T) {
	t.Parallel()

	env := setupPythonEnvAt(t, "testdata_py_pkg")

	t.Run("defs are emitted under every module path suffix", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Python, "src/mypkg/_impl.py")

		for _, want := range []occurrence{
			{"mypkg._impl.Widget", symbolresolve.SymbolDef},
			{"_impl.Widget", symbolresolve.SymbolDef},
			{"mypkg._impl.make_widget", symbolresolve.SymbolDef},
			{"_impl.make_widget", symbolresolve.SymbolDef},
			{"mypkg._impl.Widget.render", symbolresolve.SymbolMethodDef},
			{"_impl.Widget.render", symbolresolve.SymbolMethodDef},
			// The module-less Type.method suffix is also indexed so a
			// bare "Widget.render" lookup resolves.
			{"Widget.render", symbolresolve.SymbolMethodDef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
		// The bare package name is not a suffix of mypkg._impl.
		assert.Zero(t, occs[occurrence{"mypkg.Widget", symbolresolve.SymbolDef}])
	})

	t.Run("package __init__ re-exports are defs", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Python, "src/mypkg/__init__.py")

		for _, want := range []occurrence{
			{"mypkg.Widget", symbolresolve.SymbolDef},
			{"mypkg.make_widget", symbolresolve.SymbolDef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
	})

	t.Run("nested package __init__ re-exports carry suffixes", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Python, "src/mypkg/sub/__init__.py")

		for _, want := range []occurrence{
			{"mypkg.sub.slug", symbolresolve.SymbolDef},
			{"sub.slug", symbolresolve.SymbolDef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
	})

	t.Run("imports outside re-export files are not defs", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Python, "main.py")

		assert.Zero(t, occs[occurrence{"main.make_widget", symbolresolve.SymbolDef}],
			"a plain module's imports must not become definitions")
		assert.NotZero(t, occs[occurrence{"main.main", symbolresolve.SymbolDef}])
	})
}

func TestExtractFileRust(t *testing.T) {
	t.Parallel()

	env := setupRustEnv(t)

	t.Run("scoped refs from main.rs", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Rust, "main.rs")

		for _, want := range []occurrence{
			{"geometry.area", symbolresolve.SymbolRef},
			{"geometry.Shape", symbolresolve.SymbolRef},
			{"requests.get", symbolresolve.SymbolRef},
			{"service.Service", symbolresolve.SymbolRef},
			{"text.slugify", symbolresolve.SymbolRef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
	})

	t.Run("defs and methods from geometry.rs", func(t *testing.T) {
		_, occs := extractFileT(t, env, symbolresolve.Rust, "geometry.rs")

		// Rust cannot observe pub visibility from tree-sitter captures,
		// so private items are kept. The module qualifier is the file
		// stem.
		for _, want := range []occurrence{
			{"geometry.area", symbolresolve.SymbolDef},
			{"geometry.perimeter", symbolresolve.SymbolDef},
			{"geometry.Shape", symbolresolve.SymbolDef},
			{"geometry.Internal", symbolresolve.SymbolDef},
			{"geometry.Shape.new", symbolresolve.SymbolMethodDef},
		} {
			assert.NotZerof(t, occs[want], "missing %+v", want)
		}
	})
}
