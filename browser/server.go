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
)

// Server serves a Browser over GRPC.
type Server struct {
	Logger *log.Logger

	broker proto.MuxBroker

	failureTimeout time.Duration

	// handler client resources are created on calls to Subscribe,
	// Split* and SetContent. They are destroyed when OnUnmount is dispatched to handler server
	// and so if server is closed, client connection
	// Connections are also monitored and cleaned if irrecoverable errors are found.
	clients map[uint64]io.Closer

	// window servers are created on calls to Split* and Focus. They are destroyed
	// when window is closed, either remotely, or locally (via onWindowClosed hook).
	servers map[uint64]io.Closer

	// Handlers opened by Open
	opened map[uint32]Handler

	browser struct {
		Browser
		sync.Locker
	}
}

// browserServerHandler wraps a handler.Client to satisfy browser.Handler
// and provide a hook on calls to OnUnmount. We could rely solely on the remote browser
// server to close and our monitor goroutine to clean up, but we do not trust the remote
// browser necessarily.
type browserServerHandler struct {
	Handler
	s         *Server
	handlerID uint32
}

func (s browserServerHandler) gracefulShutdown() {
	time.Sleep(gracefulShutdownWait)
	s.s.safeForceCloseHandler(s.handlerID, "browserServerHandler.OnUnmount()")
}

func (s browserServerHandler) OnUnmount() (err error) {
	err = s.Handler.OnUnmount()
	go s.gracefulShutdown()
	return
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, lock)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, browser Browser, lock sync.Locker,
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Locker = lock
	s.clients = make(map[uint64]io.Closer)
	s.servers = make(map[uint64]io.Closer)
	s.opened = make(map[uint32]Handler)
	s.failureTimeout = defaultFailureTimeout
}

func (s *Server) consumeErrors(ctx context.Context, ch <-chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-ch:
			err = fmt.Errorf("handler.Client error: %v", err)
			s.tryLog("%v", err)
			msgErr := s.setBrowserMessage(err.Error())
			if msgErr != nil {
				s.tryLog("error calling browser.SetMessage upon handler.Client"+
					" error: %v: %v", msgErr, err)
			}
		}
	}
}

func (s *Server) dialHandler(handlerID uint32) (Handler, error) {
	s.browser.Lock()
	h, ok := s.opened[handlerID]
	s.browser.Unlock()
	if ok {
		s.tryLog("(%p browser.Server): using return of Open handler for handlerID: %d", s, handlerID)
		return h, nil
	}
	s.browser.Lock()
	res, ok := s.clients[uint64(handlerID)]
	s.browser.Unlock()
	if ok {
		s.tryLog("(%p browser.Server): found cached client for handlerID: %d", s, handlerID)
		return res.(*handlerClientResource).client, nil
	}

	s.tryLog("(%p browser.Server): dialing handlerID: %d", s, handlerID)
	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	pbClient := proto.NewHandlerClient(handlerConn)
	cc := handler.NewClient(pbClient)
	cc.Logger = s.Logger
	client := newIOWaitUnlockHandler(cc, s.browser)

	ctx, cancelFn := context.WithCancel(context.Background())

	go proto.MonitorConnection(ctx, s.failureTimeout, handlerConn, func(reason string) {
		s.safeForceCloseHandler(handlerID, reason)
	})

	go s.consumeErrors(ctx, cc.Errors())

	s.browser.Lock()
	defer s.browser.Unlock()

	s.clients[uint64(handlerID)] = &handlerClientResource{
		handlerConn:   handlerConn,
		client:        client,
		cancelMonitor: cancelFn,
	}

	return client, nil
}

func (s *Server) serveWindow(win Window) uint32 {
	if res, ok := s.servers[win.id()]; ok {
		return res.(*windowServerResource).brokerID
	}

	brokerID, srv := proto.AcceptAndServe(s.broker, s.Logger,
		func(windowBrokerID uint32, srv proto.MuxServer) {
			winSrv := newWindowServer(s, win)
			proto.RegisterWindowServer(srv.GRPC(), winSrv)
		})

	s.servers[win.id()] = &windowServerResource{
		srv:      srv,
		win:      win,
		brokerID: brokerID,
	}

	win.onWindowClosed(func() {
		s.forceCloseWindow(win.id(), "underlying window called onWindowClosed callback")
	})
	return brokerID
}

func (s *Server) getClients() map[uint64]io.Closer {
	return s.clients
}

func (s *Server) getServers() map[uint64]io.Closer {
	return s.servers
}

func (s *Server) forceCloseWindow(winID uint64, reason string) error {
	s.tryLog("browser.Server.forceCloseWindow(%d, reason=%s)", winID, reason)
	_, err := proto.ForceCloseResource(winID, s.getServers, s.Logger, nopLocker{})
	return err
}

func (s *Server) safeForceCloseWindow(winID uint64, reason string) error {
	s.tryLog("browser.Server.safeForceCloseWindow(%d, reason=%s)", winID, reason)
	_, err := proto.ForceCloseResource(winID, s.getServers, s.Logger, &s.browser)
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
	_, err := proto.ForceCloseResource(uint64(brokerID), s.getClients, s.Logger, &s.browser)
	return err
}
func (s *Server) forceCloseHandler(brokerID uint32, reason string) error {
	s.tryLog("browser.Server.forceCloseHandler(%d, reason=%s)", brokerID, reason)
	_, err := proto.ForceCloseResource(uint64(brokerID), s.getClients, s.Logger, nopLocker{})
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

func (s *Server) setBrowserMessage(msg string) error {
	s.browser.Lock()
	defer s.browser.Unlock()

	return s.browser.SetMessage(msg)
}

// SetMessage satisfies proto.BrowserServer
func (s *Server) SetMessage(
	ctx context.Context, req *proto.SetMessageRequest,
) (*proto.SetMessageResponse, error) {
	msg := util.SanitizeLine(req.GetMsg())
	err := s.setBrowserMessage(msg)
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
	s.opened[handlerID] = h
	s.tryLog("(%p browser.Server): stored handler with ID: %d", s, handlerID)

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
	s.opened = nil
	return err
}
