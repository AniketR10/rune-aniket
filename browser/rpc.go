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

// TODO
// consume handler.Client.Errors() and close upon errors and log somehwere?
type browserClientHandler struct {
	shutdownWait time.Duration
	handlerID    uint32
	tui.Handler
	c *Client
}

func (c browserClientHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = c.Handler.Handle(ev)
	if exit {
		go closeResourceCloser(c.c, c.shutdownWait, c.handlerID)
	}
	return
}

type clientResource struct {
	srv     *grpc.Server
	winConn *grpc.ClientConn
}

func (r *clientResource) Close() (err error) {
	// gracefully shutting down this clientResource
	// means that we need to close winConn only after server
	// is done shutting down resources.
	r.srv.Stop()
	return
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

	return res.winConn.WaitForStateChange(ctx, connectivity.Ready)
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

// SplitVerticalRight satisfies Browser.
func (c *Client) SplitVerticalRight(h tui.Handler) (Window, error) {
	brokerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: brokerID}
	ctx := context.Background()
	res, err := c.wm.SplitVerticalRight(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId, brokerID)
}

// SplitVerticalLeft satisfies Browser.
func (c *Client) SplitVerticalLeft(h tui.Handler) (Window, error) {
	brokerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: brokerID}
	ctx := context.Background()
	res, err := c.wm.SplitVerticalLeft(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId, brokerID)
}

// SplitHorizontalAbove satisfies Browser.
func (c *Client) SplitHorizontalAbove(h tui.Handler) (Window, error) {
	brokerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: brokerID}
	ctx := context.Background()
	res, err := c.wm.SplitHorizontalAbove(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId, brokerID)
}

// SplitHorizontalBelow satisfies Browser.
func (c *Client) SplitHorizontalBelow(h tui.Handler) (Window, error) {
	brokerID := c.serveHandler(h)
	req := proto.SplitRequest{HandlerId: brokerID}
	ctx := context.Background()
	res, err := c.wm.SplitHorizontalBelow(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId, brokerID)
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

type serverResource struct {
	handlerID   uint32
	handlerConn *grpc.ClientConn
	cc          io.Closer
	srv         *grpc.Server
	win         Window
}

func (s *Server) waitForClientClose(ctx context.Context, handlerID uint32) bool {
	s.browser.Lock()
	res, ok := s.resources[handlerID]
	s.browser.Unlock()
	if !ok {
		return false
	}

	return res.handlerConn.WaitForStateChange(ctx, connectivity.Ready)
}

func (s *serverResource) Close() (err error) {
	err1 := s.cc.Close()
	if err1 != nil {
		err = err1
	}

	err2 := s.handlerConn.Close()
	if err2 != nil {
		err = err2
	}

	if s.srv != nil {
		s.srv.Stop()
		err3 := s.win.Close()
		if err3 != nil {
			err = err3
		}
	}
	return
}

// browserServerHandler is a helper structures to enable closing
// all resources associated with a tui.Handler when it returns exit = true
// upon calls to Handle
type browserServerHandler struct {
	tui.Handler
	s         *Server
	handlerID uint32
}

func (s browserServerHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = s.Handler.Handle(ev)
	if exit {
		// we need to run asynchronously to not double lock
		// on the runtime lock.
		go s.s.closeResources(s.handlerID)
	}
	return
}

// Server serves a Browser over GRPC.
type Server struct {
	Logger *log.Logger

	broker proto.MuxBroker

	resources    map[uint32]*serverResource
	shutdownWait time.Duration

	browser struct {
		Browser
		sync.Locker
	}

	interruptDraw   func()
	interruptHandle func()
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
	interruptDraw, interruptHandle func(),
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, lock, interruptDraw, interruptHandle)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
	interruptDraw, interruptHandle func(),
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Locker = lock
	s.interruptDraw = interruptDraw
	s.interruptHandle = interruptHandle
	s.shutdownWait = 5 * time.Second
	s.resources = make(map[uint32]*serverResource)
}

func (s *Server) dialHandler(handlerID uint32) (tui.Handler, error) {
	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	pbClient := proto.NewHandlerClient(handlerConn)
	cc := handler.NewClient(pbClient, s.interruptDraw, s.interruptHandle)
	cc.Logger = s.Logger

	s.browser.Lock()
	defer s.browser.Unlock()

	s.resources[handlerID] = &serverResource{
		handlerID:   handlerID,
		handlerConn: handlerConn,
		cc:          cc,
	}

	return cc, nil
}

func (s *Server) serveWindow(win Window, handlerID uint32) uint32 {
	winSrv := newWindowServer(s, handlerID, win)
	brokerID, srv := acceptAndServe(s.broker,
		func(windowBrokerID uint32, srv *grpc.Server) {
			proto.RegisterWindowServer(srv, winSrv)
		})
	s.resources[handlerID].srv = srv
	s.resources[handlerID].win = win
	return brokerID
}

func (s *Server) closeResources(handlerID uint32) error {
	s.browser.Lock()
	defer s.browser.Unlock()

	res, ok := s.resources[handlerID]
	if !ok {
		return nil
	}

	err := res.Close()
	if err != nil && s.Logger != nil {
		s.Logger.Error(err)
	}

	delete(s.resources, res.handlerID)

	return err
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) split(
	ctx context.Context, req *proto.SplitRequest,
	split func(WindowManager, tui.Handler) (Window, error),
) (*proto.SplitResponse, error) {
	handlerID := req.GetHandlerId()
	handler, err := s.dialHandler(handlerID)
	if err != nil {
		return nil, err
	}

	handler = browserServerHandler{
		handlerID: handlerID,
		Handler:   handler,
		s:         s,
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	win, err := split(s.browser, handler)
	if err != nil {
		return nil, err
	}

	windowID := s.serveWindow(win, handlerID)
	res := &proto.SplitResponse{WindowId: windowID}
	return res, nil
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) SplitVerticalRight(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	return s.split(ctx, req, (WindowManager).SplitVerticalRight)
}

// SplitVerticalLeft satisfies proto.BrowserServer
func (s *Server) SplitVerticalLeft(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	return s.split(ctx, req, (WindowManager).SplitVerticalLeft)
}

// SplitHorizontalAbove satisfies proto.BrowserServer
func (s *Server) SplitHorizontalAbove(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	return s.split(ctx, req, (WindowManager).SplitHorizontalAbove)
}

// SplitHorizontalBelow satisfies proto.BrowserServer
func (s *Server) SplitHorizontalBelow(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	return s.split(ctx, req, (WindowManager).SplitHorizontalBelow)
}

// MergeKeyMap satisfies proto.BrowserServer
func (s *Server) MergeKeyMap(ctx context.Context, req *proto.MergeKeyMapRequest) (
	*proto.MergeKeyMapResponse, error,
) {
	m := make(map[term.Event]term.Event)

	for _, mapping := range req.Mappings {
		from, err := mapping.From.ToModel()
		if err != nil {
			return nil, err
		}
		to, err := mapping.To.ToModel()
		if err != nil {
			return nil, err
		}
		m[from] = to
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	s.browser.MergeKeyMap(m)
	return new(proto.MergeKeyMapResponse), nil
}

// SetMessage satisfies proto.BrowserServer
func (s *Server) SetMessage(ctx context.Context, req *proto.SetMessageRequest) (
	*proto.SetMessageResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	s.browser.SetMessage(req.Msg)
	return new(proto.SetMessageResponse), nil
}

// OpenFile satisfies proto.BrowserServer
func (s *Server) OpenFile(ctx context.Context, req *proto.OpenFileRequest) (
	*proto.OpenFileResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	err := s.browser.OpenFile(req.File)
	if err != nil {
		return nil, err
	}
	return new(proto.OpenFileResponse), nil
}

// Subscribe satisfies proto.BrowserServer
func (s *Server) Subscribe(ctx context.Context, req *proto.SubscribeRequest) (
	*proto.SubscribeResponse, error,
) {
	handler, err := s.dialHandler(req.GetHandlerId())
	if err != nil {
		return nil, err
	}

	ev, err := req.GetEv().ToModel()
	if err != nil {
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	err = s.browser.Subscribe(ev, eventHandler{s: s, h: handler})
	if err != nil {
		return nil, err
	}

	return new(proto.SubscribeResponse), nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() (err error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	for _, res := range s.resources {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}

	s.browser.Close()
	s.resources = nil
	return err
}
