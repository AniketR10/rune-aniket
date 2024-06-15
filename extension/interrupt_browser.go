// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package extension

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
