package browser

import (
	"context"
	"io"

	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type handlerClientResource struct {
	handlerID   uint32
	handlerConn proto.MuxConn
	cc          io.Closer
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

type windowServerResource struct {
	srv *grpc.Server
	win Window
}

func (r *windowServerResource) Close() error {
	r.srv.Stop()
	err := r.win.Close()
	if err != nil {
		return err
	}
	return nil
}
