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
	Handler
	s         *Server
	handlerID uint32
}

func (s browserServerHandler) OnUnmount() (err error) {
	err = s.Handler.OnUnmount()
	s.s.forceCloseHandler(s.handlerID, "browserServerHandler.OnUnmount()")
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

func unwrapHandler(res *handlerClientResource) Handler {
	t, ok := res.client.(localTokenHandler)
	if ok {
		return t.Handler
	}
	return res.client
}

func (s *Server) dialHandler(handlerID uint32) (Handler, error) {
	s.browser.Lock()
	res, ok := s.clients[handlerID]
	s.browser.Unlock()
	if ok {
		s.tryLog("(%p): found cached client for handlerID: %d", s, handlerID)
		return unwrapHandler(res.(*handlerClientResource)), nil
	}

	s.tryLog("(%p): dialing handlerID: %d", s, handlerID)
	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	pbClient := proto.NewHandlerClient(handlerConn)
	// TODO use cc.Errors() to consume and log errors
	cc := handler.NewClient(pbClient, s.interruptDraw, s.interruptHandle)
	cc.Logger = s.Logger
	client := newIOWaitUnlockHandler(cc, s.browser)

	ctx, cancelFn := context.WithCancel(context.Background())

	go monitorConnection(ctx, s.failureTimeout, handlerConn, func(reason string) {
		s.safeForceCloseHandler(handlerID, reason)
	})

	s.browser.Lock()
	defer s.browser.Unlock()

	s.clients[handlerID] = &handlerClientResource{
		handlerConn:   handlerConn,
		client:        client,
		cancelMonitor: cancelFn,
	}

	return client, nil
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

func (s *Server) getClients() map[uint32]io.Closer {
	return s.clients
}

func (s *Server) getServers() map[uint32]io.Closer {
	return s.servers
}

func (s *Server) safeForceCloseWindow(brokerID uint32, reason string) error {
	s.tryLog("browser.Server.safeForceCloseWindow(%d, reason=%s)", brokerID, reason)
	_, err := forceCloseResource(brokerID, s.getServers, s.Logger, &s.browser)
	return err
}

func (s *Server) tryLog(msg string, args ...interface{}) {
	if s.Logger == nil {
		return
	}
	s.Logger.Debugf(msg, args...)
}

func (s *Server) safeForceCloseHandler(brokerID uint32, reason string) error {
	s.tryLog("browser.Server.safeForceCloseHandler(%d, reason=%s)", brokerID, reason)
	_, err := forceCloseResource(brokerID, s.getClients, s.Logger, &s.browser)
	return err
}
func (s *Server) forceCloseHandler(brokerID uint32, reason string) error {
	s.tryLog("browser.Server.forceCloseHandler(%d, reason=%s)", brokerID, reason)
	_, err := forceCloseResource(brokerID, s.getClients, s.Logger, nopLocker{})
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
		Handler:   cc,
		s:         s,
	}

	s.browser.Lock()
	defer s.browser.Unlock()
	win, err := split(s.browser, bHandler)
	if err != nil {
		reason := fmt.Sprintf("failed to create split: %s", err.Error())
		s.forceCloseHandler(handlerID, reason)
		return nil, err
	}

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

func nopMonitor() {}

// Open satisfies proto.BrowserServer
func (s *Server) Open(
	ctx context.Context, req *proto.OpenResourceRequest,
) (*proto.OpenResourceResponse, error) {
	resource := util.SanitizeResourceName(req.GetResource())

	s.browser.Lock()
	defer s.browser.Unlock()

	h, err := s.browser.Open(resource)
	if err != nil {
		return nil, err
	}

	// store proxy handler
	handlerID := s.broker.NextId()

	s.clients[handlerID] = &handlerClientResource{
		client:        localTokenHandler{h},
		cancelMonitor: nopMonitor,
	}
	s.tryLog("(%p): stored handler with ID: %d: %#v", s, handlerID, s.clients[handlerID])

	return &proto.OpenResourceResponse{HandlerId: handlerID}, nil
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
	defer s.browser.Unlock()
	err = s.browser.Subscribe(ev, h)
	if err != nil {
		reason := fmt.Sprintf("failed to subscribe: %v", err)
		s.forceCloseHandler(handlerID, reason)
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
	defer s.browser.Unlock()
	err = s.browser.PublishInterrupt()
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
