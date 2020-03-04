package handler

import (
	"errors"
	"net"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const testHandlerManualDesc = "remote SUPER plugin"

var testHandlerKeys tui.KeyMap

func init() {
	testHandlerKeys = make(tui.KeyMap)
	ev1 := term.Event{Type: term.EventKey, Key: term.KeyCtrlSpace}
	ev2 := term.Event{Mod: term.ModAlt, Type: term.EventKey, Ch: '@'}
	ev3 := term.Event{Type: term.EventMouse, MouseX: 0, MouseY: 1, Key: term.MouseLeft}
	testHandlerKeys[ev1] = tui.EventDesc{ID: "sup", Description: "media soup"}
	testHandlerKeys[ev2] = tui.EventDesc{ID: "hiperio", Description: "is dead; or is it?"}
	testHandlerKeys[ev3] = tui.EventDesc{ID: "catalonia", Description: "is not free"}
}

func testHandler() tui.Handler {
	ret := NewTestHandler()
	ret.Manual.Summary = testHandlerManualDesc
	ret.Manual.Keys = testHandlerKeys
	return ret
}

func newStubClient(t *testing.T) *Client {
	return &Client{
		client: &mockHandlerClient{
			remote: testHandler(),
		},
	}
}

func testRPCHandlerHandleDraw(t *testing.T, rpcHandler *Client) {

	cases := []TestInputSequence{
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

	BatchTestInputSequence(t, rpcHandler, 4, 4, cases)
}

func testRPCHandlerManual(t *testing.T, rpcHandler *Client) {
	expected := tui.Manual{
		Summary: testHandlerManualDesc,
		Keys:    testHandlerKeys,
	}
	assert.Equal(t, expected, rpcHandler.Man(), rpcHandler.errors)
}

func testRPCHandlerCursor(t *testing.T, rpcHandler *Client) {
	// force call to underlying Cursor
	rpcHandler.Draw(term.NewStringWriter(0, 0))

	pos, ok := rpcHandler.Cursor()
	require.False(t, ok)

	assert.Equal(t, term.Coordinates{X: -1, Y: -1}, pos)
}

func TestUnitClientHandlerDraw(t *testing.T) {
	stubClient := newStubClient(t)
	testRPCHandlerHandleDraw(t, stubClient)
}

func TestUnitClientHandlerManual(t *testing.T) {
	stubClient := newStubClient(t)
	testRPCHandlerManual(t, stubClient)
}

func TestUnitClientHandlerCursor(t *testing.T) {
	stubClient := newStubClient(t)
	testRPCHandlerCursor(t, stubClient)
}

func TestClientHandleErrors(t *testing.T) {
	myErr := errors.New("functional programming is overrated")
	stubClient := NewClient(&mockHandlerClient{
		remote:   testHandler(),
		manError: myErr,
	})
	errChan := stubClient.Errors()

	go func() {
		_ = stubClient.Man()
	}()

	assert.Equal(t, myErr, <-errChan)
}

func newServerClient(t *testing.T) (*Client, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	proto.RegisterHandlerServer(grpcServer, NewServer(testHandler()))

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	return NewClient(proto.NewHandlerClient(conn)), grpcServer.Stop
}

func TestIntegrationClientHandlerDraw(t *testing.T) {
	serverClient, closeFn := newServerClient(t)
	defer closeFn()
	testRPCHandlerHandleDraw(t, serverClient)
}

func TestIntegrationClientHandlerManual(t *testing.T) {
	serverClient, closeFn := newServerClient(t)
	defer closeFn()
	testRPCHandlerManual(t, serverClient)
}

func TestIntegrationClientHandlerCursor(t *testing.T) {
	serverClient, closeFn := newServerClient(t)
	defer closeFn()
	testRPCHandlerCursor(t, serverClient)
}
