package rpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/rpc"
)

var _ browserapi.Window = (*windowClientImpl)(nil)

// tipically this doesn't need to be handled by the rest
// of clients but Window is a special case, because it's delivered
// in textapi.Command and so it's possible that user tries to
// use the Window without having access to the right permissions.
var errMissingPermissions = errors.New("missing extension.PermissionWindowManager")

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClientImpl struct {
	windowID  uint64
	pbClient  WindowManagerClient
	broker    rpc.MuxBroker
	clientCtx context.Context
}

func newWindowClient(
	ctx context.Context,
	windowID uint64,
	pbClient WindowManagerClient,
	broker rpc.MuxBroker,
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
		if status.Code(err) == codes.Unimplemented {
			return false, errMissingPermissions
		}
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
		if status.Code(err) == codes.Unimplemented {
			return errMissingPermissions
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
	if status.Code(err) == codes.Unimplemented {
		return errMissingPermissions
	}
	runtime.KeepAlive(w)
	return
}
