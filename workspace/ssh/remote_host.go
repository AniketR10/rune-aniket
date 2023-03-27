package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	workspacepb "unstable.build/go-tui/workspace/rpc"
)

type stdioListener struct {
	writer   *os.File
	reader   *os.File
	acceptCh chan struct{}
	onlyConn net.Conn
	onClose  func()
	stdio    bool
	logger   *log.Logger
}

func newReaderWriterListener(
	logger *log.Logger,
	reader, writer *os.File,
	stdio bool,
	onClose func(),
) *stdioListener {
	ret := new(stdioListener)
	ret.reader = reader
	ret.logger = logger
	ret.writer = writer
	ret.stdio = stdio
	ret.acceptCh = make(chan struct{}, 1)
	ret.acceptCh <- struct{}{}
	ret.onClose = onClose
	return ret
}

func (lis *stdioListener) Accept() (net.Conn, error) {
	_, ok := <-lis.acceptCh
	if !ok {
		return nil, errors.New("closed listener")
	}

	conn, err := newStdConn(lis.logger, lis.reader, lis.writer,
		lis.stdio, lis.onClose)
	if err != nil {
		return nil, err
	}
	lis.onlyConn = conn

	return lis.onlyConn, nil
}

func (lis *stdioListener) Close() error {
	close(lis.acceptCh)
	return nil
}

func (lis *stdioListener) Addr() net.Addr {
	return lis.onlyConn.LocalAddr()
}

// StartSchemeServer installs server to handle incoming workspacepb requests
// over the calling process' os.Stdin and sends responses over os.Stdout.
func StartSchemeServer(
	logger *log.Logger, server *workspacepb.Server,
) error {
	grpcServer := grpc.NewServer()
	lis := newReaderWriterListener(
		logger, nil /*reader*/, nil, /*writer*/
		true, /* use stdio instead of reader and writer */
		func() {
			logger.Debugf("connection closed unexpectedly")
			go grpcServer.Stop()
		})
	workspacepb.RegisterSchemeServer(grpcServer, server)
	workspacepb.RegisterFilesServer(grpcServer, server)
	workspacepb.RegisterExecutorServer(grpcServer, server)
	workspacepb.RegisterTerminalServer(grpcServer, server)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("Server: %s", err)
	}
	return nil
}
