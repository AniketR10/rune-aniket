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
)

// TODO make sure that connections are monitored and if
// something occurs, all resources are cleaned up.

func newServerWithNoBroker(ctrl *gomock.Controller) (
	*Server, *MockBrowser,
) {
	mock := NewMockBrowser(ctrl)
	s := NewServer(nil, mock, new(sync.Mutex), nop, nop)
	return s, mock
}

func newTestServer(ctrl *gomock.Controller) (
	*Server, *MockBrowser, *proto.MockMuxBroker,
) {
	mockBrowser := NewMockBrowser(ctrl)
	mockBroker := proto.NewMockMuxBroker(ctrl)
	s := NewServer(mockBroker, mockBrowser, new(sync.Mutex), nop, nop)
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

func TestServerOpenFile(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates OpenFile to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().OpenFile(gomock.Eq("/tmp/coronavirus.sql")).Return(nil)

		req := proto.OpenFileRequest{File: "/tmp/coronavirus.sql"}
		res, err := s.OpenFile(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up OpenFile Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().OpenFile(gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.OpenFile(ctx, new(proto.OpenFileRequest))
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
		s, mock, _ := newTestServer(ctrl)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeInterrupt}}

		mock.EXPECT().PublishInterrupt().Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("rejects any event other than an interrupt event", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, _, _ := newTestServer(ctrl)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeNone}}

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("handles interrupt handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, _ := newTestServer(ctrl)
		req := proto.PublishRequest{Ev: &proto.Event{Type: proto.Event_TypeInterrupt}}

		mock.EXPECT().PublishInterrupt().Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestServerSubscribe(t *testing.T) {
	ctx := context.Background()
	handlerID := uint32(31)
	protoEv := proto.Event{Mod: proto.Event_Alt, Char: '5'}
	termEv := term.Event{Mod: term.ModAlt, Ch: '5'}
	req := proto.SubscribeRequest{
		Ev:        &protoEv,
		HandlerId: handlerID,
	}

	t.Run("dials to remote handler and delegates Subscribe to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, mockBroker := newTestServer(ctrl)

		var h EventHandler
		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		mock.EXPECT().Subscribe(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ev term.Event, _h EventHandler) error {
				assert.Equal(t, termEv, ev)
				h = _h
				return nil
			})

		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		assertServerHandlerExitClose(t, handlerConn, nil,
			eventHandlerToHandler{h}, s, termEv)
	})

	t.Run("returns browser Subscribe dial to handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, _, mockBroker := newTestServer(ctrl)

		expectBrokerDialError(t, ctrl, mockBroker, handlerID)

		res, err := s.Subscribe(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		assert.Equal(t, 0, len(s.resources))
	})

	t.Run("returns browser Subscribe rpc error and so closes handler connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, mockBroker := newTestServer(ctrl)

		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		mock.EXPECT().Subscribe(gomock.Any(), gomock.Any()).
			Return(errors.New("woopsie"))

		handlerConn.EXPECT().Close().Times(1).Return(nil)
		res, err := s.Subscribe(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		assert.Equal(t, 0, len(s.resources))
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

func assertServerHandlerExitClose(
	t *testing.T, handlerConn *proto.MockMuxConn,
	mockWindow *MockWindow, h tui.Handler, s *Server,
	termEv term.Event,
) {
	expectHandlerInvokeExit(t, handlerConn)

	handlerConn.EXPECT().Close().Times(1).Return(nil)
	if mockWindow != nil {
		mockWindow.EXPECT().Close().Times(1).Return(nil)
	}

	exit, _ := h.Handle(termEv)
	// rpc handler event delivery is asynchronous
	// so second Handle response will trigger exit
	time.Sleep(200 * time.Millisecond)
	exit, _ = h.Handle(termEv)
	assert.True(t, exit)
	// close sequence is performed asynchronously
	time.Sleep(200 * time.Millisecond)

	s.browser.Lock()
	defer s.browser.Unlock()

	assert.Equal(t, 0, len(s.resources))
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
		s, mock, mockBroker := newTestServer(ctrl)
		mockWindow := NewMockWindow(ctrl)

		var h tui.Handler
		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		expect(mock.EXPECT(), gomock.Any()).
			DoAndReturn(func(_h tui.Handler) (Window, error) {
				h = _h
				return mockWindow, nil
			})
		expectBrokerServe(t, windowID, mockBroker)

		res, err := split(s, ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		// verify that handler works
		expectHandlerInvoke(handlerConn, &protoEv)
		exit, handled := h.Handle(termEv)
		assert.False(t, exit)
		assert.True(t, handled)

		time.Sleep(100 * time.Millisecond)

		// verify that handler is closeable by its handlerId
		handlerConn.EXPECT().Close().Times(1).Return(nil)
		mockWindow.EXPECT().Close().Times(1).Return(nil)
		require.NoError(t, s.forceClose(handlerID))
		assert.Equal(t, 0, len(s.resources))
	})

	t.Run("bubbles up dial error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, _, mockBroker := newTestServer(ctrl)

		expectBrokerDialError(t, ctrl, mockBroker, handlerID)

		res, err := s.SplitHorizontalAbove(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, 0, len(s.resources))
	})

	t.Run("bubbles up browser split error and so closes handler connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, mockBroker := newTestServer(ctrl)

		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		mock.EXPECT().SplitHorizontalAbove(gomock.Any()).
			Return(nil, errors.New("woopsie"))
		handlerConn.EXPECT().Close().Times(1).Return(nil)

		res, err := s.SplitHorizontalAbove(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)

		assert.Equal(t, 0, len(s.resources))
	})

	t.Run("closes resources of handler if exit = true", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, mockBroker := newTestServer(ctrl)
		mockWindow := NewMockWindow(ctrl)

		var h tui.Handler
		handlerConn := expectBrokerDial(t, ctrl, mockBroker, handlerID)
		expect(mock.EXPECT(), gomock.Any()).
			DoAndReturn(func(_h tui.Handler) (Window, error) {
				h = _h
				return mockWindow, nil
			})
		expectBrokerServe(t, windowID, mockBroker)

		_, err := split(s, ctx, &req)
		require.NoError(t, err)

		assertServerHandlerExitClose(t, handlerConn, mockWindow, h, s, termEv)
	})

	goleak.VerifyNone(t)
}
