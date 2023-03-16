package rpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/proto"
)

const (
	defaultClientTimeout    = 4 * time.Second
	handleBackpressureThres = 128
)

type eventHandlerClient struct {
	channelID string
	conn      proto.MuxConn
	pb        EditorEventHandlerClient
	evChan    chan EditorEvent
	errChan   chan error
	quitChan  chan struct{}
}

func newEventHandlerClient(
	parentCtx context.Context, channelID string, cc proto.MuxConn,
) *eventHandlerClient {
	ret := new(eventHandlerClient)
	ret.pb = NewEditorEventHandlerClient(cc)
	ret.quitChan = make(chan struct{})
	ret.evChan = make(chan EditorEvent, handleBackpressureThres)
	ret.conn = cc
	ret.channelID = channelID

	go pipelineEvents(parentCtx, channelID,
		ret.quitChan, ret.evChan, ret.pb, &ret.conn)

	runtime.SetFinalizer(ret, func(c *eventHandlerClient) {
		c.Close()
	})
	return ret
}

// make sure closeFn is not preventing client for being garbage collected
func closeFn(
	conn *proto.MuxConn,
	quitChan chan struct{}, evChan chan EditorEvent,
) {
	if *conn == nil {
		return
	}
	*conn = nil
	close(quitChan)
	close(evChan)
}

func (c *eventHandlerClient) errors() <-chan error {
	return c.errChan
}

func pipelineEvents(
	ctx context.Context, channelID string,
	quitChan chan struct{}, evChan chan EditorEvent,
	pb EditorEventHandlerClient,
	conn *proto.MuxConn,
) {
	var protoEv EditorEvent
	for {
		select {
		case <-quitChan:
			return
		case <-ctx.Done():
			return
		case protoEv = <-evChan:
		}

		ctx := context.Background()
		ctx, cancelFn := context.WithTimeout(ctx, defaultClientTimeout)
		req := EditorEventHandleRequest{Event: &protoEv}
		resp, err := pb.Handle(ctx, &req)
		cancelFn()
		if err != nil {
			log.WithFields(log.Fields{
				logging.KeyClass: "textpb.eventHandlerClient",
				"channelID":      channelID,
			}).Errorf("handle: %v", err)
			continue
		}

		if resp.GetQuit() {
			closeFn(conn, quitChan, evChan)
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
	select {
	case c.evChan <- protoEv:
	default:
		log.Warnf("event handler with channel id %q is falling behind processing",
			c.channelID)
		c.evChan <- protoEv
	}

	return false
}

func (c *eventHandlerClient) Close() error {
	closeFn(&c.conn, c.quitChan, c.evChan)
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
