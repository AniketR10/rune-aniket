package test

import (
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/handler"
)

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

func (t *TestHandler) Close() error {
	return nil
}
