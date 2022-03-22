package plugin

import (
	"sync"

	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "_PermEditor"
)

type editorResourceServer struct {
	mu  sync.Mutex
	b   text.Editor
	srv proto.MuxServer
}

func newEditorResourceServer(b text.Editor) *editorResourceServer {
	ret := new(editorResourceServer)
	ret.b = b
	return ret
}

func (s *editorResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	l *log.Logger, lock sync.Locker,
) {
	broker.AcceptAndServe(grantID, func(opts []grpc.ServerOption) proto.MuxServer {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.srv == nil {
			var srv proto.MuxServer
			if l != nil && l.IsLevelEnabled(log.TraceLevel) {
				srv = proto.LoggingGRPCServer(l, opts...)
			} else {
				srv = proto.GRPCServer(opts...)
			}
			grpc := srv.GRPC()
			s.srv = srv
			server := text.NewServer(broker, s.b, lock)
			lock.Lock()
			server.Logger = l
			lock.Unlock()
			proto.RegisterEditorServer(grpc, interruptEditorServer(server, term.Interrupt))
		}
		return s.srv
	})
}

// EditorResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Editor's resources.
func EditorResources(b text.Editor) map[Permission]ResourceServer {
	s := newEditorResourceServer(b)
	return map[Permission]ResourceServer{
		PermissionEditor: s,
	}
}

func dialEditor(token uint32, broker proto.MuxBroker) (
	text.Editor, error,
) {
	if c, ok := clients.Load(token); ok {
		return c.(text.Editor), nil
	}
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := text.NewClient(broker, conn)
	c.Logger = &pluginLogger
	clients.Store(token, c)
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(token uint32, broker proto.MuxBroker) (
	text.Editor, error,
) {
	return dialEditor(token, broker)
}
