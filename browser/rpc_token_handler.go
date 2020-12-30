package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// remoteHandler is a token handler used to indicate which of the remote handlers
// to set as content to a remote server. It satisfies tui.Handler so that clients
// can take the result of an browser.Open request and pass it to Split* or SetContent
// responses.
type remoteTokenHandler struct {
	handlerID uint32
}

const errMsg = "this Handler is a token handler that cannot be used directly"

func (h remoteTokenHandler) Handle(term.Event) (exit, handled bool) {
	panic(errMsg)
}

func (h remoteTokenHandler) Cursor() (pos term.Coordinates, show bool) {
	panic(errMsg)
}

func (h remoteTokenHandler) Man() tui.Manual {
	panic(errMsg)
}

func (h remoteTokenHandler) Resize(width, height int) {
	panic(errMsg)
}

func (h remoteTokenHandler) Draw(w term.Writer) {
	panic(errMsg)
}

func (h remoteTokenHandler) OnUnmount() error {
	return nil
}

// used as a counterpart of remoteTokenHandler in browser.Server
// it's used to add a Close method to a local Handler.
type localTokenHandler struct {
	Handler
}

func (p localTokenHandler) Close() error {
	return nil
}
