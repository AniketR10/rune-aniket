package main

import (
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	grantee, perms := plugutil.NewEditorEventHandler(sedHandlerCommands, newSedHandler,
		sedHandlerEvents, sedHandlerPermissions...)
	process.Serve(grantee, perms...)
}
