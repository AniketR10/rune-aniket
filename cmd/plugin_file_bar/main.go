package main

import (
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	grantee, perms := plugutil.NewEditorEventHandler(fileBarHandlerCommands, newFileBarEditorHandler,
		fileBarHandlerEvents, fileBarHandlerPermissions...)
	process.Serve(grantee, perms...)
}
