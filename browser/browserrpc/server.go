// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package browserrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"

	"unstable.build/go-tui/handler/handlerrpc"
	"unstable.build/go-tui/rpc"
)

var (
	errWindowNotFound = errors.New("window with id not found or already closed")
)

const (
	defaultFailureTimeout = 5 * time.Second
)

// Server serves a Browser over GRPC.
type Server struct {
	UnimplementedEventPublisherServer
	UnimplementedNotificationsServer
	UnimplementedResourceOpenerServer
	UnimplementedWindowManagerServer

	broker   rpc.MuxBroker
	syncMode bool

	browser struct {
		browser.Browser
		sync.Locker
	}

	serverCtx       context.Context
	serverCancelCtx func()
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker rpc.MuxBroker, browser browser.Browser, lock sync.Locker,
) *Server {
	ret := new(Server)
	ret.Init(broker, browser, lock)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker rpc.MuxBroker, browser browser.Browser, lock sync.Locker,
) {
	s.broker = broker
	s.browser.Browser = browser
	s.browser.Locker = lock
	s.serverCtx, s.serverCancelCtx = context.WithCancel(context.Background())
}

// SetSyncMode ensures that all future handler clients are fully synchronous.
// This should be used only for testing.
func (s *Server) SetSyncMode() {
	s.syncMode = true
}

func (s *Server) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "browser.Server").Logf(level, msg, args...)
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
			msgErr := s.setBrowserMessage(notifications.LevelError, err.Error(), false)
			if msgErr != nil {
				s.log(log.WarnLevel, "error calling browser.Notify upon handler.Client"+
					" error: %v: %v", msgErr, err)
			}
		}
	}
}

func (s *Server) dialHandler(ctx context.Context, channelID string, tags ...string) (
	browserapi.Handler, error,
) {
	s.log(log.DebugLevel, "dialing handler at %q", channelID)
	handlerConn, err := s.broker.DialChannel(ctx, channelID, tags...)
	if err != nil {
		return nil, err
	}

	interrupter := browser.EventPublisherInterrupter(s.browser.Browser)
	pbClient := handlerrpc.NewHandlerClient(handlerConn)
	pbClient = newIOWaitUnlockHandlerClient(pbClient, s.browser.Locker)
	var handlercc interface {
		browserapi.Handler
		Errors() <-chan error
	}
	if s.syncMode {
		handlercc = handlerrpc.NewClient(pbClient)
	} else {
		handlercc = handlerrpc.NewAsyncClient(interrupter, pbClient)
	}

	ctx, cancelFn := context.WithCancel(s.serverCtx)
	fClient := NewFloatingClient(handlerConn)
	cc := newFloatingClient(handlercc, fClient, cancelFn, handlerConn)

	go s.consumeErrors(ctx, channelID, handlercc.Errors())
	go s.consumeErrors(ctx, channelID, cc.errorCh)
	go rpc.MonitorConnection(ctx, defaultFailureTimeout, handlerConn,
		func(reason string) {
			cancelFn()
			handlerConn.Close()
		})

	return cc, nil
}

func (s *Server) getContentHandler(
	ctx context.Context, channelID string, tags ...string,
) (browserapi.Handler, error) {
	// if it's not a URI, then it must be a remote handler
	uri, err := workspaceapi.ParseURI(channelID)
	if err != nil {
		return s.dialHandler(ctx, channelID, tags...)
	}
	s.browser.Lock()
	h, ok := s.browser.Resource(uri)
	s.browser.Unlock()
	if !ok {
		return s.dialHandler(ctx, channelID, tags...)
	}

	s.log(log.DebugLevel, "(%p browser.Server): using return of Open/Content handler for channelID: %s",
		s, channelID)
	return h, nil
}

func (s *Server) newRemoteResource(
	ctx context.Context, channelID string,
	action func(browser.WindowManager, browserapi.Handler) (browser.Window, error),
	tags ...string,
) (uint64, error) {
	handler, err := s.getContentHandler(ctx, channelID, tags...)
	if err != nil {
		return 0, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	win, err := action(s.browser, handler)
	if err != nil {
		return 0, err
	}
	if win == nil {
		return 0, nil
	}
	return win.ID(), nil
}

// Split satisfies BrowserServer
func (s *Server) Split(
	ctx context.Context, req *SplitRequest,
) (*SplitResponse, error) {
	windowID, err := s.newRemoteResource(ctx, req.GetChannelId(),
		func(wm browser.WindowManager, h browserapi.Handler) (browser.Window, error) {
			win, ok := s.browser.Window(req.GetWindowId())
			if !ok {
				return nil, fmt.Errorf("cannot find window with windowID: %d", req.GetWindowId())
			}
			if win.Closed() {
				return nil, fmt.Errorf("cannot split over a closed window: %d", req.GetWindowId())
			}
			return wm.Split(protoToModelOrientation(req.GetOrientation()), win, h)
		}, "browserrpc.Server", "split")
	if err != nil {
		return nil, err
	}
	return &SplitResponse{
		WindowId: windowID,
	}, nil
}

// Bar satisfies BrowserServer
func (s *Server) Bar(
	ctx context.Context, req *BarRequest,
) (*BarResponse, error) {
	handlerID := req.GetChannelId()
	handler, err := s.getContentHandler(ctx, handlerID, "browserrpc.Server", "bar")
	if err != nil {
		return nil, err
	}

	cfg := browserapi.BarConfig{}
	cfg.Orientation = protoToModelOrientation(req.GetOrientation())
	cfg.Size = int(req.GetSize())
	cfg.Frame = protoToModelBarFrame(req.GetFrame())

	s.browser.Lock()
	defer s.browser.Unlock()
	err = s.browser.Bar(cfg, handler)
	if err != nil {
		return nil, err
	}
	return new(BarResponse), nil
}

// Notify satisfies BrowserServer
func (s *Server) Notify(
	ctx context.Context, req *NotifyRequest,
) (*NotifyResponse, error) {
	return s.notify(ctx, req, false)
}

// NotifyOnce satisfies BrowserServer
func (s *Server) NotifyOnce(
	ctx context.Context, req *NotifyRequest,
) (*NotifyResponse, error) {
	return s.notify(ctx, req, true)
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
		s.log(log.WarnLevel, "error converting rpc event to model: %v", err)
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()
	err = s.browser.PublishEvent(ev)
	if err != nil {
		return nil, err
	}
	s.log(log.TraceLevel, "publish event: %v", ev)

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

	res := &FocusResponse{
		WindowId: win.ID(),
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

	res := &FocusResponse{
		WindowId: prev.ID(),
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
	windowID, err := s.newRemoteResource(ctx, req.GetChannelId(),
		func(wm browser.WindowManager, h browserapi.Handler) (browser.Window, error) {
			return wm.Floating(h.(browser.Floating), cfg)
		}, "browserrpc.Server", "floating")
	if err != nil {
		return nil, err
	}
	return &FloatingWindowResponse{
		WindowId: windowID,
	}, nil
}

// Tab satisfies BrowserServer
func (s *Server) Tab(ctx context.Context, req *TabRequest,
) (*TabResponse, error) {
	name := req.GetResourceName()
	iconStr := req.GetResourceIcon()
	id := req.GetResourceId()
	uri, err := workspaceapi.ParseURI(id)
	if err != nil {
		return nil, err
	}

	var icon rune
	if len(iconStr) != 0 {
		icon = []rune(iconStr)[0]
	}

	handler, err := s.getContentHandler(ctx, req.GetChannelId(),
		"browserrpc.Server", "tab")
	if err != nil {
		return nil, err
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	_, err = s.browser.Tab(uri, icon, name, handler)
	return &TabResponse{}, err
}

// SetContent satisfies BrowserServer.
func (s *Server) SetContent(
	ctx context.Context, req *WindowSetContentRequest,
) (*WindowSetContentResponse, error) {
	client, err := s.getContentHandler(ctx, req.GetChannelId(),
		"browserrpc.Server", "setContent")
	if err != nil {
		return nil, fmt.Errorf("failed to dial to remote handler: %v", err)
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	win, ok := s.browser.Window(req.GetWindowId())
	if !ok {
		return nil, errWindowNotFound
	}
	if win.Closed() {
		return nil, fmt.Errorf("cannot split over a closed window: %d",
			req.GetWindowId())
	}
	err = win.SetContent(client)
	if err != nil {
		return nil, fmt.Errorf("window set content: %v", err)
	}

	return new(WindowSetContentResponse), nil
}

// Close satisfies BrowserServer.
func (s *Server) Close(
	ctx context.Context, req *WindowCloseRequest,
) (*WindowCloseResponse, error) {
	s.browser.Lock()
	defer s.browser.Unlock()

	win, ok := s.browser.Window(req.GetWindowId())
	if !ok || win.Closed() {
		// close is idempotent
		return new(WindowCloseResponse), nil
	}

	err := win.Close()
	if err != nil {
		return nil, err
	}
	return new(WindowCloseResponse), nil
}

// Stop closes all resources associated with this server.
func (s *Server) Stop() (err error) {
	if s.serverCancelCtx != nil {
		s.serverCancelCtx()
		s.serverCancelCtx = nil
	}
	return nil
}

func (s *Server) setBrowserMessage(
	level notifications.Level, msg string, once bool,
) error {
	s.browser.Lock()
	defer s.browser.Unlock()

	if !once {
		return s.browser.Notify(level, msg)
	}
	return s.browser.NotifyOnce(level, msg)
}

func (s *Server) notify(
	ctx context.Context, req *NotifyRequest, once bool,
) (*NotifyResponse, error) {
	msg := sanitizeLine(req.GetMsg())
	level := notifications.Level(req.GetLevel())
	switch level {
	case notifications.LevelInfo,
		notifications.LevelSuccess,
		notifications.LevelWarn,
		notifications.LevelError:
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid level")
	}
	err := s.setBrowserMessage(level, msg, once)
	if err != nil {
		return nil, err
	}
	return new(NotifyResponse), nil
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

func protoToModelBarFrame(p BarRequest_Frame) (o browserapi.BarFrame) {
	switch p {
	case BarRequest_Default:
		o = browserapi.BarFrameDefault
	case BarRequest_Always:
		o = browserapi.BarFrameAlways
	case BarRequest_Never:
		o = browserapi.BarFrameNever
	}
	return
}

func sanitizeLine(in string) string {
	var b strings.Builder
	for _, r := range in {
		switch r {
		case '\x00':
		case '\n':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
