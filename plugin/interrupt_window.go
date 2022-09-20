package plugin

import (
	"context"

	"github.com/ernestrc/go-tui/browser"
	browserpb "github.com/ernestrc/go-tui/browser/rpc"
	"github.com/ernestrc/go-tui/term"
)

type interruptWindow struct {
	browserpb.UnimplementedWindowServer
	srv           browserpb.WindowServer
	interruptDraw func()
}

func interruptWindowServer(s *browser.Server, win browser.Window) browserpb.WindowServer {
	return &interruptWindow{
		srv:           browser.NewWindowServer(s, win),
		interruptDraw: term.Interrupt,
	}
}

func (w *interruptWindow) SetContent(
	ctx context.Context, req *browserpb.WindowSetContentRequest,
) (*browserpb.WindowSetContentResponse, error) {
	defer w.interruptDraw()
	return w.srv.SetContent(ctx, req)
}

func (w *interruptWindow) Content(
	ctx context.Context, req *browserpb.WindowContentRequest,
) (*browserpb.WindowContentResponse, error) {
	return w.srv.Content(ctx, req)
}

func (w *interruptWindow) Close(
	ctx context.Context, req *browserpb.WindowCloseRequest,
) (*browserpb.WindowCloseResponse, error) {
	defer w.interruptDraw()
	return w.srv.Close(ctx, req)
}
