package plugin

import (
	"context"

	storepb "github.com/ernestrc/blue/datastore/rpc"
	browserpb "github.com/ernestrc/go-tui/browser/rpc"
)

type browserServer interface {
	browserpb.WindowManagerServer
	browserpb.EventPublisherServer
	browserpb.MessengerServer
	browserpb.ResourceOpenerServer
	storepb.DocumentStoreServer
}

// this structure wraps a browser.Browser to
// provide interrupt on write requests coming from the wire
type interruptBrowser struct {
	storepb.UnimplementedDocumentStoreServer
	browserpb.UnimplementedEventPublisherServer
	browserpb.UnimplementedMessengerServer
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

// SetMessage satisfies browserpb.BrowserServer
func (s *interruptBrowser) SetMessage(
	ctx context.Context, req *browserpb.SetMessageRequest,
) (*browserpb.SetMessageResponse, error) {
	res, err := s.browserServer.SetMessage(ctx, req)
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

func (s *interruptBrowser) Create(
	ctx context.Context, req *storepb.CreateDocumentRequest,
) (*storepb.CreateDocumentResponse, error) {
	res, err := s.browserServer.Create(ctx, req)
	return res, err
}

func (s *interruptBrowser) Set(
	ctx context.Context, req *storepb.SetDocumentRequest,
) (*storepb.DocumentResponse, error) {
	res, err := s.browserServer.Set(ctx, req)
	return res, err
}

func (s *interruptBrowser) Update(
	ctx context.Context, req *storepb.UpdateDocumentRequest,
) (*storepb.UpdateDocumentResponse, error) {
	res, err := s.browserServer.Update(ctx, req)
	return res, err
}

func (s *interruptBrowser) Get(
	ctx context.Context, req *storepb.GetDocumentRequest,
) (*storepb.GetDocumentResponse, error) {
	res, err := s.browserServer.Get(ctx, req)
	return res, err
}

func (s *interruptBrowser) Delete(
	ctx context.Context, req *storepb.DeleteDocumentRequest,
) (*storepb.DocumentResponse, error) {
	res, err := s.browserServer.Delete(ctx, req)
	return res, err
}

func (s *interruptBrowser) List(
	req *storepb.ListDocumentRequest, srv storepb.DocumentStore_ListServer,
) error {
	return s.browserServer.List(req, srv)
}
