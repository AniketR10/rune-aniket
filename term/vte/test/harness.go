package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

type Case struct {
	InputSequence string
	Expected      string
}

// TestSequence tests the given handler with the given test cases.
// This differs from the default testing tui.Handler harness' in that
// it waits up to drawTimeout for interruptChan to stop sending requests
// to advance to the test case's draw and assertions.
//
// This is mostly useful for asynchronous tui.Handler that are 'ready'
// for assertion when idle for drawTimeout, as defined by not sending interrupt
// requests.
func TestSequence(
	t *testing.T, handler tui.Handler, width, height int,
	drawTimeout time.Duration, interruptChan chan struct{},
	cases []Case, nextTick func(int),
) {
	writer := term.NewStringWriter(width, height)
	handler.Resize(width, height)

	// wait for first handler to initialize,
	// first interrupt should be a good indicator
	<-interruptChan

	for i, tcase := range cases {
		handleTestCase(t, i, writer, handler, tcase,
			width, height, drawTimeout, interruptChan, nextTick)
	}
}

func handleTestCase(
	t *testing.T, i int, w *term.StringWriter,
	h tui.Handler, tcase Case, width, height int,
	drawTimeout time.Duration,
	interruptChan chan struct{}, nextTick func(int),
) {
	err := w.Clear(term.Attributes{})
	require.NoError(t, err)

	// vte needs Raw field set
	callHandle := func(ev term.Event) {
		ev.Raw = []byte(string(ev.Ch))
		if len(ev.Raw) == 0 {
			switch ev.Key {
			case term.KeyCtrlBackslash:
				ev.Raw = []byte("\x1C")
			case term.KeyCtrlV:
				ev.Raw = []byte("\x16")
			case term.KeyBackspace:
				ev.Raw = []byte("\x08")
			case term.KeyCtrlL:
				ev.Raw = []byte("\x0c")
			case term.KeyEnter:
				ev.Raw = []byte("\x0A")
			case term.KeyEsc:
				ev.Raw = []byte("\x1b")
			case term.KeyTab:
				ev.Raw = []byte("\x09")
			case term.KeyArrowDown:
				ev.Raw = []byte("\x50")
			case term.KeyArrowUp:
				ev.Raw = []byte("\x48")
			default:
				t.Logf("WARNING: could not find raw vte sequence for input event: %+v", ev)
			}
		}
		h.Handle(ev)
	}

	var escapeNext bool
	for i, r := range tcase.InputSequence {
		if escapeNext {
			escapeNext = false
			callHandle(term.Event{Ch: r, Type: term.EventKey})
			continue
		}
		switch r {
		case '^':
			callHandle(term.Event{Key: term.KeyBackspace, Type: term.EventKey})
		case '#':
			callHandle(term.Event{Key: term.KeyCtrlC, Type: term.EventKey})
		case '$':
			callHandle(term.Event{Key: term.KeyCtrlL, Type: term.EventKey})
		case '>':
			callHandle(term.Event{Key: term.KeyEnter, Type: term.EventKey})
		case '<':
			callHandle(term.Event{Key: term.KeyEsc, Type: term.EventKey})
		case '✌':
			callHandle(term.Event{Key: term.KeyTab, Type: term.EventKey})
		case '⬇':
			callHandle(term.Event{Key: term.KeyArrowDown, Type: term.EventKey})
		case '⬆':
			callHandle(term.Event{Key: term.KeyArrowUp, Type: term.EventKey})
		case '\\':
			escapeNext = true
		default:
			callHandle(term.Event{Ch: r, Type: term.EventKey})
		}

		timer := time.NewTimer(drawTimeout)
	loop:
		for {
			select {
			case <-timer.C:
				break loop
			case <-interruptChan:
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(drawTimeout)
			}
		}
		nextTick(i)
	}

	h.Draw(w)

	cursor, _, ok := h.Cursor()
	if ok {
		w.SetCursor(cursor)
	}

	err = w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Equal(t, tcase.Expected, out, "test case %d", i)
}
