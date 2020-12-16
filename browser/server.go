package browser

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// TODO consume handler.Client.Errors() and close upon errors and log somehwere?
type serverResource struct {
	handlerID   uint32
	handlerConn proto.MuxConn
	cc          io.Closer
	srv         *grpc.Server
	win         Window
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

// browserServerHandler is a helper structures to enable closing
// all resources associated with a tui.Handler when it returns exit = true
// upon calls to Handle
type browserServerHandler struct {
	tui.Handler
	s         *Server
	handlerID uint32
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

func (s browserServerHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = s.Handler.Handle(ev)
	if exit {
		// we need to run asynchronously to not double lock
		// on the runtime lock.
		go s.s.forceClose(s.handlerID)
	}
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
	s.shutdownWait = 5 * time.Second
	s.resources = make(map[uint32]*serverResource)
}

func (s *Server) waitForClientClose(ctx context.Context, handlerID uint32) bool {
	s.browser.Lock()
	res, ok := s.resources[handlerID]
	s.browser.Unlock()
	if !ok {
		return false
	}

	realConn, ok := res.handlerConn.(*grpc.ClientConn)
	if !ok {
		return true
	}

	return realConn.WaitForStateChange(ctx, connectivity.Ready)
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

func (s *Server) forceClose(handlerID uint32) error {
	s.browser.Lock()
	defer s.browser.Unlock()

	res, ok := s.resources[handlerID]
	if !ok {
		if s.Logger != nil {
			s.Logger.Warnf("handler %d already closed", handlerID)
		}
		return nil
	}

	err := res.Close()
	if err != nil && s.Logger != nil {
		s.Logger.Error(err)
	}

	delete(s.resources, handlerID)

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
	win, err := split(s.browser, handler)
	s.browser.Unlock()
	if err != nil {
		s.forceClose(handlerID)
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	windowID := s.serveWindow(win, handlerID)
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

// OpenFile satisfies proto.BrowserServer
func (s *Server) OpenFile(
	ctx context.Context, req *proto.OpenFileRequest,
) (*proto.OpenFileResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	filename := util.SanitizeFilename(req.GetFile())
	err := s.browser.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	return new(proto.OpenFileResponse), nil
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
		s.forceClose(handlerID)
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
