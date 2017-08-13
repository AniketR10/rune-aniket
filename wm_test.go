package fractal

import (
	"termbox"
	"testing"
)

func altEvent(ch rune) termbox.Event {
	return termbox.Event{
		Type: termbox.EventKey,
		Mod:  termbox.ModAlt,
		Ch:   ch,
	}
}

func prepareTest(width, height int, border bool, root Handler) (*StringWriter, *Tile, *WindowManager) {
	writer := NewStringWriter(width, height)
	handler, tile := NewWindowManager(root, border)

	err := handler.Resize(width, height)
	if err != nil {
		panic(err)
	}

	return writer, tile, handler
}

func TestWindowManagerSetFocus(t *testing.T) {
	width, height := 8, 4
	_, left, handler := prepareTest(width, height, false, NewTestHandler())
	right, err := handler.SplitHorizontal(NewTestHandler())

	if err != nil {
		t.Fatal(err)
	}

	if focus := handler.Focus(); focus != left {
		t.Errorf("focus should be %+v, instead of %+v", left, focus)
	}

	handler.SetFocus(right)

	if focus := handler.Focus(); focus != right {
		t.Errorf("focus should be %+v, instead of %+v", right, focus)
	}
}

// TestHandler signals that it's handling event by incrementing it's fill rune
func TestWindowManagerHandle(t *testing.T) {
	topLeftHandler := NewTestHandler()
	width, height := 8, 4
	writer, topleft, handler := prepareTest(width, height, false, topLeftHandler)

	bottomLeftHandler := NewTestHandler()
	bottomleft, err := handler.SplitHorizontal(bottomLeftHandler)
	if err != nil {
		t.Fatal(err)
	}

	topRightHandler := NewTestHandler()
	_, err2 := handler.SplitVertical(topRightHandler)
	if err2 != nil {
		t.Fatal(err2)
	}

	handler.SetFocus(bottomleft)
	bottomRightHandler := NewTestHandler()
	_, err3 := handler.SplitVertical(bottomRightHandler)
	if err3 != nil {
		t.Fatal(err3)
	}

	handler.SetFocus(topleft)

	cases := []handlerTestCase{
		{
			termbox.Event{}, `
BBBBAAAA
BBBBAAAA
AAAAAAAA
AAAAAAAA`,
		},
		{
			altEvent('l'), `
BBBBAAAA
BBBBAAAA
AAAAAAAA
AAAAAAAA`,
		},
		{
			termbox.Event{}, `
BBBBBBBB
BBBBBBBB
AAAAAAAA
AAAAAAAA`,
		},
		{
			altEvent('l'), `
BBBBBBBB
BBBBBBBB
AAAAAAAA
AAAAAAAA`,
		},
		{
			termbox.Event{}, `
BBBBCCCC
BBBBCCCC
AAAAAAAA
AAAAAAAA`,
		},
		{
			altEvent('h'), `
BBBBCCCC
BBBBCCCC
AAAAAAAA
AAAAAAAA`,
		},
		{
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
AAAAAAAA
AAAAAAAA`,
		},
		{
			altEvent('j'), `
CCCCCCCC
CCCCCCCC
AAAAAAAA
AAAAAAAA`,
		},
		{
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBAAAA
BBBBAAAA`,
		},
		{
			altEvent('l'), `
CCCCCCCC
CCCCCCCC
BBBBAAAA
BBBBAAAA`,
		},
		{
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBBBBB
BBBBBBBB`,
		},
		{
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBCCCC
BBBBCCCC`,
		},
		{
			altEvent('k'), `
CCCCCCCC
CCCCCCCC
BBBBCCCC
BBBBCCCC`,
		},
		{
			termbox.Event{}, `
CCCCDDDD
CCCCDDDD
BBBBCCCC
BBBBCCCC`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)

	topRightHandler.Exit = true

	cases = []handlerTestCase{
		{
			// testhandler will return after this event active = false
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBCCCC
BBBBCCCC`,
		},
		{
			termbox.Event{}, `
DDDDDDDD
DDDDDDDD
BBBBCCCC
BBBBCCCC`,
		},
		{
			altEvent('j'), `
DDDDDDDD
DDDDDDDD
BBBBCCCC
BBBBCCCC`,
		},
		{
			altEvent('h'), `
DDDDDDDD
DDDDDDDD
BBBBCCCC
BBBBCCCC`,
		},
		{
			termbox.Event{}, `
DDDDDDDD
DDDDDDDD
CCCCCCCC
CCCCCCCC`,
		},
		{
			termbox.Event{}, `
DDDDDDDD
DDDDDDDD
DDDDCCCC
DDDDCCCC`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)

	bottomLeftHandler.Exit = true

	cases = []handlerTestCase{
		{
			// testhandler will return after this event active = false
			termbox.Event{}, `
DDDDDDDD
DDDDDDDD
CCCCCCCC
CCCCCCCC`,
		},
		{
			altEvent('k'), `
DDDDDDDD
DDDDDDDD
CCCCCCCC
CCCCCCCC`,
		},
		{
			termbox.Event{}, `
EEEEEEEE
EEEEEEEE
CCCCCCCC
CCCCCCCC`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)

	topLeftHandler.Exit = true

	cases = []handlerTestCase{
		{
			// testhandler will return after this event active = false
			termbox.Event{}, `
CCCCCCCC
CCCCCCCC
CCCCCCCC
CCCCCCCC`,
		},
		{
			termbox.Event{}, `
DDDDDDDD
DDDDDDDD
DDDDDDDD
DDDDDDDD`,
		},
		{
			altEvent('k'), `
DDDDDDDD
DDDDDDDD
DDDDDDDD
DDDDDDDD`,
		},
		{
			altEvent('l'), `
DDDDDDDD
DDDDDDDD
DDDDDDDD
DDDDDDDD`,
		},
		{
			termbox.Event{}, `
EEEEEEEE
EEEEEEEE
EEEEEEEE
EEEEEEEE`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)
}

func TestWindowManagerHandleBorder(t *testing.T) {
	leftHandler := NewTestHandler()
	width, height := 12, 4
	writer, _, handler := prepareTest(width, height, true, leftHandler)

	rightHandler := NewTestHandler()
	_, err := handler.SplitVertical(rightHandler)
	if err != nil {
		t.Fatal(err)
	}

	cases := []handlerTestCase{
		{
			termbox.Event{}, `
┌────┐┌────┐
│BBBB││AAAA│
│BBBB││AAAA│
└────┘└────┘`,
		},
		{
			altEvent('l'), `
┌────┐┌────┐
│BBBB││AAAA│
│BBBB││AAAA│
└────┘└────┘`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)

	// TODO check that border for focus window is highlighted
	// cells := writer.Cells()

	// for i, cell := range writer.Cells() {
	// 	if i ==
	// }
}
