package component

import (
	"testing"

	"github.com/ernestrc/fractal"
)

var fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

var fortune_width = 44

func newScroll(tabspaces int, wrap bool, width, height int) (buf *fractal.Buffer, window *Scroll) {
	buf = &fractal.Buffer{}
	window = NewScroll(buf, width, height, 0, 0)
	window.Tabspaces = tabspaces
	window.Wrap = wrap
	return
}

func TestScrollNew(t *testing.T) {
	buf, window := newScroll(5, true, 100, 100)
	if window.cells == nil || window.buffer != buf ||
		window.Wrap != true || window.Tabspaces != 5 ||
		window.width != 100 || window.height != 100 {
		t.Errorf("window not initialized properly: %+v", window)
	}
}

func TestScrollscan(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	buf, window := newScroll(tabspaces, false, width, height)
	buf.Write([]byte(fortune))
	if err := window.scan(); err != nil {
		t.Fatal(err)
	}

	xexpt, yexpt := fortune_width-width, 1
	if window.maxoffset.X != xexpt || window.maxoffset.Y != yexpt {
		t.Errorf("max offsets not correct: x: %d shouldbe %d, y: %d should be %d", window.maxoffset.X, xexpt, window.maxoffset.Y, yexpt)
	}

	var loveL fractal.Cell

	loveL = window.CellAt(0)
	if loveL.Y != 0 || loveL.X != 0 || loveL.Ch != 'L' {
		t.Errorf("failed to set content: %+v: %c", loveL, loveL.Ch)
	}

	loveL = window.CellAt(fortune_width + 1)
	if loveL.Y != 1 || loveL.X != 0 || loveL.Ch != 'L' {
		t.Errorf("failed to scan newline: %+v: %c", loveL, loveL.Ch)
	}

	oscarO := window.CellAt(89)
	if rune(fortune[89]) != window.CellAt(89).Ch {
		t.Errorf("something is wrong with the cell mapping: fortune: %d, cell: %d", fortune[89], window.CellAt(89))
	}
	if oscarO.Y != 2 || oscarO.X != tabspaces*2+3 || oscarO.Ch != 'O' {
		t.Errorf("failed to scan newline: %+v: %c", oscarO, oscarO.Ch)
	}

	window.Resize(20, 1)
	xexpt2, yexpt2 := fortune_width-20, 2
	if window.maxoffset.X != xexpt2 || window.maxoffset.Y != yexpt2 {
		t.Errorf("max offsets not correct: x: %d shouldbe %d, y: %d should be %d", window.maxoffset.X, xexpt2, window.maxoffset.Y, yexpt2)
	}
}

func TestScrollDraw(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := false
	buf, window := newScroll(tabspaces, wrap, width, height)
	buf.Write([]byte(fortune))
	if err := window.scan(); err != nil {
		t.Fatal(err)
	}

	w := fractal.String(width, height)

	tests := []struct {
		action   func()
		expected string
	}{
		{nil, "Love in \nLove isn"},
		{window.SeekUp, "Love in \nLove isn"},
		{window.SeekLeft, "Love in \nLove isn"},
		{window.SeekRight, "ove in y\nove isn'"},
		{window.SeekLeft, "Love in \nLove isn"},
		{window.SeekDown, "Love isn\n        "},
		{window.SeekDown, "Love isn\n        "},
		{window.SeekRight, "ove isn'\n       -"},
		{window.SeekStartFile, "ove in y\nove isn'"},
		{window.SeekEndFile, "ove isn'\n       -"},
		{window.SeekStartLine, "Love isn\n        "},
		{window.SeekStartFile, "Love in \nLove isn"},
		{func() { window.Move(1, 1) }, "        \n Love in"},
		{func() { window.Move(0, 0) }, "Love in \nLove isn"},
		{func() { window.Search("Love") }, "Love in \nLove isn"},
		{window.SeekNextResult, "Love in \nLove isn"},
		{window.SeekPrevResult, "Love in \nLove isn"},
		{func() { window.Resize(20, 1); w = fractal.String(20, 1) }, "Love in your heart w"},
		{func() { window.Search("you") }, "Love in your heart w"},
		{window.SeekNextResult, "Love in your heart w"},
		{window.SeekNextResult, " isn't love 'til you"},
		{window.SeekPrevResult, " in your heart wasn'"},
		{func() { window.Search("中国"); window.SeekNextResult() }, "Oscar Hammerstein 中国"},
		{func() { window.Search("Oscar"); window.SeekNextResult() }, "Oscar Hammerstein 中国"},
	}

	for _, tcase := range tests {
		w.Clear(0, 0)
		if tcase.action != nil {
			tcase.action()
		}

		if err := window.Draw(w); err != nil {
			t.Fatal(err)
		}

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		if w.String() != tcase.expected {
			t.Errorf("expected: %q; found: %q", tcase.expected, w.String())
		}
	}
}

func TestScrollDrawWrap(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := true
	buf, window := newScroll(tabspaces, wrap, width, height)
	buf.Write([]byte(fortune))
	if err := window.scan(); err != nil {
		t.Fatal(err)
	}

	w := fractal.String(width, height)

	tests := []testCase{
		{nil, "Love in \nyour hea"},
		{window.SeekUp, "Love in \nyour hea"},
		{window.SeekLeft, "Love in \nyour hea"},
		{window.SeekRight, "Love in \nyour hea"},
		{window.SeekLeft, "Love in \nyour hea"},
		{window.SeekDown, "Love isn\n't love "},
		{window.SeekDown, "Love isn\n't love "},
		{window.SeekRight, "Love isn\n't love "},
		{window.SeekStartFile, "Love in \nyour hea"},
		{window.SeekEndFile, "Love isn\n't love "},
		{window.SeekStartLine, "Love isn\n't love "},
		{window.SeekStartFile, "Love in \nyour hea"},
		{func() { window.Move(1, 1) }, "        \n Love in"},
		{func() { window.Move(0, 0) }, "Love in \nyour hea"},
		{func() { window.Search("Love") }, "Love in \nyour hea"},
		{window.SeekNextResult, "Love in \nyour hea"},
		{func() { window.Resize(20, 1); w.Resize(20, 1) }, "Love in your heart w"},
		{func() { window.Search("you") }, "Love in your heart w"},
		{window.SeekNextResult, "Love in your heart w"},
	}

	testWorkflow(t, window, w, tests)
}

func TestScrollNilBuffer(t *testing.T) {
	scroll := NewScroll(nil, 10, 10, 0, 0)
	scroll.SeekNextResult()
	scroll.SeekPrevResult()
	if r := scroll.Search("jfklwjl"); r != 0 {
		t.Errorf("unexpected result for search: %d", r)
	}
}

// func TestScrollGetCell(t *testing.T) {
// 	width, height := 8, 2
// 	tabspaces := 4
// 	buf, window := newScroll(tabspaces, false, width, height)
// 	buf.Write([]byte(fortune))
// 	if err := window.scan(); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	var loveL fractal.Cell
// 	var idx int
//
// 	idx, loveL = window.Cell(0, 0)
// 	if idx != 0 || loveL.Ch != 'L' {
// 		t.Errorf("failed to set content: %+v: %c", loveL, loveL.Ch)
// 	}
//
// 	idx, loveL = window.Cell(0, 1)
// 	if idx != fortune_width+1 || loveL.Ch != 'L' {
// 		t.Errorf("failed to scan newline: %+v: %c", loveL, loveL.Ch)
// 	}
//
// 	idx2, oscarO := window.Cell(2, tabspaces*2+3)
// 	if idx2 != 89 || oscarO.Ch != 'O' {
// 		t.Errorf("failed to scan newline: %+v: %c", oscarO, oscarO.Ch)
// 	}
// }

// TODO add tests for changing buffer
