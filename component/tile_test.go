package component

import (
	"testing"

	"github.com/ernestrc/fractal"
)

func TestNew(t *testing.T) {
	m, _, e := NewTileManager(10, 10, 0, 0, &Fill{})
	if e != nil {
		t.Fatal(e)
	}

	if m.height != 10 || m.width != 10 {
		t.Errorf("not initialized correcty: %+v", m)
	}
}

func testScrollSize(t *testing.T, w *Tile, width, height int) {
	if w.Width() != width {
		t.Errorf("window.Width(%d) != %d", w.Width(), width)
	}

	if w.Height() != height {
		t.Errorf("window.Height(%d) != %d", w.Height(), height)
	}
}

func testScrollPos(t *testing.T, w *Tile, x, y int) {
	xpos, ypos := w.Position()
	if xpos != x {
		t.Errorf("window.x(%d) != %d", xpos, x)
	}
	if ypos != y {
		t.Errorf("window.y(%d) != %d", ypos, y)
	}
}

func TestSplitVertical(t *testing.T) {
	var m *TileManager
	var root *Tile
	var e error

	width := 100
	height := 100

	if m, root, e = NewTileManager(width, height, 0, 0, &Fill{}); e != nil {
		t.Fatal(e)
	}

	var w1 *Tile
	if w1, e = m.SplitVertical(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, 50, height)
	testScrollSize(t, w1, 50, height)
	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 50, 0)

	var w2 *Tile
	if w2, e = m.SplitVertical(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, 33, height)
	testScrollSize(t, w1, 33, height)
	// 33 + 1 to use last cell available
	testScrollSize(t, w2, 34, height)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 33, 0)
	testScrollPos(t, w2, 66, 0)

	var w3 *Tile
	if w3, e = m.SplitVertical(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, 25, height)
	testScrollSize(t, w3, 25, height)
	testScrollSize(t, w1, 25, height)
	testScrollSize(t, w2, 25, height)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 25, 0)
	testScrollPos(t, w2, 50, 0)
	testScrollPos(t, w3, 75, 0)
}

func TestSplitHorizontal(t *testing.T) {
	var m *TileManager
	var root *Tile
	var e error

	width := 100
	height := 100

	if m, root, e = NewTileManager(width, height, 0, 0, &Fill{}); e != nil {
		t.Fatal(e)
	}

	var w1 *Tile
	if w1, e = m.SplitHorizontal(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, width, 50)
	testScrollSize(t, w1, width, 50)
	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 50)

	var w2 *Tile
	if w2, e = m.SplitHorizontal(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, width, 33)
	testScrollSize(t, w1, width, 33)
	testScrollSize(t, w2, width, 34)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 33)
	testScrollPos(t, w2, 0, 66)

	var w3 *Tile
	if w3, e = m.SplitHorizontal(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, width, 25)
	testScrollSize(t, w3, width, 25)
	testScrollSize(t, w1, width, 25)
	testScrollSize(t, w2, width, 25)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 25)
	testScrollPos(t, w2, 0, 50)
	testScrollPos(t, w3, 0, 75)
}

func TestSplitHorizontalVertical(t *testing.T) {
	var m *TileManager
	var root *Tile
	var e error
	var w1, w2, w3, w4, w5, w6 *Tile

	width := 100
	height := 100

	if m, root, e = NewTileManager(width, height, 0, 0, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitVertical(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w3, e = m.SplitVertical(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, 25, height)
	testScrollSize(t, w1, 25, height)
	testScrollSize(t, w2, 25, height)
	testScrollSize(t, w3, 25, height)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 25, 0)
	testScrollPos(t, w2, 50, 0)
	testScrollPos(t, w3, 75, 0)

	if w4, e = m.SplitHorizontal(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, root, 25, 50)
	testScrollSize(t, w4, 25, 50)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w4, 0, 50)

	if w5, e = m.SplitHorizontal(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w6, e = m.SplitHorizontal(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	testScrollSize(t, w1, 25, 33)
	testScrollSize(t, w5, 25, 33)
	testScrollSize(t, w6, 25, 34)

	testScrollPos(t, w1, 25, 0)
	testScrollPos(t, w5, 25, 33)
	testScrollPos(t, w6, 25, 66)
}

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	if m, root, e := NewTileManager(width, height, 0, 0, &Fill{}); e != nil {
		t.Fatal(e)
	} else {
		w1, _ := m.SplitHorizontal(root, &Fill{})
		w2, _ := m.SplitVertical(w1, &Fill{})

		testScrollSize(t, root, 1, 0)
		testScrollSize(t, w1, 0, 1)
		testScrollSize(t, w2, 1, 1)

		testScrollPos(t, root, 0, 0)
		testScrollPos(t, w1, 0, 0)
		testScrollPos(t, w2, 0, 0)
	}
}

func setupTestCase(t *testing.T, gwidth, gheight int) (m *TileManager, root *Tile, w1 *Tile, w2 *Tile) {
	var e error

	if m, root, e = NewTileManager(gwidth, gheight, 0, 0, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitHorizontal(root, &Fill{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &Fill{}); e != nil {
		t.Fatal(e)
	}

	return
}

func TestResize(t *testing.T) {
	m, root, w1, w2 := setupTestCase(t, 100, 100)

	testScrollSize(t, root, 100, 50)
	testScrollSize(t, w1, 50, 50)
	testScrollSize(t, w2, 50, 50)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 50)
	testScrollPos(t, w2, 50, 50)

	if err := m.Resize(50, 50); err != nil {
		t.Fatal(err)
	}

	testScrollSize(t, root, 50, 25)
	testScrollSize(t, w1, 25, 25)
	testScrollSize(t, w2, 25, 25)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 25)
	testScrollPos(t, w2, 25, 25)
}

func TestResizeRounding(t *testing.T) {
	m, root, w1, w2 := setupTestCase(t, 3, 3)
	testScrollSize(t, root, 3, 1)
	testScrollSize(t, w1, 1, 2)
	testScrollSize(t, w2, 2, 2)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 1)
	testScrollPos(t, w2, 1, 1)

	m.Resize(2, 2)
	testScrollSize(t, root, 2, 1)
	testScrollSize(t, w1, 1, 1)
	testScrollSize(t, w2, 1, 1)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 1)
	testScrollPos(t, w2, 1, 1)

	m.Resize(3, 3)
	testScrollSize(t, root, 3, 1)
	testScrollSize(t, w1, 1, 2)
	testScrollSize(t, w2, 2, 2)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 1)
	testScrollPos(t, w2, 1, 1)

	m.Resize(100, 100)
	testScrollSize(t, root, 100, 50)
	testScrollSize(t, w1, 50, 50)
	testScrollSize(t, w2, 50, 50)

	testScrollPos(t, root, 0, 0)
	testScrollPos(t, w1, 0, 50)
	testScrollPos(t, w2, 50, 50)
}

func TestTileManagerDraw(t *testing.T) {
	width, height := 8, 4
	w := fractal.String(width, height)
	m, root, err := NewTileManager(width, height, 0, 0, &Fill{Ch: 'A'})

	var m1 *Tile
	var m2 *Tile
	var m3 *Tile
	var m4 *Tile
	var m5 *Tile
	var m6 *Tile

	if err != nil {
		t.Fatal(err)
	}

	tests := []testCase{
		{
			nil, `
AAAAAAAA
AAAAAAAA
AAAAAAAA
AAAAAAAA`,
		}, {
			func() { m1, err = m.SplitVertical(root, &Fill{Ch: 'B'}) }, `
AAAABBBB
AAAABBBB
AAAABBBB
AAAABBBB`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &Fill{Ch: 'C'}) }, `
AABBBCCC
AABBBCCC
AABBBCCC
AABBBCCC`,
		}, {
			func() { m3, err = m.SplitVertical(m2, &Fill{Ch: 'D'}) }, `
AABBCCDD
AABBCCDD
AABBCCDD
AABBCCDD`,
		}, {
			func() { m4, err = m.SplitVertical(m3, &Fill{Ch: 'E'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDEE
ABCCDDEE`,
		}, {
			func() { m5, err = m.SplitHorizontal(m4, &Fill{Ch: 'X'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDXX
ABCCDDXX`,
		}, {
			func() { m6, err = m.SplitHorizontal(m2, &Fill{Ch: 'Z'}) }, `
ABCCDDEE
ABCCDDEE
ABZZDDXX
ABZZDDXX`,
		}, {
			func() { err = m5.Close() }, `
ABCCDDEE
ABCCDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { err = m2.Close() }, `
ABZZDDEE
ABZZDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { err = root.Close() }, `
BBZZDDEE
BBZZDDEE
BBZZDDEE
BBZZDDEE`,
		}, {
			func() { err = m1.Close() }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { m1, err = m.SplitHorizontal(m3, &Fill{Ch: 'A'}) }, `
ZZDDDEEE
ZZDDDEEE
ZZAAAEEE
ZZAAAEEE`,
		}, {
			func() { err = m1.Close() }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { err = m3.Close() }, `
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE`,
		}, {
			func() { err = m4.Close() }, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		}, {
			func() { m1, err = m.SplitHorizontal(m6, &Fill{Ch: 'Y'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &Fill{Ch: 'X'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYXXXX
YYYYXXXX`,
		}, {
			func() { err = m.Resize(16, 4); w.Resize(16, 4) }, `
ZZZZZZZZZZZZZZZZ
ZZZZZZZZZZZZZZZZ
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { err = m6.Close() }, `
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { err = m.Resize(4, 2); w.Resize(4, 2) }, `
YYXX
YYXX`,
		}, {
			func() { err = m1.Close() }, `
XXXX
XXXX`,
		},
	}

	testWorkflow(t, m, w, tests)
}
