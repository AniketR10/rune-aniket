package rpc

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	handlerpb "unstable.build/go-tui/handler/rpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/workspace"
)

// without access to underlying stream (i.e. SendClose),
// waiting a prudent amount of time for all rpcs to finish
// is the best we can do. See https://github.com/grpc/grpc-go/issues/1714
const (
	gracefulShutdownWait = 100 * time.Millisecond
)

var _ browserapi.Browser = (*Client)(nil)

// Client satisfies Browser by talking to a browser server over RPC.
type Client struct {
	// resources invariant
	mu sync.Mutex

	broker proto.MuxBroker
	cc     grpc.ClientConnInterface
	wm     WindowManagerClient
	msg    MessengerClient
	f      ResourceOpenerClient
	p      EventPublisherClient

	clientCtx       context.Context
	clientCancelCtx func()
}

type browserClientHandler struct {
	browserapi.Handler
	srv proto.MuxServer
}

func (c browserClientHandler) gracefulShutdown(reason string) {
	time.Sleep(gracefulShutdownWait)
	c.srv.Stop()
}

func (c browserClientHandler) Close() error {
	go c.gracefulShutdown("Close")
	return c.Handler.Close()
}

func (c browserClientHandler) Dimensions() (width, height int) {
	return c.Handler.(browserapi.Floating).Dimensions()
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
) *Client {
	ret := new(Client)
	ret.Init(broker, cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "browser.Client").Logf(level, msg, args...)
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
) {
	c.wm = NewWindowManagerClient(cc)
	c.msg = NewMessengerClient(cc)
	c.cc = cc
	c.f = NewResourceOpenerClient(cc)
	c.p = NewEventPublisherClient(cc)
	c.broker = broker
	c.clientCtx, c.clientCancelCtx = context.WithCancel(context.Background())
}

func (c *Client) serveHandler(h browserapi.Handler) (channelID string, srv proto.MuxServer, err error) {
	tokenHandler, ok := h.(browser.Token)
	if ok {
		channelID = tokenHandler.ID
	} else {
		channelID, err = proto.AcceptAndServeChannel(c.clientCtx, c.broker,
			func(channelID string, srv proto.MuxServer) {
				h = browserClientHandler{
					Handler: h,
					srv:     srv,
				}
				hsrv := handlerpb.NewServer(h)
				handlerpb.RegisterHandlerServer(srv.Registrar(), hsrv)

				// if it satisfies Floating as well then register it
				if floating, ok := h.(browserapi.Floating); ok {
					fsrv := newFloatingServer(floating)
					RegisterFloatingServer(srv.Registrar(), fsrv)
				}
			}, "browser", "client", "handler")
		if err != nil {
			return
		}
	}

	return
}

// DialWindow dials the window with the given windowID token.
func (c *Client) DialWindow(channelID string, windowID uint64) (browserapi.Window, error) {
	ret, err := c.dialWindow(channelID, windowID)
	c.log(log.TraceLevel, "dial window with id %d: %#v, %v", windowID, ret, err)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *Client) dialWindow(channelID string, windowID uint64) (browserapi.Window, error) {
	winConn, err := c.broker.DialChannel(channelID)
	if err != nil {
		return nil, err
	}

	cc := NewWindowClient(winConn)
	client := newWindowClient(channelID, windowID, c, cc)
	runtime.SetFinalizer(client, func(*windowClientImpl) {
		winConn.Close()
	})
	return client, nil
}

type clientSplit func(cc WindowManagerClient,
	ctx context.Context, req *SplitRequest,
	opts ...grpc.CallOption) (*SplitResponse, error)

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

func (c *Client) split(
	split clientSplit, o browserapi.Orientation, in browserapi.Window, h browserapi.Handler,
) (browserapi.Window, error) {
	channelID, srv, err := c.serveHandler(h)
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
	out, err := c.DialWindow(res.GetWindowChannelId(), res.GetWindowId())
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return nil, err
	}
	return out, nil
}

// Split satisfies Browser.
func (c *Client) Split(o browserapi.Orientation, win browserapi.Window, h browserapi.Handler) (browserapi.Window, error) {
	win, err := c.split((WindowManagerClient).Split, o, win, h)
	runtime.KeepAlive(c)
	return win, err
}

// Resource satisfies Browser.
func (c *Client) Resource(workspace.URI) (browserapi.Handler, bool) {
	panic("Resource unimplemented in browser client")
}

// Window satisfies Browser.
func (c *Client) Window(uint64) (browserapi.Window, bool) {
	panic("Window unimplemented in browser client")
}

// Bar satisfies Browser.
func (c *Client) Bar(o browserapi.Orientation, h tui.Handler) error {
	channelID, srv, err := c.serveHandler(browser.NopHandler(h))
	if err != nil {
		return fmt.Errorf("serve handler: %w", err)
	}
	req := BarRequest{ChannelId: channelID, Orientation: toProtoOrientation(o)}
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

// SetMessage satisfies Browser.
func (c *Client) SetMessage(msg string, args ...interface{}) error {
	msg = fmt.Sprintf(msg, args...)

	ctx := context.Background()
	req := SetMessageRequest{Msg: msg}

	_, err := c.msg.SetMessage(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// Open satisfies Browser.
func (c *Client) Open(resource workspace.URI) (browserapi.Handler, error) {
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
	protoEv := new(termpb.Event)
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
func (c *Client) Interrupt() error {
	ctx := context.Background()
	protoEv := new(termpb.Event)
	err := protoEv.FromModel(term.Event{Type: term.EventInterrupt})
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
	return c.DialWindow(res.GetWindowChannelId(), res.GetWindowId())
}

// Focus satisfies Browser.
func (c *Client) Focus() (browserapi.Window, error) {
	ctx := context.Background()
	req := FocusRequest{}
	res, err := c.wm.Focus(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	return c.DialWindow(res.GetWindowChannelId(), res.GetWindowId())
}

// Floating satisfies browser.WindowManager
func (c *Client) Floating(
	h browserapi.Floating, cfg component.FloatingConfig,
) (browserapi.Window, error) {
	var atProto termpb.Coordinates
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
			WindowChannelId: fres.GetWindowChannelId(),
			WindowId:        fres.GetWindowId(),
		}, nil
	}, browserapi.OrientationDefault, &fakeWindow, h)
	runtime.KeepAlive(c)
	return win, err
}

// Tab satisfies browser.WindowManager
func (c *Client) Tab(uri workspace.URI, name string, h browserapi.Handler) (browserapi.Handler, error) {
	channelID, srv, err := c.serveHandler(h)
	if err != nil {
		return nil, fmt.Errorf("serve handler: %w", err)
	}
	req := TabRequest{
		ChannelId:    channelID,
		ResourceId:   uri.String(),
		ResourceName: name,
	}
	ctx := context.Background()
	res, err := c.wm.Tab(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return nil, err
	}
	return browser.Token{ID: res.GetChannelId()}, err
}

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	if c.clientCancelCtx != nil {
		c.clientCancelCtx()
		c.clientCancelCtx = nil
	}
	runtime.SetFinalizer(c, nil)
	return
}
