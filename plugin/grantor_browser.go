package plugin

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/proto"
	textpb "unstable.build/go-tui/text/rpc"
)

// lazy textpb.BrowserServer to allow for order of perission grants
// to not matter when constructing a new textpb.Server
func grantorTextBrowser(
	pluginID string, grantor Grantor, registrar grpc.ServiceRegistrar,
	broker proto.MuxBroker, lock sync.Locker,
) textpb.BrowserServer {
	return &grantorBrowser{
		pluginID:  pluginID,
		grantor:   grantor,
		registrar: registrar,
		broker:    broker,
		lock:      lock,
	}
}

type grantorBrowser struct {
	pluginID  string
	grantor   Grantor
	registrar grpc.ServiceRegistrar
	broker    proto.MuxBroker
	lock      sync.Locker
	srv       textpb.BrowserServer
}

func (g *grantorBrowser) ServeWindow(win browser.Window) (string, error) {
	if g.srv != nil {
		return g.srv.ServeWindow(win)
	}

	perm := Permission(browserplugin.PermissionBrowserWindowManager)
	srv, ok := g.grantor.Grant(g.pluginID, perm)
	if !ok {
		// if browser perm is not granted, allow for the browser API
		// to bubble it up. This ensures that text.Commands are still
		// able to be dispatched to command handlers even if browser
		// permissions are not required nor granted.
		//
		// NOTE: what we should really do here is serve a window
		// server that does this validation there, so error message
		// is accurate rather than "unknown service Browser"
		log.Warnf("usage of command's window field will fail:" +
			"PermissionBrowserWindowManager not granted")
		return "", nil
	}
	g.srv = srv.(browserResourcePermissionServer).server
	return g.srv.ServeWindow(win)
}
