package plugin

import (
	"io"
	"runtime"
	"sync"

	"google.golang.org/grpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
	textpb "unstable.build/go-tui/text/rpc"
)

const (
	// PermissionEditor requests access to the editor.
	PermissionEditor Permission = "_PermEditor"
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
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	browser := grantorTextBrowser(pluginID, grantor)
	server := textpb.NewServer(broker, s.b, lock, browser)
	textpb.RegisterEditorServer(registrar, interruptEditorServer(server, interrupt))
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

func dialEditor(token uint32, broker proto.MuxBroker) (
	text.Editor, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := textpb.NewClient(broker, conn)
	runtime.SetFinalizer(c, func(c *textpb.Client) { c.Close() })
	return c, nil
}

// Editor acquires the remote Editor with the given token.
func Editor(token uint32, broker proto.MuxBroker) (
	text.Editor, error,
) {
	return dialEditor(token, broker)
}
