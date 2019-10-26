package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
)

var fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国
`

var fortunewidth = 44

func newScroll(tabspaces int, wrap bool, width, height int) (scroll *Scroll) {
	scroll = new(Scroll)
	scroll.Init()
	scroll.Buffer().Init(tabspaces)
	scroll.Resize(width, height)
	scroll.Wrap = wrap
	return
}

func TestScrollNew(t *testing.T) {
	scroll := newScroll(5, true, 100, 100)
	if scroll.Wrap != true || scroll.buf.Tabspaces() != 5 ||
		scroll.width != 100 || scroll.height != 100 {
		t.Errorf("scroll not initialized properly: %+v", scroll)
	}
}

func TestScrollDraw(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := false
	scroll := newScroll(tabspaces, wrap, width, height)
	scroll.Buffer().WriteString(fortune)

	w := term.NewStringWriter(width, height)

	tests := []testCase{
		{nil, "Love in \nLove isn"},
		{func() { scroll.SeekUp() }, "Love in \nLove isn"},
		{func() { scroll.SeekLeft() }, "Love in \nLove isn"},
		{func() { scroll.SeekRight() }, "ove in y\nove isn'"},
		{func() { scroll.SeekLeft() }, "Love in \nLove isn"},
		{func() { scroll.SeekDown() }, "Love isn\n        "},
		{func() { scroll.SeekDown() }, "Love isn\n        "},
		{func() { scroll.SeekRight() }, "ove isn'\n       -"},
		{func() { scroll.SeekStartFile() }, "ove in y\nove isn'"},
		{func() { scroll.SeekEndFile() }, "ove isn'\n       -"},
		{func() { scroll.SeekStartLine() }, "Love isn\n        "},
		{func() { scroll.SeekStartFile() }, "Love in \nLove isn"},
		{func() { scroll.Search("Love") }, "Love in \nLove isn"},
		{func() { scroll.SeekNextResult() }, "Love in \nLove isn"},
		{func() { scroll.Resize(20, 1); w = term.NewStringWriter(20, 1) }, "Love in your heart w"},
		{func() { scroll.Search("you") }, "Love in your heart w"},
		{func() { scroll.SeekNextResult() }, "Love in your heart w"},
		{func() { scroll.SeekNextResult() }, " isn't love 'til you"},
		{func() { scroll.SeekPrevResult() }, " in your heart wasn'"},
		{func() { scroll.Search("中国"); scroll.SeekNextResult() }, "Oscar Hammerstein 中国"},
		{func() { scroll.Search("Oscar"); scroll.SeekNextResult() }, "Oscar Hammerstein 中国"},
		{func() { scroll.SeekStartFile(); scroll.SeekStartLine() }, "Love in your heart w"},
		{func() { scroll.Buffer().TruncateCellAt(term.Coordinates{X: 0, Y: 0}) }, "ove in your heart wa"},
		{func() { scroll.Buffer().TruncateCellAt(term.Coordinates{X: 14, Y: 0}) }, "ove in your hert was"},
		{func() { scroll.SeekDown() }, "Love isn't love 'til"},
		{func() { scroll.Buffer().TruncateCellAt(term.Coordinates{X: 16, Y: 1}) }, "Love isn't love til "},
		{func() { scroll.Buffer().InsertAt(term.Coordinates{X: 16, Y: 1}, '中') }, "Love isn't love 中til"},
		// {scroll.SeekDown, "        -- Oscar Ham"},
		// {scroll.SeekEndLine, "rstein 中            "},
		// {func() { scroll.InsertAt(term.Coordinates{X: 20, Y: 2}, '中') }, "rstein 中中           "},
	}

	for _, tcase := range tests {
		w.Clear(term.Attributes{})
		if tcase.action != nil {
			tcase.action()
		}

		scroll.Draw(w)

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
	scroll := newScroll(tabspaces, wrap, width, height)
	scroll.Buffer().ReadFrom(strings.NewReader(fortune))

	w := term.NewStringWriter(width, height)

	tests := []testCase{
		{nil, "Love in \nyour hea"},
		{func() { scroll.SeekUp() }, "Love in \nyour hea"},
		{func() { scroll.SeekLeft() }, "Love in \nyour hea"},
		{func() { scroll.SeekRight() }, "Love in \nyour hea"},
		{func() { scroll.SeekLeft() }, "Love in \nyour hea"},
		{func() { scroll.SeekDown() }, "Love isn\n't love "},
		{func() { scroll.SeekDown() }, "Love isn\n't love "},
		{func() { scroll.SeekRight() }, "Love isn\n't love "},
		{func() { scroll.SeekStartFile() }, "Love in \nyour hea"},
		{func() { scroll.SeekEndFile() }, "Love isn\n't love "},
		{func() { scroll.SeekStartLine() }, "Love isn\n't love "},
		{func() { scroll.SeekStartFile() }, "Love in \nyour hea"},
		{func() { scroll.Search("Love") }, "Love in \nyour hea"},
		{func() { scroll.SeekNextResult() }, "Love in \nyour hea"},
		{func() { scroll.Resize(20, 1); w.Resize(20, 1) }, "Love in your heart w"},
		{func() { scroll.Search("you") }, "Love in your heart w"},
		{func() { scroll.SeekNextResult() }, "Love in your heart w"},
	}

	testWorkflow(t, scroll, w, tests)
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
		scroll.Buffer().Reset()
		scroll.Buffer().WriteString(tcase.content)
		i, _ := scroll.Buffer().RowLastIdx(tcase.line)
		lines := strings.Split(tcase.content, "\n")

		if i != tcase.expected {
			t.Errorf("expected last index of line \"%s\" to be %d instead of %d", lines[tcase.line], tcase.expected, i)
		}
	}
}

func TestScrollInit(t *testing.T) {
	t.Run("does not mutate buffer on initialization", func(t *testing.T) {
		var buf cell.Buffer
		str := "hola"
		buf.Init(4)
		buf.WriteString(str)
		var scroll Scroll
		scroll.InitWithBuffer(&buf)
		if found := scroll.String(); found != str {
			t.Errorf("expected '%s' but found '%s'", str, found)
		}
	})
}

func newBigScroll(fortunes int) (scroll *Scroll) {
	scroll = NewScroll()
	for i := 0; i < fortunes; i++ {
		_ = scroll.Buffer().WriteString(fortune)
	}
	// assume big screen
	scroll.Resize(3000, 2000)
	return
}

func benchmarkScrollDraw(b *testing.B, fortunes int, offset float32) {
	scroll := newBigScroll(fortunes)
	seekPercRows(scroll, offset)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scroll.Draw(term.NoopWriter{})
	}
	// b.Logf("benchmark draw using payload of %d bytes\n", fortunes*len(fortune))
}

func seekPercRows(scroll *Scroll, offset float32) {
	offsetRows := int(float32(scroll.Buffer().Rows()) * offset)
	for i := 0; i < offsetRows; i++ {
		scroll.SeekDown()
	}
}

func benchmarkScrollWrapDraw(b *testing.B, fortunes int, offset float32) {
	scroll := newBigScroll(int(float32(fortunes) * (1 + offset)))
	scroll.Wrap = true
	seekPercRows(scroll, offset)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scroll.Draw(term.NoopWriter{})
	}
	// b.Logf("benchmark draw using payload of %d bytes\n", fortunes*len(fortune))
}

func BenchmarkScrollWrapDraw10(b *testing.B) {
	benchmarkScrollWrapDraw(b, 10, 0)
}
func BenchmarkScrollWrapDraw100(b *testing.B) {
	benchmarkScrollWrapDraw(b, 100, 0)
}
func BenchmarkScrollWrapDraw1000(b *testing.B) {
	benchmarkScrollWrapDraw(b, 1000, 0)
}
func BenchmarkScrollWrapDrawBigOffset1000(b *testing.B) {
	benchmarkScrollWrapDraw(b, 1000, 0.7)
}

func BenchmarkScrollDraw10(b *testing.B) {
	benchmarkScrollDraw(b, 10, 0)
}
func BenchmarkScrollDraw100(b *testing.B) {
	benchmarkScrollDraw(b, 100, 0)
}
func BenchmarkScrollDraw1000(b *testing.B) {
	benchmarkScrollDraw(b, 1000, 0)
}
func BenchmarkScrollDrawBigOffset1000(b *testing.B) {
	benchmarkScrollDraw(b, 1000, 0.7)
}

func BenchmarkScrollDraw100MB(b *testing.B) {
	benchmarkScrollDraw(b, 1000000, 0)
}
func BenchmarkScrollWrapDraw100MB(b *testing.B) {
	benchmarkScrollDraw(b, 1000000, 0)
}
