package browser

import (
	"context"
	"fmt"

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
		reason := fmt.Sprintf("error on call to SetContent: %v", err)
		w.browserClient.safeForceCloseHandler(brokerID, reason)
		return fmt.Errorf("error on pbClient.SetContent: %v", err)
	}
	return nil
}

func (w *windowClient) Close() (err error) {
	ctx := context.Background()
	req := proto.WindowCloseRequest{}
	_, _ = w.pbClient.Close(ctx, &req)

	// now we're ready to finally close window client connection and remove
	reason := "windowClient.Close"
	connErr := w.browserClient.safeForceCloseWindow(w.brokerID, reason)
	if connErr != nil {
		err = connErr
	}
	return
}

// satisfies proto.WindowServer
type windowServer struct {
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

	client, err := s.s.dialHandler(req.GetHandlerId())
	if err != nil {
		return nil, fmt.Errorf("failed to dial to remote handler: %v", err)
	}

	s.s.browser.Lock()
	err = s.win.SetContent(client)
	s.s.browser.Unlock()
	if err != nil {
		reason := fmt.Sprintf("error on windowServer.SetContent: %v", err)
		s.s.safeForceCloseHandler(req.GetHandlerId(), reason)
		return nil, fmt.Errorf("error on Window.SetContent: %v", err)
	}

	return new(proto.WindowSetContentResponse), nil
}

func (s *windowServer) Close(
	ctx context.Context, req *proto.WindowCloseRequest,
) (*proto.WindowCloseResponse, error) {
	err := s.s.safeForceCloseWindow(s.brokerID, "received WindowCloseRequest")
	if err != nil {
		return nil, err
	}
	return new(proto.WindowCloseResponse), nil
}
