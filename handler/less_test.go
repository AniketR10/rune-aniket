package handler

import (
	"strings"
	"testing"

	"github.com/ernestrc/tcell/v3"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

const content = `AAAAABBBBB
CCCCCDDDDD
EEEEEFFFFF
GGGGGHHHHH
IIIIIJJJJJ
KKKKXXLLLL
9999999999
8888888888
3333333333
11111111XX`

func TestLessDrawSuperimposedBar(t *testing.T) {
	b := NewLess(LessConfig{Wrap: true, SuperimposeMessage: true})
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
			func() { b.Resize(20, 9); b.SetMessage("P1Nav") }, `
                    
                    
                    
                    
                    
                    
                    
                    
               P1Nav`,
		}, {
			func() {
				b.Buffer().WriteString("hello world")
				b.Resize(20, 1)
			}, `
hello world    P1Nav
                    
                    
                    
                    
                    
                    
                    
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
                    
                    
               P1Nav
                    
                    
                    `,
		},
	}
	testutil.TestComponent(t, b, w, tests)
}

func TestLessDrawNoBarWrap(t *testing.T) {
	b := NewLess(LessConfig{Wrap: true, NoBar: true})
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
				b.Buffer().WriteString(". Let's test its scrolling. " +
					"Let's make it overflow below and wrap," +
					"which might just take a little bit of text.")
				b.Resize(20, 9)
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
				_, handled := b.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
				require.True(t, handled)
				_, handled = b.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
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
				_, handled := b.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
				require.True(t, handled)
				_, handled = b.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
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

func TestLessHandle(t *testing.T) {
	cases := getLessHandleTestFlow([19]term.Event{
		{},
		{Ch: 'k', Type: term.EventKey},
		{Ch: 'j', Type: term.EventKey},
		{Ch: 'h', Type: term.EventKey},
		{Ch: 'l', Type: term.EventKey},
		{Ch: '$', Type: term.EventKey},
		{Ch: '0', Type: term.EventKey},
		{Ch: 'G', Type: term.EventKey},
		{Ch: 'g', Type: term.EventKey},
		{Ch: '/', Type: term.EventKey},
		{Ch: 'X', Type: term.EventKey},
		{Key: term.KeyBackspace, Type: term.EventKey},
		{Ch: 'X', Type: term.EventKey},
		{Ch: 'X', Type: term.EventKey},
		{Key: term.KeyEnter, Type: term.EventKey},
		{Ch: 'g', Type: term.EventKey},
		{Ch: 'N', Type: term.EventKey},
		{Ch: 'n', Type: term.EventKey},
		{},
	})
	testLessHandle(t, cases)
}

func getLessHandleTestFlow(events [19]term.Event) []testutil.HandlerTestCase {
	return []testutil.HandlerTestCase{
		{
			events[0], `
AAAAABBB
CCCCCDDD
EEEEEFFF
        `,
		},
		{
			events[1], `
AAAAABBB
CCCCCDDD
EEEEEFFF
        `,
		},
		{
			events[2], `
CCCCCDDD
EEEEEFFF
GGGGGHHH
        `,
		},
		{
			events[3], `
CCCCCDDD
EEEEEFFF
GGGGGHHH
        `,
		},
		{
			events[4], `
CCCCDDDD
EEEEFFFF
GGGGHHHH
        `,
		},
		{
			events[5], `
CCCDDDDD
EEEFFFFF
GGGHHHHH
        `,
		},
		{
			events[6], `
CCCCCDDD
EEEEEFFF
GGGGGHHH
        `,
		},
		{
			events[7], `
88888888
33333333
11111111
        `,
		},
		{
			events[8], `
AAAAABBB
CCCCCDDD
EEEEEFFF
        `,
		},
		{
			events[9], `
AAAAABBB
CCCCCDDD
EEEEEFFF
/       `,
		},
		{
			events[10], `
AAAAABBB
CCCCCDDD
EEEEEFFF
/X      `,
		},
		{
			events[11], `
AAAAABBB
CCCCCDDD
EEEEEFFF
/       `,
		},
		{
			events[12], `
AAAAABBB
CCCCCDDD
EEEEEFFF
/X      `,
		},
		{
			events[13], `
AAAAABBB
CCCCCDDD
EEEEEFFF
/XX     `,
		},
		{
			events[14], `
KKKKXXLL
99999999
88888888
        `,
		},
		{
			events[15], `
AAAAABBB
CCCCCDDD
EEEEEFFF
        `,
		},
		{
			events[16], `
111111XX
        
        
        `,
		},
		{
			events[17], `
KKXXLLLL
99999999
88888888
        `,
		},
	}
}

func testLessHandle(t *testing.T, cases []testutil.HandlerTestCase) {
	var less [2]Less
	var less1 *Less
	var writer1, writer2, writer3 *term.StringWriter
	_, writer1 = setup(t, &less[0], 8, 4)
	_, writer2 = setup(t, &less[1], 8, 4)
	less1, writer3 = setup(t, nil, 8, 4)

	// test cases with allocated less
	testutil.TestHandler(t, &less[0], cases, writer1)
	testutil.TestHandler(t, &less[1], cases, writer2)

	// test cases with stack less
	testutil.TestHandler(t, less1, cases, writer3)
}

func setup(t *testing.T, less *Less, width, height int) (*Less, *term.StringWriter) {
	cfg := DefaultLessConfig()
	cfg.BarAttr = term.Attributes{Bg: tcell.ColorBlack}
	if less == nil {
		less = NewLess(cfg)
	} else {
		less.Init(cfg)
	}

	_, err := less.Buffer().ReadFrom(strings.NewReader(content))
	require.NoError(t, err)

	less.Resize(width, height)

	return less, term.NewStringWriter(width, height)
}
