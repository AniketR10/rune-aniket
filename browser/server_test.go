package browser

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

const asyncResultsSleepDuration = 300 * time.Millisecond

func nop() {}

func newServerWithNoBroker(ctrl *gomock.Controller) (
	*Server, *MockBrowser,
) {
	mock := NewMockBrowser(ctrl)
	s := NewServer(nil, mock, new(sync.Mutex), nop, nop)
	return s, mock
}

func newTestServer(ctrl *gomock.Controller, mu *sync.Mutex) (
	*Server, *MockBrowser, *proto.MockMuxBroker,
) {
	mockBrowser := NewMockBrowser(ctrl)
	mockBroker := proto.NewMockMuxBroker(ctrl)
	s := NewServer(mockBroker, mockBrowser, mu, nop, nop)
	return s, mockBrowser, mockBroker
}

func TestServerSetMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates SetMessage to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().SetMessage(gomock.Eq("blah")).Return(nil)

		req := proto.SetMessageRequest{Msg: "blah"}
		res, err := s.SetMessage(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up SetMessage Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().SetMessage(gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.SetMessage(ctx, new(proto.SetMessageRequest))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func TestServerMergeKeyMap(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates MergeKeyMap to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		req := proto.MergeKeyMapRequest{
			Mappings: []*proto.Mapping{
				&proto.Mapping{From: &protoKey1, To: &protoKey2},
				&proto.Mapping{From: &protoKey2, To: &protoKey3},
				&proto.Mapping{From: &protoKey3, To: &protoKey1},
			},
		}
		mock.EXPECT().MergeKeyMap(gomock.Any()).
			DoAndReturn(func(m map[term.Event]term.Event) error {
				expected := map[term.Event]term.Event{
					key1: key2,
					key2: key3,
					key3: key1,
				}
				assert.Equal(t, expected, m)
				return nil
			})

		res, err := s.MergeKeyMap(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up MergeKeyMap Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().MergeKeyMap(gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.MergeKeyMap(ctx, new(proto.MergeKeyMapRequest))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func TestServerOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates Open to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().Open(gomock.Eq("/tmp/coronavirus.sql")).Return(nil)

		req := proto.OpenResourceRequest{Resource: "/tmp/coronavirus.sql"}
		res, err := s.Open(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up Open Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().Open(gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.Open(ctx, new(proto.OpenResourceRequest))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func expectHandlerInvoke(handlerConn *proto.MockMuxConn, protoEv *proto.Event) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/proto.Handler/Handle"),
			gomock.Eq(&proto.HandleRequest{Event: protoEv}),
			gomock.Any()).
		Times(1).
		Return(nil)
}
func expectHandlerInvokeExit(t *testing.T, handlerConn *proto.MockMuxConn) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/proto.Handler/Handle"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(ctx context.Context,
			method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			// validate mappings with finer grained control
			res, ok := reply.(*proto.HandleResponse)
			require.True(t, ok)

			res.Quit = true
			return nil
		}).
		Times(1)
}

func TestServerPublish(t *testing.T) {
	ctx := context.Background()

	t.Run("handles interrupt event by calling interrupt handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeInterrupt}}

		mock.EXPECT().PublishInterrupt().Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("rejects any event other than an interrupt event", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, _, _ := newTestServer(ctrl, &mu)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeNone}}

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("handles interrupt handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeInterrupt}}

		mock.EXPECT().PublishInterrupt().Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func assertClientsEqual(t *testing.T, expected int, s *Server) {
	s.browser.Lock()
	defer s.browser.Unlock()
	assert.Equal(t, expected, len(s.clients))
}

func assertServersEqual(t *testing.T, expected int, s *Server) {
	s.browser.Lock()
	defer s.browser.Unlock()
	assert.Equal(t, expected, len(s.servers))
}

func TestServerSubscribe(t *testing.T) {
	ctx := context.Background()
	handlerID := uint32(31)
	protoEv := proto.Event{Mod: proto.Event_Alt, Char: '5'}
	req := proto.SubscribeRequest{
		Ev:        &protoEv,
		HandlerId: handlerID,
	}

	t.Run("dials to remote handler and delegates Subscribe to underlying Browser", func(t *testing.T) {
		termEv := term.Event{Mod: term.ModAlt, Ch: '5'}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, mockBroker := newTestServer(ctrl, &mu)

		var h EventHandler
		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		quitCh := expectMonitorConn(handlerConn)
		mock.EXPECT().Subscribe(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ev term.Event, _h EventHandler) error {
				assert.Equal(t, termEv, ev)
				h = _h
				return nil
			})

		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		assertServerHandlerExitClose(t, handlerConn,
			eventHandlerToHandler{h}, s, termEv, quitCh)
	})

	t.Run("returns browser Subscribe dial to handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, _, mockBroker := newTestServer(ctrl, &mu)

		expectBrokerDialError(t, ctrl, mockBroker, handlerID)

		res, err := s.Subscribe(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		assertServersEqual(t, 0, s)
	})

	t.Run("returns browser Subscribe rpc error and so closes handler connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, mockBroker := newTestServer(ctrl, &mu)

		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		mock.EXPECT().Subscribe(gomock.Any(), gomock.Any()).
			Return(errors.New("woopsie"))

		quitCh := expectMonitorConn(handlerConn)
		handlerConn.EXPECT().Close().Times(1).
			DoAndReturn(expectSignalExit(handlerConn, quitCh, nil))
		res, err := s.Subscribe(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		waitForMonitoringExit(quitCh)

		assertClientsEqual(t, 0, s)
		assertServersEqual(t, 0, s)
	})

	t.Run("handles transient failures by eventually shutting down connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, mockBroker := newTestServer(ctrl, &mu)
		s.failureTimeout = 50 * time.Millisecond

		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)

		quitCh := make(chan struct{})
		handlerConn.EXPECT().GetState().Return(connectivity.TransientFailure).AnyTimes()
		handlerConn.EXPECT().WaitForStateChange(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, sourceState connectivity.State) bool {
				switch sourceState {
				case connectivity.TransientFailure:
					select {
					case <-ctx.Done():
						return false
					case _, ok := <-quitCh:
						return ok
					}
				default:
					return false
				}
			}).Times(1)

		handlerConn.EXPECT().Close().Times(1).
			DoAndReturn(expectSignalExit(handlerConn, quitCh, nil))
		mock.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(nil)
		_, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)

		waitForMonitoringExit(quitCh)
	})

	goleak.VerifyNone(t)
}

func TestServerSplitHorizontalAbove(t *testing.T) {
	testServerSplit(t,
		(*MockBrowserMockRecorder).SplitHorizontalAbove,
		(*Server).SplitHorizontalAbove,
	)
}

func TestServerSplitHorizontalBelow(t *testing.T) {
	testServerSplit(t,
		(*MockBrowserMockRecorder).SplitHorizontalBelow,
		(*Server).SplitHorizontalBelow,
	)
}

func TestServerSplitVerticalLeft(t *testing.T) {
	testServerSplit(t,
		(*MockBrowserMockRecorder).SplitVerticalLeft,
		(*Server).SplitVerticalLeft,
	)
}

func TestServerSplitVerticalRight(t *testing.T) {
	testServerSplit(t,
		(*MockBrowserMockRecorder).SplitVerticalRight,
		(*Server).SplitVerticalRight,
	)
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
		DoAndReturn(expectSignalExit(handlerConn, quitCh, nil))

	// event dispatching is synchronized via browser lock
	s.browser.Lock()
	exit, _ := h.Handle(termEv)
	s.browser.Unlock()
	// rpc handler event delivery is asynchronous
	// so second Handle response will trigger exit
	time.Sleep(asyncResultsSleepDuration)
	s.browser.Lock()
	exit, _ = h.Handle(termEv)
	s.browser.Unlock()
	assert.True(t, exit)
	// close sequence is performed asynchronously
	waitForMonitoringExit(quitCh)

	assertClientsEqual(t, 0, s)
}

func testServerSplit(
	t *testing.T,
	expect func(*MockBrowserMockRecorder, interface{}) *gomock.Call,
	split func(*Server, context.Context, *proto.SplitRequest) (*proto.SplitResponse, error),
) {
	ctx := context.Background()
	handlerID := uint32(21)
	windowID := uint32(111111)
	req := proto.SplitRequest{
		HandlerId: handlerID,
	}
	protoEv := proto.Event{
		Key:    proto.Event_MouseMiddle,
		Mod:    proto.Event_Motion,
		MouseX: 10,
		MouseY: 1393291,
	}
	termEv := term.Event{
		Key:    term.MouseMiddle,
		Mod:    term.ModMotion,
		MouseX: 10,
		MouseY: 1393291,
	}

	t.Run("dials to remote handler and exposes window server for client to dial into", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, mockBroker := newTestServer(ctrl, &mu)
		mockWindow := NewMockWindow(ctrl)

		var h Handler
		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		quitCh := expectMonitorConn(handlerConn)
		expect(mock.EXPECT(), gomock.Any()).
			DoAndReturn(func(_h Handler) (Window, error) {
				h = _h
				return mockWindow, nil
			})
		expectBrokerServe(t, windowID, mockBroker)

		res, err := split(s, ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		// verify that handler works
		expectHandlerInvoke(handlerConn, &protoEv)
		s.browser.Lock()
		exit, handled := h.Handle(termEv)
		s.browser.Unlock()
		assert.False(t, exit)
		assert.True(t, handled)

		time.Sleep(asyncResultsSleepDuration)

		// verify that handler is closeable by its handlerId
		handlerConn.EXPECT().Close().Times(1).
			DoAndReturn(expectSignalExit(handlerConn, quitCh, nil))
		mockWindow.EXPECT().Close().Times(1).Return(nil)
		require.NoError(t, s.safeForceCloseHandler(handlerID, ""))
		assertClientsEqual(t, 0, s)
		waitForMonitoringExit(quitCh)

		// not ideal but it's hard to mock grpc.Server Register calls
		// windowServer calls forceCloseWindow on close.
		require.NoError(t, s.safeForceCloseWindow(windowID, ""))
		assertServersEqual(t, 0, s)
	})

	t.Run("bubbles up dial error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, _, mockBroker := newTestServer(ctrl, &mu)

		expectBrokerDialError(t, ctrl, mockBroker, handlerID)

		res, err := split(s, ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		assertClientsEqual(t, 0, s)
		assertServersEqual(t, 0, s)
	})

	t.Run("bubbles up browser split error and so closes handler connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, mockBroker := newTestServer(ctrl, &mu)

		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		quitCh := expectMonitorConn(handlerConn)
		expect(mock.EXPECT(), gomock.Any()).
			Return(nil, errors.New("woopsie"))
		handlerConn.EXPECT().Close().Times(1).
			DoAndReturn(expectSignalExit(handlerConn, quitCh, nil))

		res, err := split(s, ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
		waitForMonitoringExit(quitCh)

		assertClientsEqual(t, 0, s)
		assertServersEqual(t, 0, s)
	})

	goleak.VerifyNone(t)
}

func TestServerSetContent(t *testing.T) {
	/* tested via ex integration tests */
}
