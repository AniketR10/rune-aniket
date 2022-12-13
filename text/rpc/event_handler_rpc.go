package rpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/proto"
)

const (
	defaultClientTimeout    = 4 * time.Second
	handleBackpressureThres = 128
)

type eventHandlerClient struct {
	conn         proto.MuxConn
	pb           EditorEventHandlerClient
	evChan       chan EditorEvent
	errChan      chan error
	quitChan     chan struct{}
	quitCallback func()
}

func newEventHandlerClient(
	cc proto.MuxConn, quitCallback func(),
) *eventHandlerClient {
	ret := new(eventHandlerClient)
	ret.pb = NewEditorEventHandlerClient(cc)
	ret.errChan = make(chan error)
	ret.quitChan = make(chan struct{})
	ret.evChan = make(chan EditorEvent, handleBackpressureThres)
	ret.quitCallback = quitCallback
	ret.conn = cc

	go pipelineEvents(ret.quitChan, ret.evChan, ret.errChan,
		ret.pb, &ret.conn, ret.quitCallback)

	runtime.SetFinalizer(ret, func(c *eventHandlerClient) {
		c.Close()
	})
	return ret
}

// make sure closeFn is not preventing client for being garbage collected
func closeFn(
	conn *proto.MuxConn, quitCallback func(),
	quitChan chan struct{}, evChan chan EditorEvent,
) {
	if *conn == nil {
		return
	}
	*conn = nil
	quitCallback()
	close(quitChan)
	close(evChan)
}

func (c *eventHandlerClient) errors() <-chan error {
	return c.errChan
}

func pipelineEvents(
	quitChan chan struct{}, evChan chan EditorEvent,
	errChan chan error,
	pb EditorEventHandlerClient,
	conn *proto.MuxConn, quitCallback func(),
) {
	var protoEv EditorEvent
	for {
		select {
		case <-quitChan:
			return
		case protoEv = <-evChan:
		}

		ctx := context.Background()
		ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
		req := EditorEventHandleRequest{Event: &protoEv}
		resp, err := pb.Handle(ctx, &req)
		cancelFn()
		if err != nil {
			log.Errorf("editor.eventHandlerClient.Handle error: %v", err)

			select {
			case errChan <- err:
			default:
			}
		} else if resp.GetQuit() {
			closeFn(conn, quitCallback, quitChan, evChan)
			return
		}
	}
}

func (c *eventHandlerClient) Handle(ctx context.Context, ev textapi.Event) bool {
	if c.conn == nil {
		// unsubscribe if already closed
		return true
	}

	protoEv := toProto(ev)
	c.evChan <- protoEv
	runtime.KeepAlive(c)

	return false
}

func (c *eventHandlerClient) Close() error {
	closeFn(&c.conn, c.quitCallback, c.quitChan, c.evChan)
	runtime.SetFinalizer(c, nil)
	return nil
}

type eventHandlerServer struct {
	UnimplementedEditorEventHandlerServer
	handler textapi.EventHandler
	onExit  func(context.Context)
}

func newEventHandlerServer(
	handler textapi.EventHandler, onExit func(context.Context),
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

	quit := s.handler.Handle(ctx, ev)
	resp := &EditorEventHandleResponse{Quit: quit}

	if quit {
		s.onExit(ctx)
	}
	return resp, nil
}
