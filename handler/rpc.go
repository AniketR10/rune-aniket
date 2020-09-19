package handler

import (
	"context"
	"errors"
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

type nopCloser struct {
	proto.HandlerClient
}

func (n *nopCloser) Close() error { return nil }

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
	c.client = withClientBreaker(pbClient, interruptDraw, interruptHandle)

	c.errors = make(chan error)
}

// Errors returns a channel which receives RPC errors.
// The tui.Handler API is not designed for remote handlers, so errors must
// be bubbled up asynchronously.
func (c *Client) Errors() <-chan error {
	return c.errors
}

func (c *Client) collectError(err error) {
	if c.Logger != nil {
		c.Logger.Error(err)
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
		c.collectError(err)
		return
	}

	for y, row := range resp.Rows {
		for x, c := range row.Cells {
			cell := c.ToModel()
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}

	if resp.Cursor == nil || resp.Cursor.Position == nil {
		c.collectError(errors.New("invalid Cursor from server's Draw response"))
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
		c.collectError(err)
		return
	}

	req := proto.HandleRequest{Event: &protoEv}
	resp, err := c.client.Handle(ctx, &req)
	if err != nil {
		c.collectError(err)
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
		c.collectError(err)
		return tui.Manual{}
	}

	man := resp.GetMan()
	if man == nil {
		c.collectError(errors.New("missing Man field in ManResponse"))
		return tui.Manual{}
	}

	tuiMan, err := man.ToModel()
	if err != nil {
		c.collectError(err)
		return tui.Manual{}
	}

	return tuiMan
}

// Close closes this client and all associated resources.
func (c *Client) Close() error {
	return c.client.Close()
}

// Server serves a tui.Handler implementation over GRPC.
type Server struct {
	mu      sync.Mutex
	handler tui.Handler
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(handler tui.Handler) *Server {
	ret := new(Server)
	ret.Init(handler)
	return ret
}

// Init initializes this Server to serve handler.
func (s *Server) Init(handler tui.Handler) {
	s.handler = handler
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
	res.Cursor = &proto.DrawResponse_Cursor{
		Position: &proto.Coordinates{
			X: int32(cursor.X),
			Y: int32(cursor.Y),
		},
		Show: show,
	}
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
