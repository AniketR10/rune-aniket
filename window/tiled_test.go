package window

import (
	"testing"

	"github.com/ernestrc/fractal/config"
)

var cfg *config.Config = config.New()

func TestNewManager(t *testing.T) {
	m, e := NewManager(cfg, 10, 10)
	if e != nil {
		t.Fatal(e)
	}

	if m.focus != m.Root() || m.config != cfg || m.tree == nil || m.height != 10 || m.width != 10 {
		t.Errorf("not initialized correcty: %+v", m)
	}

	if m.Root().Height() != 10 || m.Root().Width() != 10 {
		t.Errorf("root window not initialized correctly: %+v", m.tree)
	}
}

func TestGetFocus(t *testing.T) {
	m, e := NewManager(cfg, 0, 0)
	if e != nil {
		t.Fatal(e)
	}

	if m.GetFocus() != m.Root() {
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
	var e error

	width := 100
	height := 100

	if m, e = NewManager(cfg, width, height); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitVertical(nil); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), 50, height)
	testWindowSize(t, w1, 50, height)
	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 50, 0)

	var w2 *TiledWindow
	if w2, e = m.SplitVertical(w1); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), 50, height)
	testWindowSize(t, w1, 25, height)
	testWindowSize(t, w2, 25, height)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 50, 0)
	testWindowPos(t, w2, 75, 0)

	var w3 *TiledWindow
	if w3, e = m.SplitVertical(nil); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), 25, height)
	testWindowSize(t, w3, 25, height)
	testWindowSize(t, w1, 25, height)
	testWindowSize(t, w2, 25, height)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w3, 25, 0)
	testWindowPos(t, w1, 50, 0)
	testWindowPos(t, w2, 75, 0)
}

func TestSplitHorizontal(t *testing.T) {
	var m *WindowManager
	var e error

	width := 100
	height := 100

	if m, e = NewManager(cfg, width, height); e != nil {
		t.Fatal(e)
	}

	var w1 *TiledWindow
	if w1, e = m.SplitHorizontal(nil); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), width, 50)
	testWindowSize(t, w1, width, 50)
	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 50)

	var w2 *TiledWindow
	if w2, e = m.SplitHorizontal(w1); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), width, 50)
	testWindowSize(t, w1, width, 25)
	testWindowSize(t, w2, width, 25)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 50)
	testWindowPos(t, w2, 0, 75)

	var w3 *TiledWindow
	if w3, e = m.SplitHorizontal(nil); e != nil {
		t.Fatal(e)
	}

	testWindowSize(t, m.Root(), width, 25)
	testWindowSize(t, w3, width, 25)
	testWindowSize(t, w1, width, 25)
	testWindowSize(t, w2, width, 25)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w3, 0, 25)
	testWindowPos(t, w1, 0, 50)
	testWindowPos(t, w2, 0, 75)
}

func TestStackWhenNoSpace(t *testing.T) {
	width := 1
	height := 1

	if m, e := NewManager(cfg, width, height); e != nil {
		t.Fatal(e)
	} else {
		w1, _ := m.SplitHorizontal(nil)
		w2, _ := m.SplitVertical(nil)

		// keep size 1 so resizing up works
		testWindowSize(t, m.Root(), 1, 1)
		testWindowSize(t, w1, 1, 1)
		testWindowSize(t, w2, 1, 1)

		// keep relative position so that we
		// can resize up
		testWindowPos(t, m.Root(), 0, 0)
		testWindowPos(t, w1, 0, 1)
		testWindowPos(t, w2, 1, 1)
	}
}

func setupTestCase(t *testing.T, gwidth, gheight int) (m *WindowManager, w1 *TiledWindow, w2 *TiledWindow) {
	var e error

	if m, e = NewManager(cfg, gwidth, gheight); e != nil {
		t.Fatal(e)
	}

	if w1, e = m.SplitHorizontal(nil); e != nil {
		t.Fatal(e)
	}

	if w2, e = m.SplitVertical(w1); e != nil {
		t.Fatal(e)
	}

	return
}

func TestResize(t *testing.T) {
	m, w1, w2 := setupTestCase(t, 100, 100)

	testWindowSize(t, m.Root(), 100, 50)
	testWindowSize(t, w1, 50, 50)
	testWindowSize(t, w2, 50, 50)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 50)
	testWindowPos(t, w2, 50, 50)

	if err := m.Resize(50, 50); err != nil {
		t.Fatal(err)
	}

	testWindowSize(t, m.Root(), 50, 25)
	testWindowSize(t, w1, 25, 25)
	testWindowSize(t, w2, 25, 25)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 25)
	testWindowPos(t, w2, 25, 25)
}

func TestResizeRounding(t *testing.T) {
	m, w1, w2 := setupTestCase(t, 3, 3)
	testWindowSize(t, m.Root(), 3, 2)
	testWindowSize(t, w1, 2, 1)
	testWindowSize(t, w2, 1, 1)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 2)
	testWindowPos(t, w2, 2, 2)

	m.Resize(2, 2)
	testWindowSize(t, m.Root(), 2, 1)
	testWindowSize(t, w1, 1, 1)
	testWindowSize(t, w2, 1, 1)

	testWindowPos(t, m.Root(), 0, 0)
	testWindowPos(t, w1, 0, 1)
	testWindowPos(t, w2, 1, 1)

	// m.Resize(3, 3)
	// testWindowSize(t, m.Root(), 3, 2)
	// testWindowSize(t, w1, 2, 1)
	// testWindowSize(t, w2, 1, 1)

	// testWindowPos(t, m.Root(), 0, 0)
	// testWindowPos(t, w1, 0, 2)
	// testWindowPos(t, w2, 2, 2)

	// m.Resize(100, 100)
	// testWindowSize(t, m.Root(), 100, 50)
	// testWindowSize(t, w1, 50, 50)
	// testWindowSize(t, w2, 50, 50)

	// testWindowPos(t, m.Root(), 0, 0)
	// testWindowPos(t, w1, 0, 50)
	// testWindowPos(t, w2, 50, 50)
}
