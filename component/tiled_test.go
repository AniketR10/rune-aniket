package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/writer"
)

func TestNew(t *testing.T) {
	m, _, e := NewTiledManager(10, 10, &testComponent{})
	if e != nil {
		t.Fatal(e)
	}

	if m.height != 10 || m.width != 10 {
		t.Errorf("not initialized correcty: %+v", m)
	}

	if m.root.Height() != 10 || m.root.Width() != 10 {
		t.Errorf("root window not initialized correctly: %+v", m.root)
	}
}

func testWindowSize(t *testing.T, w *TiledWindow, width, height int) {
	if w.Width() != width {
		t.Errorf("window.Width(%d) != %d", w.Width(), width)
	}

	if w.Height() != height {
		t.Errorf("window.Height(%d) != %d", w.Height(), height)
	}
}

func testWindowPos(t *testing.T, w *TiledWindow, x, y int) {
	xpos, ypos := w.Position()
	if xpos != x {
		t.Errorf("window.x(%d) != %d", xpos, x)
	}
	if ypos != y {
		t.Errorf("window.y(%d) != %d", ypos, y)
	}
}

func TestSplitVertical(t *testing.T) {
	var m *WindowManager
	var root *TiledWindow
	var e error

	width := 100
	height := 100

	if m, root, e = NewTiledManager(width, height, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitVertical(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 50, height)
	testWindowSize(t, w1, 50, height)
	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 50, 0)

	var w2 *TiledWindow
	if w2, e = m.SplitVertical(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 33, height)
	testWindowSize(t, w1, 33, height)
	// 33 + 1 to use last cell available
	testWindowSize(t, w2, 34, height)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 33, 0)
	testWindowPos(t, w2, 66, 0)

	var w3 *TiledWindow
	if w3, e = m.SplitVertical(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 25, height)
	testWindowSize(t, w3, 25, height)
	testWindowSize(t, w1, 25, height)
	testWindowSize(t, w2, 25, height)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 25, 0)
	testWindowPos(t, w2, 50, 0)
	testWindowPos(t, w3, 75, 0)
}

func TestSplitHorizontal(t *testing.T) {
	var m *WindowManager
	var root *TiledWindow
	var e error

	width := 100
	height := 100

	if m, root, e = NewTiledManager(width, height, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitHorizontal(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, width, 50)
	testWindowSize(t, w1, width, 50)
	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 50)

	var w2 *TiledWindow
	if w2, e = m.SplitHorizontal(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, width, 33)
	testWindowSize(t, w1, width, 33)
	testWindowSize(t, w2, width, 34)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 33)
	testWindowPos(t, w2, 0, 66)

	var w3 *TiledWindow
	if w3, e = m.SplitHorizontal(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, width, 25)
	testWindowSize(t, w3, width, 25)
	testWindowSize(t, w1, width, 25)
	testWindowSize(t, w2, width, 25)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 25)
	testWindowPos(t, w2, 0, 50)
	testWindowPos(t, w3, 0, 75)
}

func TestSplitHorizontalVertical(t *testing.T) {
	var m *WindowManager
	var root *TiledWindow
	var e error
	var w1, w2, w3, w4, w5, w6 *TiledWindow

	width := 100
	height := 100

	if m, root, e = NewTiledManager(width, height, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitVertical(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w3, e = m.SplitVertical(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 25, height)
	testWindowSize(t, w1, 25, height)
	testWindowSize(t, w2, 25, height)
	testWindowSize(t, w3, 25, height)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 25, 0)
	testWindowPos(t, w2, 50, 0)
	testWindowPos(t, w3, 75, 0)

	if w4, e = m.SplitHorizontal(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 25, 50)
	testWindowSize(t, w4, 25, 50)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w4, 0, 50)

	if w5, e = m.SplitHorizontal(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w6, e = m.SplitHorizontal(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, w1, 25, 33)
	testWindowSize(t, w5, 25, 33)
	testWindowSize(t, w6, 25, 34)

	testWindowPos(t, w1, 25, 0)
	testWindowPos(t, w5, 25, 33)
	testWindowPos(t, w6, 25, 66)
}

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	if m, root, e := NewTiledManager(width, height, &testComponent{}); e != nil {
		t.Fatal(e)
	} else {
		w1, _ := m.SplitHorizontal(root, &testComponent{})
		w2, _ := m.SplitVertical(w1, &testComponent{})

		testWindowSize(t, root, 1, 0)
		testWindowSize(t, w1, 0, 1)
		testWindowSize(t, w2, 1, 1)

		testWindowPos(t, root, 0, 0)
		testWindowPos(t, w1, 0, 0)
		testWindowPos(t, w2, 0, 0)
	}
}

func setupTestCase(t *testing.T, gwidth, gheight int) (m *WindowManager, root *TiledWindow, w1 *TiledWindow, w2 *TiledWindow) {
	var e error

	if m, root, e = NewTiledManager(gwidth, gheight, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitHorizontal(root, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &testComponent{}); e != nil {
		t.Fatal(e)
	}

	return
}

func TestResize(t *testing.T) {
	m, root, w1, w2 := setupTestCase(t, 100, 100)

	testWindowSize(t, root, 100, 50)
	testWindowSize(t, w1, 50, 50)
	testWindowSize(t, w2, 50, 50)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 50)
	testWindowPos(t, w2, 50, 50)

	if err := m.Resize(50, 50); err != nil {
		t.Fatal(err)
	}

	testWindowSize(t, root, 50, 25)
	testWindowSize(t, w1, 25, 25)
	testWindowSize(t, w2, 25, 25)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 25)
	testWindowPos(t, w2, 25, 25)
}

func TestResizeRounding(t *testing.T) {
	m, root, w1, w2 := setupTestCase(t, 3, 3)
	testWindowSize(t, root, 3, 1)
	testWindowSize(t, w1, 1, 2)
	testWindowSize(t, w2, 2, 2)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 1)
	testWindowPos(t, w2, 1, 1)

	m.Resize(2, 2)
	testWindowSize(t, root, 2, 1)
	testWindowSize(t, w1, 1, 1)
	testWindowSize(t, w2, 1, 1)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 1)
	testWindowPos(t, w2, 1, 1)

	m.Resize(3, 3)
	testWindowSize(t, root, 3, 1)
	testWindowSize(t, w1, 1, 2)
	testWindowSize(t, w2, 2, 2)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 1)
	testWindowPos(t, w2, 1, 1)

	m.Resize(100, 100)
	testWindowSize(t, root, 100, 50)
	testWindowSize(t, w1, 50, 50)
	testWindowSize(t, w2, 50, 50)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 50)
	testWindowPos(t, w2, 50, 50)
}

func TestWindowManagerDraw(t *testing.T) {
	width, height := 8, 4
	w := writer.String(width, height)
	m, root, err := NewTiledManager(width, height, &testComponent{fill: 'A'})

	var m1 *TiledWindow
	var m2 *TiledWindow
	var m3 *TiledWindow
	var m4 *TiledWindow
	var m5 *TiledWindow
	var m6 *TiledWindow

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		action   func()
		expected string
	}{
		{
			nil, `
AAAAAAAA
AAAAAAAA
AAAAAAAA
AAAAAAAA`,
		}, {
			func() { m1, err = m.SplitVertical(root, &testComponent{fill: 'B'}) }, `
AAAABBBB
AAAABBBB
AAAABBBB
AAAABBBB`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &testComponent{fill: 'C'}) }, `
AABBBCCC
AABBBCCC
AABBBCCC
AABBBCCC`,
		}, {
			func() { m3, err = m.SplitVertical(m2, &testComponent{fill: 'D'}) }, `
AABBCCDD
AABBCCDD
AABBCCDD
AABBCCDD`,
		}, {
			func() { m4, err = m.SplitVertical(m3, &testComponent{fill: 'E'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDEE
ABCCDDEE`,
		}, {
			func() { m5, err = m.SplitHorizontal(m4, &testComponent{fill: 'X'}) }, `
ABCCDDEE
ABCCDDEE
ABCCDDXX
ABCCDDXX`,
		}, {
			func() { m6, err = m.SplitHorizontal(m2, &testComponent{fill: 'Z'}) }, `
ABCCDDEE
ABCCDDEE
ABZZDDXX
ABZZDDXX`,
		}, {
			func() { err = m.Close(m5) }, `
ABCCDDEE
ABCCDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { err = m.Close(m2) }, `
ABZZDDEE
ABZZDDEE
ABZZDDEE
ABZZDDEE`,
		}, {
			func() { err = m.Close(root) }, `
BBZZDDEE
BBZZDDEE
BBZZDDEE
BBZZDDEE`,
		}, {
			func() { err = m.Close(m1) }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { m1, err = m.SplitHorizontal(m3, &testComponent{fill: 'A'}) }, `
ZZDDDEEE
ZZDDDEEE
ZZAAAEEE
ZZAAAEEE`,
		}, {
			func() { err = m.Close(m1) }, `
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE
ZZDDDEEE`,
		}, {
			func() { err = m.Close(m3) }, `
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE
ZZZZEEEE`,
		}, {
			func() { err = m.Close(m4) }, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		}, {
			func() { m1, err = m.SplitHorizontal(m6, &testComponent{fill: 'Y'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { m2, err = m.SplitVertical(m1, &testComponent{fill: 'X'}) }, `
ZZZZZZZZ
ZZZZZZZZ
YYYYXXXX
YYYYXXXX`,
		}, {
			func() { err = m.Resize(16, 4); w = writer.String(16, 4) }, `
ZZZZZZZZZZZZZZZZ
ZZZZZZZZZZZZZZZZ
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { err = m.Close(m6) }, `
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX
YYYYYYYYXXXXXXXX`,
		}, {
			func() { err = m.Resize(4, 2); w = writer.String(4, 2) }, `
YYXX
YYXX`,
		}, {
			func() { err = m.Close(m1) }, `
XXXX
XXXX`,
		},
	}

	for _, tcase := range tests {
		if err = w.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		if tcase.action != nil {
			tcase.action()
		}

		if err != nil {
			t.Fatal(err)
		}

		if err := m.Draw(w); err != nil {
			t.Fatal(err)
		}

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.expected, "\n")
		if expected != w.String() {
			t.Errorf("expected %q found %q", expected, w.String())
		}
	}
}
