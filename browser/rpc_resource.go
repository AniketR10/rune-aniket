package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
)

type windowServerResource struct {
	srv      proto.MuxServer
	win      Window
	brokerID uint64
}

type handlerClientResource struct {
	handlerConn   proto.MuxConn
	client        Handler
	cancelMonitor func()
}

func (r *handlerClientResource) Close() (err error) {
	defer r.cancelMonitor()

	err2 := r.handlerConn.Close()
	if err2 != nil {
		err = err2
	}

	return err
}

func (r *windowServerResource) Close() error {
	r.srv.Stop()
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
