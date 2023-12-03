package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"unstable.build/go-tui/cmd/extension_ai/extension"
	"unstable.build/go-tui/extension/process"
	plugutil "unstable.build/go-tui/extension/util"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8886", nil))
	}()

	grantee, perms := plugutil.NewEditorEventHandler(extension.AIHandlerCommands,
		extension.CommandEventHandler, extension.AIHandlerEvents,
		extension.AIHandlerPermissions...)
	process.Serve(grantee, perms...)
}
