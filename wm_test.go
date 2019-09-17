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

func prepareTest(width, height int, border bool, root Handler) (*stringWriter, *WindowManager) {
	writer := newStringWriter(width, height)
	handler := NewWindowManager(root, border)
	handler.Resize(width, height)

	return writer, handler
}

func TestWindowManagerSetFocus(t *testing.T) {
	width, height := 8, 4
	_, handler := prepareTest(width, height, false, NewTestHandler())
	right := handler.SplitHorizontal(NewTestHandler())

	handler.SetFocus(right)

	if focus := handler.Focus(); focus != right {
		t.Errorf("focus should be %+v, instead of %+v", right, focus)
	}
}

// TestHandler signals that it's handling event by incrementing it's fill rune
func TestWindowManagerHandle(t *testing.T) {
	topLeftHandler := NewTestHandler()
	width, height := 8, 4
	writer, handler := prepareTest(width, height, false, topLeftHandler)

	bottomLeftHandler := NewTestHandler()
	bottomleft := handler.SplitHorizontal(bottomLeftHandler)

	topRightHandler := NewTestHandler()
	_ = handler.SplitVertical(topRightHandler)

	topLeft := handler.SetFocus(bottomleft)
	bottomRightHandler := NewTestHandler()
	_ = handler.SplitVertical(bottomRightHandler)

	handler.SetFocus(topLeft)

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
	writer, handler := prepareTest(width, height, true, leftHandler)

	rightHandler := NewTestHandler()
	_ = handler.SplitVertical(rightHandler)

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
}
