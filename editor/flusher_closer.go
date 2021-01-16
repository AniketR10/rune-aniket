package editor

import (
	"github.com/ernestrc/go-tui/browser"
)

// used to intercept calls to Close and Flush to dispatch
// corresponding events to subscribers.
type editorFlusherCloser struct {
	parent *Ex
	fc     browser.FlusherCloser
	name   string
	h      Handler
}

func (e *editorFlusherCloser) Flush() error {
	ev := Event{
		Type:         EventTypeFlush,
		ResourceName: e.name,
		Resource:     e.h,
	}
	e.parent.dispatchEvent(ev)
	return e.fc.Flush()
}
func (e *editorFlusherCloser) Close() error {
	ev := Event{
		Type:         EventTypeClose,
		ResourceName: e.name,
		Resource:     e.h,
	}
	e.parent.dispatchEvent(ev)
	return e.fc.Close()
}
