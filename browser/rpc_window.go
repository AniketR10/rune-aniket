package browser

import (
	"context"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

type gracefulCloser interface {
	forceClose(handlerID uint32) error
	waitForClientClose(ctx context.Context, handlerID uint32) bool
}

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClient struct {
	handlerID     uint32
	logger        *log.Logger
	pbClient      proto.WindowClient
	browserClient *Client
}

func newWindowClient(
	handlerID uint32,
	browserClient *Client,
	pbClient proto.WindowClient,
) *windowClient {
	ret := new(windowClient)
	ret.browserClient = browserClient
	ret.pbClient = pbClient
	ret.handlerID = handlerID
	return ret
}

func (w *windowClient) Close() (err error) {
	// shutsdown the handler server associated with this window
	cliErr := w.browserClient.closePhase1(w.handlerID)
	if cliErr != nil {
		err = cliErr
	}

	// tell server to wait close resources: waits for handler server connection
	// to change to shutdown mode, then shutsdown window grpc server
	// error is ignored because pbClient's server is shutdown preemptively
	// and even if error was legitimate, there's nothing else we could do from here
	req := new(proto.WindowCloseRequest)
	_, _ = w.pbClient.Close(context.Background(), req)

	// now we're ready to finally close window client connection and remove
	connErr := w.browserClient.closePhase2(w.handlerID)
	if connErr != nil {
		err = connErr
	}
	return
}

// satisfies proto.WindowServer
type windowServer struct {
	mu           sync.Mutex
	handlerID    uint32
	win          Window
	s            *Server
	shutdownWait time.Duration
}

func newWindowServer(
	s *Server, handlerID uint32, win Window,
) *windowServer {
	ret := new(windowServer)
	ret.win = win
	ret.handlerID = handlerID
	ret.s = s
	ret.shutdownWait = s.shutdownWait
	return ret
}

func gracefullyShutdown(
	closer gracefulCloser,
	shutdownWait time.Duration, handlerID uint32,
) (err error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, shutdownWait)
	defer cancel()

	closer.waitForClientClose(ctx, handlerID)
	err1 := closer.forceClose(handlerID)
	if err1 != nil {
		err = err1
	}

	return
}

func (s *windowServer) Close(
	ctx context.Context, req *proto.WindowCloseRequest,
) (*proto.WindowCloseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := gracefullyShutdown(s.s, s.shutdownWait, s.handlerID)
	if err != nil {
		return nil, err
	}
	return new(proto.WindowCloseResponse), nil
}
