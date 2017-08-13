package fractal

import (
	"termbox"
	"testing"
)

func moveEvent(ch rune) termbox.Event {
	return termbox.Event{
		Type: termbox.EventKey,
		Mod:  termbox.ModAlt,
		Ch:   ch,
	}
}

func prepareTest() (*StringWriter, *Tile, *WindowManager) {
	width, height := 8, 4
	writer := NewStringWriter(width, height)
	handler, tile, err := NewWindowManager(NewTestHandler(), width, height, 0, 0)
	if err != nil {
		panic(err)
	}

	return writer, tile, handler
}

func TestWindowManagerSetFocus(t *testing.T) {
	_, left, handler := prepareTest()
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
	writer, topleft, handler := prepareTest()

	bottomleft, err := handler.SplitHorizontal(NewTestHandler())
	if err != nil {
		t.Fatal(err)
	}

	_, err2 := handler.SplitVertical(NewTestHandler())
	if err2 != nil {
		t.Fatal(err2)
	}

	handler.SetFocus(bottomleft)
	_, err3 := handler.SplitVertical(NewTestHandler())
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
			moveEvent('l'), `
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
			moveEvent('l'), `
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
			moveEvent('h'), `
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
			moveEvent('j'), `
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
			moveEvent('l'), `
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
			moveEvent('k'), `
CCCCCCCC
CCCCCCCC
BBBBBBBB
BBBBBBBB`,
		},
		{
			termbox.Event{}, `
CCCCDDDD
CCCCDDDD
BBBBBBBB
BBBBBBBB`,
		},
	}

	testHandlerWorkflow(t, handler, cases, writer)
}
