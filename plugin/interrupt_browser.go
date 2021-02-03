package plugin

import (
	"context"

	"github.com/ernestrc/go-tui/proto"
)

type browserServer interface {
	proto.WindowManagerServer
	proto.EventPublisherServer
	proto.EventSubscriberServer
	proto.KeyMapperServer
	proto.MessengerServer
	proto.ResourceOpenerServer
}

// this structure wraps a browser.Browser to
// provide interrupt on write requests coming from the wire
type interruptBrowser struct {
	browserServer
	interruptDraw func()
}

func interruptBrowserServer(srv browserServer, interruptDraw func()) browserServer {
	return &interruptBrowser{browserServer: srv, interruptDraw: interruptDraw}
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
