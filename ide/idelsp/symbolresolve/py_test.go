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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/workspace"
)

func TestResolvePythonE2E(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	mainURI := env.fileURI("main.py")
	geometryURI := env.fileURI("geometry.py")
	requestsURI := env.fileURI("requests.py")
	appInitURI := env.fileURI("app/__init__.py")
	serviceURI := env.fileURI("app/service.py")
	modelsURI := env.fileURI("app/models.py")
	textURI := env.fileURI("utils/text.py")
	utilsConfigURI := env.fileURI("utils/config.py")
	appConfigURI := env.fileURI("app/config.py")
	arithmeticURI := env.fileURI("calc/arithmetic.py")

	tests := []struct {
		name string
		// symbol is the package-qualified name passed to Resolve.
		symbol string
		// wantURIs, when set, must match the resolved URIs exactly.
		wantURIs []string
		// wantURICount, when set, asserts the number of matches.
		wantURICount    int
		wantErrIs       error
		wantErrContains string
	}{
		{
			// geometry.area is referenced from main.py and app/models.py.
			// The reference (attribute) phase surfaces both call sites
			// and the definition file geometry.py is not added because
			// phase 1 already found matches.
			name:     "qualified reference surfaces all call sites",
			symbol:   "geometry.area",
			wantURIs: []string{mainURI, modelsURI},
		},
		{
			// A single qualified reference resolves to the one call
			// site; the definition geometry.py is not surfaced.
			name:     "single reference resolves to its call site",
			symbol:   "geometry.Shape",
			wantURIs: []string{mainURI},
		},
		{
			// perimeter is defined in geometry.py but never referenced
			// as geometry.perimeter, so the definitions phase surfaces
			// the declaration using the module file stem.
			name:     "definition-only function resolves via module stem",
			symbol:   "geometry.perimeter",
			wantURIs: []string{geometryURI},
		},
		{
			// Python has no enforced visibility: a leading-underscore
			// class must still resolve through the definitions phase.
			name:     "underscore class is still resolvable",
			symbol:   "geometry._Internal",
			wantURIs: []string{geometryURI},
		},
		{
			name:     "underscore function is still resolvable",
			symbol:   "geometry._make_internal",
			wantURIs: []string{geometryURI},
		},
		{
			// A dependency-style module ("requests") referenced from
			// main.py resolves to the call site.
			name:     "dependency module reference resolves to call site",
			symbol:   "requests.get",
			wantURIs: []string{mainURI},
		},
		{
			// The dependency's own class is never referenced
			// qualified, so it resolves to its definition file.
			name:     "dependency definition resolves to its module file",
			symbol:   "requests.Response",
			wantURIs: []string{requestsURI},
		},
		{
			// __init__.py qualifies as the parent directory name, so a
			// class declared there resolves as app.App.
			name:     "package __init__ qualifies as directory name",
			symbol:   "app.App",
			wantURIs: []string{appInitURI},
		},
		{
			name:     "package __init__ function qualifies as directory name",
			symbol:   "app.create",
			wantURIs: []string{appInitURI},
		},
		{
			// service.Service is referenced in main.py via
			// service.Service(); phase 1 finds main.py.
			name:     "class reference resolves to call site",
			symbol:   "service.Service",
			wantURIs: []string{mainURI},
		},
		{
			// service.boot is defined but unreferenced; definitions
			// phase surfaces app/service.py via the file stem.
			name:     "definition-only resolves to nested module file",
			symbol:   "service.boot",
			wantURIs: []string{serviceURI},
		},
		{
			// text.slugify is referenced from both main.py and
			// app/service.py.
			name:     "helper referenced from multiple modules",
			symbol:   "text.slugify",
			wantURIs: []string{mainURI, serviceURI},
		},
		{
			name:     "underscore helper resolves via definitions phase",
			symbol:   "text._private_helper",
			wantURIs: []string{textURI},
		},
		{
			// config.load is defined in BOTH utils/config.py and
			// app/config.py. The two files share the module name
			// "config", so the ambiguous lookup must surface both
			// definition files (Python has no import-path dedup).
			name:         "ambiguous module name surfaces both definition files",
			symbol:       "config.load",
			wantURIs:     []string{utilsConfigURI, appConfigURI},
			wantURICount: 2,
		},
		{
			// config.reset exists only in utils/config.py; the
			// homonymous app/config.py must not be surfaced for a name
			// it does not define.
			name:     "ambiguous module disambiguated by symbol name (utils)",
			symbol:   "config.reset",
			wantURIs: []string{utilsConfigURI},
		},
		{
			name:     "ambiguous module disambiguated by symbol name (app)",
			symbol:   "config.save",
			wantURIs: []string{appConfigURI},
		},
		{
			// arithmetic.helper is homonymous in name only with other
			// "helper" definitions; the distinct module name keeps it
			// isolated to calc/arithmetic.py.
			name:     "homonymous symbol in another module is isolated",
			symbol:   "arithmetic.helper",
			wantURIs: []string{arithmeticURI},
		},
		{
			name:            "missing symbol returns not-found",
			symbol:          "geometry.DoesNotExist",
			wantErrContains: "no symbols found",
		},
		{
			// slugify lives in text, not the utils package __init__;
			// asking for utils.slugify must not synthesise a match.
			name:            "valid symbol in wrong module returns not-found",
			symbol:          "utils.slugify",
			wantErrContains: "no symbols found",
		},
		{
			// helper is defined in arithmetic and service modules but
			// never in text; the resolver must not cross modules.
			name:            "symbol from a different module is not surfaced",
			symbol:          "text.helper",
			wantErrContains: "no symbols found",
		},
		{
			// strings.Cut splits on the first dot, so "geometry.area.foo"
			// searches sym="area.foo", which no definition can match.
			name:            "multi-segment dotted name returns not-found",
			symbol:          "geometry.area.foo",
			wantErrContains: "no symbols found",
		},
		{
			name:      "empty string returns ErrNoDot",
			symbol:    "",
			wantErrIs: symbolresolve.ErrNoDot,
		},
		{
			name:      "name without dot returns ErrNoDot",
			symbol:    "JustAName",
			wantErrIs: symbolresolve.ErrNoDot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &recordingProgress{}
			matches, err := symbolresolve.Resolve(
				context.Background(), env.parser, symbolresolve.Python,
				tt.symbol, rec,
			)

			if tt.wantErrIs != nil {
				require.Error(t, err)
				assert.Truef(t, errors.Is(err, tt.wantErrIs),
					"want errors.Is(%v, %v)", err, tt.wantErrIs)
				return
			}
			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, matches)
			assert.NotEmpty(t, rec.events,
				"Resolve must report progress on success")

			gotURIs := uriStrings(matches)
			if len(tt.wantURIs) > 0 {
				assert.ElementsMatchf(t, tt.wantURIs, gotURIs,
					"resolved URIs mismatch; matches=%+v", matches)
			}
			if tt.wantURICount > 0 {
				assert.Equalf(t, tt.wantURICount, len(gotURIs),
					"match count mismatch; matches=%+v", matches)
			}
		})
	}
}

func TestResolvePythonSelfAttributeQuirk(t *testing.T) {
	t.Parallel()

	// service.run() calls self.helper(); the attribute phase treats
	// "self" as a module alias, so a lookup of self.helper surfaces the
	// file that defines helper. This documents a known limitation: the
	// tree-sitter resolver cannot tell instance attributes from
	// module-qualified references, and pyright is expected to provide
	// precise resolution on follow-up.
	env := setupPythonEnv(t)
	matches, err := symbolresolve.Resolve(
		context.Background(), env.parser, symbolresolve.Python,
		"self.helper", nil,
	)
	require.NoError(t, err)
	assert.Contains(t, uriStrings(matches), env.fileURI("app/service.py"))
}

func TestResolvePythonConcurrent(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	symbols := []string{
		"geometry.area", "geometry.Shape", "geometry.perimeter",
		"requests.get", "app.App", "service.boot", "text.slugify",
		"config.load", "arithmetic.helper", "models.User",
	}

	var wg sync.WaitGroup
	wg.Add(len(symbols))
	for _, s := range symbols {
		go func(symbol string) {
			defer wg.Done()
			matches, err := symbolresolve.Resolve(
				context.Background(), env.parser, symbolresolve.Python,
				symbol, nil,
			)
			assert.NoErrorf(t, err, "Resolve(%q)", symbol)
			assert.NotEmptyf(t, matches, "Resolve(%q)", symbol)
		}(s)
	}
	wg.Wait()
}

func TestSearchDefinitionsPythonE2E(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	packages, err := symbolresolve.FilePackages(
		context.Background(), env.parser, symbolresolve.Python,
	)
	require.NoError(t, err)
	assert.Nil(t, packages, "Python has no package clause")

	ch := make(chan string, 64)
	done := make(chan error, 1)
	go func() {
		defer close(ch)
		done <- symbolresolve.SearchDefinitions(
			context.Background(), env.parser, symbolresolve.Python,
			packages, ch, nil,
		)
	}()

	got := make(map[string]bool)
	for s := range ch {
		got[s] = true
	}
	require.NoError(t, <-done)

	// Every function and class across the workspace is streamed,
	// qualified by its module file stem. __init__.py contributes the
	// directory name, and leading-underscore names are included
	// because Python has no enforced visibility.
	for _, name := range []string{
		"geometry.area", "geometry.perimeter", "geometry.Shape",
		"geometry._Internal", "geometry._make_internal",
		"requests.get", "requests.post", "requests.Response",
		"app.App", "app.create",
		"service.Service", "service.boot", "service.run",
		"service.helper",
		"models.User", "models.Account",
		"text.slugify", "text._private_helper",
		"config.load", "config.reset", "config.save",
		"arithmetic.add", "arithmetic.helper",
	} {
		assert.Truef(t, got[name],
			"expected Python definition %q to be streamed", name)
	}

	// A keep predicate filters streamed names just like the Go path.
	kept := make(chan string, 64)
	keepDone := make(chan error, 1)
	go func() {
		defer close(kept)
		keepDone <- symbolresolve.SearchDefinitions(
			context.Background(), env.parser, symbolresolve.Python,
			packages, kept, func(name string) bool {
				return !strings.HasPrefix(name, "_")
			},
		)
	}()
	keptSet := make(map[string]bool)
	for s := range kept {
		keptSet[s] = true
	}
	require.NoError(t, <-keepDone)
	assert.True(t, keptSet["geometry.area"])
	assert.False(t, keptSet["geometry._Internal"],
		"keep predicate must drop underscore names")
	assert.False(t, keptSet["text._private_helper"])
}

func setupPythonEnv(t *testing.T) *resolveEnv {
	t.Helper()

	root := t.TempDir()
	copyDirT(t, "testdata_py", root)

	rootURI := "file://" + root
	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), uri,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	parser := syntax.NewParser(scheme, pythonPkgManager(t), uri)
	return &resolveEnv{root: root, parser: parser}
}

func pythonPkgManager(t *testing.T) syntax.PkgManager {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	pyDir := filepath.Join(wd, "..", "..", "syntax", "syntaxtest", "python")
	files := []string{
		filepath.Join(pyDir, "tree-sitter.so"),
		filepath.Join(pyDir, "locals.scm"),
		filepath.Join(pyDir, "highlights.scm"),
		filepath.Join(pyDir, "indents.scm"),
		filepath.Join(pyDir, "folds.scm"),
	}
	for _, p := range files {
		_, err := os.Stat(p)
		require.NoErrorf(t, err, "missing tree-sitter fixture %s", p)
	}
	return &stubPkgManager{files: files}
}
