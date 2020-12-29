package browser

import (
	"context"
	"io"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type windowServerResource struct {
	srv *grpc.Server
	win Window
}

type handlerCloser interface {
	Handler
	io.Closer
}

type handlerClientResource struct {
	handlerConn   proto.MuxConn
	cc            handlerCloser
	cancelMonitor func()
}

func (r *handlerClientResource) WaitForStateChange(
	ctx context.Context, sourceState connectivity.State,
) bool {
	realConn, ok := r.handlerConn.(*grpc.ClientConn)
	if !ok {
		return true
	}
	return realConn.WaitForStateChange(ctx, sourceState)
}

func (r *handlerClientResource) Close() (err error) {
	defer r.cancelMonitor()

	err1 := r.cc.Close()
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
	err := r.win.Close()
	if err != nil {
		return err
	}
	return nil
}

type handlerServerResource struct {
	srv *grpc.Server

	h tui.Handler
}

type windowClientResource struct {
	winConn       proto.MuxConn
	cancelMonitor func()
	cc            *windowClient
}

func (r *handlerServerResource) Close() error {
	r.srv.Stop()
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
