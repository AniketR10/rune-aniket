package rpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	termrpc "unstable.build/go-tui/term/rpc"
)

const defaultRPCTimeout = 4 * time.Second

var _ tui.Handler = (*Client)(nil)

// Client satisfies tui.Handler by talking to a remote handler over GRPC.
// All calls are synchronous.
//
// Note that Draw,Resize and Cursor are conflated into one RPC. This
// Client relies on the fact that runtime first Resizes, then calls Draw,
// and then gets the Cursor.
//
// Errors produced by the different underlying RPCs can be consumed via
// the chan error returned by Errors.
type Client struct {
	ctx    context.Context
	cancel func()
	client HandlerClient
	errors chan error

	width, height int

	cursor struct {
		term.Coordinates
		show  bool
		style term.CursorStyle
	}
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(pbClient HandlerClient) *Client {
	ret := new(Client)
	ret.Init(pbClient)
	return ret
}

// Init initializes this Client with the given underlying GPRC HandlerClient.
func (c *Client) Init(pbClient HandlerClient) {
	c.client = pbClient
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.errors = make(chan error)
	runtime.SetFinalizer(c, func(c *Client) {
		// do not make an RPC on a runtime finalizer
		c.cancel()
	})
}

// Errors returns a channel which receives RPC errors.
// The tui.Handler API is not designed for remote handlers, so errors must
// be bubbled up asynchronously.
func (c *Client) Errors() <-chan error {
	return c.errors
}

// Resize satisfies tui.Handler.
func (c *Client) Resize(width, height int) {
	c.width, c.height = width, height
}

// Draw satisfies tui.Handler
func (c *Client) Draw(w term.Writer) {
	ctx, cancel := context.WithTimeout(c.ctx, defaultRPCTimeout)
	defer cancel()

	req := DrawRequest{Height: int32(c.height), Width: int32(c.width)}
	resp, err := c.client.Draw(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.collectError("Draw", err)
		return
	}

	cursor := resp.GetCursor()
	c.cursor.show = cursor.GetShow()
	c.cursor.style = term.CursorStyle(cursor.GetStyle())
	c.cursor.Coordinates.X = int(cursor.GetPosition().GetX())
	c.cursor.Coordinates.Y = int(cursor.GetPosition().GetY())

	doDraw(w, resp)
}

// Cursor satisfies tui.Handler
func (c *Client) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return c.cursor.Coordinates, c.cursor.style, c.cursor.show
}

// Handle satisfies tui.Handler
func (c *Client) Handle(ev term.Event) (bool, bool) {
	ctx, cancel := context.WithTimeout(c.ctx, defaultRPCTimeout)
	defer cancel()

	req := HandleRequest{Event: new(termrpc.Event)}
	err := req.Event.FromModel(ev)
	if err != nil {
		c.collectError("Handle", fmt.Errorf("convert ev to proto ev: %w", err))
		return false, false
	}

	resp, err := c.client.Handle(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.collectError("Handle", err)
		return false, false
	}
	return resp.GetQuit(), resp.GetHandled()
}

// Man satisfies tui.Handler
func (c *Client) Man() tui.Manual {
	ctx, cancel := context.WithTimeout(c.ctx, defaultRPCTimeout)
	defer cancel()

	req := ManRequest{}

	resp, err := c.client.Man(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		c.collectError("Man", err)
		return tui.Manual{}
	}

	man := resp.GetMan()
	if man == nil {
		c.collectError("Man", errors.New("missing Man field in ManResponse"))
		return tui.Manual{}
	}

	tuiMan, err := man.ToModel()
	if err != nil {
		c.collectError("Man", err)
		return tui.Manual{}
	}

	return tuiMan
}

// Close satisfies browser.Handler
// Close closes this client and all associated resources.
func (c *Client) Close() error {
	// cancel all other activity
	c.cancel()

	// use a new context for the last close req
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, defaultRPCTimeout)
	defer cancel()

	req := CloseRequest{}
	_, err := c.client.Close(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return fmt.Errorf("close rpc: %w", err)
	}
	return nil
}

func (c *Client) collectError(call string, err error) {
	err = fmt.Errorf("%s: %w", call, err)
	// canceled usually indicates that response is no longer necessary
	// so it's an expected error.
	if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Canceled {
		select {
		case c.errors <- err:
			return
		default:
			c.log(log.ErrorLevel, "%v", err)
		}
	} else {
		c.log(log.DebugLevel, "canceled rpc: %v", err)
	}
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{
		logging.KeyClass: "handler.Client",
		"ptr":            fmt.Sprintf("%p", c),
	}).Logf(level, msg, args...)
}

func doDraw(w term.Writer, resp *DrawResponse) {
	for y, row := range resp.GetRows() {
		for x, c := range row.Cells {
			cell := c.ToModel()
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}
}
