package handler

import (
	"context"
	"errors"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

const defaultRPCTimeout = 5 * time.Second

// Client satisfies Handler by talking to a remote handler over GRPC.
type Client struct {
	Logger *log.Logger

	width, height int
	errors        chan error
	client        proto.HandlerClient
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(pbClient proto.HandlerClient) *Client {
	ret := new(Client)
	ret.Init(pbClient)
	return ret
}

// Init initialies this Client with pbClient.
func (c *Client) Init(pbClient proto.HandlerClient) {
	c.client = withClientBreaker(pbClient, defaultRPCTimeout)
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
func (c *Client) Draw(w tui.Writer) {
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
	ctx := context.Background()
	req := proto.CursorRequest{}

	resp, err := c.client.Cursor(ctx, &req)
	if err != nil {
		c.collectError(err)
		return
	}

	if resp.GetPosition() == nil {
		c.collectError(errors.New("missing Position field in Cursor response"))
		return
	}

	pos = resp.GetPosition().ToModel()
	show = resp.GetShow()
	return
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

// Server serves a tui.Handler implementation over GRPC.
type Server struct {
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
	s.handler.Resize(int(in.Width), int(in.Height))
	return proto.NewDrawResponse(s.handler, int(in.Width), int(in.Height)), nil
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
	exit, handled := s.handler.Handle(ev)
	return &proto.HandleResponse{Quit: exit, Handled: handled}, nil
}

// Cursor is an RPC that handles request to an underlying Handler's
// Cursor over RPC.
func (s *Server) Cursor(ctx context.Context, req *proto.CursorRequest) (
	*proto.CursorResponse, error,
) {
	pos, show := s.handler.Cursor()
	return &proto.CursorResponse{
		Position: &proto.Coordinates{X: int32(pos.X), Y: int32(pos.Y)},
		Show:     show,
	}, nil
}

// Man is an RPC that handles request to an underlying
// Handler's Man over RPC.
func (s *Server) Man(context.Context, *proto.ManRequest) (
	*proto.ManResponse, error,
) {
	man := s.handler.Man()
	protoMan := new(proto.Manual)
	protoMan.FromModel(man)
	return &proto.ManResponse{Man: protoMan}, nil
}
