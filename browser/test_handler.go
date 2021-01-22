package browser

import (
	"github.com/ernestrc/go-tui/handler"
)

// TestHandler is a testing Handler.
type TestHandler struct {
	handler.TestHandler
	OnUnmountCallback func() error
}

// NewTestHandler allocates storage for a new TestHandler and initializes it.
func NewTestHandler() *TestHandler {
	ret := new(TestHandler)
	ret.TestHandler = *handler.NewTestHandler()
	return ret
}

// OnUnmount calls t.OnUnmount.
func (t *TestHandler) OnUnmount() error {
	if t.OnUnmountCallback != nil {
		return t.OnUnmountCallback()
	}
	return nil
}
