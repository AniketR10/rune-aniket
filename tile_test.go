package fractal

import (
	"testing"
)

func TestNew(t *testing.T) {
	m, _ := NewTileNode(&TestComponent{})
	if e := m.Resize(10, 10); e != nil {
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

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	m, root := NewTileNode(&TestComponent{})
	if e := m.Resize(width, height); e != nil {
		t.Fatal(e)
	} else {
		w1, _ := m.SplitHorizontal(root, &TestComponent{})
		w2, _ := m.SplitVertical(w1, &TestComponent{})

		testScrollSize(t, root, 1, 0)
		testScrollSize(t, w1, 0, 1)
		testScrollSize(t, w2, 1, 1)

		testScrollPos(t, root, 0, 0)
		testScrollPos(t, w1, 0, 0)
		testScrollPos(t, w2, 0, 0)
	}
}

func setupTestCase(t *testing.T, gwidth, gheight int) (m *TileNode, root *Tile, w1 *Tile, w2 *Tile) {
	var e error

	m, root = NewTileNode(&TestComponent{})
	if e = m.Resize(gwidth, gheight); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitHorizontal(root, &TestComponent{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &TestComponent{}); e != nil {
		t.Fatal(e)
	}

	return
}

func TestNeighbours(t *testing.T) {
	m, root, w1, w2 := setupTestCase(t, 100, 100)

	if root.TileUp() != nil {
		t.Errorf("%+v vs nil", root.TileUp())
	}

	if root.TileLeft() != nil {
		t.Errorf("%+v vs nil", root.TileLeft())
	}

	if root.TileRight() != nil {
		t.Errorf("%+v vs nil", root.TileRight())
	}

	if root.TileDown() != w1 {
		t.Errorf("%+v vs %+v", root.TileDown(), w1)
	}

	if w1.TileUp() != root {
		t.Errorf("%+v vs %+v", w1.TileUp(), root)
	}

	if w2.TileUp() != root {
		t.Errorf("%+v vs %+v", w2.TileUp(), root)
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

	w3, err := m.SplitVertical(w1, &TestComponent{})
	if err != nil {
		t.Fatal(err)
	}

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

	w4, err2 := m.SplitHorizontal(w3, &TestComponent{})
	if err2 != nil {
		t.Fatal(err2)
	}

	w5, err3 := m.SplitVertical(w4, &TestComponent{})
	if err3 != nil {
		t.Fatal(err3)
	}

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

func TestTileNodeDraw(t *testing.T) {
	var err error

	width, height := 8, 4
	w := NewStringWriter(width, height)
	m, root := NewTileNode(&TestComponent{Ch: 'A'})

	if err = m.Resize(width, height); err != nil {
		t.Fatal(err)
	}

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
			func() { m1, err = m.SplitVertical(root, &TestComponent{Ch: 'B'}) }, `
AAAABBBB
AAAABBBB
AAAABBBB
AAAABBBB`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &TestComponent{Ch: 'C'}) }, `
AABBBCCC
AABBBCCC
AABBBCCC
AABBBCCC`,
		}, {
			func() { m3, err = m.SplitVertical(m2, &TestComponent{Ch: 'D'}) }, `
AABBCCDD
AABBCCDD
AABBCCDD
AABBCCDD`,
		}, {
			func() { m4, err = m.SplitVertical(m3, &TestComponent{Ch: 'E'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDEE
ABCCDDEE`,
		}, {
			func() { m5, err = m.SplitHorizontal(m4, &TestComponent{Ch: 'X'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDXX
ABCCDDXX`,
		}, {
			func() { m6, err = m.SplitHorizontal(m2, &TestComponent{Ch: 'Z'}) }, `
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
			func() { m1, err = m.SplitHorizontal(m3, &TestComponent{Ch: 'A'}) }, `
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
			func() { m1, err = m.SplitHorizontal(m6, &TestComponent{Ch: 'Y'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &TestComponent{Ch: 'X'}) }, `
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
		}, {
			func() { _, err = m.SplitVertical(m2, &TestComponent{Ch: 'Z'}) }, `
XXZZ
XXZZ`,
		}, {
			func() { err = m.Resize(8, 4); w.Resize(8, 4) }, `
XXXXZZZZ
XXXXZZZZ
XXXXZZZZ
XXXXZZZZ`,
		}, {
			func() { _, err = m.SplitVertical(m2, &TestComponent{Ch: 'I'}) }, `
XXIIIZZZ
XXIIIZZZ
XXIIIZZZ
XXIIIZZZ`,
		}, {
			func() { err = m.Move(2, 2); w.Resize(8, 8) }, `
        
        
  XXIIIZ
  XXIIIZ
  XXIIIZ
  XXIIIZ
        
        `,
		},
	}

	testWorkflow(t, m, w, tests)
}
