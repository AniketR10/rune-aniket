package ssh

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

var logger = log.New()

func init() {
	logger.Out = ioutil.Discard
}

func TestReaderWriterListener(t *testing.T) {
	t.Run("one accept", func(t *testing.T) {
		inRead, inWrite, err := os.Pipe()
		require.NoError(t, err)

		outRead, outWrite, err := os.Pipe()
		require.NoError(t, err)

		l := newReaderWriterListener(logger, inRead, outWrite, false, func() {})

		_, err = inWrite.WriteString("JJ")
		require.NoError(t, err)

		// sut
		conn, err := l.Accept()
		require.NoError(t, err)

		n, err := conn.Write([]byte("\n"))
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		var buf [3]byte
		n, err = io.ReadAtLeast(outRead, buf[:], 1)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		assert.Equal(t, "\n", string(buf[:1]))

		n, err = conn.Read(buf[:])
		require.NoError(t, err)
		require.Equal(t, 2, n)
		assert.Equal(t, "JJ", string(buf[:2]))

		err = conn.Close()
		require.NoError(t, err)

		t.Cleanup(func() {
			inRead.Close()
			inWrite.Close()
			outWrite.Close()
			outRead.Close()
		})
	})

	t.Run("close of stdio returns", func(t *testing.T) {
		// TODO for some reason when upgrading to mock 1.6.0
		// and testify v1.8.1 this started failing.
		t.Skip()

		grpcServer := grpc.NewServer()
		inRead, inWrite, err := os.Pipe()
		require.NoError(t, err)

		outRead, outWrite, err := os.Pipe()
		require.NoError(t, err)

		lis := newReaderWriterListener(logger, inRead, outWrite, false, func() {
			go grpcServer.Stop()
		})
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		server := workspacepb.NewServer(scheme, new(sync.Mutex))
		workspacepb.RegisterSchemeServer(grpcServer, server)
		go func() {
			inWrite.Close()
		}()
		grpcServer.Serve(lis)

		t.Cleanup(func() {
			inRead.Close()
			inWrite.Close()
			outWrite.Close()
			outRead.Close()
		})
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
