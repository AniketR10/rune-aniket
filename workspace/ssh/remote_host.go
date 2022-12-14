package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

// readerWriterListener satisfies net.Listener by accepting
// one connection at a time over a io.Reader/io.Writer.
type readerWriterListener struct {
	mu     sync.Mutex
	closed bool
	writer io.Writer
	reader io.Reader
	onEOF  func()

	onlyConn *stdConn
}

func newReaderWriterListener(
	reader io.Reader, writer io.Writer, onEOF func(),
) *readerWriterListener {
	ret := new(readerWriterListener)
	ret.reader = reader
	ret.writer = writer
	ret.onEOF = onEOF
	return ret
}

func (lis *readerWriterListener) Accept() (net.Conn, error) {
	lis.mu.Lock()
	defer lis.mu.Unlock()

	if lis.closed {
		return nil, errors.New("closed listener")
	}

	lis.onlyConn = newStdConn(lis.reader, lis.writer, func() {
		lis.Close()
		lis.onEOF()
	})
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

// StartSchemeServer installs server to handle incoming workspacepb requests
// over the calling process' os.Stdin and sends responses over os.Stdout.
func StartSchemeServer(server workspacepb.SchemeServer) error {
	grpcServer := grpc.NewServer()
	lis := newReaderWriterListener(os.Stdin, os.Stdout, func() {
		// this is called within a grpc goroutine so running
		// in a separate goroutine avoids deadlock
		go grpcServer.Stop()
	})
	workspacepb.RegisterSchemeServer(grpcServer, server)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("Server: %s", err)
	}
	return nil
}
