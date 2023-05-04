package main

import (
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	/* go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()*/

	grantee, perms := plugutil.NewEditorEventHandler(gfHandlerCommands, newGFHandler,
		gfHandlerEvents, gfHandlerPermissions...)
	process.Serve(grantee, perms...)
}
