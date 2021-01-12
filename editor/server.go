package editor

import (
	"context"
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

// Server serves an Editor over GRPC.
type Server struct {
	Logger *log.Logger

	broker proto.MuxBroker

	// Handlers opened by Edit
	opened map[uint32]tui.Handler

	editor struct {
		Editor
		sync.Locker
	}
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, editor Editor, lock sync.Locker,
) *Server {
	ret := new(Server)
	ret.Init(broker, editor, lock)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, editor Editor, lock sync.Locker,
) {
	s.broker = broker
	s.editor.Editor = editor
	s.editor.Locker = lock
	s.opened = make(map[uint32]tui.Handler)
}

func (s *Server) tryLog(msg string, args ...interface{}) {
	if s.Logger == nil {
		return
	}
	s.Logger.Debugf(msg, args...)
}

// Edit satisfies proto.EditorServer
func (s *Server) Edit(ctx context.Context, in *proto.EditRequest) (
	*proto.EditResponse, error,
) {
	s.editor.Lock()
	defer s.editor.Unlock()

	buf := proto.EditRequestToBuffer(in)
	h, err := s.editor.Edit(in.GetResourceName(), buf)
	if err != nil {
		return nil, err
	}

	handlerID := s.broker.NextId()
	s.opened[handlerID] = h
	s.tryLog("(%p): stored handler with ID: %d", s, handlerID)

	res := &proto.EditResponse{
		HandlerId: handlerID,
	}

	return res, nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() (err error) {
	s.editor.Lock()
	defer s.editor.Unlock()
	return err
}
