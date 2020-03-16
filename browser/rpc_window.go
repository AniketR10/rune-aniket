package browser

import (
	"context"
	"io"
	"sync"

	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClient struct {
	logger     *log.Logger
	pbClient   proto.WindowClient
	connCloser io.Closer
}

func newWindowClient(
	pbClient proto.WindowClient, connCloser io.Closer,
) *windowClient {
	ret := new(windowClient)
	ret.pbClient = pbClient
	ret.connCloser = connCloser
	return ret
}

func (w *windowClient) Close() error {
	req := new(proto.WindowCloseRequest)
	_, err := w.pbClient.Close(context.Background(), req)
	err2 := w.connCloser.Close()
	if err2 != nil {
		return err2
	}
	return err
}

// satisfies proto.WindowServer
type windowServer struct {
	lock sync.Locker
	win  Window
}

func newWindowServer(win Window, lock sync.Locker) *windowServer {
	ret := new(windowServer)
	ret.lock = lock
	ret.win = win
	return ret
}

func (s *windowServer) Close(
	ctx context.Context, req *proto.WindowCloseRequest,
) (*proto.WindowCloseResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	err := s.win.Close()
	if err != nil {
		return nil, err
	}
	return new(proto.WindowCloseResponse), nil
}
