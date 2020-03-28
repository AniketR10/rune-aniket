package browser

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
)

func newMockedClient(ctrl *gomock.Controller) (
	client *Client,
	mockCC *MockClientConnInterface,
	mockMux *proto.MockMuxBroker,
) {
	mockCC = NewMockClientConnInterface(ctrl)
	mockMux = proto.NewMockMuxBroker(ctrl)
	client = NewClient(mockMux, mockCC)
	return
}

func expectInvokeError(mockCC *MockClientConnInterface) {
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

func TestClientMergeKeyMap(t *testing.T) {
	key1 := term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}
	key2 := term.Event{Type: term.EventKey,
		Mod: term.ModAlt, Key: term.KeyBackspace}
	key3 := term.Event{Type: term.EventMouse, MouseX: 10, MouseY: 11111}
	protoKey1 := proto.Event{
		Type: proto.Event_TypeKey,
		Key:  proto.Event_Ctrl4,
	}
	protoKey2 := proto.Event{
		Type: proto.Event_TypeKey,
		Key:  proto.Event_CtrlH,
		Mod:  proto.Event_Alt,
	}
	protoKey3 := proto.Event{
		Type:   proto.Event_TypeMouse,
		MouseX: 10,
		MouseY: 11111,
	}

	fixture := make(map[term.Event]term.Event)
	fixture[key1] = key2
	fixture[key2] = key3
	fixture[key3] = key1

	t.Run("maps term mappings to a proto merge key map request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		out := new(proto.MergeKeyMapResponse)

		expectedReq := []*proto.Mapping{
			&proto.Mapping{From: &protoKey1, To: &protoKey2},
			&proto.Mapping{From: &protoKey2, To: &protoKey3},
			&proto.Mapping{From: &protoKey3, To: &protoKey1},
		}

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/proto.KeyMapper/MergeKeyMap"), gomock.Any(),
				gomock.Eq(out)).
			DoAndReturn(func(ctx context.Context,
				method string, args interface{},
				reply interface{}, opts ...grpc.CallOption) error {
				// validate mappings with finer grained control
				req, ok := args.(*proto.MergeKeyMapRequest)
				require.True(t, ok)

				assert.ElementsMatch(t, expectedReq, req.Mappings)
				return nil
			}).
			Times(1)

		err := client.MergeKeyMap(fixture)
		require.NoError(t, err)
	})

	t.Run("bubbles up grpc error to caller", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		expectInvokeError(mockCC)

		err := client.MergeKeyMap(fixture)
		assertInvokeError(t, err)
	})

	t.Run("sends request anyway if map is nil or empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)
		mockCC.EXPECT().
			Invoke(gomock.Any(), gomock.Any(),
				gomock.Any(), gomock.Any()).
			Times(2)

		{
			err := client.MergeKeyMap(make(map[term.Event]term.Event))
			assert.NoError(t, err)
		}

		{
			err := client.MergeKeyMap(nil)
			assert.NoError(t, err)
		}
	})
}

func TestClientSetMessage(t *testing.T) {
	t.Run("invokes the pbclient rpc", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		myMsg, arg1, arg2 := "oh la la: %s %d", "obla di obla da", 5
		in := &proto.SetMessageRequest{Msg: fmt.Sprintf(myMsg, arg1, arg2)}
		out := new(proto.SetMessageResponse)

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/proto.Messenger/SetMessage"),
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

func TestClientOpenFile(t *testing.T) {
	t.Run("invokes the pbclient rpc", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		myFile := "fjkelwjfeklw"
		in := &proto.OpenFileRequest{File: myFile}
		out := new(proto.OpenFileResponse)

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/proto.FileOpener/OpenFile"),
				gomock.Eq(in), gomock.Eq(out)).
			Times(1)

		err := client.OpenFile(myFile)
		require.NoError(t, err)
	})

	t.Run("bubbles up rpc error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, _ := newMockedClient(ctrl)

		expectInvokeError(mockCC)

		err := client.OpenFile("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "woopsie")
	})
}

func expectBrokerServe(t *testing.T, brokerID uint32, mockBroker *proto.MockMuxBroker) {
	mockBroker.EXPECT().NextId().Return(uint32(brokerID))
	mockBroker.EXPECT().AcceptAndServe(gomock.Any(), gomock.Any()).
		DoAndReturn(func(brokerId uint32, serverFunc func(opts []grpc.ServerOption) *grpc.Server) {
			assert.Equal(t, uint32(brokerID), brokerId)
			serverFunc(make([]grpc.ServerOption, 0))
		}).
		Times(1)
}

func expectBrokerDial(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *proto.MockMuxBroker, expectedBrokerID uint32,
) *proto.MockMuxConn {
	ret := proto.NewMockMuxConn(ctrl)

	mockBroker.EXPECT().Dial(gomock.Any()).
		DoAndReturn(func(brokerId uint32) (proto.MuxConn, error) {
			assert.Equal(t, expectedBrokerID, brokerId)
			return ret, nil
		}).
		Times(1)

	return ret
}

func expectBrokerDialError(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *proto.MockMuxBroker, expectedBrokerID uint32,
) {
	mockBroker.EXPECT().Dial(gomock.Any()).
		Return(nil, errors.New("whoopsie")).
		Times(1)
}

func expectSplit(
	t *testing.T, mockCC *MockClientConnInterface,
	handlerID, windowID uint32, rpc string,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq(rpc),
			gomock.Eq(&proto.SplitRequest{HandlerId: handlerID}),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			splitRes, ok := reply.(*proto.SplitResponse)
			require.True(t, ok)
			splitRes.WindowId = windowID
			return nil
		}).
		Times(1)
}

func expectWindowClose(t *testing.T, mockWinConn *proto.MockMuxConn) {
	mockWinConn.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/proto.Window/Close"),
			gomock.Eq(&proto.WindowCloseRequest{}),
			gomock.Eq(&proto.WindowCloseResponse{})).
		Times(1)
	mockWinConn.EXPECT().Close().Times(1).Return(nil)
}

func TestClientSubscribe(t *testing.T) {
	t.Run("bubbles up rpc error and so stops event handler resources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		expectBrokerServe(t, 1, mockBroker)
		expectInvokeError(mockCC)

		err := client.Subscribe(term.Event{}, nil)
		assertInvokeError(t, err)

		assert.Equal(t, 0, len(client.resources))
	})

	t.Run("sends event subscribe request to server", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		expectBrokerServe(t, 1, mockBroker)

		in := &proto.SubscribeRequest{Ev: &proto.Event{Char: 'a'}, HandlerId: 1}
		out := new(proto.SubscribeResponse)

		mockCC.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/proto.EventPublisher/Subscribe"),
				gomock.Eq(in), gomock.Eq(out)).
			Times(1)

		err := client.Subscribe(term.Event{Ch: 'a'}, nil)
		require.NoError(t, err)

		assert.Equal(t, 1, len(client.resources))
	})
}

func TestClientSplitHorizontalBelow(t *testing.T) {
	rpc := "/proto.WindowManager/SplitHorizontalBelow"
	testClientSplit(t, (WindowManager).SplitHorizontalBelow, rpc)
}

func TestClientSplitVerticalRight(t *testing.T) {
	rpc := "/proto.WindowManager/SplitVerticalRight"
	testClientSplit(t, (WindowManager).SplitVerticalRight, rpc)
}

func TestClientSplitHorizontalAbove(t *testing.T) {
	rpc := "/proto.WindowManager/SplitHorizontalAbove"
	testClientSplit(t, (WindowManager).SplitHorizontalAbove, rpc)
}

func TestClientSplitVerticalLeft(t *testing.T) {
	rpc := "/proto.WindowManager/SplitVerticalLeft"
	testClientSplit(t, (WindowManager).SplitVerticalLeft, rpc)
}

func testClientSplit(
	t *testing.T,
	split func(WindowManager, tui.Handler) (Window, error),
	rpc string,
) {
	t.Run("serves handler and dials to window", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		windowID := uint32(99)
		expectBrokerServe(t, 1, mockBroker)
		expectSplit(t, mockCC, 1, windowID, rpc)
		mockWinConn := expectBrokerDial(t, ctrl, mockBroker, windowID)

		win, err := split(client, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, len(client.resources))

		expectWindowClose(t, mockWinConn)
		require.NoError(t, win.Close())
		assert.Equal(t, 0, len(client.resources))
	})

	t.Run("bubbles up rpc error and so stops handler server", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		expectBrokerServe(t, 1, mockBroker)
		expectInvokeError(mockCC)

		win, err := split(client, nil)
		assertInvokeError(t, err)
		assert.Nil(t, win)

		assert.Equal(t, 0, len(client.resources))
	})

	t.Run("bubbles up dial to window error and so stops handler server", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		windowID := uint32(99)
		expectBrokerServe(t, 1, mockBroker)
		expectSplit(t, mockCC, 1, windowID, rpc)
		expectBrokerDialError(t, ctrl, mockBroker, windowID)

		win, err := split(client, nil)
		require.Error(t, err)
		assert.Nil(t, win)
		assert.Equal(t, 0, len(client.resources))
	})

	t.Run("gracefully closes resources if handler returns exit = true", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client, mockCC, mockBroker := newMockedClient(ctrl)

		windowID := uint32(63)
		expectBrokerServe(t, 1, mockBroker)
		expectSplit(t, mockCC, 1, windowID, rpc)
		mockWinConn := expectBrokerDial(t, ctrl, mockBroker, windowID)

		h := handler.NewTestHandler()
		_, err := split(client, h)
		require.NoError(t, err)
		require.Equal(t, 1, len(client.resources))

		h.Exit = true
		mockWinConn.EXPECT().Close().Times(1).Return(nil)

		cliRes, ok := client.getResources(1)
		require.True(t, ok)

		exit, handled := cliRes._h.Handle(term.Event{Type: term.EventNone})
		assert.True(t, exit)
		assert.True(t, handled)

		// unfortunately gracefulshutdowns are asynchronous
		// because they wait on grpc connection to be shutdown fist by
		// server
		time.Sleep(100 * time.Millisecond)

		client.mu.Lock()
		defer client.mu.Unlock()

		assert.Equal(t, 0, len(client.resources))
	})

	goleak.VerifyNone(t)
}

func TestClientClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client, mockCC, mockBroker := newMockedClient(ctrl)

	for i := 0; i < 10; i++ {
		windowID := uint32(i)
		expectBrokerServe(t, uint32(i), mockBroker)
		expectSplit(t, mockCC, uint32(i), windowID,
			"/proto.WindowManager/SplitVerticalRight")
		mockWinConn := expectBrokerDial(t, ctrl, mockBroker, windowID)

		_, err := client.SplitVerticalRight(nil)
		require.NoError(t, err)
		assert.Equal(t, i+1, len(client.resources))

		if i%2 == 0 {
			mockWinConn.EXPECT().Close().Times(1).Return(nil)
		} else {
			mockWinConn.EXPECT().Close().Times(1).Return(errors.New("let's see"))
		}
	}

	err := client.Close()
	require.Error(t, err)
	assert.Contains(t, "let's see", err.Error())
	goleak.VerifyNone(t)
}
