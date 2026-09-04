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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

func TestNew(t *testing.T) {
	tree, m := NewTileTree(&component.TestComponent{})
	tree.Resize(10, 10)

	if m.height != 10 || m.width != 10 {
		t.Errorf("not initialized correcty: %+v", m)
	}
}

func assertTileSize(t *testing.T, w *TileNode, width, height int) {
	if w.width != width {
		t.Errorf("window.Width(%d) != %d", w.width, width)
	}

	if w.height != height {
		t.Errorf("window.Height(%d) != %d", w.height, height)
	}
}

func assertTilePos(t *testing.T, tree *TileTree, node *TileNode, x, y int) {
	expected := term.Coordinates{X: x, Y: y}
	assert.Equal(t, expected, node.Position())
}

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	tree, m := NewTileTree(&component.TestComponent{})
	tree.Resize(width, height)
	w1 := tree.SplitHorizontal(m, &component.TestComponent{})
	w2 := tree.SplitVertical(w1, &component.TestComponent{})

	assertTileSize(t, m, 1, 0)
	assertTileSize(t, w1, 0, 1)
	assertTileSize(t, w2, 1, 1)

	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 0)
	assertTilePos(t, tree, w2, 0, 0)
}

func setupTestCase(t *testing.T, gwidth, gheight int) (tree *TileTree, m *TileNode, w1 *TileNode, w2 *TileNode) {
	tree, m = NewTileTree(&component.TestComponent{})
	tree.Resize(gwidth, gheight)

	w1 = tree.SplitHorizontal(m, &component.TestComponent{})
	w2 = tree.SplitVertical(w1, &component.TestComponent{})

	return
}

func TestNeighbours(t *testing.T) {
	tree, m, w1, w2 := setupTestCase(t, 100, 100)

	if m.TileUp() != nil {
		t.Errorf("%+v vs nil", m.TileUp())
	}

	if m.TileLeft() != nil {
		t.Errorf("%+v vs nil", m.TileLeft())
	}

	if m.TileRight() != nil {
		t.Errorf("%+v vs nil", m.TileRight())
	}

	if m.TileDown() != w1 {
		t.Errorf("%+v vs %+v", m.TileDown(), w1)
	}

	if w1.TileUp() != m {
		t.Errorf("%+v vs %+v", w1.TileUp(), m)
	}

	if w2.TileUp() != m {
		t.Errorf("%+v vs %+v", w2.TileUp(), m)
	}

	if w1.TileLeft() != nil {
		t.Errorf("%+v vs nil", w1.TileLeft())
	}

	if w2.TileRight() != nil {
		t.Errorf("%+v vs nil", w1.TileRight())
	}

	if w1.TileRight() != w2 {
		t.Errorf("%+v vs %+v", w1.TileRight(), w2)
	}

	if w2.TileLeft() != w1 {
		t.Errorf("%+v vs %+v", w2.TileLeft(), w1)
	}

	w3 := tree.SplitVertical(w1, &component.TestComponent{})

	if w1.TileRight() != w3 {
		t.Errorf("%+v vs %+v", w1.TileRight(), w3)
	}

	if w2.TileLeft() != w3 {
		t.Errorf("%+v vs %+v", w2.TileLeft(), w3)
	}

	if w3.TileLeft() != w1 {
		t.Errorf("%+v vs %+v", w3.TileLeft(), w1)
	}

	if w3.TileRight() != w2 {
		t.Errorf("%+v vs %+v", w3.TileRight(), w2)
	}

	w4 := tree.SplitHorizontal(w3, &component.TestComponent{})
	w5 := tree.SplitVertical(w4, &component.TestComponent{})

	if w5.TileRight() != w2 {
		t.Errorf("%+v vs %+v", w5.TileRight(), w2)
	}

	if w2.TileLeft() != w5 {
		t.Errorf("%+v vs %+v", w2.TileLeft(), w5)
	}

	if w5.TileUp() != w3 {
		t.Errorf("%+v vs %+v", w5.TileUp(), w3)
	}

	if w3.TileDown() != w4 {
		t.Errorf("%+v vs %+v", w3.TileDown(), w5)
	}

	if w5.TileDown() != nil {
		t.Errorf("%+v vs %+v", w5.TileDown(), nil)
	}

	if w5.TileLeft() != w4 {
		t.Errorf("%+v vs %+v", w5.TileLeft(), w4)
	}

	if w4.TileRight() != w5 {
		t.Errorf("%+v vs %+v", w4.TileRight(), w5)
	}
}

func TestResizeSimple(t *testing.T) {
	tree, m, w1, w2 := setupTestCase(t, 100, 100)

	assertTileSize(t, m, 100, 50)
	assertTileSize(t, w1, 50, 50)
	assertTileSize(t, w2, 50, 50)

	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 50)
	assertTilePos(t, tree, w2, 50, 50)

	tree.Resize(50, 50)

	assertTileSize(t, m, 50, 25)
	assertTileSize(t, w1, 25, 25)
	assertTileSize(t, w2, 25, 25)

	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 25)
	assertTilePos(t, tree, w2, 25, 25)
}

func TestTileTreeLayoutRoundTrip(t *testing.T) {
	t.Run("layout extraction ignores artificial root", func(t *testing.T) {
		tree, m, w1, w2 := setupTestCase(t, 100, 100)
		require.Equal(t, TileLayout{
			Split: SplitOrientationHorizontal,
			Children: []TileLayout{
				{WindowID: m.ID()},
				{
					Split: SplitOrientationVertical,
					Children: []TileLayout{
						{WindowID: w1.ID()},
						{WindowID: w2.ID()},
					},
				},
			},
		}, tree.Layout())
	})

	tests := []struct {
		name       string
		layout     TileLayout
		wantLeaves []uint64
		wantShape  TileLayout
	}{
		{
			name:       "single leaf",
			layout:     TileLayout{WindowID: 1},
			wantLeaves: []uint64{1},
			wantShape:  TileLayout{WindowID: 1},
		},
		{
			name: "two horizontal leaves",
			layout: TileLayout{
				Split: SplitOrientationHorizontal,
				Children: []TileLayout{
					{WindowID: 1},
					{WindowID: 2},
				},
			},
			wantLeaves: []uint64{1, 2},
		},
		{
			name: "three vertical siblings",
			layout: TileLayout{
				Split: SplitOrientationVertical,
				Children: []TileLayout{
					{WindowID: 1},
					{WindowID: 2},
					{WindowID: 3},
				},
			},
			wantLeaves: []uint64{1, 2, 3},
		},
		{
			name: "deep mixed nesting",
			layout: TileLayout{
				Split: SplitOrientationVertical,
				Children: []TileLayout{
					{WindowID: 1},
					{
						Split: SplitOrientationHorizontal,
						Children: []TileLayout{
							{WindowID: 2},
							{
								Split: SplitOrientationVertical,
								Children: []TileLayout{
									{WindowID: 3},
									{WindowID: 4},
								},
							},
						},
					},
				},
			},
			wantLeaves: []uint64{1, 2, 3, 4},
		},
		{
			name: "zero window leaf uses nop and is not mapped",
			layout: TileLayout{
				Split: SplitOrientationHorizontal,
				Children: []TileLayout{
					{WindowID: 0},
					{WindowID: 2},
				},
			},
			wantLeaves: []uint64{2},
		},
		{
			name: "empty stem normalizes to fallback leaf",
			layout: TileLayout{
				Split:    SplitOrientationHorizontal,
				Children: []TileLayout{},
			},
			wantLeaves: []uint64{},
			wantShape:  TileLayout{Children: []TileLayout{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restored := new(TileTree)
			mapping := restored.RestoreLayout(tt.layout, func(windowID uint64) tui.Component {
				if windowID == 0 {
					return nil
				}
				return &component.TestComponent{Ch: rune('A' + windowID)}
			})
			restored.Resize(120, 60)

			require.Len(t, mapping, len(tt.wantLeaves))
			for _, oldID := range tt.wantLeaves {
				require.Contains(t, mapping, oldID)
				require.NotZero(t, mapping[oldID].ID())
				require.NotEqual(t, oldID, mapping[oldID].ID())
			}
			wantShape := tt.layout
			if tt.wantShape.WindowID != 0 || tt.wantShape.Children != nil {
				wantShape = tt.wantShape
			}
			assertLayoutShape(t, wantShape, restored.Layout(), mapping)
		})
	}
}

func assertLayoutShape(
	t *testing.T,
	want TileLayout,
	got TileLayout,
	mapping map[uint64]*TileNode,
) {
	t.Helper()
	require.Equal(t, want.Split, got.Split)
	require.Len(t, got.Children, len(want.Children))
	if len(want.Children) == 0 {
		if want.WindowID == 0 {
			require.NotZero(t, got.WindowID)
			return
		}
		require.Equal(t, mapping[want.WindowID].ID(), got.WindowID)
		return
	}
	for i := range want.Children {
		assertLayoutShape(t, want.Children[i], got.Children[i], mapping)
	}
}

// TestTileAtNoGapWithFixedTailChild reproduces crash report 823596052:
// "could not find tile at {X:215 Y:46}". Mouse coordinate lookup panicked
// at component/tile.go:622 because TileNode.tileAt iterates children and
// expects them to fully tile the parent. When the last child of a
// vertical split is fixed-width and (width - fixedWidth) is odd, the
// non-fixed children share an even integer width and the leftover
// "spare" column gets assigned to a fixed-size child via
// useSpareIdx, which discards it. The result is a one-column gap at the
// right edge that produces no tile match.
func TestTileAtNoGapWithFixedTailChild(t *testing.T) {
	tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
	w1 := tree.SplitVertical(m, &component.TestComponent{Ch: 'B'})
	w2 := tree.SplitVertical(w1, &component.TestComponent{Ch: 'C'})

	// Choose width so (width - fixedSize) is odd: 216 - 7 = 209.
	const width, height = 216, 47
	tree.Resize(width, height)
	require.True(t, w2.SetFixedWidth(7), "SetFixedWidth on tail child")
	tree.Resize(width, height)

	for x := 0; x < width; x++ {
		require.NotPanics(t, func() {
			tree.TileAt(term.Coordinates{X: x, Y: 0})
		}, "TileAt({%d, 0})", x)
	}
}

// TestTileAtNoGapWithFixedTailChildHorizontal mirrors the bug for
// horizontal splits: when the last child has a fixed height and
// (height - fixedHeight) is odd, the rightmost spare row is lost.
func TestTileAtNoGapWithFixedTailChildHorizontal(t *testing.T) {
	tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
	w1 := tree.SplitHorizontal(m, &component.TestComponent{Ch: 'B'})
	w2 := tree.SplitHorizontal(w1, &component.TestComponent{Ch: 'C'})

	const width, height = 80, 48
	tree.Resize(width, height)
	require.True(t, w2.SetFixedHeight(5), "SetFixedHeight on tail child")
	tree.Resize(width, height)

	for y := 0; y < height; y++ {
		require.NotPanics(t, func() {
			tree.TileAt(term.Coordinates{X: 0, Y: y})
		}, "TileAt({0, %d})", y)
	}
}

func TestResizeRounding(t *testing.T) {
	tree, m, w1, w2 := setupTestCase(t, 3, 3)
	assertTileSize(t, m, 3, 1)
	assertTileSize(t, w1, 1, 2)
	assertTileSize(t, w2, 2, 2)

	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 1)
	assertTilePos(t, tree, w2, 1, 1)

	tree.Resize(2, 2)
	assertTileSize(t, m, 2, 1)
	assertTileSize(t, w1, 1, 1)
	assertTileSize(t, w2, 1, 1)
	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 1)
	assertTilePos(t, tree, w2, 1, 1)

	tree.Resize(3, 3)
	assertTileSize(t, m, 3, 1)
	assertTileSize(t, w1, 1, 2)
	assertTileSize(t, w2, 2, 2)
	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 1)
	assertTilePos(t, tree, w2, 1, 1)

	tree.Resize(100, 100)
	assertTileSize(t, m, 100, 50)
	assertTileSize(t, w1, 50, 50)
	assertTileSize(t, w2, 50, 50)
	assertTilePos(t, tree, m, 0, 0)
	assertTilePos(t, tree, w1, 0, 50)
	assertTilePos(t, tree, w2, 50, 50)
}

func TestTileNodeClose(t *testing.T) {
	t.Run("panics if try to close last node", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("did not panic when closing last node")
			}
		}()
		_, m := NewTileTree(&component.TestComponent{Ch: 'A'})
		m.Close()
	})
	t.Run("does not panic if try to close first node", func(t *testing.T) {
		tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
		tree.SplitVertical(m, &component.TestComponent{Ch: 'X'})
		m.Close()
	})
	t.Run("does not panic if try to close second node after removing first", func(t *testing.T) {
		tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
		m2 := tree.SplitVertical(m, &component.TestComponent{Ch: 'X'})
		m2.Close()
		m3 := tree.SplitVertical(m, &component.TestComponent{Ch: 'X'})
		m.Close()
		m4 := tree.SplitVertical(m3, &component.TestComponent{Ch: 'X'})
		m4.Close()
	})
	t.Run("panics if try to close same node twice", func(t *testing.T) {
		tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
		m2 := tree.SplitVertical(m, &component.TestComponent{Ch: 'X'})
		assert.NotPanics(t, func() {
			m2.Close()
			m2.Close()
		})
	})

}

func testTileAt(t *testing.T, tree *TileTree) {
	// ABCCDDEE
	// ABCCDDEE
	// ABCCDDXX
	// ABCCDDXX

	ta := tree.TileAt(term.Coordinates{})
	assert.Equal(t, ta.Content().(*component.TestComponent).Ch, 'A')
	assert.Equal(t, ta, tree.TileAt(term.Coordinates{Y: 1}))
	assert.Equal(t, ta, tree.TileAt(term.Coordinates{Y: 2}))
	assert.Equal(t, ta, tree.TileAt(term.Coordinates{Y: 3}))

	tb := tree.TileAt(term.Coordinates{X: 1})
	assert.Equal(t, tb.Content().(*component.TestComponent).Ch, 'B')
	assert.Equal(t, tb, tree.TileAt(term.Coordinates{X: 1, Y: 1}))
	assert.Equal(t, tb, tree.TileAt(term.Coordinates{X: 1, Y: 2}))
	assert.Equal(t, tb, tree.TileAt(term.Coordinates{X: 1, Y: 3}))

	tc := tree.TileAt(term.Coordinates{X: 3})
	assert.Equal(t, tc.Content().(*component.TestComponent).Ch, 'C')
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 3, Y: 1}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 3, Y: 2}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 3, Y: 3}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 2, Y: 0}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 2, Y: 1}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 2, Y: 2}))
	assert.Equal(t, tc, tree.TileAt(term.Coordinates{X: 2, Y: 3}))

	td := tree.TileAt(term.Coordinates{X: 5})
	assert.Equal(t, td.Content().(*component.TestComponent).Ch, 'D')
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 5, Y: 1}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 5, Y: 2}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 5, Y: 3}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 4, Y: 0}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 4, Y: 1}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 4, Y: 2}))
	assert.Equal(t, td, tree.TileAt(term.Coordinates{X: 4, Y: 3}))

	te := tree.TileAt(term.Coordinates{X: 6})
	assert.Equal(t, te.Content().(*component.TestComponent).Ch, 'E')
	assert.Equal(t, te, tree.TileAt(term.Coordinates{X: 6, Y: 1}))
	assert.Equal(t, te, tree.TileAt(term.Coordinates{X: 7, Y: 0}))
	assert.Equal(t, te, tree.TileAt(term.Coordinates{X: 7, Y: 1}))

	tx := tree.TileAt(term.Coordinates{Y: 2, X: 6})
	assert.Equal(t, tx.Content().(*component.TestComponent).Ch, 'X')
	assert.Equal(t, tx, tree.TileAt(term.Coordinates{X: 6, Y: 2}))
	assert.Equal(t, tx, tree.TileAt(term.Coordinates{X: 7, Y: 3}))
	assert.Equal(t, tx, tree.TileAt(term.Coordinates{X: 7, Y: 3}))
}

func TestTileNodeDraw(t *testing.T) {
	var err error

	width, height := 8, 4
	w := term.NewStringWriter(width, height)
	tree, m := NewTileTree(&component.TestComponent{Ch: 'A'})
	tree.Resize(width, height)

	var m1 *TileNode
	var m2 *TileNode
	var m3 *TileNode
	var m4 *TileNode
	var m5 *TileNode
	var m6 *TileNode

	if err != nil {
		t.Fatal(err)
	}

	tests := []comptest.TestCase{
		{
			nil, `
AAAAAAAA
AAAAAAAA
AAAAAAAA
AAAAAAAA`,
		}, {
			func() { m1 = tree.SplitVertical(m, &component.TestComponent{Ch: 'B'}) }, `
AAAABBBB
AAAABBBB
AAAABBBB
AAAABBBB`,
		}, {
			func() { m2 = tree.SplitVertical(m1, &component.TestComponent{Ch: 'C'}) }, `
AABBBCCC
AABBBCCC
AABBBCCC
AABBBCCC`,
		}, {
			func() { m3 = tree.SplitVertical(m2, &component.TestComponent{Ch: 'D'}) }, `
AABBCCDD
AABBCCDD
AABBCCDD
AABBCCDD`,
		}, {
			func() { m4 = tree.SplitVertical(m3, &component.TestComponent{Ch: 'E'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDEE
ABCCDDEE`,
		}, {
			func() { m5 = tree.SplitHorizontal(m4, &component.TestComponent{Ch: 'X'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDXX
ABCCDDXX`,
		}, {
			func() {
				// NOTE: I'm feeling lazy today; this should be moved
				// to a separate test... :shrug
				testTileAt(t, tree)

				m6 = tree.SplitHorizontal(m2, &component.TestComponent{Ch: 'Z'})
			}, `
ABCCDDEE
ABCCDDEE
ABZZDDXX
ABZZDDXX`,
		}, {
			func() { m5.Close() }, `
ABCCDDEE
ABCCDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { m2.Close() }, `
ABZZDDEE
ABZZDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { m.Close() }, `
BBZZDDEE
BBZZDDEE
BBZZDDEE
BBZZDDEE`,
		}, {
			func() { m1.Close() }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { m1 = tree.SplitHorizontal(m3, &component.TestComponent{Ch: 'A'}) }, `
ZZDDDEEE
ZZDDDEEE
ZZAAAEEE
ZZAAAEEE`,
		}, {
			func() { m1.Close() }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { m3.Close() }, `
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE`,
		}, {
			func() { m4.Close() }, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		}, {
			func() { m1 = tree.SplitHorizontal(m6, &component.TestComponent{Ch: 'Y'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { m2 = tree.SplitVertical(m1, &component.TestComponent{Ch: 'X'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYXXXX
YYYYXXXX`,
		}, {
			func() { tree.Resize(16, 4); w.Resize(16, 4) }, `
ZZZZZZZZZZZZZZZZ
ZZZZZZZZZZZZZZZZ
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { m6.Close() }, `
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { tree.Resize(4, 2); w.Resize(4, 2) }, `
YYXX
YYXX`,
		}, {
			func() { m1.Close() }, `
XXXX
XXXX`,
		}, {
			func() { _ = tree.SplitVertical(m2, &component.TestComponent{Ch: 'Z'}) }, `
XXZZ
XXZZ`,
		}, {
			func() { tree.Resize(8, 4); w.Resize(8, 4) }, `
XXXXZZZZ
XXXXZZZZ
XXXXZZZZ
XXXXZZZZ`,
		}, {
			func() { _ = tree.SplitVertical(m2, &component.TestComponent{Ch: 'I'}) }, `
XXIIIZZZ
XXIIIZZZ
XXIIIZZZ
XXIIIZZZ`,
		}, {
			func() {
				tree.Iterate(func(node *TileNode) {
					node.Content().(*component.TestComponent).Ch = 'X'
				})
			}, `
XXXXXXXX
XXXXXXXX
XXXXXXXX
XXXXXXXX`,
		},
	}

	comptest.TestComponent(t, tree, w, tests)
}

func TestTileNodeSize(t *testing.T) {
	tree, _, _, _ := setupTestCase(t, 100, 100)
	if size := tree.Size(); size != 3 {
		t.Errorf("size should be 3; found: %d", size)
	}
}

func assertNotNil(t *testing.T, c tui.Component) {
	if c == nil {
		t.Errorf("unexpected nil component")
	}
}

func TestTiledNodeContent(t *testing.T) {
	_, m1, m2, m3 := setupTestCase(t, 100, 100)
	assertNotNil(t, m1.Content())
	assertNotNil(t, m2.Content())
	assertNotNil(t, m3.Content())
}

func TestTiledIterateInit(t *testing.T) {
	tree, m := NewTileTree(&component.TestComponent{})
	var i int
	tree.Iterate(func(node *TileNode) {
		i++
	})
	assert.Equal(t, 1, i)
	i = 0

	tree.SplitHorizontal(m, &component.TestComponent{})
	tree.SplitVertical(m, &component.TestComponent{})

	tree.Iterate(func(node *TileNode) {
		i++
	})
	assert.Equal(t, 3, i)
}

func TestTileDimensions(t *testing.T) {
	tree, t1 := NewTileTree(&component.TestComponent{Ch: '1'})
	t2 := tree.SplitVertical(t1, &component.TestComponent{Ch: '2'})
	t3 := tree.SplitHorizontal(t2, &component.TestComponent{Ch: '3'})
	t4 := tree.SplitVertical(t3, &component.TestComponent{Ch: '4'})
	tree.Resize(8, 8)

	assertDimensions(t, 4, 8, t1)
	assertDimensions(t, 4, 4, t2)
	assertDimensions(t, 2, 4, t3)
	assertDimensions(t, 2, 4, t4)
}

func TestTilePosition(t *testing.T) {
	tree, t1 := NewTileTree(&component.TestComponent{Ch: '1'})
	t2 := tree.SplitVertical(t1, &component.TestComponent{Ch: '2'})
	t3 := tree.SplitHorizontal(t2, &component.TestComponent{Ch: '3'})
	t4 := tree.SplitVertical(t3, &component.TestComponent{Ch: '4'})
	tree.Resize(8, 8)

	assertTilePos(t, tree, t1, 0, 0)
	assertTilePos(t, tree, t2, 4, 0)
	assertTilePos(t, tree, t3, 4, 4)
	assertTilePos(t, tree, t4, 6, 4)
}

func assertDimensions(t *testing.T, expectedWidth, expectedHeight int, tile *TileNode) {
	actualWidth, actualHeight := tile.Width(), tile.Height()
	assert.Equal(t, expectedWidth, actualWidth)
	assert.Equal(t, expectedHeight, actualHeight)
}
