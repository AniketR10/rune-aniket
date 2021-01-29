package util

import (
	"context"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	prototest "github.com/ernestrc/go-tui/proto/test"
	"github.com/ernestrc/go-tui/term"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var emptyConfig = plugin.MapConfig(make(map[string]interface{}))

type handlerCloser struct {
	tui.Handler
	calledClosed int
}

func (h *handlerCloser) Close() error {
	h.calledClosed++
	return nil
}

func expectInitialization(
	t *testing.T, ctrl *gomock.Controller, h *keySplitHandler,
) *proto.MockMuxBroker {
	broker := proto.NewMockMuxBroker(ctrl)
	h.OnConnected(broker, emptyConfig)
	h.Health()
	return broker
}

func expectSubscribe(
	t *testing.T, ctrl *gomock.Controller,
	broker *proto.MockMuxBroker, ev term.Event, browserConnToken uint32,
) *proto.MockMuxConn {
	subscribedHandlerToken := uint32(124)

	protoEv := new(proto.Event)
	require.NoError(t, protoEv.FromModel(ev))

	conn := prototest.ExpectBrokerDial(t, ctrl, broker, browserConnToken)
	prototest.ExpectBrokerServe(t, subscribedHandlerToken, broker)

	conn.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/proto.EventSubscriber/Subscribe"),
			gomock.Eq(&proto.SubscribeRequest{HandlerId: uint64(subscribedHandlerToken), Ev: protoEv}),
			gomock.Any()).
		Times(1)

	return conn
}

func expectBrokerDialAnyTimes(t *testing.T, broker *proto.MockMuxBroker) {
	broker.EXPECT().Dial(gomock.Any()).
		DoAndReturn(func(brokerId uint32) (proto.MuxConn, error) {
			return nopConn{}, nil
		}).AnyTimes()
}

func expectSplitAndFocus(
	t *testing.T, ctrl *gomock.Controller,
	mockCC *proto.MockMuxConn, broker *proto.MockMuxBroker,
) {
	anyBrokerID := uint32(152)
	broker.EXPECT().NextId().Return(anyBrokerID).AnyTimes()
	broker.EXPECT().AcceptAndServe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(brokerId uint32, serverFunc func(opts []grpc.ServerOption) proto.MuxServer) {
			serverFunc(make([]grpc.ServerOption, 0))
		}).
		AnyTimes()

	windowID := uint64(1888)
	splitWindowID := uint64(99)
	mockCC.EXPECT().
		Invoke(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			if method == "/proto.WindowManager/SplitVerticalLeft" {
				splitRes, ok := reply.(*proto.SplitResponse)
				require.True(t, ok)
				splitRes.WindowId = splitWindowID
			} else if method == "/proto.WindowManager/Focus" {
				splitRes, ok := reply.(*proto.FocusResponse)
				require.True(t, ok)
				splitRes.WindowId = windowID
			}
			return nil
		}).AnyTimes()

	expectBrokerDialAnyTimes(t, broker)
}

type nopConn struct {
}

func (n nopConn) GetState() connectivity.State {
	return connectivity.Ready
}

func (n nopConn) WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool {
	return true
}

func (n nopConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return nil
}

func (n nopConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func (n nopConn) Close() error {
	return nil
}

func TestKeySplitHandlerEmpty(t *testing.T) {

	t.Run("panics if key configuration is missing", func(t *testing.T) {
		assert.Panics(t, func() {
			ServeKeySplitHandler(KeySplitHandlerConfig{})
		})
	})

	t.Run("grantee initialization runs correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := KeySplitHandlerConfig{}
		h := &keySplitHandler{config: config}

		expectInitialization(t, ctrl, h)
	})

	t.Run("subscribe to key event when subscriber permission is received", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		config := KeySplitHandlerConfig{Key: term.Event{Type: term.EventKey, Ch: 'a'}}
		h := &keySplitHandler{config: config}
		broker := expectInitialization(t, ctrl, h)

		browserConnToken := uint32(123)
		expectSubscribe(t, ctrl, broker, config.Key, browserConnToken)

		h.OnPermissionGranted(browserConnToken, plugin.PermissionBrowserEventSubscriber)
	})

	t.Run("does nothing if shutdown is called when window not active", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		config := KeySplitHandlerConfig{Key: term.Event{Type: term.EventKey, Ch: 'a'}}
		h := &keySplitHandler{config: config}
		expectInitialization(t, ctrl, h)
		h.OnShutdown("you are being naughty")
	})

	t.Run("open a split window if key event is received", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		keyEvent := term.Event{Type: term.EventKey, Ch: 'a'}
		handlerFn := func(r browser.ResourceOpener, e browser.EventPublisher,
			focus browser.Window, config plugin.Config) tui.Handler {
			return handler.NewTestHandler()
		}
		config := KeySplitHandlerConfig{
			Key:     keyEvent,
			Split:   browser.WindowManager.SplitVerticalLeft,
			Handler: handlerFn,
		}
		h := &keySplitHandler{config: config}
		broker := expectInitialization(t, ctrl, h)

		browserConnToken := uint32(155)
		conn := prototest.ExpectBrokerDial(t, ctrl, broker, browserConnToken)
		h.OnPermissionGranted(browserConnToken, plugin.PermissionBrowserWindowManager)

		expectSplitAndFocus(t, ctrl, conn, broker)

		assert.False(t, h.Handle(keyEvent))
	})
}
