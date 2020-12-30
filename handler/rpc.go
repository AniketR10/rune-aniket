package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

const defaultRPCTimeout = 5 * time.Second

type clientCloser interface {
	proto.HandlerClient
	io.Closer
}

// Client satisfies Handler by talking to a remote handler over GRPC.
//
// Note that Draw,Resize and Cursor are conflated into one RPC. This
// Client relies on the fact that runtime first Resizes, then calls Draw,
// and then gets the Cursor.
type Client struct {
	Logger *log.Logger

	width, height int
	cursor        struct {
		term.Coordinates
		show bool
	}

	errors chan error
	client clientCloser
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	pbClient proto.HandlerClient,
	interruptDraw, interruptHandle func(),
) *Client {
	ret := new(Client)
	ret.Init(pbClient, interruptDraw, interruptHandle)
	return ret
}

// Init initialies this Client with pbClient and the given interrupt func.
func (c *Client) Init(
	pbClient proto.HandlerClient, interruptDraw, interruptHandle func(),
) {
	// the ALWAYS 'async' feature of the rpc breaker is essential
	// to avoid the following deadlock:
	//
	// browser server          browser client
	//   Lock()          ->       Handle(), tries to open window
	//   Split()         <-
	//   Lock()
	//
	// The original call to Handle will block forever, because
	// the handler client is invoking an RPC which requires the
	// original lock to be unlocked.
	pbClient = withClientTimeout(pbClient, defaultRPCTimeout)
	c.client = withClientBreaker(pbClient, interruptDraw, interruptHandle, c.Logger)

	c.errors = make(chan error)
}

// Errors returns a channel which receives RPC errors.
// The tui.Handler API is not designed for remote handlers, so errors must
// be bubbled up asynchronously.
func (c *Client) Errors() <-chan error {
	return c.errors
}

func (c *Client) collectError(call string, err error) {
	if c.Logger != nil {
		c.Logger.Errorf("handler.Client error: %s: %s", call, err)
	}

	select {
	case c.errors <- err:
	default:
	}
}

// Resize satisfies tui.Handler
func (c *Client) Resize(width, height int) {
	c.width, c.height = width, height
}

// Draw satisfies tui.Handler
func (c *Client) Draw(w term.Writer) {
	ctx := context.Background()
	req := proto.DrawRequest{Width: int32(c.width), Height: int32(c.height)}

	resp, err := c.client.Draw(ctx, &req)
	if err != nil {
		c.collectError("Draw", err)
		return
	}

	for y, row := range resp.Rows {
		for x, c := range row.Cells {
			cell := c.ToModel()
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}

	if resp.Cursor == nil || resp.Cursor.Position == nil {
		c.collectError("Draw", errors.New("invalid Cursor from server's Draw response"))
		return
	}

	c.cursor.Coordinates.X = int(resp.Cursor.Position.X)
	c.cursor.Coordinates.Y = int(resp.Cursor.Position.Y)
	c.cursor.show = resp.Cursor.Show
}

// Handle satisfies tui.Handler
func (c *Client) Handle(ev term.Event) (exit, handled bool) {
	ctx := context.Background()

	protoEv := proto.Event{}
	err := protoEv.FromModel(ev)
	if err != nil {
		c.collectError("Handle", err)
		return
	}

	req := proto.HandleRequest{Event: &protoEv}
	resp, err := c.client.Handle(ctx, &req)
	if err != nil {
		c.collectError("Handle", err)
		return
	}

	return resp.GetQuit(), resp.GetHandled()
}

// Cursor satisfies tui.Handler
func (c *Client) Cursor() (pos term.Coordinates, show bool) {
	return c.cursor.Coordinates, c.cursor.show
}

// Man satisfies tui.Handler
func (c *Client) Man() tui.Manual {
	ctx := context.Background()
	req := proto.ManRequest{}

	resp, err := c.client.Man(ctx, &req)
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

func tryLog(logger *log.Logger, msg string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(msg, args...)
	}
}

// OnUnmount satisfies browser.Handler
func (c *Client) OnUnmount() error {
	ctx := context.Background()
	req := proto.OnUnmountRequest{}

	_, err := c.client.OnUnmount(ctx, &req)
	if err != nil {
		c.collectError("OnUnmount", err)
		return err
	}
	return nil
}

// Close closes this client and all associated resources.
func (c *Client) Close() error {
	return c.client.Close()
}

// Server serves a tui.Handler implementation over GRPC.
type Server struct {
	mu      sync.Locker
	handler tui.Handler
	Logger  *log.Logger
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(handler tui.Handler, mu sync.Locker) *Server {
	ret := new(Server)
	ret.Init(handler, mu)
	return ret
}

// Init initializes this Server to serve handler. It uses locker
// to synchronize access to handler, so it's goroutine-safe
// to share a handler between multiple servers, as long as the
// same locker is used.
func (s *Server) Init(handler tui.Handler, locker sync.Locker) {
	s.handler = handler
	s.mu = locker
}

// Draw is an RPC that handles request to an underlying
// Handler's Draw over RPC.
func (s *Server) Draw(ctx context.Context, in *proto.DrawRequest) (
	*proto.DrawResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handler.Resize(int(in.Width), int(in.Height))
	cursor, show := s.handler.Cursor()
	res := proto.NewDrawResponse(s.handler, int(in.Width), int(in.Height))
	res.Cursor.Position.X = int32(cursor.X)
	res.Cursor.Position.Y = int32(cursor.Y)
	res.Cursor.Show = show
	return res, nil
}

// Handle is an RPC that handles request to an underlying Handler's
// Handle over RPC.
func (s *Server) Handle(ctx context.Context, req *proto.HandleRequest) (
	*proto.HandleResponse, error,
) {
	pev := req.GetEvent()
	if pev == nil {
		return nil, errors.New("missing Event field in HandleRequest")
	}
	ev, err := pev.ToModel()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exit, handled := s.handler.Handle(ev)
	return &proto.HandleResponse{Quit: exit, Handled: handled}, nil
}

// Man is an RPC that handles request to an underlying
// Handler's Man over RPC.
func (s *Server) Man(context.Context, *proto.ManRequest) (
	*proto.ManResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	man := s.handler.Man()
	protoMan := new(proto.Manual)
	protoMan.FromModel(man)
	return &proto.ManResponse{Man: protoMan}, nil
}

// OnUnmount is an RPC that handles request to an underlying
// Handler's OnUnmount if it implements it, otherwise it ignores request.
func (s *Server) OnUnmount(context.Context, *proto.OnUnmountRequest) (
	*proto.OnUnmountResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	unmounter, ok := s.handler.(interface{ OnUnmount() error })
	if ok {
		err := unmounter.OnUnmount()
		if err != nil {
			return nil, fmt.Errorf("error OnUnmount: %v", err)
		}
	}
	tryLog(s.Logger, "handler.Server.OnUnmount: %v", ok)
	return &proto.OnUnmountResponse{}, nil
}
