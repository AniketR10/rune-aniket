package rpc

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
)

var _ browserapi.Window = (*windowClientImpl)(nil)

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClientImpl struct {
	windowID      uint64
	channelID     string
	pbClient      WindowClient
	browserClient *Client
	doClose       func()
}

func newWindowClient(
	channelID string, windowID uint64,
	browserClient *Client,
	pbClient WindowClient,
) *windowClientImpl {
	ret := new(windowClientImpl)
	ret.browserClient = browserClient
	ret.pbClient = pbClient
	ret.channelID = channelID
	ret.windowID = windowID
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

	req := WindowSetContentRequest{ChannelId: brokerID}
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

func (w *windowClientImpl) OnWindowClosed(fn func(context.Context)) {
	// window client cannot truly hook into when window
	// is closed remotely (we do that at the browser client level
	// via grpc conn monitor goroutine). For that we would
	// need a new RPC but there's no real use-case yet.
	panic("OnWindowClosed not implemented on window client")
}

func (w *windowClientImpl) ID() uint64 {
	return w.windowID
}

func (w *windowClientImpl) Close() (err error) {
	req := WindowCloseRequest{}
	_, err = w.pbClient.Close(context.Background(), &req)
	// avoid w's finalizer getting called before this RPC is done
	// see runtime.SetFinalizer
	runtime.KeepAlive(w)
	return
}

// satisfies WindowServer
type windowServer struct {
	UnimplementedWindowServer
	win browser.Window
	s   *Server
}

// NewWindowServer returns a WindowServer.
func NewWindowServer(s *Server, win browser.Window) WindowServer {
	ret := new(windowServer)
	ret.win = win
	ret.s = s
	return ret
}

func (s *windowServer) SetContent(
	ctx context.Context, req *WindowSetContentRequest,
) (*WindowSetContentResponse, error) {
	client, err := s.s.getContentHandler(req.GetChannelId())
	if err != nil {
		return nil, fmt.Errorf("failed to dial to remote handler: %v", err)
	}

	s.s.browser.Lock()
	err = s.win.SetContent(client)
	s.s.browser.Unlock()
	if err != nil {
		return nil, fmt.Errorf("error on Window.SetContent: %v", err)
	}

	return new(WindowSetContentResponse), nil
}

func (s *windowServer) Close(
	ctx context.Context, req *WindowCloseRequest,
) (*WindowCloseResponse, error) {
	s.s.browser.Lock()
	defer s.s.browser.Unlock()
	_, ok := s.s.browser.Window(s.win.ID())
	if !ok {
		// close is idempotent
		return new(WindowCloseResponse), nil
	}
	err := s.win.Close(s.s.serverCtx)
	if err != nil {
		return nil, err
	}
	return new(WindowCloseResponse), nil
}
