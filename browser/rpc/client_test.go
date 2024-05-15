package rpc

import (
	context "context"
	"errors"
	"fmt"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

var (
	key1 = term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}
	key2 = term.Event{Type: term.EventKey,
		Mod: term.ModAlt, Key: term.KeyBackspace}
	key3      = term.Event{Type: term.EventMouse, MouseX: 10, MouseY: 11111}
	protoKey1 = termpb.Event{
		Type: termpb.Event_TypeKey,
		Key:  termpb.Event_Ctrl4,
	}
	protoKey2 = termpb.Event{
		Type: termpb.Event_TypeKey,
		Key:  termpb.Event_CtrlH,
		Mod:  termpb.Event_Alt,
	}
	protoKey3 = termpb.Event{
		Type:   termpb.Event_TypeMouse,
		MouseX: 10,
		MouseY: 11111,
	}
)

func assertNoLeaks(t *testing.T) {
	ignoreOpenCensus := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
	goleak.VerifyNone(t, ignoreOpenCensus)
}

func newMockedClient(ctrl *gomock.Controller) (
	client *Client,
	mockCC *rpc.MockClientConnInterface,
	mockMux *rpc.MockMuxBroker,
) {
	mockCC = rpc.NewMockClientConnInterface(ctrl)
	mockMux = rpc.NewMockMuxBroker(ctrl)
	client = NewClient(context.Background(), mockMux, mockCC)
	return
}

func expectInvokeRPC(mockCC *rpc.MockClientConnInterface) {
	mockCC.EXPECT().
		Invoke(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)
}

func expectInvokeError(mockCC *rpc.MockClientConnInterface) {
	mockCC.EXPECT().
		Invoke(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).
		Times(1).
		Return(errors.New("woopsie"))
}

func assertInvokeError(t *testing.T, err error) {
	require.Error(t, err)
	assert.Contains(t, err.Error(), "woopsie")
}

func TestClientNotify(t *testing.T) {
	t.Run("invokes the pbclient rpc", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		myMsg, arg1, arg2 := "oh la la: %s %d", "obla di obla da", 5
		in := &NotifyRequest{Level: uint32(notifications.LevelWarn), Msg: fmt.Sprintf(myMsg, arg1, arg2)}
		out := new(NotifyResponse)

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/browser.Notifications/Notify"),
				gomock.Eq(in), gomock.Eq(out)).
			Times(1)

		err := client.Notify(notifications.LevelWarn, myMsg, arg1, arg2)
		require.NoError(t, err)
	})
	t.Run("bubbles up rpc error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		expectInvokeError(mockCC)

		err := client.Notify(notifications.LevelSuccess, "")
		assertInvokeError(t, err)
	})
}

func expectResourceOpen(mockCC *rpc.MockClientConnInterface, myResource workspaceapi.URI) {
	in := &OpenResourceRequest{Resource: myResource.String()}
	out := new(OpenResourceResponse)
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/browser.ResourceOpener/Open"),
			gomock.Eq(in), gomock.Eq(out)).
		Times(1)
}

func TestClientOpen(t *testing.T) {
	t.Run("invokes the pbclient rpc", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		myResource, err := workspaceapi.ParseURI("file:///fjkelwjfeklw")
		require.NoError(t, err)

		expectResourceOpen(mockCC, myResource)

		_, err = client.Open(myResource)
		require.NoError(t, err)
	})

	t.Run("bubbles up rpc error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		expectInvokeError(mockCC)

		_, err := client.Open(workspaceapi.URI{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "woopsie")
	})
}

func TestClientPublish(t *testing.T) {
	tsuite := []struct {
		rpc string
		fn  func(*Client) error
		ev  *termpb.Event
	}{
		{"PublishEventNone", (*Client).PublishEventNone, &termpb.Event{Type: termpb.Event_TypeNone}},
		{"Interrupt", func(c *Client) error {
			return c.Interrupt(context.Background())
		}, &termpb.Event{Type: termpb.Event_TypeInterrupt}},
		{"Interrupt with context payload", func(c *Client) error {
			return c.Interrupt(term.ContextWithPayload(context.Background(), []byte("1234")))
		}, &termpb.Event{Type: termpb.Event_TypeInterrupt, Raw: []byte("1234")}},
	}
	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("%s bubbles up rpc error and so stops event handler resources", tcase.rpc),
			func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				client, mockCC, _ := newMockedClient(ctrl)
				mockCC.EXPECT().
					Invoke(gomock.Any(),
						gomock.Eq("/browser.EventPublisher/Publish"),
						gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("uRich"))

				err := tcase.fn(client)
				require.Error(t, err)
			})

		t.Run(fmt.Sprintf("%s sends interrupt event publish request to server", tcase.rpc),
			func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				client, mockCC, _ := newMockedClient(ctrl)
				ev := tcase.ev
				in := &PublishRequest{Ev: ev}
				out := new(PublishResponse)

				mockCC.EXPECT().
					Invoke(gomock.Any(),
						gomock.Eq("/browser.EventPublisher/Publish"),
						gomock.Eq(in), gomock.Eq(out)).
					Times(1)

				err := tcase.fn(client)
				require.NoError(t, err)
			})
	}
}
