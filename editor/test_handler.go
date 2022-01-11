package editor

import "github.com/ernestrc/go-tui/handler"

// TestHandler is a handler used to test composite handlers. See
// handler.TestHandler for more details.
type TestHandler struct {
	handler.TestHandler
	Resource string
}

// NewTestHandler allocates storage for a new TestHandler and initializes it.
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return t
}

func (t *TestHandler) Name() string {
	return t.Resource
}

func (t *TestHandler) Close() error {
	return nil
}
