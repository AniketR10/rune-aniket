package viewer

import (
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/writer"
)

var fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

var fortune_width = 44

func newViewer(tabspaces int, wrap bool, width, height int) (buf *Buffer, viewer *Viewer) {
	buf = NewBuffer(0, 0)
	viewer = New(buf, width, height, tabspaces, wrap)
	return
}

func TestViewerNew(t *testing.T) {
	buf, viewer := newViewer(5, true, 100, 100)
	if viewer.cells == nil || viewer.buffer != buf ||
		viewer.wrap != true || viewer.Tabspaces != 5 ||
		viewer.width != 100 || viewer.height != 100 {
		t.Errorf("viewer not initialized properly: %+v", viewer)
	}
}

func TestViewerScan(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	buf, viewer := newViewer(tabspaces, false, width, height)
	buf.Write([]byte(fortune))
	if err := viewer.Scan(); err != nil {
		t.Fatal(err)
	}

	xexpt, yexpt := fortune_width-width, 1
	if viewer.xmaxoffset != xexpt || viewer.ymaxoffset != yexpt {
		t.Errorf("max offsets not correct: x: %d shouldbe %d, y: %d should be %d", viewer.xmaxoffset, xexpt, viewer.ymaxoffset, yexpt)
	}

	var loveL fractal.Cell

	loveL = viewer.cell(0)
	if loveL.Y != 0 || loveL.X != 0 || loveL.Ch != 'L' {
		t.Errorf("failed to set content: %+v: %c", loveL, loveL.Ch)
	}

	loveL = viewer.cell(fortune_width + 1)
	if loveL.Y != 1 || loveL.X != 0 || loveL.Ch != 'L' {
		t.Errorf("failed to scan newline: %+v: %c", loveL, loveL.Ch)
	}

	oscarO := viewer.cell(89)
	if rune(fortune[89]) != viewer.cell(89).Ch {
		t.Errorf("something is wrong with the cell mapping: fortune: %d, cell: %d", fortune[89], viewer.cell(89))
	}
	if oscarO.Y != 2 || oscarO.X != tabspaces*2+3 || oscarO.Ch != 'O' {
		t.Errorf("failed to scan newline: %+v: %c", oscarO, oscarO.Ch)
	}

	viewer.Resize(20, 1)
	xexpt2, yexpt2 := fortune_width-20, 2
	if viewer.xmaxoffset != xexpt2 || viewer.ymaxoffset != yexpt2 {
		t.Errorf("max offsets not correct: x: %d shouldbe %d, y: %d should be %d", viewer.xmaxoffset, xexpt2, viewer.ymaxoffset, yexpt2)
	}
}

func TestViewerDraw(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := false
	buf, viewer := newViewer(tabspaces, wrap, width, height)
	buf.Write([]byte(fortune))
	if err := viewer.Scan(); err != nil {
		t.Fatal(err)
	}

	w := writer.New(width, height)

	tests := []struct {
		action   func()
		expected string
	}{
		{nil, "Love in \nLove isn"},
		{viewer.MoveUp, "Love in \nLove isn"},
		{viewer.MoveLeft, "Love in \nLove isn"},
		{viewer.MoveRight, "ove in y\nove isn'"},
		{viewer.MoveLeft, "Love in \nLove isn"},
		{viewer.MoveDown, "Love isn\n        "},
		{viewer.MoveDown, "Love isn\n        "},
		{viewer.MoveRight, "ove isn'\n       -"},
		{viewer.MoveStartFile, "ove in y\nove isn'"},
		{viewer.MoveEndFile, "ove isn'\n       -"},
		{viewer.MoveStartLine, "Love isn\n        "},
		{viewer.MoveStartFile, "Love in \nLove isn"},
		{func() { viewer.SetPosition(1, 1) }, "        \n Love in"},
		{func() { viewer.SetPosition(0, 0) }, "Love in \nLove isn"},
		{func() { viewer.Search("Love") }, "Love in \nLove isn"},
		{viewer.MoveNextResult, "Love in \nLove isn"},
		{viewer.MovePrevResult, "Love in \nLove isn"},
		{func() { viewer.Resize(20, 1); w = writer.New(20, 1) }, "Love in your heart w"},
		{func() { viewer.Search("you") }, "Love in your heart w"},
		{viewer.MoveNextResult, "Love in your heart w"},
		{viewer.MoveNextResult, " isn't love 'til you"},
		{viewer.MovePrevResult, " in your heart wasn'"},
		{func() { viewer.Search("中国"); viewer.MoveNextResult() }, "Oscar Hammerstein 中国"},
		{func() { viewer.Search("Oscar"); viewer.MoveNextResult() }, "Oscar Hammerstein 中国"},
	}

	for _, tcase := range tests {
		w.Clear(0, 0)
		if tcase.action != nil {
			tcase.action()
		}

		if err := viewer.Draw(w); err != nil {
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
