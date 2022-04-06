package workspace

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	workspacepb "github.com/ernestrc/go-tui/workspace/proto"
	"google.golang.org/grpc"
)

// readerWriterListener satisfies net.Listener by accepting
// one connection at a time over a io.Reader/io.Writer.
type readerWriterListener struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
	writer io.Writer
	reader io.Reader

	onlyConn *stdConn
}

func newReaderWriterListener(
	reader io.Reader, writer io.Writer,
) *readerWriterListener {
	ret := new(readerWriterListener)
	ret.reader = reader
	ret.writer = writer
	return ret
}

func (lis *readerWriterListener) Accept() (net.Conn, error) {
	lis.mu.Lock()

	if lis.closed {
		lis.mu.Unlock()
		return nil, errors.New("closed listener")
	}

	// wait for previous conn, if any
	lis.mu.Unlock()
	lis.wg.Wait()
	lis.mu.Lock()
	defer lis.mu.Unlock()

	lis.onlyConn = newStdConn(lis.reader, lis.writer, lis.wg.Done)
	lis.wg.Add(1)
	return lis.onlyConn, nil
}

func (lis *readerWriterListener) Close() error {
	lis.mu.Lock()
	defer lis.mu.Unlock()

	lis.closed = true
	return nil
}

func (lis *readerWriterListener) Addr() net.Addr {
	return newStdinAddr("listener")
}

// StartWorkspaceServer installs server to handle incoming workspacepb requests
// over the calling process' os.Stdin and sends responses over os.Stdout.
func StartWorkspaceServer(server *Server) error {
	lis := newReaderWriterListener(os.Stdin, os.Stdout)
	grpcServer := grpc.NewServer()
	workspacepb.RegisterWorkspaceServer(grpcServer, server)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("Server: %s", err)
	}
	return nil
}
