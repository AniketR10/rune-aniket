package editor

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const defaultFailureTimeout = 5 * time.Second

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
}

type clientHandler struct {
	remoteHandlerID uint32
}

func (e clientHandler) Resize(width, height int) {
}

func (e clientHandler) Draw(term.Writer) {
}

func (e clientHandler) Handle(ev term.Event) (exit, handled bool) {
	return
}

func (e clientHandler) Cursor() (pos term.Coordinates, show bool) {
	return
}

func (e clientHandler) Man() tui.Manual {
	return tui.Manual{}
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

func (c *Client) tryLog(msg string, args ...interface{}) {
	if c.Logger == nil {
		return
	}
	c.Logger.Debugf(msg, args...)
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
}

// Edit requests editor server to edit buf.
func (c *Client) Edit(name string, buf *cell.Buffer) (tui.Handler, error) {
	ctx := context.Background()
	req := proto.BufferToEditRequest(buf)
	req.ResourceName = name

	res, err := c.ed.Edit(ctx, &req)
	if err != nil {
		return nil, err
	}

	return handler.Token{ID: res.GetHandlerId()}, nil
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

	return err
}
