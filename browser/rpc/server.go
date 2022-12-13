package rpc

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"

	handlerpb "unstable.build/go-tui/handler/rpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/util"
)

// Server serves a Browser over GRPC.
type Server struct {
	UnimplementedEventPublisherServer
	UnimplementedMessengerServer
	UnimplementedResourceOpenerServer
	UnimplementedWindowManagerServer

	broker proto.MuxBroker

	browser struct {
		browser.Browser
		sync.Locker
	}

	windowServer    func(*Server, browser.Window) WindowServer
	serverCtx       context.Context
	serverCancelCtx func()
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, browser browser.Browser, lock sync.Locker,
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, lock, NewWindowServer)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, browser browser.Browser, lock sync.Locker,
	windowServer func(*Server, browser.Window) WindowServer,
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Locker = lock
	s.windowServer = windowServer
	s.serverCtx, s.serverCancelCtx = context.WithCancel(context.Background())
}

func (s *Server) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "browser.Server").Logf(level, msg, args...)
}

func (s *Server) consumeErrors(
	ctx context.Context, channelID string, ch <-chan error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-ch:
			err = fmt.Errorf("handler.Client %s error: %v", channelID, err)
			s.log(log.WarnLevel, "%v", err)
			msgErr := s.setBrowserMessage(err.Error())
			if msgErr != nil {
				s.log(log.WarnLevel, "error calling browser.SetMessage upon handler.Client"+
					" error: %v: %v", msgErr, err)
			}
		}
	}
}

func (s *Server) dialHandler(channelID string) (browser.Handler, error) {
	s.log(log.DebugLevel, "(%p browser.Server): dialing handlerID: %s", s, channelID)
	handlerConn, err := s.broker.DialChannel(channelID)
	if err != nil {
		return nil, err
	}

	pbClient := handlerpb.NewHandlerClient(handlerConn)
	fClient := NewFloatingClient(handlerConn)
	pbClient = newIOWaitUnlockHandlerClient(pbClient, s.browser.Locker)
	handlercc := handlerpb.NewClient(pbClient)
	cc := newFloatingClient(handlercc, fClient)

	ctx, cancelFn := context.WithCancel(s.serverCtx)

	go s.consumeErrors(ctx, channelID, handlercc.Errors())
	go s.consumeErrors(ctx, channelID, cc.errorCh)

	runtime.SetFinalizer(cc, func(*floatingClientImpl) {
		cancelFn()
		handlerConn.Close()
	})
	return cc, nil
}

// ServeWindow serves window and returns the token that can be used to reference it.
// Callers are responsible for synchronizing with the locker passed to NewServer.
func (s *Server) ServeWindow(win browser.Window) (string, error) {
	return s.serveWindow(win)
}

func (s *Server) serveWindow(win browser.Window) (string, error) {
	ret, err := s.doServeWindow(win)
	s.log(log.TraceLevel, "%p: serve window %d: %s, %v", s, win.ID(), ret, err)
	return ret, err
}

func (s *Server) doServeWindow(win browser.Window) (string, error) {
	serveCtx, cancelServe := context.WithCancel(s.serverCtx)
	channelID, err := proto.AcceptAndServeChannel(serveCtx, s.broker,
		func(_ string, srv proto.MuxServer) {
			winSrv := s.windowServer(s, win)
			RegisterWindowServer(srv.Registrar(), winSrv)
		}, "browser", "server", "window")
	if err != nil {
		cancelServe()
		return "", err
	}

	win.OnWindowClosed(func(ctx context.Context) {
		s.log(log.TraceLevel, "%p: stopping mux server for window %s", s, channelID)
		// asynchronously wait for close context to be done before stopping server
		// set a timeout in case ctx never cancels
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		go func() {
			<-ctx.Done()
			cancelServe()
			cancel()
			s.log(log.DebugLevel, "%p: stopped mux server for window %s", s, channelID)
		}()
	})
	return channelID, nil
}

func (s *Server) getContentHandler(channelID string) (browser.Handler, error) {
	// if it's not a URI, then it must be a remote handler
	uri, err := workspaceapi.ParseURI(channelID)
	if err != nil {
		return s.dialHandler(channelID)
	}
	s.browser.Lock()
	h, ok := s.browser.Resource(uri)
	s.browser.Unlock()
	if !ok {
		return s.dialHandler(channelID)
	}

	s.log(log.DebugLevel, "(%p browser.Server): using return of Open/Content handler for channelID: %s",
		s, channelID)
	return h, nil
}

func (s *Server) newRemoteResource(
	ctx context.Context, channelID string,
	action func(browser.WindowManager, browser.Handler) (browser.Window, error),
) (string, uint64, error) {
	handler, err := s.getContentHandler(channelID)
	if err != nil {
		return "", 0, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()
	win, err := action(s.browser, handler)
	if err != nil {
		return "", 0, err
	}
	if win == nil {
		return "", 0, nil
	}

	winID, err := s.serveWindow(win)
	if err != nil {
		return "", 0, fmt.Errorf("serve window: %w", err)
	}
	return winID, win.ID(), nil
}

func protoToModelOrientation(p Orientation) (o browserapi.Orientation) {
	switch p {
	case Orientation_Default:
		o = browserapi.OrientationDefault
	case Orientation_Top:
		o = browserapi.OrientationTop
	case Orientation_Bottom:
		o = browserapi.OrientationBottom
	case Orientation_Left:
		o = browserapi.OrientationLeft
	case Orientation_Right:
		o = browserapi.OrientationRight
	}
	return
}

// Split satisfies BrowserServer
func (s *Server) Split(
	ctx context.Context, req *SplitRequest,
) (*SplitResponse, error) {
	windowChannelID, windowID, err := s.newRemoteResource(ctx, req.GetChannelId(),
		func(wm browser.WindowManager, h browser.Handler) (browser.Window, error) {
			win, ok := s.browser.Window(req.GetWindowId())
			if !ok {
				return nil, fmt.Errorf("cannot find window with windowID: %d", req.GetWindowId())
			}
			if win.Closed() {
				return nil, fmt.Errorf("cannot split over a closed window: %d", req.GetWindowId())
			}
			return wm.Split(protoToModelOrientation(req.GetOrientation()), win, h)
		})
	if err != nil {
		return nil, err
	}
	return &SplitResponse{
		WindowChannelId: windowChannelID,
		WindowId:        windowID,
	}, nil
}

// Bar satisfies BrowserServer
func (s *Server) Bar(
	ctx context.Context, req *BarRequest,
) (*BarResponse, error) {
	handlerID := req.GetChannelId()
	handler, err := s.getContentHandler(handlerID)
	if err != nil {
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()
	err = s.browser.Bar(protoToModelOrientation(req.GetOrientation()), handler)
	if err != nil {
		return nil, err
	}
	return new(BarResponse), nil
}

func (s *Server) setBrowserMessage(msg string) error {
	s.browser.Lock()
	defer s.browser.Unlock()

	return s.browser.SetMessage(msg)
}

// SetMessage satisfies BrowserServer
func (s *Server) SetMessage(
	ctx context.Context, req *SetMessageRequest,
) (*SetMessageResponse, error) {
	msg := util.SanitizeLine(req.GetMsg())
	err := s.setBrowserMessage(msg)
	if err != nil {
		return nil, err
	}
	return new(SetMessageResponse), nil
}

// Open satisfies BrowserServer
func (s *Server) Open(
	ctx context.Context, req *OpenResourceRequest,
) (*OpenResourceResponse, error) {
	uri, err := workspaceapi.ParseURI(req.GetResource())
	if err != nil {
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	_, err = s.browser.Open(uri)
	if err != nil {
		return nil, err
	}

	return &OpenResourceResponse{ChannelId: uri.String()}, nil
}

// Publish satisfies BrowserServer
func (s *Server) Publish(
	ctx context.Context, req *PublishRequest,
) (*PublishResponse, error) {
	ev, err := req.GetEv().ToModel()
	if err != nil {
		return nil, err
	}

	var fn func() error
	switch ev.Type {
	case term.EventInterrupt:
		fn = s.browser.Interrupt
	case term.EventNone:
		fn = s.browser.PublishEventNone
	default:
		return nil, fmt.Errorf("invalid event type: %v", ev.Type)
	}

	s.browser.Lock()
	defer s.browser.Unlock()
	err = fn()
	if err != nil {
		return nil, err
	}

	return new(PublishResponse), nil
}

// Focus satisfies BrowserServer
func (s *Server) Focus(
	ctx context.Context, req *FocusRequest,
) (*FocusResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()
	win, err := s.browser.Focus()

	if err != nil {
		return nil, err
	}

	windowChannelID, err := s.serveWindow(win)
	if err != nil {
		return nil, fmt.Errorf("serve window: %w", err)
	}

	res := &FocusResponse{
		WindowId:        win.ID(),
		WindowChannelId: windowChannelID,
	}

	return res, nil
}

// SetFocus satisfies BrowserServer
func (s *Server) SetFocus(
	ctx context.Context, req *SetFocusRequest,
) (*FocusResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	win, ok := s.browser.Window(req.GetWindowId())
	if !ok {
		return nil, fmt.Errorf("cannot find window with windowID: %d", req.GetWindowId())
	}

	if win.Closed() {
		return nil, fmt.Errorf("cannot set focus to a closed window: %d", req.GetWindowId())
	}

	prev, err := s.browser.SetFocus(win)
	if err != nil {
		return nil, err
	}

	windowChannelID, err := s.serveWindow(prev)
	if err != nil {
		return nil, fmt.Errorf("serve window: %w", err)
	}

	res := &FocusResponse{
		WindowId:        prev.ID(),
		WindowChannelId: windowChannelID,
	}

	return res, nil
}

// Floating satisfies BrowserServer
func (s *Server) Floating(
	ctx context.Context, req *FloatingWindowRequest,
) (*FloatingWindowResponse, error) {
	at := req.GetOffset().ToModel()
	alignment := component.Alignment(req.GetAlignment())
	cfg := component.FloatingConfig{Offset: at, Alignment: alignment}
	windowChannelID, windowID, err := s.newRemoteResource(ctx, req.GetChannelId(),
		func(wm browser.WindowManager, h browser.Handler) (browser.Window, error) {
			return wm.Floating(h.(browser.Floating), cfg)
		})
	if err != nil {
		return nil, err
	}
	return &FloatingWindowResponse{
		WindowChannelId: windowChannelID,
		WindowId:        windowID,
	}, nil
}

// Tab satisfies BrowserServer
func (s *Server) Tab(
	ctx context.Context, req *TabRequest,
) (*TabResponse, error) {
	name := req.GetResourceName()
	id := req.GetResourceId()
	uri, err := workspaceapi.ParseURI(id)
	if err != nil {
		return nil, err
	}
	_, _, err = s.newRemoteResource(ctx, req.GetChannelId(),
		func(wm browser.WindowManager, h browser.Handler) (w browser.Window, err error) {
			_, err = wm.Tab(uri, name, h)
			return nil, err
		})
	if err != nil {
		return nil, err
	}
	return &TabResponse{ChannelId: id}, nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() (err error) {
	if s.serverCancelCtx != nil {
		s.serverCancelCtx()
		s.serverCancelCtx = nil
	}
	return nil
}
