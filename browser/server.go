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
	"github.com/ernestrc/go-tui/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Server serves a Browser over GRPC.
type Server struct {
	Logger *log.Logger

	broker proto.MuxBroker

	failureTimeout time.Duration

	// handler client resources are created on calls to Subscribe,
	// Split* and SetContent. They are destroyed when OnUnmount is invoked.
	// Connections are also monitored and cleaned if irrecoverable errors are found.
	clients map[uint32]io.Closer

	// window servers are created on calls to Split* and Focus. They are destroyed
	// when window is closed, either remotely,
	// TODO locally, (we do not have a hook yet on Component.windowClose)
	// TODO or because the handler exited (this happens naturally if we have a hook on WindowManager).
	servers map[uint32]io.Closer

	browser struct {
		Browser
		sync.Locker
	}

	interruptDraw   func()
	interruptHandle func()
}

// browserServerHandler wraps a handler.Client to satisfy browser.Handler.
type browserServerHandler struct {
	*handler.Client
	s         *Server
	handlerID uint32
}

func (s browserServerHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = s.Client.Handle(ev)
	if exit {
		// we need to run asynchronously to not double lock
		// on the runtime lock. This is a bit defensive since
		// we could trust that browser.Browser implementation
		// would call OnUnmount on browserServerHandler.
		go s.s.forceCloseHandler(s.handlerID)
	}
	return
}

func (s browserServerHandler) OnUnmount() (err error) {
	err = s.Client.OnUnmount()
	go s.s.forceCloseHandler(s.handlerID)
	return
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
	s.clients = make(map[uint32]io.Closer)
	s.servers = make(map[uint32]io.Closer)
	s.failureTimeout = defaultFailureTimeout
}

func (s *Server) dialHandler(handlerID uint32) (*handler.Client, error) {
	s.browser.Lock()
	if res, ok := s.clients[handlerID]; ok {
		return res.(*handlerClientResource).cc, nil
	}
	s.browser.Unlock()

	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	pbClient := proto.NewHandlerClient(handlerConn)
	// TODO use cc.Errors() to consume and log errors
	cc := handler.NewClient(pbClient, s.interruptDraw, s.interruptHandle)
	cc.Logger = s.Logger

	ctx, cancelFn := context.WithCancel(context.Background())

	s.browser.Lock()
	defer s.browser.Unlock()

	go monitorConnection(ctx, s.failureTimeout, handlerConn, func() {
		s.forceCloseHandler(handlerID)
	})

	s.clients[handlerID] = &handlerClientResource{
		handlerConn:   handlerConn,
		cc:            cc,
		cancelMonitor: cancelFn,
	}

	return cc, nil
}

func (s *Server) serveWindow(win Window) uint32 {
	brokerID, srv := acceptAndServe(s.broker,
		func(windowBrokerID uint32, srv *grpc.Server) {
			winSrv := newWindowServer(s, windowBrokerID, win)
			proto.RegisterWindowServer(srv, winSrv)
		})
	s.servers[brokerID] = &windowServerResource{
		srv: srv,
		win: win,
	}
	return brokerID
}

func (s *Server) getServers() map[uint32]io.Closer {
	return s.servers
}

func (s *Server) getClients() map[uint32]io.Closer {
	return s.clients
}

func (s *Server) forceCloseWindow(brokerID uint32) error {
	_, err := forceCloseResource(s.browser.Locker, brokerID, s.getServers, s.Logger)
	return err
}

func (s *Server) forceCloseHandler(brokerID uint32) error {
	_, err := forceCloseResource(s.browser.Locker, brokerID, s.getClients, s.Logger)
	return err
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) split(
	ctx context.Context, req *proto.SplitRequest,
	split func(WindowManager, Handler) (Window, error),
) (*proto.SplitResponse, error) {
	handlerID := req.GetHandlerId()
	cc, err := s.dialHandler(handlerID)
	if err != nil {
		return nil, err
	}

	bHandler := browserServerHandler{
		handlerID: handlerID,
		Client:    cc,
		s:         s,
	}

	s.browser.Lock()
	win, err := split(s.browser, bHandler)
	s.browser.Unlock()
	if err != nil {
		s.forceCloseHandler(handlerID)
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	windowID := s.serveWindow(win)
	res := &proto.SplitResponse{WindowId: windowID}
	return res, nil
}

// SplitVerticalRight satisfies proto.BrowserServer
func (s *Server) SplitVerticalRight(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	return s.split(ctx, req, (WindowManager).SplitVerticalRight)
}

// SplitVerticalLeft satisfies proto.BrowserServer
func (s *Server) SplitVerticalLeft(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	return s.split(ctx, req, (WindowManager).SplitVerticalLeft)
}

// SplitHorizontalAbove satisfies proto.BrowserServer
func (s *Server) SplitHorizontalAbove(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	return s.split(ctx, req, (WindowManager).SplitHorizontalAbove)
}

// SplitHorizontalBelow satisfies proto.BrowserServer
func (s *Server) SplitHorizontalBelow(
	ctx context.Context, req *proto.SplitRequest,
) (*proto.SplitResponse, error) {
	return s.split(ctx, req, (WindowManager).SplitHorizontalBelow)
}

// MergeKeyMap satisfies proto.BrowserServer
func (s *Server) MergeKeyMap(
	ctx context.Context, req *proto.MergeKeyMapRequest,
) (*proto.MergeKeyMapResponse, error) {
	m := make(map[term.Event]term.Event)

	for _, mapping := range req.GetMappings() {
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

	err := s.browser.MergeKeyMap(m)
	if err != nil {
		return nil, err
	}
	return new(proto.MergeKeyMapResponse), nil
}

// SetMessage satisfies proto.BrowserServer
func (s *Server) SetMessage(
	ctx context.Context, req *proto.SetMessageRequest,
) (*proto.SetMessageResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	err := s.browser.SetMessage(util.SanitizeLine(req.GetMsg()))
	if err != nil {
		return nil, err
	}
	return new(proto.SetMessageResponse), nil
}

// Open satisfies proto.BrowserServer
func (s *Server) Open(
	ctx context.Context, req *proto.OpenResourceRequest,
) (*proto.OpenResourceResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	resource := util.SanitizeResourceName(req.GetResource())
	err := s.browser.Open(resource)
	if err != nil {
		return nil, err
	}
	return new(proto.OpenResourceResponse), nil
}

// Subscribe satisfies proto.BrowserServer
func (s *Server) Subscribe(
	ctx context.Context, req *proto.SubscribeRequest,
) (*proto.SubscribeResponse, error) {
	handlerID := req.GetHandlerId()
	handler, err := s.dialHandler(handlerID)
	if err != nil {
		return nil, err
	}

	ev, err := req.GetEv().ToModel()
	if err != nil {
		return nil, err
	}

	h := serverEventHandler{handlerID: handlerID, s: s, h: handler}
	s.browser.Lock()
	err = s.browser.Subscribe(ev, h)
	s.browser.Unlock()

	if err != nil {
		s.forceCloseHandler(handlerID)
		return nil, err
	}

	return new(proto.SubscribeResponse), nil
}

// Publish satisfies proto.BrowserServer
func (s *Server) Publish(
	ctx context.Context, req *proto.PublishRequest,
) (*proto.PublishResponse, error) {
	ev, err := req.GetEv().ToModel()
	if err != nil {
		return nil, err
	}

	// NOTE: for now it's the only event allowed
	if ev.Type != term.EventInterrupt {
		return nil, fmt.Errorf("invalid event type: %v", ev.Type)
	}

	s.browser.Lock()
	err = s.browser.PublishInterrupt()
	s.browser.Unlock()

	if err != nil {
		return nil, err
	}

	return new(proto.PublishResponse), nil
}

// Focus satisfies proto.BrowserServer
func (s *Server) Focus(
	ctx context.Context, req *proto.FocusRequest,
) (*proto.FocusResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()
	win, err := s.browser.Focus()

	if err != nil {
		return nil, err
	}

	windowID := s.serveWindow(win)
	res := &proto.FocusResponse{
		WindowId: windowID,
	}

	return res, nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() (err error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	for _, res := range s.clients {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}
	for _, res := range s.servers {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}

	s.browser.Close()
	s.clients = nil
	s.servers = nil
	return err
}
