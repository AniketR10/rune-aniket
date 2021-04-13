package util

import (
	"context"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/editor"
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
	h.Connected(broker, emptyConfig)
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

		grants := []plugin.Grant{
			{
				Token:      browserConnToken,
				Permission: plugin.PermissionBrowserEventSubscriber,
			},
		}
		h.PermissionGranted(grants)
	})

	t.Run("does nothing if shutdown is called when window not active", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		config := KeySplitHandlerConfig{Key: term.Event{Type: term.EventKey, Ch: 'a'}}
		h := &keySplitHandler{config: config}
		expectInitialization(t, ctrl, h)
		h.Shutdown("you are being naughty")
	})
}

func TestKeySplitHandlerOpenWindow(t *testing.T) {
	t.Run("open a split window if key event is received", func(t *testing.T) {
		keyEvent := term.Event{Type: term.EventKey, Ch: 'a'}
		config := KeySplitHandlerConfig{
			Key:              keyEvent,
			SplitOrientation: browser.OrientationLeft,
		}
		grants := plugin.Grant{
			Token:      1556,
			Permission: plugin.PermissionBrowserWindowManager,
		}
		testSplitWindow(t, config, grants, func(h *keySplitHandler) {
			assert.False(t, h.Handle(keyEvent))
		})
	})

	t.Run("open a split window if command received", func(t *testing.T) {
		cmdName := "blah"
		config := KeySplitHandlerConfig{
			Command:          cmdName,
			SplitOrientation: browser.OrientationLeft,
		}
		grants := plugin.Grant{
			Token:      1555,
			Permission: plugin.PermissionEditor,
		}
		testSplitWindow(t, config, grants, func(h *keySplitHandler) {
			assert.False(t, h.HandleCommand(editor.Command{Name: cmdName}))
		})
	})
}

func testSplitWindow(
	t *testing.T, config KeySplitHandlerConfig,
	grant plugin.Grant, action func(*keySplitHandler),
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config.Handler = func(grants []plugin.Grant, broker proto.MuxBroker,
		focus browser.Window, config plugin.Config) (tui.Handler, error) {
		return handler.NewTestHandler(), nil
	}
	mockWm := browser.NewMockWindowManager(ctrl)
	h := &keySplitHandler{config: config, wm: mockWm}
	mockWm.EXPECT().Focus().Return(nil, nil)
	mockWm.EXPECT().
		Split(gomock.Eq(config.SplitOrientation), gomock.Any()).
		Return(nil, nil)
	action(h)
}
