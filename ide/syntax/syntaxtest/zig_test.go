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
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/text"
)

func TestZigTreeFoldsIntegration(t *testing.T) {
	tree, cleanup := newZigTree(t, zigFileContent)

	it, ok := tree.Folds()
	require.True(t, ok)

	actual, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.NotEmpty(t, actual)

	require.NoError(t, tree.Close())
	cleanup()
}

func TestZigTreeStateIntegration(t *testing.T) {
	t.Run("ready tree reports a fully initialized zig state", func(t *testing.T) {
		tree, cleanup := newZigTree(t, zigFileContent)

		it := tree.State()
		actual, ok := it.Next(context.Background())
		require.True(t, ok)
		defer it.Close()
		assert.Equal(t, syntax.State{
			LangID:     "zig",
			Folds:      true,
			Indents:    true,
			Highlights: true,
			Progress:   1,
		}, actual)

		require.NoError(t, tree.Close())
		cleanup()
	})

	t.Run("missing folds.scm still streams state with Folds false", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/tree-sitter.so",
			"zig/highlights.scm",
			"zig/indents.scm",
		)
		tree, cleanup := newZigTreeWithPkgManager(t, pkgs, zigFileContent)

		it := tree.State()
		actual, ok := it.Next(context.Background())
		require.True(t, ok)
		defer it.Close()
		assert.Equal(t, syntax.State{
			LangID:     "zig",
			Folds:      false,
			Indents:    true,
			Highlights: true,
			Progress:   1,
		}, actual)

		require.NoError(t, tree.Close())
		cleanup()
	})
}

func TestZigTreeQueryIntegration(t *testing.T) {
	t.Run("missing locals.scm makes Query error", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/tree-sitter.so",
			"zig/highlights.scm",
			"zig/indents.scm",
		)
		tree, cleanup := newZigTreeWithPkgManager(t, pkgs, zigFileContent)

		_, err := tree.Query("locals.scm", "query")
		require.Error(t, err)

		require.NoError(t, tree.Close())
		cleanup()
	})

	t.Run("locals.scm resolves the function definition capture", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/tree-sitter.so",
			"zig/locals.scm",
		)
		tree, cleanup := newZigTreeWithPkgManager(t, pkgs, zigFileContent)

		it, err := tree.Query("locals.scm", "local.definition.function")
		require.NoError(t, err)

		matches, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		require.NotEmpty(t, matches)
		for _, m := range matches {
			assert.Equal(t, "local.definition.function", m.CaptureName)
		}

		require.NoError(t, tree.Close())
		cleanup()
	})

	t.Run("custom query file drives the query", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/tree-sitter.so",
			"abc_zig.scm",
		)
		_, comp, tree, cleanup := newZigTreeFull(t, pkgs, zigFileContent)

		w := comp.Workspace()
		f, err := w.OpenFile("abc_zig.scm", os.O_CREATE|os.O_WRONLY, 0o666)
		require.NoError(t, err)
		_, err = f.Write([]byte(`((function_declaration name: (identifier) @local.reference)
  (#set! reference.kind "function"))`))
		require.NoError(t, err)
		require.NoError(t, f.Close())

		it, err := tree.Query(f.Name(), "local.reference")
		require.NoError(t, err)

		matches, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		require.NotEmpty(t, matches)
		for _, m := range matches {
			assert.Equal(t, "local.reference", m.CaptureName)
		}

		require.NoError(t, tree.Close())
		cleanup()
	})

	t.Run("missing language parser makes the Query iterator error", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/indents.scm",
		)
		tree, cleanup := newZigTreeWithPkgManager(t, pkgs, zigFileContent)

		it, err := tree.Query("indents.scm", "query")
		require.NoError(t, err)
		_, ok := it.Next(context.Background())
		assert.False(t, ok)
		require.Error(t, it.Err())

		require.NoError(t, tree.Close())
		cleanup()
	})
}

func TestZigTreeCommentCoverageIntegration(t *testing.T) {
	t.Run("returns exact line comment ranges when fully covered", func(t *testing.T) {
		content := "fn main() void {}\n\n// alpha\n// beta\nfn other() void {}\n"
		tree, cleanup := newZigTree(t, content)

		ranges, ok := tree.CommentCoverage(term.Range{
			Start: term.Coordinates{Y: 2, X: 0},
			End:   term.Coordinates{Y: 3, X: len("// beta")},
		})
		require.True(t, ok)
		assert.Equal(t, []term.Range{
			{Start: term.Coordinates{Y: 2, X: 0}, End: term.Coordinates{Y: 2, X: len("// alpha")}},
			{Start: term.Coordinates{Y: 3, X: 0}, End: term.Coordinates{Y: 3, X: len("// beta")}},
		}, ranges)

		require.NoError(t, tree.Close())
		cleanup()
	})

	t.Run("returns false when selection is only partially covered", func(t *testing.T) {
		content := "fn main() void {}\n\n// alpha\nfn other() void {}\n"
		tree, cleanup := newZigTree(t, content)

		ranges, ok := tree.CommentCoverage(term.Range{
			Start: term.Coordinates{Y: 2, X: 0},
			End:   term.Coordinates{Y: 3, X: 4},
		})
		assert.False(t, ok)
		assert.Nil(t, ranges)

		require.NoError(t, tree.Close())
		cleanup()
	})
}

func TestZigTreeHighlightsIntegration(t *testing.T) {
	var wg sync.WaitGroup
	ready := func(context.Context) error { wg.Done(); return nil }
	const width, height = 30, 15
	pkgs := newInstalledZigPkgManager(t)
	mu, comp, cleanup := newTestCase(t, pkgs, width, height, ready)
	defer cleanup()
	w := newWriter(width, height)

	wg.Add(1)
	mu.Lock()
	newEditFileName(t, mu, comp, zigHighlightContent, "highlight.zig")
	mu.Unlock()
	wg.Wait()

	cases := []handlertest.SingleTestCase{
		{Event: term.Event{Ch: 'g', Type: term.EventKey}, Expected: zigHighlightFrame},
	}
	handlertest.TestHandler(t, comp.Browser(), cases, w)
}

const zigHighlightContent = `fn add(a: i32, b: i32) i32 {
    const total = a + b;
    return total;
}
`

const zigHighlightFrame = `┌###############─────────────┐
│# #############             │
├────────────────────────────┤
│fn add(a: i32, b: i32) i32 {│
│    ##### total = a + b;    │
│    return total;           │
│}                           │
│                            │
│                            │
│                            │
│                            │
│                            │
│                            │
│                      NORMAL│
└────────────────────────────┘`

func TestZigTreeIndentsIntegration(t *testing.T) {
	var wg sync.WaitGroup
	ready := func(context.Context) error { wg.Done(); return nil }
	const width, height = 30, 15
	pkgs := newInstalledZigPkgManager(t)
	mu, comp, cleanup := newTestCase(t, pkgs, width, height, ready)
	defer cleanup()
	w := newWriter(width, height)

	wg.Add(1)
	mu.Lock()
	newEditFileName(t, mu, comp, zigIndentContent, "indent.zig")
	mu.Unlock()
	wg.Wait()

	cases := []handlertest.SequenceTestCase{
		{InputSequence: "ggjo", Expected: zigIndentFrame},
	}
	handlertest.TestHandlerSequenceWriter(t, w, comp.Browser(), width, height, cases)
}

const zigIndentContent = `fn main() void {
    const x = 1;
}
`

const zigIndentFrame = `┌#############───────────────┐
│# ###########               │
├────────────────────────────┤
│fn main() void {            │
│    ##### x = #;            │
│    ▐                       │
│}                           │
│                            │
│                            │
│                            │
│                            │
│                            │
│                            │
│                     #######│
└────────────────────────────┘`

func TestZigTreeSelectionIntegration(t *testing.T) {
	const content = `const Person = struct {
    name: []const u8,
};

fn greet(p: Person) []const u8 {
    const values = [_][]const u8{
        p.name,
        p.name,
    };
    if (values.len > 0) {
        for (values) |value| {
            return value;
        }
    }
    return "";
}
`

	tree, cleanup := newZigTree(t, content)
	defer cleanup()
	defer func() { require.NoError(t, tree.Close()) }()

	t.Run("expand climbs the node ancestry", func(t *testing.T) {
		from := term.Range{
			Start: term.Coordinates{Y: 6, X: 8},
			End:   term.Coordinates{Y: 6, X: 8},
		}
		current := from
		var got []term.Range
		for i := range 4 {
			next, ok := tree.SelectionExpand(current)
			require.True(t, ok, "expand %d from %#v", i, current)
			require.NotEqual(t, current, next, "expand %d did not grow", i)
			got = append(got, next)
			current = next
		}
		for i := 1; i < len(got); i++ {
			assert.True(t, rangeContains(got[i], got[i-1]),
				"expansion %d (%#v) must contain previous (%#v)", i, got[i], got[i-1])
		}
	})

	t.Run("shrink reverses an expansion around the caret", func(t *testing.T) {
		caret := term.Coordinates{Y: 6, X: 8}
		outer, ok := tree.SelectionExpand(term.Range{
			Start: term.Coordinates{Y: 6, X: 8},
			End:   term.Coordinates{Y: 6, X: 14},
		})
		require.True(t, ok)
		inner, ok := tree.SelectionShrink(outer, caret)
		require.True(t, ok)
		assert.True(t, rangeContains(outer, inner),
			"shrunk range %#v must sit inside %#v", inner, outer)
	})

	t.Run("returns false when the syntax tree is unavailable", func(t *testing.T) {
		pkgs := newInstalledPkgManagerWithFiles(t,
			"zig/highlights.scm",
			"zig/indents.scm",
			"zig/folds.scm",
		)
		bad, badCleanup := newZigTreeWithPkgManager(t, pkgs, content)
		defer badCleanup()
		defer func() { require.NoError(t, bad.Close()) }()

		_, ok := bad.SelectionExpand(term.Range{
			Start: term.Coordinates{Y: 4, X: 3},
			End:   term.Coordinates{Y: 4, X: 8},
		})
		assert.False(t, ok)
	})
}

func newInstalledZigPkgManager(t *testing.T) *mockPkgManager {
	return newInstalledPkgManagerWithFiles(t,
		"zig/tree-sitter.so",
		"zig/highlights.scm",
		"zig/indents.scm",
		"zig/folds.scm",
		"zig/locals.scm",
	)
}

func newZigEditFile(
	t *testing.T, mu sync.Locker, comp *text.Component, content string,
) (cell.Editor, text.Handler) {
	n := rand.Int()
	return newEditFileName(t, mu, comp, content, strconv.Itoa(n)+".zig")
}

func newZigTree(t *testing.T, content string) (*syntax.Tree, func()) {
	_, _, tree, cleanup := newZigTreeFull(t, newInstalledZigPkgManager(t), content)
	return tree, cleanup
}

func newZigTreeWithPkgManager(
	t *testing.T, pkgs syntax.PkgManager, content string,
) (*syntax.Tree, func()) {
	_, _, tree, cleanup := newZigTreeFull(t, pkgs, content)
	return tree, cleanup
}

func newZigTreeFull(
	t *testing.T, pkgs syntax.PkgManager, content string,
) (*cell.Buffer, *text.Component, *syntax.Tree, func()) {
	var wg sync.WaitGroup
	ready := func(context.Context) error { wg.Done(); return nil }
	const width, height = 30, 15
	mu, comp, cleanup := newTestCase(t, pkgs, width, height, ready)

	wg.Add(1)
	mu.Lock()
	_, h := newZigEditFile(t, mu, comp, content)
	mu.Unlock()
	wg.Wait()

	cref, ok := h.(*text.StatusBar)
	require.True(t, ok)
	tree, ok := cref.Buffer().View().(*syntax.Tree)
	require.True(t, ok)

	return cref.Buffer(), comp, tree, cleanup
}

const zigFileContent = `const std = @import("std");

const Greeter = struct {
    name: []const u8,

    fn greet(self: Greeter) []const u8 {
        return self.name;
    }
};

pub fn main() void {
    const g = Greeter{ .name = "World" };
    for (0..3) |i| {
        std.debug.print("{s} {d}\n", .{ g.greet(), i });
    }
}
`
