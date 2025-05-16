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
	"sync/atomic"
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

	go s.consumeErrors(ctx, channelID, handlercc.Errors())
	go rpc.MonitorConnection(ctx, defaultFailureTimeout, handlerConn,
		func(reason string) {
			cancelFn()
			handlerConn.Close()
		})

	return handlercc, nil
}

// Split satisfies BrowserServer
func (s *Server) Split(srv WindowManager_SplitServer) error {
	msg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("receive initial request: %w", err)
	}
	req := msg.GetRequest()
	if msg.GetType() != handlerrpc.MessageType_Request || req == nil {
		return errors.New("receive initial request: missing request")
	}
	inWin, ok := s.browser.Window(req.GetWindowId())
	if !ok {
		return fmt.Errorf("cannot find window with windowID: %d", req.GetWindowId())
	}
	if inWin.Closed() {
		return fmt.Errorf("cannot split over a closed window: %d", req.GetWindowId())
	}

	orientation := protoToModelOrientation(req.GetOrientation())
	uri := req.GetUri()

	var handler browserapi.Handler
	var client *handlerrpc.ClientStream[*SplitWindowMessage]
	if uri == "" {
		client = handlerrpc.NewClientStream(s.serverCtx, srv,
			func() *SplitWindowMessage {
				return new(SplitWindowMessage)
			})
		handler = &streamHandler{mu: s.browser, Handler: client}
	} else {
		h, err := s.getResourceHandler(uri)
		if err != nil {
			return fmt.Errorf("get resource handler: %w", err)
		}
		handler = h
	}

	s.browser.Lock()
	outWin, err := s.browser.Split(orientation, inWin, handler)
	s.browser.Unlock()
	if err != nil {
		return fmt.Errorf("new split window: %w", err)
	}

	id := outWin.WindowID()
	resp := handlerrpc.InstallResourceResponse{WindowId: id}
	respMsg := handlerrpc.ServerMessage{Response: &resp}
	if err := srv.SendMsg(&respMsg); err != nil {
		return fmt.Errorf("send install response: %w", err)
	}

	if client == nil {
		return nil
	}
	handler.(*streamHandler).doneSetup()
	return client.ReceiveMessages(id)
}

// Bar satisfies BrowserServer
func (s *Server) Bar(srv WindowManager_BarServer) error {
	msg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("receive bar request: %w", err)
	}
	req := msg.GetRequest()
	if msg.GetType() != handlerrpc.MessageType_Request || req == nil {
		return errors.New("receive bar request: missing request")
	}
	client := handlerrpc.NewClientStream(s.serverCtx, srv,
		func() *BarMessage {
			return new(BarMessage)
		})

	cfg := browserapi.BarConfig{}
	cfg.Orientation = protoToModelOrientation(req.GetOrientation())
	cfg.Size = int(req.GetSize())
	cfg.Frame = protoToModelBarFrame(req.GetFrame())

	streamHandler := &streamHandler{mu: s.browser, Handler: client}

	s.browser.Lock()
	err = s.browser.Bar(cfg, streamHandler)
	s.browser.Unlock()
	if err != nil {
		return err
	}
	resp := handlerrpc.InstallResourceResponse{}
	respMsg := handlerrpc.ServerMessage{Response: &resp}
	if err := srv.SendMsg(&respMsg); err != nil {
		return fmt.Errorf("send bar install response: %w", err)
	}

	streamHandler.doneSetup()
	return client.ReceiveMessages(0)
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

	return &OpenResourceResponse{Uri: uri.String()}, nil
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
		WindowId: win.WindowID(),
	}

	return res, nil
}

// CloseWindow satisfies BrowserServer.
func (s *Server) CloseWindow(
	ctx context.Context, req *WindowCloseRequest,
) (*WindowCloseResponse, error) {
	id := req.GetWindowId()
	if id == 0 {
		return nil, fmt.Errorf("missing request window id: %d", id)
	}

	s.browser.Lock()
	defer s.browser.Unlock()

	// if window could not be found, then we should
	// mimic idempotent close behaviour
	win, ok := s.browser.Window(id)
	if !ok {
		return new(WindowCloseResponse), nil
	}

	err := win.Close()
	if err != nil {
		return nil, err
	}

	return new(WindowCloseResponse), nil
}

// Floating satisfies BrowserServer
func (s *Server) Floating(srv WindowManager_FloatingServer) error {
	msg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("receive initial request: %w", err)
	}
	req := msg.GetRequest()
	if msg.GetType() != handlerrpc.MessageType_Request || req == nil {
		return errors.New("receive initial request: missing request")
	}

	client := handlerrpc.NewClientStream[*FloatingWindowMessage](s.serverCtx, srv,
		func() *FloatingWindowMessage {
			return new(FloatingWindowMessage)
		})

	at := req.GetOffset().ToModel()
	alignment := component.Alignment(req.GetAlignment())
	cfg := component.FloatingConfig{Offset: at, Alignment: alignment}

	// NOTE: intercept the first calls to Dimensions and Resize
	// so send install response before stream starts exchanging
	// messages.
	streamHandler := &floatingStreamHandler{Floating: client}

	s.browser.Lock()
	win, err := s.browser.Floating(streamHandler, cfg)
	s.browser.Unlock()
	if err != nil {
		return fmt.Errorf("new floating window: %w", err)
	}

	id := win.WindowID()
	resp := handlerrpc.InstallResourceResponse{WindowId: id}
	respMsg := handlerrpc.ServerMessage{Response: &resp}
	if err := srv.SendMsg(&respMsg); err != nil {
		// don't close window, let next call to client stream tui.Handler
		// to error out and bubble up to user accordingly.
		return fmt.Errorf("send install response: %w", err)
	}

	streamHandler.setup.Store(true)
	return client.ReceiveMessages(id)
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
func (s *Server) SetContent(srv WindowManager_SetContentServer) error {
	msg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("receive initial request: %w", err)
	}
	req := msg.GetRequest()
	if msg.GetType() != handlerrpc.MessageType_Request || req == nil {
		return errors.New("receive initial request: missing request")
	}
	inWin, ok := s.browser.Window(req.GetWindowId())
	if !ok {
		return fmt.Errorf("cannot find window with windowID: %d", req.GetWindowId())
	}
	if inWin.Closed() {
		return fmt.Errorf("cannot set content to a closed window: %d", req.GetWindowId())
	}

	uri := req.GetUri()

	var handler browserapi.Handler
	var client *handlerrpc.ClientStream[*WindowSetContentMessage]
	if uri == "" {
		client = handlerrpc.NewClientStream(s.serverCtx, srv,
			func() *WindowSetContentMessage {
				return new(WindowSetContentMessage)
			})
		handler = &streamHandler{mu: s.browser, Handler: client}
	} else {
		h, err := s.getResourceHandler(uri)
		if err != nil {
			return fmt.Errorf("get resource handler: %w", err)
		}
		handler = h
	}

	s.browser.Lock()
	err = inWin.SetContent(handler)
	s.browser.Unlock()
	if err != nil {
		return fmt.Errorf("window set content: %w", err)
	}

	resp := handlerrpc.InstallResourceResponse{}
	respMsg := handlerrpc.ServerMessage{Response: &resp}
	if err := srv.SendMsg(&respMsg); err != nil {
		return fmt.Errorf("send install response: %w", err)
	}

	if client == nil {
		return nil
	}
	handler.(*streamHandler).doneSetup()
	return client.ReceiveMessages(0)
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

func (s *Server) getResourceHandler(uriStr string) (browserapi.Handler, error) {
	// if it's not a URI, then it must be a remote handler
	uri, err := workspaceapi.ParseURI(uriStr)
	if err != nil {
		return nil, fmt.Errorf("parse uri %q: %w", uriStr, err)
	}
	s.browser.Lock()
	h, ok := s.browser.Resource(uri)
	s.browser.Unlock()
	if !ok {
		return nil, fmt.Errorf("resource with uri %s not found", uriStr)
	}

	return h, nil
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

type streamHandler struct {
	browserapi.Handler
	mu     sync.Locker
	setup  atomic.Bool
	width  int
	height int
}

func (f *streamHandler) Resize(width, height int) {
	if !f.setup.Load() {
		f.width = width
		f.height = height
		return
	}
	f.Handler.Resize(width, height)
}

func (f *streamHandler) doneSetup() {
	f.mu.Lock()
	f.Handler.Resize(f.width, f.height)
	f.mu.Unlock()
	f.setup.Store(true)
}

type floatingStreamHandler struct {
	browserapi.Floating
	setup atomic.Bool
}

func (f *floatingStreamHandler) Dimensions() (width, height int) {
	if !f.setup.Load() {
		return
	}
	return f.Floating.Dimensions()
}

func (f *floatingStreamHandler) Resize(width, height int) {
	if !f.setup.Load() {
		return
	}
	f.Floating.Resize(width, height)
}
