package plugin

import (
	"context"

	browserpb "unstable.build/go-tui/browser/rpc"
)

type browserServer interface {
	browserpb.WindowManagerServer
	browserpb.EventPublisherServer
	browserpb.NotificationsServer
	browserpb.ResourceOpenerServer
}

// this structure wraps a browser.Browser to
// provide interrupt on write requests coming from the wire
type interruptBrowser struct {
	browserpb.UnimplementedEventPublisherServer
	browserpb.UnimplementedNotificationsServer
	browserpb.UnimplementedResourceOpenerServer
	browserpb.UnimplementedWindowManagerServer
	browserServer browserServer
	interruptDraw func()
}

func interruptBrowserServer(srv browserServer, interruptDraw func()) browserServer {
	return &interruptBrowser{browserServer: srv, interruptDraw: interruptDraw}
}

// Focus satisfies browserpb.BrowserServer
func (s *interruptBrowser) Publish(
	ctx context.Context, req *browserpb.PublishRequest,
) (*browserpb.PublishResponse, error) {
	res, err := s.browserServer.Publish(ctx, req)
	return res, err
}

// Focus satisfies browserpb.BrowserServer
func (s *interruptBrowser) Focus(
	ctx context.Context, req *browserpb.FocusRequest,
) (*browserpb.FocusResponse, error) {
	res, err := s.browserServer.Focus(ctx, req)
	return res, err
}

// Floating satisfies browserpb.BrowserServer
func (s *interruptBrowser) Floating(
	ctx context.Context, req *browserpb.FloatingWindowRequest,
) (*browserpb.FloatingWindowResponse, error) {
	res, err := s.browserServer.Floating(ctx, req)
	s.interruptDraw()
	return res, err
}

// Tab satisfies browserpb.BrowserServer
func (s *interruptBrowser) Tab(
	ctx context.Context, req *browserpb.TabRequest,
) (*browserpb.TabResponse, error) {
	res, err := s.browserServer.Tab(ctx, req)
	s.interruptDraw()
	return res, err
}

// Split satisfies browserpb.BrowserServer
func (s *interruptBrowser) Split(
	ctx context.Context, req *browserpb.SplitRequest,
) (*browserpb.SplitResponse, error) {
	res, err := s.browserServer.Split(ctx, req)
	s.interruptDraw()
	return res, err
}

// Bar satisfies browserpb.BrowserServer
func (s *interruptBrowser) Bar(
	ctx context.Context, req *browserpb.BarRequest,
) (*browserpb.BarResponse, error) {
	res, err := s.browserServer.Bar(ctx, req)
	s.interruptDraw()
	return res, err
}

// Notify satisfies browserpb.BrowserServer
func (s *interruptBrowser) Notify(
	ctx context.Context, req *browserpb.NotifyRequest,
) (*browserpb.NotifyResponse, error) {
	res, err := s.browserServer.Notify(ctx, req)
	s.interruptDraw()
	return res, err
}

// Open satisfies browserpb.BrowserServer
func (s *interruptBrowser) Open(
	ctx context.Context, req *browserpb.OpenResourceRequest,
) (*browserpb.OpenResourceResponse, error) {
	res, err := s.browserServer.Open(ctx, req)
	s.interruptDraw()
	return res, err
}

// SetContent satisfies browserpb.BrowserServer
func (s *interruptBrowser) SetContent(
	ctx context.Context, req *browserpb.WindowSetContentRequest,
) (*browserpb.WindowSetContentResponse, error) {
	res, err := s.browserServer.SetContent(ctx, req)
	s.interruptDraw()
	return res, err
}

// Close satisfies browserpb.BrowserServer
func (s *interruptBrowser) Close(
	ctx context.Context, req *browserpb.WindowCloseRequest,
) (*browserpb.WindowCloseResponse, error) {
	res, err := s.browserServer.Close(ctx, req)
	s.interruptDraw()
	return res, err
}
