package ssh

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

func TestReaderWriterListener(t *testing.T) {
	t.Run("one accept only", func(t *testing.T) {
		var in, out bytes.Buffer
		l := newReaderWriterListener(&in, &out, func() {})

		_, err := in.WriteString("JJ")
		require.NoError(t, err)

		// sut
		conn, err := l.Accept()
		require.NoError(t, err)

		n, err := conn.Write([]byte("\n"))
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, "\n", out.String())

		var buf [3]byte
		n, err = conn.Read(buf[:])
		require.NoError(t, err)
		require.Equal(t, 2, n)
		assert.Equal(t, "JJ", string(buf[:2]))

		err = conn.Close()
		require.NoError(t, err)

		out.Reset()
		in.Reset()

		conn, err = l.Accept()
		require.Error(t, err)
	})

	t.Run("close of stdio returns", func(t *testing.T) {
		in := closer{}
		grpcServer := grpc.NewServer()
		lis := newReaderWriterListener(&in, &closer{}, func() {
			go grpcServer.Stop()
		})
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(config.NopConfig(), uri)
		require.NoError(t, err)
		server := workspacepb.NewSchemeServer(scheme, new(sync.Mutex))
		workspacepb.RegisterSchemeServer(grpcServer, server)
		go func() {
			in.Close()
		}()
		grpcServer.Serve(lis)
	})
}

type closer struct {
	mu     sync.Mutex
	closed bool
}

func (c *closer) Read(p []byte) (n int, err error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return 0, io.EOF
	}
	// timeout tests
	time.Sleep(300 * time.Minute)
	return 0, io.EOF
}

func (c *closer) Write(p []byte) (n int, err error) {
	return 0, errors.New("nope")
}

func (c *closer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
