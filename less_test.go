package fractal

import (
	"termbox"
	"testing"
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

func setup(width, height int, handler LessHandler, config *LessConfig) (*Less, *StringWriter) {
	less, err := NewLess(content, handler, config)
	if err != nil {
		panic(err)
	}

	if err = less.Resize(width, height); err != nil {
		panic(err)
	}

	return less, NewStringWriter(width, height)
}

func TestLessHandle(t *testing.T) {
	less, writer := setup(8, 4, nil, nil)

	cases := []handlerTestCase{
		{
			termbox.Event{}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
:       `,
		},
		{
			termbox.Event{Ch: 'k', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
:       `,
		},
		{
			termbox.Event{Ch: 'j', Type: termbox.EventKey}, `
CCCCCDDD
EEEEEFFF
GGGGGHHH
:       `,
		},
		{
			termbox.Event{Ch: 'h', Type: termbox.EventKey}, `
CCCCCDDD
EEEEEFFF
GGGGGHHH
:       `,
		},
		{
			termbox.Event{Ch: 'l', Type: termbox.EventKey}, `
CCCCDDDD
EEEEFFFF
GGGGHHHH
:       `,
		},
		{
			termbox.Event{Ch: '$', Type: termbox.EventKey}, `
CCCDDDDD
EEEFFFFF
GGGHHHHH
:       `,
		},
		{
			termbox.Event{Ch: '0', Type: termbox.EventKey}, `
CCCCCDDD
EEEEEFFF
GGGGGHHH
:       `,
		},
		{
			termbox.Event{Ch: 'G', Type: termbox.EventKey}, `
88888888
33333333
11111111
:       `,
		},
		{
			termbox.Event{Ch: 'g', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
:       `,
		},
		{
			termbox.Event{Ch: '/', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
/       `,
		},
		{
			termbox.Event{Ch: 'X', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
/X      `,
		},
		{
			termbox.Event{Key: termbox.KeyBackspace, Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
/       `,
		},
		{
			termbox.Event{Ch: 'X', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
/X      `,
		},
		{
			termbox.Event{Ch: 'X', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
/XX     `,
		},
		{
			termbox.Event{Key: termbox.KeyEnter, Type: termbox.EventKey}, `
KKKKXXLL
99999999
88888888
:       `,
		},
		{
			termbox.Event{Ch: 'g', Type: termbox.EventKey}, `
AAAAABBB
CCCCCDDD
EEEEEFFF
:       `,
		},
		{
			termbox.Event{Ch: 'N', Type: termbox.EventKey}, `
88888888
33333333
111111XX
:       `,
		},
		{
			termbox.Event{Ch: 'n', Type: termbox.EventKey}, `
KKXXLLLL
99999999
88888888
:       `,
		},
	}

	testHandlerWorkflow(t, less, cases, writer)

	if err := less.SetMessage("hi"); err != nil {
		t.Fatal(err)
	}

	if err := less.SetContent("blonde"); err != nil {
		t.Fatal(err)
	}

	if err := less.Resize(7, 4); err != nil {
		t.Fatal(err)
	}

	if err := less.Move(1, 0); err != nil {
		t.Fatal(err)
	}

	cases = []handlerTestCase{
		{
			termbox.Event{}, `
 blonde 
        
        
 :    hi`,
		},
	}

	testHandlerWorkflow(t, less, cases, writer)

}
