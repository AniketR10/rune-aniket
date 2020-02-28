package browser

import (
	"context"
	"fmt"
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

	broker  proto.MuxBroker
	wm      proto.WindowManagerClient
	msg     proto.MessengerClient
	mp      proto.KeyMapperClient
	f       proto.FileOpenerClient
	servers []*grpc.Server
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(broker proto.MuxBroker, cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.wm = proto.NewWindowManagerClient(cc)
	ret.msg = proto.NewMessengerClient(cc)
	ret.mp = proto.NewKeyMapperClient(cc)
	ret.f = proto.NewFileOpenerClient(cc)
	ret.Init(broker)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(broker proto.MuxBroker) {
	c.broker = broker
	c.servers = make([]*grpc.Server, 0)
}

func (c *Client) serveHandler(h tui.Handler) proto.SplitRequest {
	brokerID := c.broker.NextId()

	var wg sync.WaitGroup
	var s *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		defer wg.Done()

		s = grpc.NewServer(opts...)
		proto.RegisterHandlerServer(s, handler.NewServer(h))
		return s
	}

	wg.Add(1)
	go c.broker.AcceptAndServe(brokerID, serverFunc)

	wg.Wait()
	c.servers = append(c.servers, s)

	return proto.SplitRequest{HandlerId: brokerID}

}

// SplitVerticalRight satisfies Browser.
func (c *Client) SplitVerticalRight(h tui.Handler) error {
	req := c.serveHandler(h)
	ctx := context.Background()
	_, err := c.wm.SplitVerticalRight(ctx, &req)
	return err
}

// SplitVerticalLeft satisfies Browser.
func (c *Client) SplitVerticalLeft(h tui.Handler) error {
	req := c.serveHandler(h)
	ctx := context.Background()
	_, err := c.wm.SplitVerticalLeft(ctx, &req)
	return err
}

// SplitHorizontalAbove satisfies Browser.
func (c *Client) SplitHorizontalAbove(h tui.Handler) error {
	req := c.serveHandler(h)
	ctx := context.Background()
	_, err := c.wm.SplitHorizontalAbove(ctx, &req)
	return err
}

// SplitHorizontalBelow satisfies Browser.
func (c *Client) SplitHorizontalBelow(h tui.Handler) error {
	req := c.serveHandler(h)
	ctx := context.Background()
	_, err := c.wm.SplitHorizontalBelow(ctx, &req)
	return err
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

// Close closes all resources associated with this Client.
// This client should not be used after this method is called.
func (c *Client) Close() error {
	for _, server := range c.servers {
		server.Stop()
	}
	c.servers = c.servers[:0]
	return nil
}

// Server serves a Browser over GRPC.
type Server struct {
	Logger *log.Logger

	broker  proto.MuxBroker
	conns   []*grpc.ClientConn
	browser struct {
		Browser
		*sync.Mutex
	}
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, browser Browser, rmu *sync.Mutex,
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, rmu)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, browser Browser, rmu *sync.Mutex,
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Mutex = rmu
	s.conns = make([]*grpc.ClientConn, 0)
}

func (s *Server) browserSplitPlugin(req *proto.SplitRequest) (tui.Handler, error) {
	conn, err := s.broker.Dial(req.GetHandlerId())
	if err != nil {
		return nil, err
	}

	a := handler.NewClient(proto.NewHandlerClient(conn))
	a.Logger = s.Logger

	s.conns = append(s.conns, conn)

	return a, nil
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) SplitVerticalRight(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.browserSplitPlugin(req)
	if err != nil {
		return nil, err
	}

	err = s.browser.SplitVerticalRight(handler)
	if err != nil {
		return nil, err
	}

	return new(proto.SplitResponse), nil
}

// SplitVerticalLeft satisfies proto.BrowserServer
func (s *Server) SplitVerticalLeft(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.browserSplitPlugin(req)
	if err != nil {
		return nil, err
	}

	err = s.browser.SplitVerticalLeft(handler)
	if err != nil {
		return nil, err
	}

	return new(proto.SplitResponse), nil
}

// SplitHorizontalAbove satisfies proto.BrowserServer
func (s *Server) SplitHorizontalAbove(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.browserSplitPlugin(req)
	if err != nil {
		return nil, err
	}

	err = s.browser.SplitHorizontalAbove(handler)
	if err != nil {
		return nil, err
	}

	return new(proto.SplitResponse), nil
}

// SplitHorizontalBelow satisfies proto.BrowserServer
func (s *Server) SplitHorizontalBelow(ctx context.Context, req *proto.SplitRequest) (
	*proto.SplitResponse, error,
) {
	s.browser.Lock()
	defer s.browser.Unlock()

	handler, err := s.browserSplitPlugin(req)
	if err != nil {
		return nil, err
	}

	err = s.browser.SplitHorizontalBelow(handler)
	if err != nil {
		return nil, err
	}

	return new(proto.SplitResponse), nil
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

// Close closes all resources associated with this server.
func (s *Server) Close() error {
	for _, conn := range s.conns {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	s.conns = s.conns[:0]
	return nil
}
