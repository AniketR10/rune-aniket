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
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
)

// listReferences drains ListReferences for the given specs into a set.
func listReferences(
	t *testing.T, parser symbolresolve.Searcher,
	qc symbolresolve.QualifierContext,
	specs iterator.Iterator[symbolresolve.Spec],
) map[string]bool {
	t.Helper()
	ch := make(chan string, 64)
	done := make(chan error, 1)
	go func() {
		defer close(ch)
		done <- symbolresolve.ListReferences(context.Background(), parser, qc, specs, ch)
	}()
	got := make(map[string]bool)
	for s := range ch {
		got[s] = true
	}
	require.NoError(t, <-done)
	return got
}

func TestListReferencesGoE2E(t *testing.T) {
	t.Parallel()

	env := setupResolveEnv(t)

	got := listReferences(t, env.parser, env.qc, specIter(symbolresolve.Go))

	// Qualified references (selectors and qualified types) plus
	// workspace definitions surfaced by SearchDefinitions.
	for _, name := range []string{
		"mylib.MyType", "mylib.MyFunc", "mylib.New",
		"iter.Iterator", "iter.OnlyDefined", "iter.Aggregate",
		"iterator.Iterator", "iterator.Map", "iterator.Filter",
		"manyfiles.Token",
	} {
		assert.Truef(t, got[name], "expected Go reference %q to be listed", name)
	}

	// Unexported workspace definitions must be filtered by the Go
	// export predicate.
	for _, name := range []string{
		"mylib.unexportedHelper", "iter.privateOnlyDefined",
	} {
		assert.Falsef(t, got[name], "unexported %q must not be listed", name)
	}
}

// TestListReferencesGoImportFiltering pins the reference-query filtering rules
// against the real fixtures: dot- and blank-imported packages must contribute
// no qualified references, and a qualified type from a legitimately imported
// package must pass the RequireImport check.
func TestListReferencesGoImportFiltering(t *testing.T) {
	t.Parallel()

	env := setupResolveEnv(t)
	got := listReferences(t, env.parser, env.qc, specIter(symbolresolve.Go))

	// dotimport.go calls MyFunc("dot") through a dot import, so it has no
	// package qualifier and must not surface a mylib.* reference. mylib.MyFunc
	// still appears because main.go calls it qualified, so assert the dot-import
	// file's own bare call did not invent a spurious selector reference.
	assert.False(t, got["mylib.DotUser"], "dot-imported call must not be listed as a reference")
	assert.False(t, got[".MyFunc"], "dot import must not produce an empty qualifier reference")

	// blankimport.go blank-imports blueiter purely for its side effects; no
	// blueiter.* reference may be attributed to it.
	assert.False(t, got["blueiter.Iterator"], "blank-imported package must contribute no references")

	// Qualified types from genuinely imported packages pass RequireImport.
	assert.True(t, got["iterator.Iterator"], "qualified type from an imported package must be listed")
}

// TestListReferencesGoStreamsDuplicates documents that the engine streams one
// entry per occurrence (the same qualified name appears across files), so
// deduplication is the caller's responsibility (see completeReferencedSymbol).
func TestListReferencesGoStreamsDuplicates(t *testing.T) {
	t.Parallel()

	env := setupResolveEnv(t)

	ch := make(chan string, 256)
	done := make(chan error, 1)
	go func() {
		defer close(ch)
		done <- symbolresolve.ListReferences(
			context.Background(), env.parser, env.qc, specIter(symbolresolve.Go), ch,
		)
	}()
	counts := make(map[string]int)
	for s := range ch {
		counts[s]++
	}
	require.NoError(t, <-done)

	// mylib.MyType is referenced from several files, so the unbuffered stream
	// carries it more than once.
	assert.Greater(t, counts["mylib.MyType"], 1,
		"engine streams one entry per occurrence; callers deduplicate")
}

func TestListReferencesPythonSpecOnGoWorkspaceEmpty(t *testing.T) {
	t.Parallel()

	env := setupResolveEnv(t)

	// A workspace with no Python files must contribute nothing for the
	// Python spec.
	got := listReferences(t, env.parser, env.qc, specIter(symbolresolve.Python))
	assert.Empty(t, got)
}

func TestListReferencesPythonE2E(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	got := listReferences(t, env.parser, env.qc, specIter(symbolresolve.Python))

	for _, name := range []string{
		"geometry.area", "geometry.Shape", "requests.get", "service.Service",
	} {
		assert.Truef(t, got[name], "expected Python reference %q to be listed", name)
	}

	// Go-only qualified names must never appear in a Python-only run.
	for _, name := range []string{"mylib.MyType", "iterator.Iterator"} {
		assert.Falsef(t, got[name], "Go name %q must not be listed for Python spec", name)
	}
}

func TestListReferencesGoSpecOnPythonWorkspaceEmpty(t *testing.T) {
	t.Parallel()

	env := setupPythonEnv(t)

	got := listReferences(t, env.parser, env.qc, specIter(symbolresolve.Go))
	assert.Empty(t, got)
}
