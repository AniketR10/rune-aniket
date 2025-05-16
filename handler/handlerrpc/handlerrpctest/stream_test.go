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

package handlerrpctest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"unstable.build/go-tui"
	browserapitest "unstable.build/go-tui/api/browserapi/browsertest"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/handler/handlerrpc"
	"unstable.build/go-tui/term"
)

func TestClientServerStreamIntegration(t *testing.T) {
	t.Run("dimensions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedWidth, expectedHeight := 11, 19
		mock.EXPECT().Dimensions().Return(expectedWidth, expectedHeight)
		actualWidth, actualHeight := client.Dimensions()

		assert.Equal(t, expectedHeight, actualHeight)
		assert.Equal(t, expectedWidth, actualWidth)

	})

	t.Run("selection returns selection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedSelection := "1234"
		mock.EXPECT().Selection().Return(expectedSelection, true)
		actualSelection, actualOk := client.Selection()

		require.True(t, actualOk)
		assert.Equal(t, expectedSelection, actualSelection)

	})

	t.Run("selection returns nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		var expectedSelection string
		mock.EXPECT().Selection().Return(expectedSelection, false)
		actualSelection, actualOk := client.Selection()

		assert.False(t, actualOk)
		assert.Equal(t, expectedSelection, actualSelection)
	})

	t.Run("resize", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		mock.EXPECT().Resize(gomock.Eq(29), gomock.Eq(38))
		client.Resize(29, 38)
	})

	t.Run("draw", func(t *testing.T) {
		client, closeFn := setupIntTest(t, browsertest.NewTestFloating(20, 10))
		defer closeFn()

		client.Resize(20, 10)
		w := term.NewStringWriter(21, 11)

		tests := []comptest.TestCase{
			{Expected: `
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
AAAAAAAAAAAAAAAAAAAA 
                     `,
			},
		}
		comptest.TestComponent(t, client, w, tests)
	})

	t.Run("handle returns true, true", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedHandled, expectedQuit := true, true
		expectedEv := term.Event{Type: term.EventKey, Ch: 'a', Raw: []byte("a")}
		mock.EXPECT().Handle(gomock.Any()).DoAndReturn(func(actualEv term.Event) (bool, bool) {
			assert.Equal(t, expectedEv, actualEv)
			return expectedHandled, expectedQuit
		})
		actualHandled, actualQuit := client.Handle(expectedEv)

		assert.Equal(t, expectedHandled, actualHandled)
		assert.Equal(t, expectedQuit, actualQuit)
	})

	t.Run("handle returns false, false", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		var expectedHandled, expectedQuit bool
		expectedEv := term.Event{Type: term.EventMouse, MouseX: 99, MouseY: 129999}
		mock.EXPECT().Handle(gomock.Any()).DoAndReturn(func(actualEv term.Event) (bool, bool) {
			assert.Equal(t, expectedEv, actualEv)
			return expectedHandled, expectedQuit
		})
		actualHandled, actualQuit := client.Handle(expectedEv)

		assert.Equal(t, expectedHandled, actualHandled)
		assert.Equal(t, expectedQuit, actualQuit)
	})

	t.Run("cursor returns nothing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedCoordinates := term.Coordinates{}
		var expectedStyle term.CursorStyle
		expectedCursor := false
		mock.EXPECT().Cursor().Return(expectedCoordinates, expectedStyle, expectedCursor)
		actualCoordinates, actualStyle, actualCursor := client.Cursor()

		assert.Equal(t, expectedCoordinates, actualCoordinates)
		assert.Equal(t, expectedStyle, actualStyle)
		assert.Equal(t, expectedCursor, actualCursor)
	})

	t.Run("cursor returns cursor, style", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedCoordinates := term.Coordinates{X: 99, Y: 11}
		expectedStyle := term.CursorStyleSteadyBlock
		expectedCursor := true
		mock.EXPECT().Cursor().Return(expectedCoordinates, expectedStyle, expectedCursor)
		actualCoordinates, actualStyle, actualCursor := client.Cursor()

		assert.Equal(t, expectedCoordinates, actualCoordinates)
		assert.Equal(t, expectedStyle, actualStyle)
		assert.Equal(t, expectedCursor, actualCursor)
	})

	t.Run("man", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mock := browserapitest.NewMockFloating(ctrl)

		client, closeFn := setupIntTest(t, mock)
		defer closeFn()

		expectedMan := tui.Manual{Summary: "blabla", Keys: tui.KeyMap{}}
		mock.EXPECT().Man().Return(expectedMan)
		actualMan := client.Man()

		assert.Equal(t, expectedMan, actualMan)
	})
}

type testServer struct {
	UnimplementedTestServiceServer
	windowID uint64
	client   *handlerrpc.ClientStream[*TestMessage]
}

func (t *testServer) TestStream(srv TestService_TestStreamServer) error {
	if t.client != nil {
		return errors.New("cannot re-use test server")
	}
	t.client = handlerrpc.NewClientStream[*TestMessage](context.Background(), srv,
		func() *TestMessage {
			return new(TestMessage)

		})
	msg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("receive initial request: %w", err)
	}
	req := msg.GetRequest()
	if msg.GetType() != handlerrpc.MessageType_Request || req == nil {
		return errors.New("receive initial request: missing request")
	}

	resp := handlerrpc.InstallResourceResponse{WindowId: uint64(t.windowID)}
	respMsg := handlerrpc.ServerMessage{Response: &resp}
	if err := srv.SendMsg(&respMsg); err != nil {
		return fmt.Errorf("send install response: %w", err)
	}

	return t.client.ReceiveMessages()
}

func setupIntTest(t *testing.T, mock handlerrpc.Handler) (
	*handlerrpc.ClientStream[*TestMessage], func(),
) {
	const windowID = 99
	ts := &testServer{windowID: windowID}

	conn, closeFn := doSetupIntTest(t, func(grpcServer *grpc.Server) {
		RegisterTestServiceServer(grpcServer, ts)
	})

	serverStream, err := NewTestServiceClient(conn).TestStream(context.Background())
	require.NoError(t, err)

	server := handlerrpc.NewServerStream[*TestMessage](serverStream,
		mock, func() *TestMessage { return new(TestMessage) })

	req := TestRequest{}
	sendMsg := TestMessage{
		Type:    handlerrpc.MessageType_Request,
		Request: &req,
	}

	err = serverStream.SendMsg(&sendMsg)
	require.NoError(t, err)

	var recvMsg handlerrpc.ServerMessage
	err = serverStream.RecvMsg(&recvMsg)
	require.NoError(t, err)

	require.NotNil(t, recvMsg.GetResponse())

	go server.ReceiveMessages()
	return ts.client, func() {
		closeFn()
	}
}

func doSetupIntTest(t *testing.T, register func(*grpc.Server)) (
	conn *grpc.ClientConn, closeFn func(),
) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	register(grpcServer)

	go grpcServer.Serve(lis)

	conn, err = grpc.Dial(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	closeFn = func() {
		grpcServer.Stop()
		lis.Close()
	}
	return
}
