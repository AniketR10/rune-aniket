package rpc

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/proto"
)

var _ browserapi.Window = (*windowClientImpl)(nil)

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClientImpl struct {
	windowID  uint64
	pbClient  WindowManagerClient
	broker    proto.MuxBroker
	clientCtx context.Context
}

func newWindowClient(
	ctx context.Context,
	windowID uint64,
	pbClient WindowManagerClient,
	broker proto.MuxBroker,
) *windowClientImpl {
	ret := new(windowClientImpl)
	ret.clientCtx = ctx
	ret.windowID = windowID
	ret.pbClient = pbClient
	ret.broker = broker
	return ret
}

func (w *windowClientImpl) Focus() (bool, error) {
	fw, err := focus(w.clientCtx, w.pbClient, w.broker)
	runtime.KeepAlive(w)
	if err != nil {
		return false, err
	}
	return fw == w, nil
}

func (w *windowClientImpl) SetContent(h browserapi.Handler) error {
	ctx := context.Background()

	brokerID, srv, err := serveHandler(w.clientCtx, w.broker, h)
	if err != nil {
		return fmt.Errorf("serveHandler: %w", err)
	}

	req := WindowSetContentRequest{ChannelId: brokerID, WindowId: w.windowID}
	_, err = w.pbClient.SetContent(ctx, &req)
	runtime.KeepAlive(w)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		if strings.Contains(err.Error(), browserapi.ErrTabNotFree.Error()) {
			return browserapi.ErrTabNotFree
		}
		return fmt.Errorf("pbClient.SetContent: %v", err)
	}
	return nil
}

func (w *windowClientImpl) Content() (browserapi.Handler, error) {
	panic("Content not implemented on window client")
}

func (w *windowClientImpl) ID() uint64 {
	return w.windowID
}

func (w *windowClientImpl) Close() (err error) {
	req := WindowCloseRequest{WindowId: w.windowID}
	_, err = w.pbClient.Close(context.Background(), &req)
	runtime.KeepAlive(w)
	return
}
