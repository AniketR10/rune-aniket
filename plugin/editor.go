package plugin

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
	textpb "unstable.build/go-tui/text/rpc"
)

const (
	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "_PermEditor"
)

type editorResourceServer struct {
	mu     sync.Mutex
	b      text.Editor
	srv    proto.MuxServer
	server *textpb.Server
}

func newEditorResourceServer(b text.Editor) *editorResourceServer {
	ret := new(editorResourceServer)
	ret.b = b
	return ret
}

func (s *editorResourceServer) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID,
		func(opts []grpc.ServerOption) proto.MuxServer {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.srv == nil {
				var srv proto.MuxServer
				l := debug.StandardLogger()
				if l != nil && l.IsLevelEnabled(log.TraceLevel) {
					srv = proto.LoggingGRPCServer(l, opts...)
				} else {
					srv = proto.GRPCServer(opts...)
				}
				grpc := srv.GRPC()
				s.srv = srv
				s.server = textpb.NewServer(broker, s.b, lock)
				lock.Lock()
				s.server.Logger = l
				lock.Unlock()
				textpb.RegisterEditorServer(grpc, interruptEditorServer(s.server, interrupt))
			}
			return s.srv
		})
}

func (s *editorResourceServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		s.srv.Stop()
		return s.server.Close()
	}
	return nil
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
	c := textpb.NewClient(broker, conn)
	clients.Store(token, c)
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(token uint32, broker proto.MuxBroker) (
	text.Editor, error,
) {
	return dialEditor(token, broker)
}
