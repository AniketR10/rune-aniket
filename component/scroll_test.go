package component

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

const (
	fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein ⌘⌘`
	wrapCopy = `module github.com/ernestrc/blue

go 1.14

require (
	cloud.google.com/go v0.63.0 // indirect
	cloud.google.com/go/firestore v1.2.0
	github.com/adrianmo/go-nmea v1.2.0
	github.com/ernestrc/go-multierror v1.1.2 // indirect
	github.com/ernestrc/logd-go v0.0.0-20180509171507-65871c1d5504
	github.com/ernestrc/sensible v0.0.0-20170704153812-102a955adfdf
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/jacobsa/go-serial v0.0.0-20180131005756-15cf729a72d4
)`
)

var fortunewidth = 44

func newScroll(tabspaces int, wrap bool, width, height int) (scroll *Scroll) {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(tabspaces)
	scroll = NewScroll(buf)
	scroll.Wrap = wrap
	scroll.Resize(width, height)
	return
}

func newScrollWrapTestCase(t *testing.T, width, height int) (*Scroll, *term.StringWriter) {
	tabspaces := 4
	wrap := true
	scroll := newScroll(tabspaces, wrap, width, height)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(wrapCopy))
	require.NoError(t, err)

	w := term.NewStringWriter(width, height)
	return scroll, w
}

func TestScrollNew(t *testing.T) {
	scroll := newScroll(5, true, 100, 100)
	assert.True(t, scroll.Wrap)
	assert.Equal(t, 100, scroll.width)
	assert.Equal(t, 100, scroll.height)
}

func TestScrollWordAt(t *testing.T) {
	var fortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it_away.
		-- Oscar Hammerstein ⌘⌘
`
	scroll := newScroll(4, false, 100, 100)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)

	tsuite := []struct {
		in      term.Coordinates
		wantOut string
	}{
		{term.Coordinates{}, "Love"},
		{term.Coordinates{X: 4}, ""},
		{term.Coordinates{X: 6}, "in"},
		{term.Coordinates{X: 7}, ""},
		{term.Coordinates{X: 8}, "your"},
		{term.Coordinates{X: 23}, ""},
		{term.Coordinates{X: 39}, "stay"},
		{term.Coordinates{X: 43}, ""},
		{term.Coordinates{Y: 1}, "Love"},
		{term.Coordinates{Y: 2, X: 11}, "Oscar"},
		{term.Coordinates{X: 999}, ""},
		{term.Coordinates{X: -1}, ""},
		{term.Coordinates{Y: 5}, ""},
		{term.Coordinates{Y: 1, X: 32}, "it_away"},
	}

	for _, tcase := range tsuite {
		start, end, out := scroll.WordAt(tcase.in)
		assert.Equal(t, tcase.wantOut, out)
		// start, end used in a select statement should return
		// return string
		if out != "" {
			cells, ok := scroll.Buffer().Select(start, end)
			require.True(t, ok)
			assert.Equal(t, tcase.wantOut, cell.CellsToString(cells))
		}
	}
}

func TestScrollDraw(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := false
	scroll := newScroll(tabspaces, wrap, width, height)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)

	var dispatchedSubscribe int
	var prevAt term.Coordinates
	scroll.Subscribe(FuncScrollSubscriber(func(from, to term.Coordinates) {
		dispatchedSubscribe++
		require.Equal(t, prevAt, from, scroll.Buffer().String())
		prevAt = to
	}))

	w := term.NewStringWriter(width, height)

	tests := []testutil.ComponentTestCase{
		{nil, "Love in \nLove isn"},
		{func() { assert.False(t, scroll.SeekUp()) }, "Love in \nLove isn"},
		{func() { assert.False(t, scroll.SeekLeft()) }, "Love in \nLove isn"},
		{func() { assert.True(t, scroll.SeekRight()) }, "ove in y\nove isn'"},
		{func() { assert.True(t, scroll.SeekLeft()) }, "Love in \nLove isn"},
		{func() { assert.True(t, scroll.SeekDown()) }, "Love isn\n        "},
		{func() { assert.False(t, scroll.SeekDown()) }, "Love isn\n        "},
		{func() { assert.True(t, scroll.SeekRight()) }, "ove isn'\n       -"},
		{func() { assert.True(t, scroll.SeekStartFile()) }, "ove in y\nove isn'"},
		{func() { assert.True(t, scroll.SeekEndFile()) }, "ove isn'\n       -"},
		{func() { assert.True(t, scroll.SeekStartLine()) }, "Love isn\n        "},
		// 11
		{func() { assert.True(t, scroll.SeekStartFile()) }, "Love in \nLove isn"},
		{func() { scroll.Search("Love") }, "Love in \nLove isn"},
		{func() { assert.False(t, scroll.SeekNextResult()) }, "Love in \nLove isn"},
		{func() { scroll.Resize(20, 1); w = term.NewStringWriter(20, 1) }, "Love in your heart w"},
		{func() { scroll.Search("you") }, "Love in your heart w"},
		{func() { assert.False(t, scroll.SeekNextResult()) }, "Love in your heart w"},
		{func() { assert.True(t, scroll.SeekNextResult()) }, " isn't love 'til you"},
		{func() { assert.True(t, scroll.SeekPrevResult()) }, " in your heart wasn'"},
		{func() { assert.Equal(t, 1, scroll.Search("⌘⌘")); assert.True(t, scroll.SeekNextResult()) }, "Oscar Hammerstein ⌘⌘"},
		{func() { assert.Equal(t, 2, scroll.Search("⌘")); assert.False(t, scroll.SeekNextResult()) }, "Oscar Hammerstein ⌘⌘"},
		// 21
		{func() { scroll.Search("Oscar"); assert.False(t, scroll.SeekNextResult()) }, "Oscar Hammerstein ⌘⌘"},
		{func() { assert.True(t, scroll.SeekStartFile()); assert.True(t, scroll.SeekStartLine()) }, "Love in your heart w"},
		{func() { scroll.Buffer().DeleteCell(term.Coordinates{X: 0, Y: 0}) }, "ove in your heart wa"},
		{func() { scroll.Buffer().DeleteCell(term.Coordinates{X: 14, Y: 0}) }, "ove in your hert was"},
		{func() { assert.True(t, scroll.SeekDown()) }, "Love isn't love 'til"},
		{func() { scroll.Buffer().DeleteCell(term.Coordinates{X: 16, Y: 1}) }, "Love isn't love til "},
		// note that there's a "space" after 中 that's because scroll skips drawing the second cell
		// in the double cell rune.
		{func() { scroll.Buffer().Insert(term.Coordinates{X: 16, Y: 1}, '中') }, "Love isn't love 中 ti"},
		{func() {
			buf := cell.NewBuffer()
			buf.WriteString("aa\nbb\ncc")
			scroll.searcher.Reset()
			scroll.offset = term.Coordinates{Y: 1}
			scroll.searchText = nil
			scroll.initBuffer(buf)
		}, "bb                  "},
		{func() {
			assert.Equal(t, 1, scroll.Search("aa"))
			assert.True(t, scroll.SeekNextResult())
		}, "aa                  "},
		{func() {
			assert.True(t, scroll.SetOffset(term.Coordinates{Y: 1}))
		}, "bb                  "},
		// {scroll.SeekDown, "        -- Oscar Ham"},
		// {scroll.SeekEndLine, "rstein 中            "},
		// {func() { scroll.Insert(term.Coordinates{X: 20, Y: 2}, '中') }, "rstein 中中           "},
	}

	for i, tcase := range tests {
		w.Clear(term.Attributes{})
		if tcase.Action != nil {
			tcase.Action()
		}

		scroll.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, tcase.Expected, w.String(), "test case %d", i)
	}

	assert.Equal(t, 18, dispatchedSubscribe)
}

func TestScrollDrawWrap(t *testing.T) {
	width, height := 8, 2
	tabspaces := 4
	wrap := true
	scroll := newScroll(tabspaces, wrap, width, height)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)

	w := term.NewStringWriter(width, height)

	tests := []testutil.ComponentTestCase{
		{nil, "Love in \nyour hea"},
		{func() { scroll.SeekUp() }, "Love in \nyour hea"},
		{func() { scroll.SeekLeft() }, "Love in \nyour hea"},
		{func() { scroll.SeekRight() }, "Love in \nyour hea"},
		{func() { scroll.SeekLeft() }, "Love in \nyour hea"},
		{func() { scroll.SeekDown() }, "your hea\nrt wasn'"},
		{func() { scroll.SeekDown() }, "rt wasn'\nt put th"},
		{func() { scroll.SeekUp() }, "your hea\nrt wasn'"},
		{func() { scroll.SeekRight() }, "your hea\nrt wasn'"},
		{func() { scroll.SeekStartFile() }, "Love in \nyour hea"},
		{func() { scroll.SeekEndFile() }, " Hammers\ntein ⌘⌘ "},
		{func() { scroll.SeekStartLine() }, " Hammers\ntein ⌘⌘ "},
		{func() { scroll.SeekStartFile() }, "Love in \nyour hea"},
		{func() { scroll.Search("Love") }, "Love in \nyour hea"},
		{func() { scroll.SeekNextResult() }, "Love in \nyour hea"},
		{func() { scroll.Resize(20, 1); w.Resize(20, 1) }, "Love in your heart w"},
		{func() { scroll.Search("you") }, "Love in your heart w"},
		{func() { scroll.SeekNextResult() }, "Love in your heart w"},
	}

	testutil.TestComponent(t, scroll, w, tests)
}

func TestScrollDrawWrap2(t *testing.T) {
	width, height := 8, 1
	tabspaces := 4
	wrap := true
	scroll := newScroll(tabspaces, wrap, width, height)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)

	w := term.NewStringWriter(width, height)

	tests := []testutil.ComponentTestCase{
		{nil, "Love in "},
	}

	testutil.TestComponent(t, scroll, w, tests)
}

func TestScrollDraw3(t *testing.T) {
	width, height := 51, 17
	scroll, w := newScrollWrapTestCase(t, width, height)

	tests := []testutil.ComponentTestCase{
		{nil, `module github.com/ernestrc/blue                    
                                                   
go 1.14                                            
                                                   
require (                                          
    cloud.google.com/go v0.63.0 // indirect        
    cloud.google.com/go/firestore v1.2.0           
    github.com/adrianmo/go-nmea v1.2.0             
    github.com/ernestrc/go-multierror v1.1.2 // ind
irect                                              
    github.com/ernestrc/logd-go v0.0.0-201805091715
07-65871c1d5504                                    
    github.com/ernestrc/sensible v0.0.0-20170704153
812-102a955adfdf                                   
    github.com/golang/mock v1.4.4                  
    github.com/golang/protobuf v1.4.2              
    github.com/google/uuid v1.1.1                  `},
		{func() { assert.True(t, scroll.SeekDown()) }, `                                                   
go 1.14                                            
                                                   
require (                                          
    cloud.google.com/go v0.63.0 // indirect        
    cloud.google.com/go/firestore v1.2.0           
    github.com/adrianmo/go-nmea v1.2.0             
    github.com/ernestrc/go-multierror v1.1.2 // ind
irect                                              
    github.com/ernestrc/logd-go v0.0.0-201805091715
07-65871c1d5504                                    
    github.com/ernestrc/sensible v0.0.0-20170704153
812-102a955adfdf                                   
    github.com/golang/mock v1.4.4                  
    github.com/golang/protobuf v1.4.2              
    github.com/google/uuid v1.1.1                  
    github.com/jacobsa/go-serial v0.0.0-20180131005`},
	}

	testutil.TestComponent(t, scroll, w, tests)
}

func TestScrollDrawInvalidSize(t *testing.T) {
	constructors := []func(*testing.T, int, int) (*Scroll, *term.StringWriter){
		newScrollWrapTestCase, func(t *testing.T, width, height int) (*Scroll, *term.StringWriter) {
			w := term.NewStringWriter(width, height)
			return newScroll(4, false, width, height), w
		},
	}
	for i, constructor := range constructors {
		t.Run(fmt.Sprintf("%d: negative width is considered as 0", i),
			func(t *testing.T) {
				width, height := 10, 1
				scroll, w := constructor(t, width, height)
				scroll.Resize(-22, 1)

				tests := []testutil.ComponentTestCase{
					{nil, `          `},
				}
				testutil.TestComponent(t, scroll, w, tests)
			})
		t.Run(fmt.Sprintf("%d: negative height is considered as 0", i),
			func(t *testing.T) {
				width, height := 10, 1
				scroll, w := constructor(t, width, height)
				scroll.Resize(10, -1)

				tests := []testutil.ComponentTestCase{
					{nil, `          `},
				}
				testutil.TestComponent(t, scroll, w, tests)
			})
	}

}

func TestScrollWraps(t *testing.T) {

	t.Run("returns empty map if Draw has not been called yet", func(t *testing.T) {
		scroll, _ := newScrollWrapTestCase(t, 10, 10)
		assert.Zero(t, nil, scroll.Wraps())
	})
	t.Run("returns wrapped lines with at most 1 wrap", func(t *testing.T) {
		scroll, w := newScrollWrapTestCase(t, 51, 17)
		expected := []int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 1, 0}
		scroll.Draw(w)
		assert.Equal(t, expected, scroll.Wraps())
	})
	t.Run("returns wrapped lines with more than 1 wrap", func(t *testing.T) {
		scroll, w := newScrollWrapTestCase(t, 20, 20)
		expected := []int{1, 0, 0, 0, 0, 2, 1, 1, 2, 3, 3, 1, 1, 1, 3, 0}
		scroll.Draw(w)
		assert.Equal(t, expected, scroll.Wraps())
	})
}

func TestRowLastIndex(t *testing.T) {
	scroll := NewScroll(cell.NewBuffer())
	cases := []struct {
		content  string
		line     int
		expected int
	}{
		{fortune, 0, 44},
		{fortune, 1, 38},
		{fortune, 2, 31},
		{"\t\n1\t\t\t222\n\n\n4\n", 0, 4},
		{"\t\n1\t\t\t222\n\n\n4\n", 1, 16},
		{"\t\n1\t\t\t222\n\n\n4\n", 2, 0},
		{"\t\n1\t\t\t222\n\n\n4\n", 3, 0},
		{"\t\n1\t\t\t222\n\n\n4\n", 4, 1},
	}

	for _, tcase := range cases {
		scroll.Buffer().Reset()
		_, err := scroll.Buffer().ReadFrom(strings.NewReader(tcase.content))
		require.NoError(t, err)
		i := scroll.Buffer().Columns(tcase.line)
		assert.Equal(t, tcase.expected, i)
	}
}

func TestScrollHeightNoWrap(t *testing.T) {
	scroll := newScroll(4, false, 20, 1)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)
	for _, width := range []int{0, 1, 10, 100} {
		assert.Equal(t, 3, scroll.Height(width))
	}
}

func TestScrollHeightWrap(t *testing.T) {
	scroll := newScroll(4, true, 20, 1)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)
	assert.Equal(t, 0, scroll.Height(0))
	assert.Equal(t, len(fortune), scroll.Height(1))
	assert.Equal(t, 13, scroll.Height(10))
	assert.Equal(t, 3, scroll.Height(100))
}

func TestScrollSeekTo(t *testing.T) {
	t.Run("no subscribers", func(t *testing.T) {
		scroll := newScroll(4, false, 20, 1)
		_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
		require.NoError(t, err)
		assert.True(t, scroll.SeekTo(term.Coordinates{Y: 100}))
		assert.Equal(t, term.Coordinates{Y: scroll.Buffer().Rows() - 1}, scroll.Offset())
		assert.False(t, scroll.SeekTo(term.Coordinates{Y: 100}))
	})

	t.Run("dispatches OnWillSeek, OnDidSeek on Y changes", func(t *testing.T) {
		for _, wrap := range []bool{true, false} {
			t.Run(fmt.Sprintf("wrap: %v", wrap), func(t *testing.T) {
				scroll := newScroll(4, wrap, 10, 1)
				_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
				require.NoError(t, err)

				var calledWill, calledDid int
				subs := subscriber{
					expectWillSeek: func(from term.Coordinates) {
						calledWill++
						assert.Equal(t, term.Coordinates{}, from)
					},
					expectDidSeek: func(from, to term.Coordinates) {
						calledDid++
						assert.Equal(t, term.Coordinates{}, from)
						assert.Equal(t, term.Coordinates{Y: 2}, to)
					},
				}
				scroll.Subscribe(subs)
				assert.True(t, scroll.SeekTo(term.Coordinates{Y: 100}))
				assert.Equal(t, 1, calledWill)
				assert.Equal(t, 1, calledDid)
			})
		}
	})

	t.Run("dispatches OnWillSeek, OnDidSeek on X changes", func(t *testing.T) {
		scroll := newScroll(4, false, 20, 1)
		_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
		require.NoError(t, err)

		var calledWill, calledDid int
		subs := subscriber{
			expectWillSeek: func(from term.Coordinates) {
				calledWill++
				assert.Equal(t, term.Coordinates{}, from)
			},
			expectDidSeek: func(from, to term.Coordinates) {
				calledDid++
				assert.Equal(t, term.Coordinates{}, from)
				assert.Equal(t, term.Coordinates{X: 6}, to)
			},
		}
		scroll.Subscribe(subs)
		assert.True(t, scroll.SeekTo(term.Coordinates{X: 100}))
		assert.Equal(t, 1, calledWill)
		assert.Equal(t, 1, calledDid)
	})
}

func TestScrollSetOffset(t *testing.T) {
	scroll := newScroll(4, false, 20, 1)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)
	assert.True(t, scroll.SetOffset(term.Coordinates{Y: 100}))
	assert.Equal(t, term.Coordinates{Y: 100}, scroll.Offset())

	assert.True(t, scroll.SetOffset(term.Coordinates{Y: 2}))
	assert.Equal(t, term.Coordinates{Y: scroll.Buffer().Rows() - 1}, scroll.Offset())
}

func TestScrollDrawOffsetOOB(t *testing.T) {
	scroll := newScroll(4, false, 20, 1)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)
	assert.True(t, scroll.SeekEndFile())

	// modify buffer such that cells now are empty
	assert.True(t, scroll.Buffer().TruncateFrom(term.Coordinates{}))

	w := term.NewStringWriter(10, 10)
	assert.NotPanics(t, func() {
		// this should not panic
		scroll.Draw(w)
	})
}

func TestScrollDrawWrapZeroWidth(t *testing.T) {
	scroll := newScroll(4, true, 0, 0)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(fortune))
	require.NoError(t, err)
	w := term.NewStringWriter(10, 10)
	assert.NotPanics(t, func() {
		scroll.Draw(w)
	})
}

func TestScrollDrawWrapWithBufferUpdates(t *testing.T) {
	buf := cell.NewBuffer()
	b := NewScroll(buf)
	b.Wrap = true
	b.Resize(20, 4)

	w := term.NewStringWriter(20, 9)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() { b.Resize(20, 1) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() { b.Resize(20, 9) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				b.Buffer().WriteString("hello world")
				b.Resize(20, 1)
			}, `
hello world         
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				b.Resize(20, 9)
				b.Buffer().WriteString(". Let's test its responsiveness")
				b.Resize(20, 6)
			}, `
hello world. Let's t
est its responsivene
ss                  
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				b.Buffer().WriteString(". Let's test its responsiveness")
				b.Resize(20, 8)
			}, `
hello world. Let's t
est its responsivene
ss. Let's test its r
esponsiveness       
                    
                    
                    
                    
                    `,
		}, {
			func() {
				b.Buffer().WriteString(". Let's test its responsiveness")
				b.Resize(20, 9)
				b.Buffer().WriteString(". Let's test its scrolling. " +
					"Let's make it overflow below and wrap," +
					"which might just take a little bit of text.")
			}, `
hello world. Let's t
est its responsivene
ss. Let's test its r
esponsiveness. Let's
 test its responsive
ness. Let's test its
 scrolling. Let's ma
ke it overflow below
 and wrap,which migh`,
		}, {
			func() {
				// NOTE: seek ops do not detect wraps without a call to draw
				// between writing new data and attempting to seek.
				handled := b.SeekDown()
				require.True(t, handled)
				handled = b.SeekDown()
				require.True(t, handled)
			}, `
ss. Let's test its r
esponsiveness. Let's
 test its responsive
ness. Let's test its
 scrolling. Let's ma
ke it overflow below
 and wrap,which migh
t just take a little
 bit of text.       `,
		}, {
			func() {
				handled := b.SeekUp()
				require.True(t, handled)
				handled = b.SeekUp()
				require.True(t, handled)
			}, `
hello world. Let's t
est its responsivene
ss. Let's test its r
esponsiveness. Let's
 test its responsive
ness. Let's test its
 scrolling. Let's ma
ke it overflow below
 and wrap,which migh`,
		},
	}
	testutil.TestComponent(t, b, w, tests)
}

func newBigScroll(fortunes int) (scroll *Scroll) {
	scroll = NewScroll(cell.NewBuffer())
	for i := 0; i < fortunes; i++ {
		_, _ = scroll.Buffer().ReadFrom(strings.NewReader(fortune))
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

type subscriber struct {
	expectWillSeek func(term.Coordinates)
	expectDidSeek  func(from, to term.Coordinates)
}

func (s subscriber) OnWillSeek(from term.Coordinates) {
	s.expectWillSeek(from)
}
func (s subscriber) OnDidSeek(from, to term.Coordinates) {
	s.expectDidSeek(from, to)
}
