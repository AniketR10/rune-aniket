package rpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	textapi "unstable.build/go-tui/api/text"
)

const (
	defaultClientTimeout    = 4 * time.Second
	handleBackpressureThres = 128
)

type eventHandlerClient struct {
	conn         grpc.ClientConnInterface
	pb           EditorEventHandlerClient
	evChan       chan EditorEvent
	errChan      chan error
	quitChan     chan struct{}
	quitCallback func()
}

func newEventHandlerClient(
	cc grpc.ClientConnInterface, quitCallback func(),
) *eventHandlerClient {
	ret := new(eventHandlerClient)
	ret.pb = NewEditorEventHandlerClient(cc)
	ret.errChan = make(chan error)
	ret.quitChan = make(chan struct{})
	ret.evChan = make(chan EditorEvent, handleBackpressureThres)
	ret.quitCallback = quitCallback
	ret.conn = cc

	go ret.pipelineEvents()

	return ret
}

func (c *eventHandlerClient) errors() <-chan error {
	return c.errChan
}

func (c *eventHandlerClient) handleError(err error) {
	log.Errorf("editor.eventHandlerClient.Handle error: %v", err)

	select {
	case c.errChan <- err:
	default:
	}
}

func (c *eventHandlerClient) pipelineEvents() {
	var protoEv EditorEvent
	for {
		select {
		case <-c.quitChan:
			return
		case protoEv = <-c.evChan:
		}

		ctx := context.Background()
		ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
		req := EditorEventHandleRequest{Event: &protoEv}
		resp, err := c.pb.Handle(ctx, &req)
		cancelFn()
		if err != nil {
			c.handleError(err)
		} else if resp.GetQuit() {
			c.quitCallback()
			return
		}
	}
}

func (c *eventHandlerClient) Handle(ctx context.Context, ev textapi.Event) bool {
	protoEv := toProto(ev)
	c.evChan <- protoEv

	return false
}

func (c *eventHandlerClient) Close() error {
	if c.conn == nil {
		return nil
	}
	c.conn = nil
	close(c.quitChan)
	// if Handle is called after close, then we want to panic
	// to indicate programmer error.
	close(c.evChan)
	return nil
}

type eventHandlerServer struct {
	UnimplementedEditorEventHandlerServer
	handler textapi.EventHandler
	onExit  func()
}

func newEventHandlerServer(
	handler textapi.EventHandler, onExit func(),
) *eventHandlerServer {
	ret := new(eventHandlerServer)
	ret.handler = handler
	ret.onExit = onExit
	return ret
}

func (s *eventHandlerServer) Handle(
	ctx context.Context, req *EditorEventHandleRequest,
) (*EditorEventHandleResponse, error) {
	protoEv := req.GetEvent()
	if protoEv == nil {
		return nil, errors.New("invalid handle request: missing event property")
	}

	var ev textapi.Event
	err := fromProto(&ev, protoEv)
	if err != nil {
		err = fmt.Errorf("failed to decode proto event: %s", err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	quit := s.handler.Handle(ctx, ev)

	resp := &EditorEventHandleResponse{Quit: quit}

	if quit {
		s.onExit()
	}
	return resp, nil
}
