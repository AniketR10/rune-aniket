package rpc

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	browserapi "unstable.build/go-tui/api/browser"
)

var _ browserapi.Window = (*windowClientImpl)(nil)

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClientImpl struct {
	windowID      uint64
	browserClient *Client
	pbClient      WindowManagerClient
	doClose       func()
}

func newWindowClient(
	windowID uint64,
	browserClient *Client,
	pbClient WindowManagerClient,
) *windowClientImpl {
	ret := new(windowClientImpl)
	ret.browserClient = browserClient
	ret.windowID = windowID
	ret.pbClient = pbClient
	return ret
}

func (w *windowClientImpl) Focus() (bool, error) {
	fw, err := w.browserClient.Focus()
	runtime.KeepAlive(w)
	if err != nil {
		return false, err
	}
	return fw == w, nil
}

func (w *windowClientImpl) SetContent(h browserapi.Handler) error {
	ctx := context.Background()

	brokerID, srv, err := w.browserClient.serveHandler(h)
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
