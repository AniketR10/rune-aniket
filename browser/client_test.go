package browser

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/proto"
	prototest "unstable.build/go-tui/proto/test"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/workspace"
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
	mockCC *proto.MockClientConnInterface,
	mockMux *proto.MockMuxBroker,
) {
	mockCC = proto.NewMockClientConnInterface(ctrl)
	mockMux = proto.NewMockMuxBroker(ctrl)
	client = NewClient(mockMux, mockCC)

	mockMux.EXPECT().Cleanup(gomock.Any()).AnyTimes()
	return
}

func expectInvokeRPC(mockCC *proto.MockClientConnInterface) {
	mockCC.EXPECT().
		Invoke(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)
}

func expectInvokeError(mockCC *proto.MockClientConnInterface) {
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

func assertClientHandlerExitClose(
	t *testing.T,
	h *TestHandler, mockWinConn *proto.MockMuxConn,
	client *Client, callClose bool,
) {
	assertClientServersEqual(t, 1, client)

	h.Exit = true

	client.mu.Lock()
	cliRes := client.servers[1]

	exit, handled := cliRes.(*handlerServerResource).h.
		Handle(term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash})
	client.mu.Unlock()
	assert.True(t, exit)
	assert.True(t, handled)

	if callClose {
		require.NoError(t, cliRes.(*handlerServerResource).h.(Handler).Close())
	}

	// unfortunately gracefulshutdowns are asynchronous
	// because they wait on grpc connection to be shutdown fist by
	// server
	time.Sleep(asyncResultsSleepDuration)

	assertClientServersEqual(t, 0, client)
}

func expectSplit(
	t *testing.T, mockCC *proto.MockClientConnInterface,
	handlerID, inWindowID, outWindowID uint64, orientation browserpb.Orientation,
) {
	expectedReq := &browserpb.SplitRequest{
		Orientation: orientation,
		HandlerId:   handlerID,
		WindowId:    inWindowID,
	}
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/browser.WindowManager/Split"),
			gomock.Eq(expectedReq),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			splitRes, ok := reply.(*browserpb.SplitResponse)
			require.True(t, ok)
			splitRes.WindowId = outWindowID
			return nil
		}).
		Times(1)
}

func expectFocus(
	t *testing.T, mockCC *proto.MockClientConnInterface,
	handlerID, windowID uint64,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/browser.WindowManager/Focus"),
			gomock.Eq(&browserpb.FocusRequest{}),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			focusRes, ok := reply.(*browserpb.FocusResponse)
			require.True(t, ok)
			focusRes.WindowId = windowID
			return nil
		}).
		Times(1)
}

func expectWindowClose(
	t *testing.T, mockWinConn *proto.MockMuxConn, quitCh chan struct{},
) {
	mockWinConn.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/browser.Window/Close"),
			gomock.Eq(&browserpb.WindowCloseRequest{}),
			gomock.Eq(&browserpb.WindowCloseResponse{})).
		Times(1)
	mockWinConn.EXPECT().Close().Times(1).
		DoAndReturn(prototest.ExpectSignalExit(mockWinConn, quitCh, nil))
}

func TestClientSetMessage(t *testing.T) {
	t.Run("invokes the pbclient rpc", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		myMsg, arg1, arg2 := "oh la la: %s %d", "obla di obla da", 5
		in := &browserpb.SetMessageRequest{Msg: fmt.Sprintf(myMsg, arg1, arg2)}
		out := new(browserpb.SetMessageResponse)

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/browser.Messenger/SetMessage"),
				gomock.Eq(in), gomock.Eq(out)).
			Times(1)

		err := client.SetMessage(myMsg, arg1, arg2)
		require.NoError(t, err)
	})
	t.Run("bubbles up rpc error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		expectInvokeError(mockCC)

		err := client.SetMessage("")
		assertInvokeError(t, err)
	})
}

func expectResourceOpen(mockCC *proto.MockClientConnInterface, myResource workspace.URI) {
	in := &browserpb.OpenResourceRequest{Resource: myResource.String()}
	out := new(browserpb.OpenResourceResponse)
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

		myResource, err := workspace.ParseURI("file:///fjkelwjfeklw")
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

		_, err := client.Open(workspace.URI{})
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
		{"Interrupt", (*Client).Interrupt, &termpb.Event{Type: termpb.Event_TypeInterrupt}},
		{"PublishEventNone", (*Client).PublishEventNone, &termpb.Event{Type: termpb.Event_TypeNone}},
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
				in := &browserpb.PublishRequest{Ev: ev}
				out := new(browserpb.PublishResponse)

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

func TestClientSplitHorizontalBelow(t *testing.T) {
	testClientSplit(t, OrientationBottom, browserpb.Orientation_Bottom)
}

func TestClientSplitVerticalRight(t *testing.T) {
	testClientSplit(t, OrientationRight, browserpb.Orientation_Right)
}

func TestClientSplitHorizontalAbove(t *testing.T) {
	testClientSplit(t, OrientationTop, browserpb.Orientation_Top)
}

func TestClientSplitDefault(t *testing.T) {
	testClientSplit(t, OrientationDefault, browserpb.Orientation_Default)
}

func TestClientSplitVerticalLeft(t *testing.T) {
	testClientSplit(t, OrientationLeft, browserpb.Orientation_Left)
}

func assertClientClientsEqual(t *testing.T, expected int, c *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.Equal(t, expected, len(c.clients))
}

func assertClientServersEqual(t *testing.T, expected int, c *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.Equal(t, expected, len(c.servers))
}

func testClientSplit(
	t *testing.T,
	split Orientation,
	expectedSplit browserpb.Orientation,
) {
	t.Run("serves handler and dials to window", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		inWindowID := uint64(99)
		outWindowID := uint64(100)
		prototest.ExpectBrokerServe(t, 1, mockBroker)
		mockWinConn := prototest.ExpectBrokerDial(t, ctrl, mockBroker, uint32(outWindowID))
		expectSplit(t, mockCC, 1, inWindowID, outWindowID, expectedSplit)
		quitCh := prototest.ExpectMonitorConn(mockWinConn)

		win, err := client.Split(split, &windowClient{brokerID: inWindowID}, NewTestHandler())
		require.NoError(t, err)
		assertClientServersEqual(t, 1, client)
		assertClientClientsEqual(t, 1, client)

		// caches window clients
		expectFocus(t, mockCC, 1, outWindowID)
		_, err = client.Focus()
		require.NoError(t, err)
		assertClientServersEqual(t, 1, client)
		assertClientClientsEqual(t, 1, client)

		expectWindowClose(t, mockWinConn, quitCh)
		require.NoError(t, win.Close())
		time.Sleep(asyncResultsSleepDuration)

		assertClientClientsEqual(t, 0, client)
	})

	t.Run("bubbles up rpc error and so stops handler server", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		prototest.ExpectBrokerServe(t, 1, mockBroker)
		expectInvokeError(mockCC)
		windowID := uint64(22)
		focus := &windowClient{brokerID: windowID}
		win, err := client.Split(split, focus, NewTestHandler())
		assertInvokeError(t, err)
		assert.Nil(t, win)

		assertClientServersEqual(t, 0, client)
		assertClientClientsEqual(t, 0, client)
	})

	t.Run("bubbles up dial to window error and so stops handler server", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		inWindowID := uint64(99)
		outWindowID := uint64(99)
		prototest.ExpectBrokerServe(t, 1, mockBroker)
		expectSplit(t, mockCC, 1, inWindowID, outWindowID, expectedSplit)
		prototest.ExpectBrokerDialError(t, ctrl, mockBroker, uint32(outWindowID))
		focus := &windowClient{brokerID: inWindowID}

		win, err := client.Split(split, focus, NewTestHandler())
		require.Error(t, err)
		assert.Nil(t, win)

		assertClientServersEqual(t, 0, client)
		assertClientClientsEqual(t, 0, client)
	})

	t.Run("gracefully closes resources if handler server is called Close", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		outWindowID := uint64(63)
		inWindowID := uint64(62)
		brokerID := uint64(1)
		prototest.ExpectBrokerServe(t, uint32(brokerID), mockBroker)
		expectSplit(t, mockCC, brokerID, inWindowID, outWindowID, expectedSplit)
		mockWinConn := prototest.ExpectBrokerDial(t, ctrl, mockBroker, uint32(outWindowID))
		quitCh := prototest.ExpectMonitorConn(mockWinConn)

		h := NewTestHandler()
		var wg sync.WaitGroup
		h.CloseCallback = func() error {
			wg.Done()
			return nil
		}
		focus := &windowClient{brokerID: inWindowID}
		win, err := client.Split(split, focus, h)
		require.NoError(t, err)

		wg.Add(1)
		mockWinConn.EXPECT().
			Invoke(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
		mockWinConn.EXPECT().Close().Times(1).
			DoAndReturn(func() error {
				h := client.servers[uint64(brokerID)].(*handlerServerResource).h.(Handler)
				err := prototest.ExpectSignalExit(mockWinConn, quitCh, nil)()
				// server would call this asynchronously
				go h.Close()
				return err
			})
		require.NoError(t, win.Close())

		wg.Wait()
		// monitor goroutine might not have called WaitForStateChange yet
		// because it's run asynchronously
		time.Sleep(asyncResultsSleepDuration)
	})

	assertNoLeaks(t)
}

func TestClientClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client, mockCC, mockBroker := newMockedClient(ctrl)

	for i := 0; i < 10; i++ {
		inWindowID := uint64(i + 1000)
		outWindowID := uint64(i)
		handlerID := uint64(i)
		prototest.ExpectBrokerServe(t, uint32(handlerID), mockBroker)
		expectSplit(t, mockCC, handlerID, inWindowID, outWindowID, browserpb.Orientation_Right)
		mockWinConn := prototest.ExpectBrokerDial(t, ctrl, mockBroker, uint32(outWindowID))
		quitCh := prototest.ExpectMonitorConn(mockWinConn)

		focus := &windowClient{brokerID: inWindowID}
		_, err := client.Split(OrientationRight, focus, NewTestHandler())
		require.NoError(t, err)
		assertClientServersEqual(t, i+1, client)
		assertClientClientsEqual(t, i+1, client)

		if i%2 == 0 {
			mockWinConn.EXPECT().Close().Times(1).
				DoAndReturn(prototest.ExpectSignalExit(mockWinConn, quitCh, nil))
		} else {
			mockWinConn.EXPECT().Close().Times(1).
				DoAndReturn(prototest.ExpectSignalExit(mockWinConn, quitCh, errors.New("let's see")))
		}
	}

	err := client.Close()
	require.Error(t, err)
	assert.Contains(t, "let's see", err.Error())
	assertNoLeaks(t)
}

/* tested via ex integration tests */
func TestClientSetContent(t *testing.T) {
}
func TestClientFocus(t *testing.T) {
}
