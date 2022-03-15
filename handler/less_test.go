package handler

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
	"github.com/stretchr/testify/require"
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

func setup(t *testing.T, less *Less, width, height int) (*Less, *term.StringWriter) {
	if less == nil {
		less = NewLess(DefaultLessConfig())
	} else {
		less.Init(DefaultLessConfig())
	}

	_, err := less.Buffer().ReadFrom(strings.NewReader(content))
	require.NoError(t, err)

	less.Resize(width, height)

	return less, term.NewStringWriter(width, height)
}

func TestLessHandle(t *testing.T) {
	cases := getLessHandleTestFlow([19]term.Event{
		term.Event{},
		term.Event{Ch: 'k', Type: term.EventKey},
		term.Event{Ch: 'j', Type: term.EventKey},
		term.Event{Ch: 'h', Type: term.EventKey},
		term.Event{Ch: 'l', Type: term.EventKey},
		term.Event{Ch: '$', Type: term.EventKey},
		term.Event{Ch: '0', Type: term.EventKey},
		term.Event{Ch: 'G', Type: term.EventKey},
		term.Event{Ch: 'g', Type: term.EventKey},
		term.Event{Ch: '/', Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Key: term.KeyBackspace, Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Key: term.KeyEnter, Type: term.EventKey},
		term.Event{Ch: 'g', Type: term.EventKey},
		term.Event{Ch: 'N', Type: term.EventKey},
		term.Event{Ch: 'n', Type: term.EventKey},
		term.Event{},
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
CCDDDDD 
EEFFFFF 
GGHHHHH 
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
