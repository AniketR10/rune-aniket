package browser

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Client satisfies Browser by talking to a browser server over RPC.
type Client struct {
	Logger *log.Logger

	cc      grpc.ClientConnInterface
	broker  proto.MuxBroker
	wm      proto.WindowManagerClient
	msg     proto.MessengerClient
	mp      proto.KeyMapperClient
	f       proto.FileOpenerClient
	p       proto.EventPublisherClient
	servers []*grpc.Server
	conns   []io.Closer
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
	c.servers = make([]*grpc.Server, 0)
	c.conns = make([]io.Closer, 0)
}

func acceptAndServe(
	broker proto.MuxBroker, register func(*grpc.Server),
) (uint32, *grpc.Server) {
	brokerID := broker.NextId()

	var wg sync.WaitGroup
	var srv *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		defer wg.Done()

		srv = grpc.NewServer(opts...)
		register(srv)
		return srv
	}

	wg.Add(1)
	go broker.AcceptAndServe(brokerID, serverFunc)
	wg.Wait()

	return brokerID, srv
}

func (c *Client) serveHandler(h tui.Handler) uint32 {
	brokerID, srv := acceptAndServe(c.broker, func(srv *grpc.Server) {
		proto.RegisterHandlerServer(srv, handler.NewServer(h))
	})
	c.servers = append(c.servers, srv)
	return brokerID
}

func (c *Client) dialToWindow(windowID uint32) (*windowClient, error) {
	conn, err := c.broker.Dial(windowID)
	if err != nil {
		return nil, err
	}

	cc := newWindowClient(proto.NewWindowClient(conn), conn)
	cc.logger = c.Logger

	c.conns = append(c.conns, cc)

	return cc, nil
}

// SplitVerticalRight satisfies Browser.
func (c *Client) SplitVerticalRight(h tui.Handler) (Window, error) {
	req := proto.SplitRequest{HandlerId: c.serveHandler(h)}
	ctx := context.Background()
	res, err := c.wm.SplitVerticalRight(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId)
}

// SplitVerticalLeft satisfies Browser.
func (c *Client) SplitVerticalLeft(h tui.Handler) (Window, error) {
	req := proto.SplitRequest{HandlerId: c.serveHandler(h)}
	ctx := context.Background()
	res, err := c.wm.SplitVerticalLeft(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId)
}

// SplitHorizontalAbove satisfies Browser.
func (c *Client) SplitHorizontalAbove(h tui.Handler) (Window, error) {
	req := proto.SplitRequest{HandlerId: c.serveHandler(h)}
	ctx := context.Background()
	res, err := c.wm.SplitHorizontalAbove(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId)
}

// SplitHorizontalBelow satisfies Browser.
func (c *Client) SplitHorizontalBelow(h tui.Handler) (Window, error) {
	req := proto.SplitRequest{HandlerId: c.serveHandler(h)}
	ctx := context.Background()
	res, err := c.wm.SplitHorizontalBelow(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.dialToWindow(res.WindowId)
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
	handlerID := c.serveHandler(eventHandlerToHandler{h})
	req := proto.SubscribeRequest{Ev: protoEv, HandlerId: handlerID}

	_, err = c.p.Subscribe(ctx, &req)
	return err
}

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() (err error) {
	for _, server := range c.servers {
		server.Stop()
	}
	for _, conn := range c.conns {
		connErr := conn.Close()
		if connErr != nil {
			err = connErr
		}
	}
	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	c.servers = c.servers[:0]
	c.conns = c.conns[:0]
	return nil
}

type clientConn struct {
	conn *grpc.ClientConn
	cc   io.Closer
}

// Server serves a Browser over GRPC.
type Server struct {
	Logger *log.Logger

	broker  proto.MuxBroker
	conns   []clientConn
	servers []*grpc.Server
	browser struct {
		Browser
		sync.Locker
	}
	interrupt func()
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
	interrupt func(),
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, lock, interrupt)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
	interrupt func(),
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Locker = lock
	s.interrupt = interrupt
	s.conns = make([]clientConn, 0)
	s.servers = make([]*grpc.Server, 0)
}

func (s *Server) dialHandler(handlerID uint32) (*handler.Client, error) {
	conn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	cc := handler.NewClient(proto.NewHandlerClient(conn), s.interrupt)
	cc.Logger = s.Logger

	s.conns = append(s.conns, clientConn{conn: conn, cc: cc})

	return cc, nil
}

func (s *Server) serveWindow(win Window) uint32 {
	brokerID, srv := acceptAndServe(s.broker, func(srv *grpc.Server) {
		proto.RegisterWindowServer(srv, newWindowServer(win, s.browser.Locker))
	})
	s.servers = append(s.servers, srv)
	return brokerID
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) split(
	ctx context.Context, req *proto.SplitRequest,
	split func(WindowManager, tui.Handler) (Window, error),
) (*proto.SplitResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.dialHandler(req.GetHandlerId())
	if err != nil {
		return nil, err
	}

	win, err := split(s.browser, handler)
	if err != nil {
		return nil, err
	}

	windowID := s.serveWindow(win)
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
	s.browser.Lock()
	defer s.browser.Unlock()

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
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.dialHandler(req.GetHandlerId())
	if err != nil {
		return nil, err
	}

	ev, err := req.GetEv().ToModel()
	if err != nil {
		return nil, err
	}

	err = s.browser.Subscribe(ev, eventHandler{cc: handler})
	if err != nil {
		return nil, err
	}

	return new(proto.SubscribeResponse), nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() error {
	var err error
	for _, conn := range s.conns {
		connErr := conn.conn.Close()
		if connErr != nil {
			err = connErr
		}
		ccErr := conn.cc.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	for _, s := range s.servers {
		s.Stop()
	}
	s.browser.Close()
	s.conns = s.conns[:0]
	s.servers = s.servers[:0]
	return err
}
