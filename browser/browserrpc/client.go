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

package browserrpc

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/handlerrpc"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	termrpc "unstable.build/go-tui/term/termrpc"
)

var _ browserapi.Browser = (*Client)(nil)

const closedCopy = `
          ___
         /___/\_               
        _\   \/_/\__           
      __\       \/_/\          
      \   __    __ \ \         
     __\  \_\   \_\ \ \   __   
    /_/\\   __   __  \ \_/_/\  
    \_\/_\__\/\__\/\__\/_\_\/  
       \_\/_/\       /_\_\/    
          \_\/       \_\/      
    

Oops! This should not be here.
`

// Client satisfies Browser by talking to a browser server over RPC.
type Client struct {
	broker rpc.MuxBroker
	cc     grpc.ClientConnInterface
	wm     WindowManagerClient
	msg    NotificationsClient
	f      ResourceOpenerClient
	p      EventPublisherClient

	clientCtx       context.Context
	clientCancelCtx func()
}

type browserClientHandler struct {
	browserapi.Handler
	srv    rpc.MuxServer
	cancel func()
}

func (c *browserClientHandler) Close() error {
	go func() {
		// cancel calls Stop, so wait until we are done
		defer c.cancel()
		c.srv.GracefulStop()
	}()
	err := c.Handler.Close()
	// avoid cyclical references preventing
	// runtime finalizers from running
	c.Handler = browserapi.NopFloatingHandler(
		handler.NopFloatingHandler(component.NewString(closedCopy)))
	return err
}

func (c *browserClientHandler) Dimensions() (width, height int) {
	return c.Handler.(browserapi.Floating).Dimensions()
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	ctx context.Context, broker rpc.MuxBroker, cc grpc.ClientConnInterface,
) *Client {
	ret := new(Client)
	ret.Init(ctx, broker, cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "browser.Client").Logf(level, msg, args...)
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	ctx context.Context, broker rpc.MuxBroker, cc grpc.ClientConnInterface,
) {
	c.wm = NewWindowManagerClient(cc)
	c.msg = NewNotificationsClient(cc)
	c.cc = cc
	c.f = NewResourceOpenerClient(cc)
	c.p = NewEventPublisherClient(cc)
	c.broker = broker
	ok := rpc.IsContextWithWaitGroup(ctx)
	if !ok {
		ctx = rpc.ContextWithWaitGroup(ctx, new(sync.WaitGroup))
	}
	c.clientCtx, c.clientCancelCtx = context.WithCancel(ctx)
}

func serveHandler(
	ctx context.Context, broker rpc.MuxBroker, h browserapi.Handler,
) (channelID string, srv rpc.MuxServer, err error) {
	if h == nil {
		panic("passed nil Handler to browser client")
	}
	tokenHandler, ok := h.(browser.Token)
	if ok {
		channelID = tokenHandler.ID
	} else {
		// cancel if close is called before client context is done
		ctx, cancel := context.WithCancel(ctx)
		ctxWg := rpc.WaitGroupFromContext(ctx)
		channelID, err = rpc.AcceptAndServeChannel(ctx, broker,
			func(channelID string, msrv rpc.MuxServer) {
				ctxWg.Add(1)
				srv = msrv
				h = &browserClientHandler{
					Handler: h,
					srv:     srv,
					cancel:  cancel,
				}
				hsrv := handlerrpc.NewServer(h)
				handlerrpc.RegisterHandlerServer(srv.Registrar(), hsrv)

				// if it satisfies Floating as well then register it
				if floating, ok := h.(browserapi.Floating); ok {
					fsrv := newFloatingServer(floating)
					RegisterFloatingServer(srv.Registrar(), fsrv)
				}
			}, "browser", "client", "handler")
		if err != nil {
			return
		}
		go func(ctx context.Context) {
			defer ctxWg.Done()
			// do not reference Client so finalizer can still run
			<-ctx.Done()
			srv.Stop()
		}(ctx)
	}

	return
}

// DialWindow dials the window with the given windowID token.
func (c *Client) DialWindow(windowID uint64) (browserapi.Window, error) {
	client := newWindowClient(c.clientCtx, windowID, c.wm, c.broker)
	c.log(log.TraceLevel, "get window with id %d: %#v", windowID, client)
	return client, nil
}

type clientSplit func(cc WindowManagerClient,
	ctx context.Context, req *SplitRequest,
	opts ...grpc.CallOption) (*SplitResponse, error)

func (c *Client) split(
	split clientSplit, o browserapi.Orientation, in browserapi.Window, h browserapi.Handler,
) (browserapi.Window, error) {
	channelID, srv, err := serveHandler(c.clientCtx, c.broker, h)
	if err != nil {
		return nil, fmt.Errorf("serve handler: %w", err)
	}
	req := SplitRequest{
		ChannelId:   channelID,
		Orientation: toProtoOrientation(o),
		WindowId:    in.(*windowClientImpl).windowID,
	}
	ctx := context.Background()
	res, err := split(c.wm, ctx, &req)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return nil, err
	}
	out, err := c.DialWindow(res.GetWindowId())
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return nil, err
	}
	return out, nil
}

// Split satisfies Browser.
func (c *Client) Split(
	o browserapi.Orientation, win browserapi.Window, h browserapi.Handler,
) (browserapi.Window, error) {
	win, err := c.split((WindowManagerClient).Split, o, win, h)
	runtime.KeepAlive(c)
	return win, err
}

// Resource satisfies Browser.
func (c *Client) Resource(workspaceapi.URI) (browserapi.Handler, bool) {
	panic("Resource unimplemented in browser client")
}

// Window satisfies Browser.
func (c *Client) Window(uint64) (browserapi.Window, bool) {
	panic("Window unimplemented in browser client")
}

// Bar satisfies Browser.
func (c *Client) Bar(config browserapi.BarConfig, h tui.Handler) error {
	channelID, srv, err := serveHandler(c.clientCtx, c.broker, browser.NopHandler(h))
	if err != nil {
		return fmt.Errorf("serve handler: %w", err)
	}
	req := BarRequest{
		ChannelId:   channelID,
		Orientation: toProtoOrientation(config.Orientation),
		Size:        uint32(config.Size),
		Frame:       toProtoFrame(config.Frame),
	}
	ctx := context.Background()
	_, err = c.wm.Bar(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return err
	}
	return nil
}

// Notify satisfies Browser.
func (c *Client) Notify(level notifications.Level, msg string, args ...interface{}) error {
	msg = fmt.Sprintf(msg, args...)

	ctx := context.Background()
	req := NotifyRequest{Level: uint32(level), Msg: msg}

	_, err := c.msg.Notify(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// NotifyOnce satisfies Browser.
func (c *Client) NotifyOnce(level notifications.Level, msg string, args ...interface{}) error {
	msg = fmt.Sprintf(msg, args...)

	ctx := context.Background()
	req := NotifyRequest{Level: uint32(level), Msg: msg}

	_, err := c.msg.NotifyOnce(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// Open satisfies Browser.
func (c *Client) Open(resource workspaceapi.URI) (browserapi.Handler, error) {
	ctx := context.Background()
	req := OpenResourceRequest{Resource: resource.String()}

	res, err := c.f.Open(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}

	return browser.Token{ID: res.GetChannelId()}, err
}

// PublishEventNone satisfies Browser.
func (c *Client) PublishEventNone() error {
	ctx := context.Background()
	protoEv := new(termrpc.Event)
	err := protoEv.FromModel(term.Event{Type: term.EventNone})
	if err != nil {
		return err
	}
	req := PublishRequest{Ev: protoEv}

	_, err = c.p.Publish(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// Interrupt satisfies Browser.
func (c *Client) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	protoEv := new(termrpc.Event)
	err := protoEv.FromModel(term.Event{Type: term.EventInterrupt, Raw: payload})
	if err != nil {
		return err
	}
	req := PublishRequest{Ev: protoEv}

	_, err = c.p.Publish(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// SetFocus satisfies Browser.
func (c *Client) SetFocus(win browserapi.Window) (browserapi.Window, error) {
	ctx := context.Background()
	client := win.(*windowClientImpl)
	req := SetFocusRequest{WindowId: client.windowID}
	res, err := c.wm.SetFocus(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	return c.DialWindow(res.GetWindowId())
}

// Focus satisfies Browser.
func (c *Client) Focus() (browserapi.Window, error) {
	win, err := focus(c.clientCtx, c.wm, c.broker)
	runtime.KeepAlive(c)
	return win, err
}

func focus(
	clientCtx context.Context, wm WindowManagerClient, broker rpc.MuxBroker,
) (browserapi.Window, error) {
	req := FocusRequest{}
	res, err := wm.Focus(clientCtx, &req)
	if err != nil {
		return nil, err
	}
	client := newWindowClient(clientCtx, res.GetWindowId(), wm, broker)
	return client, nil
}

// Floating satisfies browser.WindowManager
func (c *Client) Floating(
	h browserapi.Floating, cfg component.FloatingConfig,
) (browserapi.Window, error) {
	var atProto termrpc.Coordinates
	atProto.FromModel(cfg.Offset)

	freq := FloatingWindowRequest{
		Offset:    &atProto,
		Alignment: uint32(cfg.Alignment),
	}

	var fakeWindow windowClientImpl
	win, err := c.split(func(cc WindowManagerClient,
		ctx context.Context, req *SplitRequest,
		opts ...grpc.CallOption) (*SplitResponse, error) {

		freq.ChannelId = req.GetChannelId()

		fres, err := cc.Floating(ctx, &freq)
		if err != nil {
			return nil, err
		}
		return &SplitResponse{
			WindowId: fres.GetWindowId(),
		}, nil
	}, browserapi.OrientationDefault, &fakeWindow, h)
	runtime.KeepAlive(c)
	return win, err
}

// Tab satisfies browser.WindowManager
func (c *Client) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (browserapi.Handler, error) {
	channelID, srv, err := serveHandler(c.clientCtx, c.broker, h)
	if err != nil {
		return nil, fmt.Errorf("serve handler: %w", err)
	}
	uriStr := uri.String()
	req := TabRequest{
		ChannelId:    channelID,
		ResourceId:   uriStr,
		ResourceName: name,
		ResourceIcon: string(icon),
	}
	ctx := context.Background()
	_, err = c.wm.Tab(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return nil, err
	}
	return browser.Token{ID: uriStr}, err
}

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() (err error) {
	if closer, ok := c.cc.(io.Closer); ok {
		err = closer.Close()
	}
	c.cc = nil
	if c.clientCancelCtx != nil {
		c.clientCancelCtx()
		c.clientCancelCtx = nil
	}
	runtime.SetFinalizer(c, nil)
	return
}

func toProtoOrientation(o browserapi.Orientation) Orientation {
	switch o {
	case browserapi.OrientationDefault:
		return Orientation_Default
	case browserapi.OrientationTop:
		return Orientation_Top
	case browserapi.OrientationBottom:
		return Orientation_Bottom
	case browserapi.OrientationLeft:
		return Orientation_Left
	case browserapi.OrientationRight:
		return Orientation_Right
	default:
		panic("invalid orientation")
	}
}

func toProtoFrame(o browserapi.BarFrame) BarRequest_Frame {
	switch o {
	case browserapi.BarFrameDefault:
		return BarRequest_Default
	case browserapi.BarFrameAlways:
		return BarRequest_Always
	case browserapi.BarFrameNever:
		return BarRequest_Never
	default:
		panic("invalid orientation")
	}
}
