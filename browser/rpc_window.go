package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

// WindowClient satisfies Window by talking to a
// remote window over GRPC.
type windowClient struct {
	brokerID      uint32
	logger        *log.Logger
	pbClient      proto.WindowClient
	browserClient *Client
}

func newWindowClient(
	brokerID uint32, browserClient *Client,
	pbClient proto.WindowClient,
) *windowClient {
	ret := new(windowClient)
	ret.browserClient = browserClient
	ret.pbClient = pbClient
	ret.brokerID = brokerID
	return ret
}

func (w *windowClient) SetContent(h Handler) error {
	ctx := context.Background()

	brokerID := w.browserClient.serveHandler(h)

	req := proto.WindowSetContentRequest{HandlerId: brokerID}
	_, err := w.pbClient.SetContent(ctx, &req)
	if err != nil {
		w.browserClient.forceCloseHandler(brokerID)
		return fmt.Errorf("error on pbClient.SetContent: %v", err)
	}
	return nil
}

func (w *windowClient) Close() (err error) {
	ctx := context.Background()
	req := proto.WindowCloseRequest{}
	_, _ = w.pbClient.Close(ctx, &req)

	// now we're ready to finally close window client connection and remove
	connErr := w.browserClient.forceCloseWindow(w.brokerID)
	if connErr != nil {
		err = connErr
	}
	return
}

// satisfies proto.WindowServer
type windowServer struct {
	mu       sync.Mutex
	brokerID uint32
	win      Window
	s        *Server
}

func newWindowServer(
	s *Server, brokerID uint32, win Window,
) *windowServer {
	ret := new(windowServer)
	ret.win = win
	ret.brokerID = brokerID
	ret.s = s
	return ret
}

func (s *windowServer) SetContent(
	ctx context.Context, req *proto.WindowSetContentRequest,
) (*proto.WindowSetContentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, err := s.s.dialHandler(req.GetHandlerId())
	if err != nil {
		return nil, fmt.Errorf("failed to dial to remote handler: %v", err)
	}

	err = s.win.SetContent(client)
	if err != nil {
		s.s.forceCloseHandler(req.GetHandlerId())
		return nil, fmt.Errorf("error on Window.SetContent: %v", err)
	}

	return new(proto.WindowSetContentResponse), nil
}

func (s *windowServer) Close(
	ctx context.Context, req *proto.WindowCloseRequest,
) (*proto.WindowCloseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.s.forceCloseWindow(s.brokerID)
	if err != nil {
		return nil, err
	}
	return new(proto.WindowCloseResponse), nil
}
