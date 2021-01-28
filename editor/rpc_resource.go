package editor

import "github.com/ernestrc/go-tui/proto"

type handlerServerResource struct {
	srv proto.MuxServer
	h   EventHandler
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
