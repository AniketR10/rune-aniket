package browser

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Client satisfies Browser by talking to a browser server over RPC.
type Client struct {
	Logger *log.Logger

	// resources invariant
	mu sync.Mutex

	// one child Handler call at a time invariant
	handlerMu sync.Mutex

	// time this client waits for a grpc connection transient failure
	// to recover before we shutdown connection.
	failureTimeout time.Duration

	broker proto.MuxBroker
	cc     grpc.ClientConnInterface
	wm     proto.WindowManagerClient
	msg    proto.MessengerClient
	mp     proto.KeyMapperClient
	f      proto.ResourceOpenerClient
	s      proto.EventSubscriberClient
	p      proto.EventPublisherClient

	// window client resources. windowClients are created on
	// calls to Split* or Focus. They are destroyed when client
	// calls Close method, or server Closes window.
	// The latter is monitored via a separate goroutine.
	clients map[uint32]io.Closer

	// handler server resources. handler servers are created on
	// calls to Split or SetContent (if handler is not return of Open).
	// They are destroyed when underlying handler returns exit=true on
	// Handle, or when window is Closed, either locally or
	// remotely (via monitor goroutine).
	servers map[uint32]io.Closer
}

type browserClientHandler struct {
	handlerID uint32
	Handler
	c *Client
}

func (c browserClientHandler) OnUnmount() (err error) {
	c.c.mu.Lock()
	err1 := c.Handler.OnUnmount()
	if err1 != nil {
		err = err1
	}
	c.c.mu.Unlock()

	err2 := c.c.forceCloseHandler(c.handlerID)
	if err2 != nil {
		err = err2
	}
	return
}

func (c browserClientHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = c.Handler.Handle(ev)
	if exit {
		go c.OnUnmount()
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
	ret.f = proto.NewResourceOpenerClient(cc)
	ret.s = proto.NewEventSubscriberClient(cc)
	ret.p = proto.NewEventPublisherClient(cc)
	ret.Init(broker)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(broker proto.MuxBroker) {
	c.broker = broker
	c.clients = make(map[uint32]io.Closer)
	c.servers = make(map[uint32]io.Closer)
	c.failureTimeout = defaultFailureTimeout
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

func (c *Client) serveHandler(h Handler) uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	brokerID, srv := acceptAndServe(c.broker,
		func(handlerID uint32, srv *grpc.Server) {
			h = browserClientHandler{
				Handler:   h,
				c:         c,
				handlerID: handlerID,
			}
			proto.RegisterHandlerServer(srv, handler.NewServer(h, &c.handlerMu))
		})

	c.servers[brokerID] = &handlerServerResource{h: h, srv: srv}
	return brokerID
}

func (c *Client) dialWindow(windowID uint32, handlerID int) (*windowClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if res, ok := c.clients[windowID]; ok {
		return res.(*windowClientResource).cc, nil
	}

	winConn, err := c.broker.Dial(windowID)
	if err != nil {
		return nil, err
	}

	cc := newWindowClient(windowID, c, proto.NewWindowClient(winConn))
	cc.logger = c.Logger

	ctx, cancelFn := context.WithCancel(context.Background())

	go monitorConnection(ctx, c.failureTimeout, winConn, func() {
		// only applies when connection is closed remotely
		c.mu.Lock()
		delete(c.clients, windowID)
		c.mu.Unlock()

		if handlerID >= 0 {
			// window was created and populated with a local handler
			// which is ephemeral, from the server's point of view
			// so we can clean resources on the client.
			c.forceCloseHandler(uint32(handlerID))
		}
	})

	c.clients[windowID] = &windowClientResource{
		winConn:       winConn,
		cancelMonitor: cancelFn,
		cc:            cc,
	}

	return cc, nil
}

func (c *Client) getServers() map[uint32]io.Closer {
	return c.servers
}

func (c *Client) getClients() map[uint32]io.Closer {
	return c.clients
}

func (c *Client) forceCloseHandler(brokerID uint32) error {
	_, err := forceCloseResource(&c.mu, brokerID, c.getServers, c.Logger)
	return err
}

func (c *Client) forceCloseWindow(brokerID uint32) error {
	_, err := forceCloseResource(&c.mu, brokerID, c.getClients, c.Logger)
	return err
}

type clientSplit func(cc proto.WindowManagerClient,
	ctx context.Context, req *proto.SplitRequest,
	opts ...grpc.CallOption) (*proto.SplitResponse, error)

func (c *Client) split(split clientSplit, h Handler) (Window, error) {
	handlerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: handlerID}
	ctx := context.Background()
	res, err := split(c.wm, ctx, &req)
	if err != nil {
		c.forceCloseHandler(handlerID)
		return nil, err
	}
	win, err := c.dialWindow(res.GetWindowId(), int(handlerID))
	if err != nil {
		c.forceCloseHandler(handlerID)
		return nil, err
	}
	return win, nil
}

// SplitVerticalRight satisfies Browser.
func (c *Client) SplitVerticalRight(h Handler) (Window, error) {
	return c.split((proto.WindowManagerClient).SplitVerticalRight, h)
}

// SplitVerticalLeft satisfies Browser.
func (c *Client) SplitVerticalLeft(h Handler) (Window, error) {
	return c.split((proto.WindowManagerClient.SplitVerticalLeft), h)
}

// SplitHorizontalAbove satisfies Browser.
func (c *Client) SplitHorizontalAbove(h Handler) (Window, error) {
	return c.split((proto.WindowManagerClient.SplitHorizontalAbove), h)
}

// SplitHorizontalBelow satisfies Browser.
func (c *Client) SplitHorizontalBelow(h Handler) (Window, error) {
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

// Open satisfies Browser.
func (c *Client) Open(resource string) error {
	ctx := context.Background()
	req := proto.OpenResourceRequest{Resource: resource}

	_, err := c.f.Open(ctx, &req)
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

	evh := newClientEventHandler(c, h)
	handlerID := c.serveHandler(evh)
	evh.setHandlerID(handlerID)

	req := proto.SubscribeRequest{Ev: protoEv, HandlerId: handlerID}

	_, err = c.s.Subscribe(ctx, &req)
	if err != nil {
		c.forceCloseHandler(handlerID)
	}
	return err
}

// PublishInterrupt satisfies Browser.
func (c *Client) PublishInterrupt() error {
	ctx := context.Background()
	protoEv := new(proto.Event)
	err := protoEv.FromModel(term.Event{Type: term.EventInterrupt})
	if err != nil {
		return err
	}
	req := proto.PublishRequest{Ev: protoEv}

	_, err = c.p.Publish(ctx, &req)
	return err
}

// Focus satisfies Browser.
func (c *Client) Focus() (Window, error) {
	ctx := context.Background()
	req := proto.FocusRequest{}
	res, err := c.wm.Focus(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialWindow(res.GetWindowId(), -1)
}

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, res := range c.clients {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}
	for _, res := range c.servers {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}
	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	c.clients = nil
	c.servers = nil
	return
}
