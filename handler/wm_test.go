package handler

import (
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
)

func altEvent(ch rune) term.Event {
	return term.Event{
		Type: term.EventKey,
		Mod:  term.ModAlt,
		Ch:   ch,
	}
}

func prepareTest(width, height int, border bool, root fractal.Handler) (
	*term.StringWriter, *WindowManager,
) {
	writer := term.NewStringWriter(width, height)
	handler := NewWindowManager(root, border)
	handler.Resize(width, height)

	return writer, handler
}

func TestWindowManagerSetFocusBorder(t *testing.T) {
	testWindowManagerSetFocus(t, true)
}
func TestWindowManagerSetFocusNoBorder(t *testing.T) {
	testWindowManagerSetFocus(t, false)
}

func testWindowManagerSetFocus(t *testing.T, border bool) {
	width, height := 8, 4
	_, wm := prepareTest(width, height, border, NewTestHandler())
	right := wm.SplitHorizontal(NewTestHandler())

	wm.SetFocus(right)

	if focus := wm.Focus(); focus != right {
		t.Errorf("focus should be %+v, instead of %+v", right, focus)
	}
	_, ok := wm.FocusContent().(*TestHandler)
	assert.True(t, ok)
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
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBBBBB
BBBBBBBB`,
		},
		{
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
CCCCCCCC
CCCCCCCC
BBBBCCCC
BBBBCCCC`,
		},
		{
			term.Event{}, `
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
			term.Event{}, `
DDDDDDDD
DDDDDDDD
CCCCCCCC
CCCCCCCC`,
		},
		{
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
CCCCCCCC
CCCCCCCC
CCCCCCCC
CCCCCCCC`,
		},
		{
			term.Event{}, `
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
			term.Event{}, `
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
			term.Event{}, `
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
