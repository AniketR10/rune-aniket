package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui"

	termpb "unstable.build/go-tui/term/rpc"
)

type dimensions struct {
	width  int
	height int
}

// Server serves a tui.Handler implementation over GRPC.
type Server struct {
	UnimplementedHandlerServer
	// NOTE: do not synchronize access to handle
	// explicitly, and instead rely on clients to do it.
	// This is necessary because calls to Handle can call APIs
	// which might then trigger another method to be called, i.e. Close, which
	// creates a deadlock.
	handler    tui.Handler
	dimensions atomic.Value
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
	s.dimensions.Store(dimensions{})
}

// Draw is an RPC that handles request to an underlying Handler's
// Draw over RPC.
func (s *Server) Draw(ctx context.Context, in *DrawRequest) (
	*DrawResponse, error,
) {
	dim := s.dimensions.Load().(dimensions)
	if int(in.Width) != dim.width || int(in.Height) != dim.height {
		s.handler.Resize(int(in.Width), int(in.Height))
	}
	s.dimensions.Store(dimensions{width: int(in.Width), height: int(in.Height)})
	cursor, style, show := s.handler.Cursor()
	res := NewDrawResponse(ctx, s.handler, int(in.Width), int(in.Height))
	res.Cursor.Position.X = int32(cursor.X)
	res.Cursor.Position.Y = int32(cursor.Y)
	res.Cursor.Show = show
	res.Cursor.Style = int32(style)
	return res, nil
}

func (s *Server) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "handler.Server").Logf(level, msg, args...)
}

// Handle is an RPC that handles request to an underlying Handler's
// Handle over RPC.
func (s *Server) Handle(ctx context.Context, req *HandleRequest) (
	*HandleResponse, error,
) {
	pev := req.GetEvent()
	if pev == nil {
		return nil, errors.New("missing Event field in HandleRequest")
	}
	ev, err := pev.ToModel()
	if err != nil {
		return nil, err
	}

	s.log(log.TraceLevel, "handler.Server.Handle(%v)", ev)

	var exit, handled bool
	exit, handled = s.handler.Handle(ev)

	return &HandleResponse{
		Quit:    exit,
		Handled: handled,
	}, nil
}

// Man is an RPC that handles request to an underlying
// Handler's Man over RPC.
func (s *Server) Man(context.Context, *ManRequest) (
	*ManResponse, error,
) {
	man := s.handler.Man()
	protoMan := new(termpb.Manual)
	err := protoMan.FromModel(man)
	if err != nil {
		return nil, err
	}
	return &ManResponse{Man: protoMan}, nil
}

// Close is an RPC that handles request to an underlying
// Handler's Close if it implements it, otherwise it ignores request.
func (s *Server) Close(ctx context.Context, req *CloseRequest) (
	*CloseResponse, error,
) {
	closer, ok := s.handler.(io.Closer)
	if ok {
		err := closer.Close()
		if err != nil {
			return nil, fmt.Errorf("error Close: %v", err)
		}
	}
	s.log(log.DebugLevel, "handler.Server.Close: %v", ok)
	return &CloseResponse{}, nil
}
