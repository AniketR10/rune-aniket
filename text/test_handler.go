package text

import (
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/workspace"
)

// TestHandler is a handler used to test composite handlers. See
// handler.TestHandler for more details.
type TestHandler struct {
	handler.TestHandler
	URI workspace.URI
}

// NewTestHandler allocates storage for a new TestHandler and initializes it.
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return t
}

func (t *TestHandler) Resource() workspace.URI {
	return t.URI
}

func (t *TestHandler) Close() error {
	return nil
}
