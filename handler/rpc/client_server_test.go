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
package rpc

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

const testHandlerManualDesc = "remote SUPER extension"
const drawDispatchWaitTime = 20 * time.Millisecond

var testHandlerKeys tui.KeyMap

func init() {
	testHandlerKeys = make(tui.KeyMap)
	ev1 := term.KeyComb{Mod: term.ModCtrl, Key: term.KeySpace}
	ev2 := term.KeyComb{Mod: term.ModAlt, Ch: '@'}
	testHandlerKeys[ev1] = tui.EventDesc{ID: "sup", Description: "media soup"}
	testHandlerKeys[ev2] = tui.EventDesc{ID: "hiperio", Description: "is dead; or is it?"}
}

func testHandler() tui.Handler {
	ret := handler.NewTestHandler()
	ret.Manual.Summary = testHandlerManualDesc
	ret.Manual.Keys = testHandlerKeys
	return ret
}

func newStubClient(t *testing.T) *Client {
	return NewClient(&mockHandlerClient{
		remote: testHandler(),
	})
}

func assertTestManual(t *testing.T, man tui.Manual, msg ...interface{}) {
	expected := tui.Manual{
		Summary: testHandlerManualDesc,
		Keys:    testHandlerKeys,
	}
	assert.Equal(t, expected, man)
}

func testRPCHandlerManual(t *testing.T, rpcHandler interface {
	tui.Handler
	Errors() <-chan error
}) {
	assertTestManual(t, rpcHandler.Man(), rpcHandler.Errors())
}

func testRPCHandlerCursor(t *testing.T, rpcHandler tui.Handler) {
	// force call to underlying Cursor on the server side
	rpcHandler.Draw(term.NewStringWriter(0, 0))

	// give some time for the client to asynchronously receive it
	time.Sleep(drawDispatchWaitTime)

	// then force collect cursor response
	rpcHandler.Draw(term.NewStringWriter(0, 0))

	pos, _, ok := rpcHandler.Cursor()
	require.False(t, ok)

	assert.Equal(t, term.Coordinates{X: -1, Y: -1}, pos)
}

type testResizeHandler struct {
	handler.TestHandler
	width, height int
}

func (h *testResizeHandler) Resize(width, height int) {
	if h.width == width && h.height == height {
		panic("called redundant Resized")
	}
	h.width = width
	h.height = height
}

func TestClientHandlerDraw(t *testing.T) {
	t.Run("draw", func(t *testing.T) {
		stubClient := newStubClient(t)
		defer stubClient.Close()
		cases := []testutil.HandlerSequenceTestCase{
			{"",
				`AAAA
AAAA
AAAA
AAAA`}, {"j",
				`BBBB
BBBB
BBBB
BBBB`},
		}

		testutil.TestHandlerSequence(t, stubClient, 4, 4, cases)
	})

	t.Run("draw multi codepoint utf-8", func(t *testing.T) {
		stubClient := NewClient(
			&mockHandlerClient{
				remote: handler.Nop(component.NewStringWithConfig(`┏━━━━━┓
┃  中 ┃
┃     ┃
┗━━━━━┛`, component.StringConfig{Alignment: component.SpanAlignmentCentered})),
			},
		)
		cases := []testutil.HandlerSequenceTestCase{
			{"",
				`┏━━━━━┓
┃  中  ┃
┃     ┃
┗━━━━━┛`},
		}

		testutil.TestHandlerSequence(t, stubClient, 7, 4, cases)
	})

	t.Run("calls resize only when dimensions have changed", func(t *testing.T) {
		h := new(testResizeHandler)
		client, cleanup := newServerClient(t, h)
		defer client.Close()
		defer cleanup()

		client.Resize(10, 6)
		client.Handle(term.Event{Ch: 'h'})
		client.Handle(term.Event{Ch: 'j'})

		client.Resize(10, 6)
		client.Handle(term.Event{Ch: 'k'})
	})
}

func TestUnitClientHandlerManual(t *testing.T) {
	stubClient := newStubClient(t)
	defer stubClient.Close()
	testRPCHandlerManual(t, stubClient)
}

func TestUnitClientHandlerCursor(t *testing.T) {
	stubClient := newStubClient(t)
	defer stubClient.Close()
	testRPCHandlerCursor(t, stubClient)
}

func TestClientHandleErrors(t *testing.T) {
	myErr := errors.New("functional programming is overrated")
	stubClient := NewClient(&mockHandlerClient{
		remote:   testHandler(),
		rpcError: myErr,
	})
	defer stubClient.Close()
	errChan := stubClient.Errors()

	go func() {
		_ = stubClient.Man()
	}()

	assert.True(t, errors.Is(<-errChan, myErr))
}

func newServerClient(t *testing.T, testHandler tui.Handler) (*Client, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	RegisterHandlerServer(grpcServer, NewServer(testHandler))

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	return NewClient(NewHandlerClient(conn)), func() {
		conn.Close()
		grpcServer.Stop()
	}
}

func TestIntegrationClientHandlerDraw(t *testing.T) {
	serverClient, closeFn := newServerClient(t, testHandler())
	defer closeFn()
	defer serverClient.Close()

	cases := []testutil.HandlerSequenceTestCase{
		{"",
			`AAAA
AAAA
AAAA
AAAA`},
		{"j",
			`BBBB
BBBB
BBBB
BBBB`},
		{"",
			`BBBB
BBBB
BBBB
BBBB`},
	}

	width, height := 4, 4
	writer := term.NewStringWriter(width, height)
	serverClient.Resize(width, height)

	for _, tcase := range cases {
		err := writer.Clear(term.Attributes{Fg: 0, Bg: 0})
		require.NoError(t, err)

		for _, r := range tcase.InputSequence {
			serverClient.Handle(term.Event{Ch: r, Type: term.EventKey})
			time.Sleep(drawDispatchWaitTime)
		}

		serverClient.Draw(writer)
		time.Sleep(drawDispatchWaitTime)

		err = writer.Flush()
		require.NoError(t, err)

		out := writer.String()
		assert.Equal(t, tcase.Expected, out, serverClient.errors)
	}
}

func TestIntegrationClientHandlerManual(t *testing.T) {
	serverClient, closeFn := newServerClient(t, testHandler())
	defer closeFn()
	defer serverClient.Close()
	testRPCHandlerManual(t, serverClient)
}

func TestIntegrationClientHandlerCursor(t *testing.T) {
	serverClient, closeFn := newServerClient(t, testHandler())
	defer closeFn()
	defer serverClient.Close()
	testRPCHandlerCursor(t, serverClient)
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
