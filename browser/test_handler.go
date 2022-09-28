package browser

import (
	"unstable.build/go-tui/handler"
)

// TestHandler is a testing Handler.
type TestHandler struct {
	handler.TestHandler
	CloseCallback func() error
}

// NewTestHandler allocates storage for a new TestHandler and initializes it.
func NewTestHandler() *TestHandler {
	ret := new(TestHandler)
	ret.TestHandler = *handler.NewTestHandler()
	return ret
}

// Close calls t.Close.
func (t *TestHandler) Close() error {
	if t.CloseCallback != nil {
		return t.CloseCallback()
	}
	return nil
}
