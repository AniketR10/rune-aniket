package fractal

import (
	"strings"
	"testing"
)

var fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

var fortunewidth = 44

func newScroll(tabspaces int, wrap bool, width, height int) (window *Scroll) {
	window = new(Scroll)
	window.Init()
	if err := window.Resize(width, height); err != nil {
		panic(err)
	}
	window.Tabspaces = tabspaces
	window.Wrap = wrap
	return
}

func TestScrollNew(t *testing.T) {
	window := newScroll(5, true, 100, 100)
	if window.cells == nil ||
		window.Wrap != true || window.Tabspaces != 5 ||
		window.width != 100 || window.height != 100 {
		t.Errorf("window not initialized properly: %+v", window)
	}
}

func TestScrollDraw(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := false
	window := newScroll(tabspaces, wrap, width, height)
	window.WriteStr(fortune)

	w := NewStringWriter(width, height)

	tests := []testCase{
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
		{func() { window.Resize(20, 1); w = NewStringWriter(20, 1) }, "Love in your heart w"},
		{func() { window.Search("you") }, "Love in your heart w"},
		{window.SeekNextResult, "Love in your heart w"},
		{window.SeekNextResult, " isn't love 'til you"},
		{window.SeekPrevResult, " in your heart wasn'"},
		{func() { window.Search("中国"); window.SeekNextResult() }, "Oscar Hammerstein 中国"},
		{func() { window.Search("Oscar"); window.SeekNextResult() }, "Oscar Hammerstein 中国"},
		{func() { window.SeekStartFile(); window.SeekStartLine() }, "Love in your heart w"},
		{func() { window.TruncateCellAt(Coordinates{X: 0, Y: 0}) }, "ove in your heart wa"},
		{func() { window.TruncateCellAt(Coordinates{X: 14, Y: 0}) }, "ove in your hert was"},
		{window.SeekDown, "Love isn't love 'til"},
		{func() { window.TruncateCellAt(Coordinates{X: 16, Y: 1}) }, "Love isn't love til "},
		{func() { window.InsertAt(Coordinates{X: 16, Y: 1}, '中') }, "Love isn't love 中til"},
		// {window.SeekDown, "        -- Oscar Ham"},
		// {window.SeekEndLine, "rstein 中            "},
		// {func() { window.InsertAt(Coordinates{X: 20, Y: 2}, '中') }, "rstein 中中           "},
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
	window := newScroll(tabspaces, wrap, width, height)
	window.WriteStr(fortune)

	w := NewStringWriter(width, height)

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

func TestScrollDrawPosition(t *testing.T) {
	width, height := 8, 4
	tabspaces := 4
	wrap := false
	window := newScroll(tabspaces, wrap, width, height)
	window.WriteStr("AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD")

	w := NewStringWriter(12, height)

	tests := []testCase{
		{
			nil, `
AAAAAAAA    
BBBBBBBB    
CCCCCCCC    
DDDDDDDD    `,
		},
		{
			func() { window.Move(1, 1) }, `
            
 AAAAAAAA   
 BBBBBBBB   
 CCCCCCCC   `,
		},
		{
			func() { window.Move(3, 0) }, `
   AAAAAAAA 
   BBBBBBBB 
   CCCCCCCC 
   DDDDDDDD `,
		},
	}

	testWorkflow(t, window, w, tests)
}

func TestRowLastIndex(t *testing.T) {
	scroll := NewScroll()
	cases := []struct {
		content  string
		line     int
		expected int
	}{
		{fortune, 0, 43},
		{fortune, 1, 37},
		{fortune, 2, 30},
		{"\t\n1\t\t\t222\n\n\n4\n", 0, 3},
		{"\t\n1\t\t\t222\n\n\n4\n", 1, 15},
		{"\t\n1\t\t\t222\n\n\n4\n", 2, 0},
		{"\t\n1\t\t\t222\n\n\n4\n", 3, 0},
		{"\t\n1\t\t\t222\n\n\n4\n", 4, 0},
	}

	for _, tcase := range cases {
		scroll.Reset()
		scroll.WriteStr(tcase.content)
		i, _ := scroll.RowLastIdx(tcase.line)
		lines := strings.Split(tcase.content, "\n")

		if i != tcase.expected {
			t.Errorf("expected last index of line \"%s\" to be %d instead of %d", lines[tcase.line], tcase.expected, i)
		}
	}
}
