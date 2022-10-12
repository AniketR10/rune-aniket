package rpc

import (
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/proto"
)

type handlerServerResource struct {
	srv proto.MuxServer
}

func (r *handlerServerResource) Close() error {
	r.srv.Stop()
	return nil
}

type handlerClientResource struct {
	handlerConn   proto.MuxConn
	client        *serverEventHandler
	cancelMonitor func()
}

func (r *handlerClientResource) Close() (ret error) {
	defer r.cancelMonitor()
	if err := r.client.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := r.handlerConn.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

type commandClientResource struct {
	handlerConn   proto.MuxConn
	client        *commandClient
	cancelMonitor func()
}

func (r *commandClientResource) Close() (ret error) {
	defer r.cancelMonitor()
	if err := r.client.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := r.handlerConn.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}
