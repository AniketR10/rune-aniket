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

func setup(less *Less, width, height int, handler LessHandler, config *LessConfig) (*Less, *StringWriter) {
	var err error

	if less == nil {
		less, err = NewLess(handler, config)
	} else {
		err = less.Init(handler, config)
	}

	if err != nil {
		panic(err)
	}

	if err = less.SetContent(content); err != nil {
		panic(err)
	}

	if err = less.Resize(width, height); err != nil {
		panic(err)
	}

	return less, NewStringWriter(width, height)
}

func TestLessHandle(t *testing.T) {
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

	var less [2]Less
	var less1 *Less
	var writer1, writer2, writer3 *StringWriter
	_, writer1 = setup(&less[0], 8, 4, nil, nil)
	_, writer2 = setup(&less[1], 8, 4, nil, nil)
	less1, writer3 = setup(nil, 8, 4, nil, nil)

	// test cases with allocated less
	testHandlerWorkflow(t, &less[0], cases, writer1)
	testHandlerWorkflow(t, &less[1], cases, writer2)

	// test cases with stack less
	testHandlerWorkflow(t, less1, cases, writer3)

	// setup new test case
	if err := less[0].SetMessage("hi"); err != nil {
		t.Fatal(err)
	}

	if err := less[0].SetContent("blonde"); err != nil {
		t.Fatal(err)
	}

	if err := less[0].Resize(7, 4); err != nil {
		t.Fatal(err)
	}

	if err := less[0].Move(1, 0); err != nil {
		t.Fatal(err)
	}

	cases = []handlerTestCase{
		{
			termbox.Event{}, `
 blonde 
        
        
 :    hi`,
		},
	}

	testHandlerWorkflow(t, &less[0], cases, writer3)

}
