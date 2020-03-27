package browser

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type clientResource struct {
	srv     *grpc.Server
	winConn proto.MuxConn
}

// Client satisfies Browser by talking to a browser server over RPC.
type Client struct {
	Logger *log.Logger

	mu sync.Mutex

	broker proto.MuxBroker
	cc     grpc.ClientConnInterface
	wm     proto.WindowManagerClient
	msg    proto.MessengerClient
	mp     proto.KeyMapperClient
	f      proto.FileOpenerClient
	p      proto.EventPublisherClient

	shutdownWait time.Duration
	resources    map[uint32]*clientResource
}

type browserClientHandler struct {
	shutdownWait time.Duration
	handlerID    uint32
	tui.Handler
	c *Client
}

func (r *clientResource) Close() (err error) {
	// gracefully shutting down this clientResource
	// means that we need to close winConn only after server
	// is done shutting down resources.
	r.srv.Stop()
	return
}

func (c browserClientHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = c.Handler.Handle(ev)
	if exit {
		go closeResourceCloser(c.c, c.shutdownWait, c.handlerID)
	}
	return
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(broker proto.MuxBroker, cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.wm = proto.NewWindowManagerClient(cc)
	ret.msg = proto.NewMessengerClient(cc)
	ret.cc = cc
	ret.mp = proto.NewKeyMapperClient(cc)
	ret.f = proto.NewFileOpenerClient(cc)
	ret.p = proto.NewEventPublisherClient(cc)
	ret.Init(broker)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(broker proto.MuxBroker) {
	c.broker = broker
	c.shutdownWait = 5 * time.Second
	c.resources = make(map[uint32]*clientResource)
}

func acceptAndServe(
	broker proto.MuxBroker, register func(uint32, *grpc.Server),
) (uint32, *grpc.Server) {
	brokerID := broker.NextId()

	var wg sync.WaitGroup
	var srv *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		defer wg.Done()

		srv = grpc.NewServer(opts...)
		register(brokerID, srv)
		return srv
	}

	wg.Add(1)
	go broker.AcceptAndServe(brokerID, serverFunc)
	wg.Wait()

	return brokerID, srv
}

func (c *Client) serveHandler(h tui.Handler) uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	brokerID, srv := acceptAndServe(c.broker,
		func(handlerID uint32, srv *grpc.Server) {
			h = browserClientHandler{
				Handler:      h,
				c:            c,
				shutdownWait: c.shutdownWait,
				handlerID:    handlerID,
			}
			proto.RegisterHandlerServer(srv, handler.NewServer(h))
		})

	c.resources[brokerID] = &clientResource{srv: srv}
	return brokerID
}

func (c *Client) dialToWindow(
	windowID uint32, handlerID uint32,
) (*windowClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	winConn, err := c.broker.Dial(windowID)
	if err != nil {
		return nil, err
	}

	cc := newWindowClient(handlerID, c, proto.NewWindowClient(winConn))
	cc.logger = c.Logger

	c.resources[handlerID].winConn = winConn

	return cc, nil
}

func (c *Client) waitForClientClose(ctx context.Context, handlerID uint32) bool {
	c.mu.Lock()
	res, ok := c.resources[handlerID]
	c.mu.Unlock()

	if !ok {
		return false
	}

	realConn, ok := res.winConn.(*grpc.ClientConn)
	if !ok {
		return true
	}

	return realConn.WaitForStateChange(ctx, connectivity.Ready)
}

func (c *Client) getResources(handlerID uint32) (*clientResource, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	res, ok := c.resources[handlerID]
	return res, ok
}

func (c *Client) closeResources(handlerID uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	res, ok := c.resources[handlerID]
	if !ok {
		if c.Logger != nil {
			c.Logger.Warnf("closeResources: handler %d already closed", handlerID)
		}
		return nil
	}

	err := res.Close()
	if err != nil && c.Logger != nil {
		c.Logger.Error(err)
	}

	delete(c.resources, handlerID)

	return err
}

type clientSplit func(cc proto.WindowManagerClient,
	ctx context.Context, req *proto.SplitRequest,
	opts ...grpc.CallOption) (*proto.SplitResponse, error)

func (c *Client) split(split clientSplit, h tui.Handler) (Window, error) {
	handlerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: handlerID}
	ctx := context.Background()
	res, err := split(c.wm, ctx, &req)
	if err != nil {
		c.closeResources(handlerID)
		return nil, err
	}
	win, err := c.dialToWindow(res.WindowId, handlerID)
	if err != nil {
		c.closeResources(handlerID)
		return nil, err
	}
	return win, nil
}

// SplitVerticalRight satisfies Browser.
func (c *Client) SplitVerticalRight(h tui.Handler) (Window, error) {
	return c.split((proto.WindowManagerClient).SplitVerticalRight, h)
}

// SplitVerticalLeft satisfies Browser.
func (c *Client) SplitVerticalLeft(h tui.Handler) (Window, error) {
	return c.split((proto.WindowManagerClient.SplitVerticalLeft), h)
}

// SplitHorizontalAbove satisfies Browser.
func (c *Client) SplitHorizontalAbove(h tui.Handler) (Window, error) {
	return c.split((proto.WindowManagerClient.SplitHorizontalAbove), h)
}

// SplitHorizontalBelow satisfies Browser.
func (c *Client) SplitHorizontalBelow(h tui.Handler) (Window, error) {
	return c.split((proto.WindowManagerClient.SplitHorizontalBelow), h)
}

// MergeKeyMap satisfies Browser.
func (c *Client) MergeKeyMap(m map[term.Event]term.Event) error {
	ctx := context.Background()
	req := proto.MergeKeyMapRequest{}

	for from, to := range m {
		protoFrom, protoTo := new(proto.Event), new(proto.Event)
		err := protoFrom.FromModel(from)
		if err != nil {
			return err
		}
		err = protoTo.FromModel(to)
		if err != nil {
			return err
		}
		req.Mappings = append(req.Mappings, &proto.Mapping{
			From: protoFrom,
			To:   protoTo,
		})
	}

	_, err := c.mp.MergeKeyMap(ctx, &req)
	return err
}

// SetMessage satisfies Browser.
func (c *Client) SetMessage(msg string, args ...interface{}) error {
	msg = fmt.Sprintf(msg, args...)

	ctx := context.Background()
	req := proto.SetMessageRequest{Msg: msg}

	_, err := c.msg.SetMessage(ctx, &req)
	return err
}

// OpenFile satisfies Browser.
func (c *Client) OpenFile(filename string) error {
	ctx := context.Background()
	req := proto.OpenFileRequest{File: filename}

	_, err := c.f.OpenFile(ctx, &req)
	return err
}

// Subscribe satisfies Browser.
func (c *Client) Subscribe(ev term.Event, h EventHandler) error {
	ctx := context.Background()

	protoEv := new(proto.Event)
	err := protoEv.FromModel(ev)
	if err != nil {
		return err
	}

	// NOTE: for now we don't have an unsubscribe mechanism
	// so this rpc handler always stays open until Client.Close is called.
	handlerID := c.serveHandler(eventHandlerToHandler{h})
	req := proto.SubscribeRequest{Ev: protoEv, HandlerId: handlerID}

	_, err = c.p.Subscribe(ctx, &req)
	if err != nil {
		c.closeResources(handlerID)
	}
	return err
}

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, res := range c.resources {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
		if res.winConn != nil {
			winErr := res.winConn.Close()
			if winErr != nil {
				err = winErr
			}
		}
	}
	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	c.resources = nil
	return
}
