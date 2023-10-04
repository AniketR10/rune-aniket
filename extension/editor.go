package extension

import (
	"io"
	"sync"

	"unstable.build/go-tui"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	textpb "unstable.build/go-tui/text/rpc"
)

type editorResourceServer struct {
	b text.Editor
}

func newEditorResourceServer(b text.Editor) *editorResourceServer {
	ret := new(editorResourceServer)
	ret.b = b
	return ret
}

func (s *editorResourceServer) Register(
	extensionID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := textpb.NewServer(broker, s.b, lock)
	textpb.RegisterEditorServer(registrar,
		interruptEditorServer(server, func() {
			tui.PublishEvent(term.Event{Type: term.EventInterrupt})
		}))
	return server, nil
}

// EditorResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Editor's resources.
func EditorResources(b text.Editor) map[Permission]ResourceRegistrar {
	s := newEditorResourceServer(b)
	return map[Permission]ResourceRegistrar{
		PermissionEditor: s,
	}
}
