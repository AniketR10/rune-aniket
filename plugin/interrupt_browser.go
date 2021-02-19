package plugin

import (
	"context"

	bproto "github.com/ernestrc/blue/rpc"
	"github.com/ernestrc/go-tui/proto"
)

type browserServer interface {
	proto.WindowManagerServer
	proto.EventPublisherServer
	proto.EventSubscriberServer
	proto.KeyMapperServer
	proto.MessengerServer
	proto.ResourceOpenerServer
	bproto.DocumentStoreServer
}

// this structure wraps a browser.Browser to
// provide interrupt on write requests coming from the wire
type interruptBrowser struct {
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

// FloatingWindow satisfies proto.BrowserServer
func (s *interruptBrowser) FloatingWindow(
	ctx context.Context, req *proto.FloatingWindowRequest,
) (*proto.FloatingWindowResponse, error) {
	res, err := s.browserServer.FloatingWindow(ctx, req)
	s.interruptDraw()
	return res, err
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *interruptBrowser) SplitVerticalRight(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	res, err := s.browserServer.SplitVerticalRight(ctx, req)
	s.interruptDraw()
	return res, err
}

// SplitVerticalLeft satisfies proto.BrowserServer
func (s *interruptBrowser) SplitVerticalLeft(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	res, err := s.browserServer.SplitVerticalLeft(ctx, req)
	s.interruptDraw()
	return res, err
}

// SplitHorizontalAbove satisfies proto.BrowserServer
func (s *interruptBrowser) SplitHorizontalAbove(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	res, err := s.browserServer.SplitHorizontalAbove(ctx, req)
	s.interruptDraw()
	return res, err
}

// SplitHorizontalBelow satisfies proto.BrowserServer
func (s *interruptBrowser) SplitHorizontalBelow(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	res, err := s.browserServer.SplitHorizontalBelow(ctx, req)
	s.interruptDraw()
	return res, err
}

// MergeKeyMap satisfies proto.BrowserServer
func (s *interruptBrowser) MergeKeyMap(
	ctx context.Context, req *proto.MergeKeyMapRequest,
) (*proto.MergeKeyMapResponse, error) {
	res, err := s.browserServer.MergeKeyMap(ctx, req)
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

// Subscribe satisfies proto.BrowserServer
func (s *interruptBrowser) Subscribe(
	ctx context.Context, req *proto.SubscribeRequest,
) (*proto.SubscribeResponse, error) {
	res, err := s.browserServer.Subscribe(ctx, req)
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
