package browser

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/component"
	handlerpb "unstable.build/go-tui/handler/rpc"
	"unstable.build/go-tui/proto"
	prototest "unstable.build/go-tui/proto/test"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/workspace"
)

const asyncResultsSleepDuration = 300 * time.Millisecond

func newServerWithNoBroker(ctrl *gomock.Controller) (
	*Server, *MockBrowser,
) {
	mock := NewMockBrowser(ctrl)
	s := NewServer(nil, mock, new(sync.Mutex))
	return s, mock
}

func newTestServer(ctrl *gomock.Controller, mu *sync.Mutex) (
	*Server, *MockBrowser, *proto.MockMuxBroker,
) {
	mockBrowser := NewMockBrowser(ctrl)
	mockBroker := proto.NewMockMuxBroker(ctrl)
	s := NewServer(mockBroker, mockBrowser, mu)

	mockBroker.EXPECT().Cleanup(gomock.Any()).AnyTimes()

	return s, mockBrowser, mockBroker
}

func TestServerSetMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates SetMessage to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().SetMessage(gomock.Eq("blah")).Return(nil)

		req := browserpb.SetMessageRequest{Msg: "blah"}
		res, err := s.SetMessage(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up SetMessage Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().SetMessage(gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.SetMessage(ctx, new(browserpb.SetMessageRequest))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func assertHandlerStored(t *testing.T, nextID uint32, s *Server, expected Handler) {
	h, ok := s.opened[nextID]
	require.True(t, ok)
	assert.Equal(t, expected, h)
}

func TestServerOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates Open to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, broker := newTestServer(ctrl, new(sync.Mutex))
		uri, err := workspace.ParseURI("file:///tmp/coronavirus.sql")
		require.NoError(t, err)

		h := NewTestHandler()
		mock.EXPECT().Open(gomock.Eq(uri)).Return(h, nil)
		broker.EXPECT().NextId().Return(uint32(1))

		req := browserpb.OpenResourceRequest{Resource: "file:///tmp/coronavirus.sql"}
		res, err := s.Open(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up Open Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, _ := newTestServer(ctrl, new(sync.Mutex))

		mock.EXPECT().Open(gomock.Any()).Return(nil, errors.New("oopsie daisy"))

		req := browserpb.OpenResourceRequest{Resource: "file:///a"}
		_, err := s.Open(ctx, &req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func insertDrawResponse(t *testing.T, quit bool) func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return func(ctx context.Context,
		method string, args interface{},
		reply interface{}, opts ...grpc.CallOption) error {
		// validate mappings with finer grained control
		res, ok := reply.(*handlerpb.HandleResponse)
		require.True(t, ok)

		res.Draw = handlerpb.NewDrawResponse(component.NewString(""), 0, 0)
		res.Quit = quit
		res.Handled = true
		return nil
	}
}

func expectHandlerInvoke(t *testing.T, handlerConn *proto.MockMuxConn, protoEv *termpb.Event) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/handler.Handler/Handle"),
			gomock.Eq(&handlerpb.HandleRequest{Event: protoEv, Draw: &handlerpb.DrawRequest{}}),
			gomock.Any()).
		Times(1).
		DoAndReturn(insertDrawResponse(t, false))
}

func expectHandlerInvokeExit(t *testing.T, handlerConn *proto.MockMuxConn) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/handler.Handler/Handle"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(insertDrawResponse(t, true)).
		Times(1)
}

func TestServerPublish(t *testing.T) {
	ctx := context.Background()

	t.Run("handles interrupt event by calling interrupt handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := browserpb.PublishRequest{Ev: &termpb.Event{Type: termpb.Event_TypeInterrupt}}

		mock.EXPECT().Interrupt().Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("handles EventNone event by calling interrupt handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := browserpb.PublishRequest{Ev: &termpb.Event{Type: termpb.Event_TypeNone}}

		mock.EXPECT().PublishEventNone().Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("rejects any event other than an interrupt event", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, _, _ := newTestServer(ctrl, &mu)
		req := browserpb.PublishRequest{Ev: &termpb.Event{Type: termpb.Event_TypeKey}}

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("handles interrupt handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := browserpb.PublishRequest{Ev: &termpb.Event{Type: termpb.Event_TypeInterrupt}}

		mock.EXPECT().Interrupt().Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("handles event none handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := browserpb.PublishRequest{Ev: &termpb.Event{Type: termpb.Event_TypeNone}}

		mock.EXPECT().PublishEventNone().Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func assertServerClientsEqual(t *testing.T, expected int, s *Server) {
	s.browser.Lock()
	defer s.browser.Unlock()
	assert.Equal(t, expected, len(s.clients))
}

func assertServerServersEqual(t *testing.T, expected int, s *Server) {
	s.browser.Lock()
	defer s.browser.Unlock()
	assert.Equal(t, expected, len(s.servers))
}

func waitForMonitoringExit(quitCh chan struct{}) {
	<-quitCh
	time.Sleep(asyncResultsSleepDuration)
}

func assertServerHandlerExitClose(
	t *testing.T, handlerConn *proto.MockMuxConn,
	h tui.Handler, s *Server,
	termEv term.Event, quitCh chan struct{},
) {
	expectHandlerInvokeExit(t, handlerConn)

	handlerConn.EXPECT().Close().Times(1).
		DoAndReturn(prototest.ExpectSignalExit(handlerConn, quitCh, nil))

	s.browser.Lock()
	exit, _ := h.Handle(termEv)
	s.browser.Unlock()
	assert.True(t, exit)

	waitForMonitoringExit(quitCh)

	assertServerClientsEqual(t, 0, s)
}

func TestServerSetContent(t *testing.T) {
	/* tested via ex integration tests */
}
