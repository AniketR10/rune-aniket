package handler

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func altEvent(ch rune) term.Event {
	return term.Event{
		Type: term.EventKey,
		Mod:  term.ModAlt,
		Ch:   ch,
	}
}

func prepareTest(width, height int, border bool, root tui.Handler) (
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

func TestWindowFocusInitSplitVertical(t *testing.T) {
	leftHandler := NewTestHandler()
	width, height := 12, 4
	_, m := prepareTest(width, height, true, leftHandler)

	_, ok := m.Focus().TileLeft()
	assert.False(t, ok)
	assert.False(t, m.ShiftFocus())

	rightHandler := NewTestHandler()
	_ = m.SplitVertical(rightHandler)

	assert.Equal(t, leftHandler, m.FocusContent())
	assert.True(t, m.ShiftFocus())
	assert.Equal(t, rightHandler, m.FocusContent())
	assert.True(t, m.ShiftFocus())
	assert.Equal(t, leftHandler, m.FocusContent())
}

func TestWindowManagerSetFocusContent(t *testing.T) {
	leftHandler := NewTestHandler()
	width, height := 8, 4
	writer, wm := prepareTest(width, height, true, leftHandler)

	rightHandler := NewTestHandler()
	_ = wm.SplitVertical(rightHandler)

	prev := wm.SetFocusContent(rightHandler)
	assert.Equal(t, prev, leftHandler)

	cases := []handlerTestCase{
		{
			term.Event{}, `
┌──┐┌──┐
│BB││BB│
│BB││BB│
└──┘└──┘`,
		},
		{
			altEvent('l'), `
┌──┐┌──┐
│BB││BB│
│BB││BB│
└──┘└──┘`,
		},
		{
			term.Event{}, `
┌──┐┌──┐
│CC││CC│
│CC││CC│
└──┘└──┘`,
		},
	}

	testHandlerWorkflow(t, wm, cases, writer)

	fb := component.DefaultFrameBorders()
	fb.TopLeft.Ch = '╔'
	fb.BottomRight.Ch = '╝'
	fb.BottomLeft.Ch = '╚'
	fb.TopRight.Ch = '╗'

	fb.Vertical.Ch = '║'
	fb.Horizontal.Ch = '═'

	wm.SetFrameBorders(fb)

	cases = []handlerTestCase{
		{
			term.Event{}, `
╔══╗╔══╗
║DD║║DD║
║DD║║DD║
╚══╝╚══╝`,
		},
	}

	testHandlerWorkflow(t, wm, cases, writer)
}

func TestWindowManagerInit(t *testing.T) {
	wm := NewWindowManager(NewTestHandler(), false)
	require.NotNil(t, wm.Focus())
}

func TestWindowManagerSetAttr(t *testing.T) {
	wm := NewWindowManager(NewTestHandler(), true)
	cyan := term.ColorCyan
	red := term.ColorRed

	wm.SplitHorizontal(NewTestHandler())
	wm.SetAttr(term.Attributes{Bg: cyan, Fg: red}, term.Attributes{Bg: red, Fg: cyan})

	frame := wm.Focus().node.Content().(*Frame)
	assert.Equal(t, red, frame.TopLeft.Bg)
	assert.Equal(t, cyan, frame.TopLeft.Fg)

	wm.ShiftFocus()
	assert.Equal(t, cyan, frame.TopLeft.Bg)
	assert.Equal(t, red, frame.TopLeft.Fg)
}

func testWindowManagerClose(t *testing.T, border bool) {
	h1 := NewTestHandler()
	h2 := NewTestHandler()
	h2.Ch = 'D' // different char to enable assert.Equal
	wm := NewWindowManager(h1, border)
	node2 := wm.SplitHorizontal(h2)

	assert.NotEqual(t, node2, wm.Focus())
	require.NoError(t, wm.Focus().Close())
	assert.Equal(t, node2, wm.Focus())
}

func TestWindowManagerClose(t *testing.T) {
	t.Run("Close with border", func(t *testing.T) {
		testWindowManagerClose(t, true)
	})

	t.Run("Close without border", func(t *testing.T) {
		testWindowManagerClose(t, false)
	})
}

func testWindowManagerContent(t *testing.T, border bool) {
	wm := NewWindowManager(NewTestHandler(), border)
	node2 := wm.SplitHorizontal(NewTestHandler())

	_, ok := node2.Content().(*TestHandler)
	require.True(t, ok)

	prev := node2.SetContent(NewTestHandler())
	_, ok = prev.(*TestHandler)
	require.True(t, ok)

	_, ok = node2.Content().(*TestHandler)
	require.True(t, ok)
}

func TestWindowManagerContent(t *testing.T) {
	t.Run("Content with border", func(t *testing.T) {
		testWindowManagerContent(t, true)
	})
	t.Run("Content without border", func(t *testing.T) {
		testWindowManagerContent(t, false)
	})
}

func testWindowManagerCursor(t *testing.T, border bool) {
	handler := NewTestHandler()
	handler.CursorPos = term.Coordinates{X: 1, Y: 2}
	wm := NewWindowManager(handler, border)
	pos, ok := wm.Cursor()
	require.True(t, ok)
	assert.Equal(t, handler.CursorPos, pos)
}

func TestWindowManagerCursor(t *testing.T) {
	t.Run("Cursor with border", func(t *testing.T) {
		testWindowManagerCursor(t, true)
	})

	t.Run("Cursor without border", func(t *testing.T) {
		testWindowManagerCursor(t, false)
	})
}
