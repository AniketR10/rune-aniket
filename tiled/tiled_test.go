package tiled

import (
	"testing"

	"github.com/ernestrc/fractal"
)

type noopWindow struct {
	fill                rune
	x, y, width, height int
}

func (t *noopWindow) Resize(width, height int) (err error) {
	t.width, t.height = width, height
	return nil
}

func (t *noopWindow) Move(x, y int) error {
	t.x, t.y = x, y
	return nil
}

func (t *noopWindow) Draw(w fractal.Writer) (err error) {
	return nil
}

func (t *noopWindow) Height() int {
	return t.height
}

func (t *noopWindow) Width() int {
	return t.width
}

func (t *noopWindow) Position() (int, int) {
	return t.x, t.y
}

func TestNew(t *testing.T) {
	m, _, e := New(10, 10, &noopWindow{})
	if e != nil {
		t.Fatal(e)
	}

	if m.focus != m.root.tiles[0] || m.height != 10 || m.width != 10 {
		t.Errorf("not initialized correcty: %+v", m)
	}

	if m.root.Height() != 10 || m.root.Width() != 10 {
		t.Errorf("root window not initialized correctly: %+v", m.root)
	}
}

func TestGetFocus(t *testing.T) {
	m, _, e := New(0, 0, &noopWindow{})
	if e != nil {
		t.Fatal(e)
	}

	if m.GetFocus() != m.root.tiles[0] {
		t.Errorf("focus is not the root window: %+v", m)
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

	if m, root, e = New(width, height, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitVertical(root, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 50, height)
	testWindowSize(t, w1, 50, height)
	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 50, 0)

	var w2 *TiledWindow
	if w2, e = m.SplitVertical(w1, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 33, height)
	testWindowSize(t, w1, 33, height)
	testWindowSize(t, w2, 33, height)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 33, 0)
	testWindowPos(t, w2, 66, 0)

	var w3 *TiledWindow
	if w3, e = m.SplitVertical(root, &noopWindow{}); e != nil {
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

	if m, root, e = New(width, height, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitHorizontal(root, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, width, 50)
	testWindowSize(t, w1, width, 50)
	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 50)

	var w2 *TiledWindow
	if w2, e = m.SplitHorizontal(w1, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, width, 33)
	testWindowSize(t, w1, width, 33)
	testWindowSize(t, w2, width, 33)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w1, 0, 33)
	testWindowPos(t, w2, 0, 66)

	var w3 *TiledWindow
	if w3, e = m.SplitHorizontal(root, &noopWindow{}); e != nil {
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

	if m, root, e = New(width, height, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitVertical(root, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w3, e = m.SplitVertical(root, &noopWindow{}); e != nil {
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

	if w4, e = m.SplitHorizontal(root, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, root, 25, 50)
	testWindowSize(t, w4, 25, 50)

	testWindowPos(t, root, 0, 0)
	testWindowPos(t, w4, 0, 50)

	if w5, e = m.SplitHorizontal(w1, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w6, e = m.SplitHorizontal(w1, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, w1, 25, 33)
	testWindowSize(t, w5, 25, 33)
	testWindowSize(t, w6, 25, 33)

	testWindowPos(t, w1, 25, 0)
	testWindowPos(t, w5, 25, 33)
	testWindowPos(t, w6, 25, 66)
}

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	if m, root, e := New(width, height, &noopWindow{}); e != nil {
		t.Fatal(e)
	} else {
		w1, _ := m.SplitHorizontal(root, &noopWindow{})
		w2, _ := m.SplitVertical(w1, &noopWindow{})

		testWindowSize(t, root, 1, 0) // FIXME should be 1/1
		testWindowSize(t, w1, 0, 0)
		testWindowSize(t, w2, 0, 0)

		testWindowPos(t, root, 0, 0)
		testWindowPos(t, w1, 0, 0)
		testWindowPos(t, w2, 0, 0)
	}
}

func setupTestCase(t *testing.T, gwidth, gheight int) (m *WindowManager, root *TiledWindow, w1 *TiledWindow, w2 *TiledWindow) {
	var e error

	if m, root, e = New(gwidth, gheight, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitHorizontal(root, &noopWindow{}); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1, &noopWindow{}); e != nil {
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
	testWindowSize(t, w1, 1, 1)
	testWindowSize(t, w2, 1, 1)

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
	testWindowSize(t, w1, 1, 1)
	testWindowSize(t, w2, 1, 1)

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
