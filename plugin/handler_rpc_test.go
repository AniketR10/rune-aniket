package plugin

import (
	"net"
	"net/rpc"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRPCHandler(t *testing.T) (tui.Handler, func()) {
	l, err := net.Listen("tcp", ":2931")
	require.NoError(t, err)

	server := rpc.NewServer()
	testHandler := handler.NewTestHandler()
	testHandler.Manual = tui.Manual{Summary: "remote SUPER plugin"}
	err = server.RegisterName("Plugin", &RPCHandlerServer{Handler: testHandler})
	require.NoError(t, err)

	conn, err := net.DialTimeout("tcp", l.Addr().String(), 4*time.Second)
	require.NoError(t, err)

	client := rpc.NewClient(conn)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			server.ServeConn(conn)
		}
	}()

	return &RPCHandlerClient{Client: client}, func() { l.Close() }
}

func TestRPCHandlerHandleDraw(t *testing.T) {
	rpcHandler, cleanup := newRPCHandler(t)
	defer cleanup()

	cases := []handler.TestInputSequence{
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

	handler.BatchTestInputSequence(t, rpcHandler, 4, 4, cases)
}

func TestRPCHandlerManual(t *testing.T) {
	rpcHandler, cleanup := newRPCHandler(t)
	defer cleanup()
	assert.Equal(t, tui.Manual{Summary: "remote SUPER plugin"}, rpcHandler.Man())
}

func TestRPCHandlerCursor(t *testing.T) {
	rpcHandler, cleanup := newRPCHandler(t)
	defer cleanup()

	pos, ok := rpcHandler.Cursor()
	require.False(t, ok)

	assert.Equal(t, term.Coordinates{X: -1, Y: -1}, pos)
}
