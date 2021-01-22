package browser

import (
	"io"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
)

type windowServerResource struct {
	srv      proto.MuxServer
	win      Window
	brokerID uint64
}

type handlerCloser interface {
	Handler
	io.Closer
}

type handlerClientResource struct {
	handlerConn   proto.MuxConn
	client        handlerCloser
	cancelMonitor func()
}

func (r *handlerClientResource) Close() (err error) {
	defer r.cancelMonitor()

	err1 := r.client.Close()
	if err1 != nil {
		err = err1
	}

	err2 := r.handlerConn.Close()
	if err2 != nil {
		err = err2
	}

	return err
}

func (r *windowServerResource) Close() error {
	r.srv.Stop()
	// unsubscribe, since we are already aware
	r.win.onWindowClosed(nil)
	err := r.win.Close()
	if err != nil {
		return err
	}
	return nil
}

type handlerServerResource struct {
	srv      proto.MuxServer
	brokerID uint64

	h tui.Handler
}

type windowClientResource struct {
	winConn       proto.MuxConn
	cancelMonitor func()
	client        Window
}

func (r *handlerServerResource) Close() error {
	// could be token handler, in which case
	// it is not served from the browser client.
	if r.srv != nil {
		r.srv.Stop()
	}
	return nil
}

func (r *windowClientResource) Close() error {
	defer r.cancelMonitor()

	winErr := r.winConn.Close()
	if winErr != nil {
		return winErr
	}
	return nil
}
