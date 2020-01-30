package editor

import (
	"net"
	"net/rpc"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newRPCClipboard(t *testing.T) (Clipboard, func()) {
	l, err := net.Listen("tcp", ":1234")
	require.NoError(t, err)

	server := rpc.NewServer()
	server.RegisterName("Plugin", &RPCClipboardServer{Clipboard: NewEphemeralClipboard()})

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

	return &RPCClipboardClient{Client: client}, func() { l.Close() }
}

func TestRPCClipboard(t *testing.T) {
	c, closeFn := newRPCClipboard(t)
	defer closeFn()

	testClipboard(t, c)
}
