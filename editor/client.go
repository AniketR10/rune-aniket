package editor

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	gracefulShutdownWait  = 100 * time.Millisecond
	defaultFailureTimeout = 5 * time.Second
)

// Client satisfies editor.Editor by calling a remote editor over grpc.
type Client struct {
	Logger *log.Logger

	// resources invariant
	mu sync.Mutex

	// synchronize access to plugin state
	pluginLock sync.Locker

	broker proto.MuxBroker
	cc     grpc.ClientConnInterface
	ed     proto.EditorClient

	// event handler server resources. event handler servers are created on
	// calls to Subscribe.
	servers map[uint64]io.Closer
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
	pluginLock sync.Locker,
) *Client {
	ret := new(Client)
	ret.Init(broker, cc, pluginLock)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
	pluginLock sync.Locker,
) {
	c.ed = proto.NewEditorClient(cc)
	c.cc = cc
	c.broker = broker
	c.pluginLock = pluginLock
	c.servers = make(map[uint64]io.Closer)
}

func (c *Client) tryLog(msg string, args ...interface{}) {
	if c.Logger == nil {
		return
	}
	c.Logger.Debugf(msg, args...)
}

func (c *Client) getServers() map[uint64]io.Closer {
	return c.servers
}

func (c *Client) safeForceCloseHandler(brokerID uint32, reason string) error {
	c.tryLog("editor.Client.safeForceCloseHandler(%d, reason=%s)", brokerID, reason)
	_, err := proto.ForceCloseResource(uint64(brokerID), c.getServers, c.Logger, &c.mu)
	return err
}

func (c *Client) serveHandler(h EventHandler) uint32 {
	brokerID, srv := proto.AcceptAndServe(c.broker, c.Logger,
		func(handlerID uint32, srv proto.MuxServer) {
			s := newEventHandlerServer(c.pluginLock, h, func() {
				time.Sleep(gracefulShutdownWait)
				c.safeForceCloseHandler(handlerID, "editorEventHandlerServer.onExit")
			})
			s.logger = c.Logger
			proto.RegisterEditorEventHandlerServer(srv.GRPC(), s)
		})
	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers[uint64(brokerID)] = &handlerServerResource{h: h, srv: srv}
	return brokerID
}

// Edit requests editor server to edit buf.
func (c *Client) Edit(name string, buf *cell.Buffer) (Handler, error) {
	ctx := context.Background()
	req := proto.BufferToEditRequest(buf)
	req.ResourceName = name

	res, err := c.ed.Edit(ctx, &req)
	if err != nil {
		return nil, err
	}

	return browser.Token{ID: uint64(res.GetHandlerId())}, nil
}

// SubscribeEditor requests the editor server to subscribe sub to ev.
func (c *Client) SubscribeEditor(evType EventType, h EventHandler) error {
	ctx := context.Background()

	handlerID := c.serveHandler(h)

	protoType := Event{Type: evType}.protoType()
	req := proto.EditorSubscribeRequest{Type: protoType, HandlerId: handlerID}
	_, err := c.ed.Subscribe(ctx, &req)
	if err != nil {
		reason := fmt.Sprintf("editor.Client.Subscribe: %v", err)
		c.safeForceCloseHandler(handlerID, reason)
		return err
	}

	return nil
}

// Close closes all resources associated with this client.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	for _, res := range c.servers {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}

	c.servers = nil

	return err
}
