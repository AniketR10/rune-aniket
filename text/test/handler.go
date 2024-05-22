package test

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/handler"
	term "unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

var _ text.Handler = (*TestHandler)(nil)

// TestHandler is a handler used to test composite handlers. See
// handler.TestHandler for more details.
type TestHandler struct {
	handler.TestHandler
	URI workspaceapi.URI
}

// NewTestHandler allocates storage for a new TestHandler and initializes it.
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return t
}

// Resource satisfies text.Editor.
func (t *TestHandler) Resource() workspaceapi.URI {
	return t.URI
}

// SetWrap satisfies text.Editor.
func (t *TestHandler) SetWrap(wrap bool) {
}

// ShowCommandBar satisfies text.Handler.
func (t *TestHandler) ShowCommandBar(show bool) {
}

// SetCursorAtScroll satisfies text.Handler.
func (t *TestHandler) SetCursorAtScroll(term.Coordinates) bool {
	return false
}

func (t *TestHandler) Close() error {
	return nil
}
