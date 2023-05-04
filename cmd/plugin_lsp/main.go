package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6033", nil))
	}()

	grantee, perms := plugutil.NewEditorEventHandler(lspHandlerCommands, newLspHandler,
		lspHandlerEvents, lspHandlerPermissions...)
	process.Serve(grantee, perms...)
}
