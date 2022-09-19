package plugin

import (
	"context"

	bproto "github.com/ernestrc/blue/datastore/rpc"
	"github.com/ernestrc/go-tui/proto"
)

type browserServer interface {
	proto.WindowManagerServer
	proto.EventPublisherServer
	proto.MessengerServer
	proto.ResourceOpenerServer
	bproto.DocumentStoreServer
}

// this structure wraps a browser.Browser to
// provide interrupt on write requests coming from the wire
type interruptBrowser struct {
	bproto.UnimplementedDocumentStoreServer
	proto.UnimplementedEventPublisherServer
	proto.UnimplementedMessengerServer
	proto.UnimplementedResourceOpenerServer
	proto.UnimplementedWindowManagerServer
	browserServer browserServer
	interruptDraw func()
}

func interruptBrowserServer(srv browserServer, interruptDraw func()) browserServer {
	return &interruptBrowser{browserServer: srv, interruptDraw: interruptDraw}
}

// Focus satisfies proto.BrowserServer
func (s *interruptBrowser) Publish(
	ctx context.Context, req *proto.PublishRequest,
) (*proto.PublishResponse, error) {
	res, err := s.browserServer.Publish(ctx, req)
	return res, err
}

// Focus satisfies proto.BrowserServer
func (s *interruptBrowser) Focus(
	ctx context.Context, req *proto.FocusRequest,
) (*proto.FocusResponse, error) {
	res, err := s.browserServer.Focus(ctx, req)
	return res, err
}

// Floating satisfies proto.BrowserServer
func (s *interruptBrowser) Floating(
	ctx context.Context, req *proto.FloatingWindowRequest,
) (*proto.FloatingWindowResponse, error) {
	res, err := s.browserServer.Floating(ctx, req)
	s.interruptDraw()
	return res, err
}

// Tab satisfies proto.BrowserServer
func (s *interruptBrowser) Tab(
	ctx context.Context, req *proto.TabRequest,
) (*proto.TabResponse, error) {
	res, err := s.browserServer.Tab(ctx, req)
	s.interruptDraw()
	return res, err
}

// Split satisfies proto.BrowserServer
func (s *interruptBrowser) Split(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	res, err := s.browserServer.Split(ctx, req)
	s.interruptDraw()
	return res, err
}

// Bar satisfies proto.BrowserServer
func (s *interruptBrowser) Bar(
	ctx context.Context, req *proto.BarRequest,
) (*proto.BarResponse, error) {
	res, err := s.browserServer.Bar(ctx, req)
	s.interruptDraw()
	return res, err
}

// SetMessage satisfies proto.BrowserServer
func (s *interruptBrowser) SetMessage(
	ctx context.Context, req *proto.SetMessageRequest,
) (*proto.SetMessageResponse, error) {
	res, err := s.browserServer.SetMessage(ctx, req)
	s.interruptDraw()
	return res, err
}

// Open satisfies proto.BrowserServer
func (s *interruptBrowser) Open(
	ctx context.Context, req *proto.OpenResourceRequest,
) (*proto.OpenResourceResponse, error) {
	res, err := s.browserServer.Open(ctx, req)
	s.interruptDraw()
	return res, err
}

func (s *interruptBrowser) Create(
	ctx context.Context, req *bproto.CreateDocumentRequest,
) (*bproto.CreateDocumentResponse, error) {
	res, err := s.browserServer.Create(ctx, req)
	return res, err
}

func (s *interruptBrowser) Set(
	ctx context.Context, req *bproto.SetDocumentRequest,
) (*bproto.DocumentResponse, error) {
	res, err := s.browserServer.Set(ctx, req)
	return res, err
}

func (s *interruptBrowser) Update(
	ctx context.Context, req *bproto.UpdateDocumentRequest,
) (*bproto.UpdateDocumentResponse, error) {
	res, err := s.browserServer.Update(ctx, req)
	return res, err
}

func (s *interruptBrowser) Get(
	ctx context.Context, req *bproto.GetDocumentRequest,
) (*bproto.GetDocumentResponse, error) {
	res, err := s.browserServer.Get(ctx, req)
	return res, err
}

func (s *interruptBrowser) Delete(
	ctx context.Context, req *bproto.DeleteDocumentRequest,
) (*bproto.DocumentResponse, error) {
	res, err := s.browserServer.Delete(ctx, req)
	return res, err
}

func (s *interruptBrowser) List(
	req *bproto.ListDocumentRequest, srv bproto.DocumentStore_ListServer,
) error {
	return s.browserServer.List(req, srv)
}
