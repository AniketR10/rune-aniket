package plugin

import (
	"unstable.build/go-tui/browser"
	textpb "unstable.build/go-tui/text/rpc"
)

// lazy textpb.BrowserServer to allow for order of perission grants
// to not matter when constructing a new textpb.Server
func grantorTextBrowser(pluginID string, g Grantor) textpb.BrowserServer {
	return &grantorBrowser{pluginID: pluginID, g: g}
}

type grantorBrowser struct {
	pluginID string
	g        Grantor
	srv      textpb.BrowserServer
}

func (g *grantorBrowser) ServeWindow(win browser.Window) (string, error) {
	if g.srv == nil {
		// any browser permission will do
		srv, ok := g.g.Grant(g.pluginID, PermissionBrowserWindowManager)
		if !ok {
			// if browser perm is not granted, allow for the browser API
			// to bubble it up. This ensures that text.Commands are still
			// able to be dispatched to command handlers even if browser
			// permissions are not required nor granted.
			return "", nil
		}
		g.srv = srv.(browserResourcePermissionServer).server
	}
	return g.srv.ServeWindow(win)
}
