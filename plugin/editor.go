package plugin

import (
	"io"
	"sync"

	textplugin "unstable.build/go-tui/api/text/plugin"
	"unstable.build/go-tui/proto"
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
	pluginID string, grantor Grantor, registrar proto.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	browser := grantorTextBrowser(pluginID, grantor, registrar, broker, lock)
	server := textpb.NewServer(broker, s.b, lock, browser)
	textpb.RegisterEditorServer(registrar, interruptEditorServer(server, interrupt))
	return server, nil
}

// EditorResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Editor's resources.
func EditorResources(b text.Editor) map[Permission]ResourceRegistrar {
	s := newEditorResourceServer(b)
	return map[Permission]ResourceRegistrar{
		Permission(textplugin.PermissionEditor): s,
	}
}
