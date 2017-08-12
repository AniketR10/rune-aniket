package handler

import (
	"termbox"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
)

func moveEvent(ch rune) termbox.Event {
	return termbox.Event{
		Type: termbox.EventKey,
		Mod:  termbox.ModAlt,
		Ch:   ch,
	}
}

func prepareTest() (*fractal.StringWriter, *component.Tile, *WindowManager) {
	width, height := 8, 4
	writer := fractal.String(width, height)
	handler, tile, err := NewWindowManager(newTestHandler(), width, height, 0, 0)
	if err != nil {
		panic(err)
	}

	return writer, tile, handler
}

func TestWindowManagerSetFocus(t *testing.T) {
	_, left, handler := prepareTest()
	right, err := handler.SplitHorizontal(newTestHandler())

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

// testHandler signals that it's handling event by incrementing it's fill rune
func TestWindowManagerHandle(t *testing.T) {
	writer, topleft, handler := prepareTest()

	bottomleft, err := handler.SplitHorizontal(newTestHandler())
	if err != nil {
		t.Fatal(err)
	}

	_, err2 := handler.SplitVertical(newTestHandler())
	if err2 != nil {
		t.Fatal(err2)
	}

	handler.SetFocus(bottomleft)
	_, err3 := handler.SplitVertical(newTestHandler())
	if err3 != nil {
		t.Fatal(err3)
	}

	handler.SetFocus(topleft)

	cases := []testCase{
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

	testWorkflow(t, handler, cases, writer)
}
