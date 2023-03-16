package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6033", nil))
	}()

	plugutil.ServeEditorEventHandler(lspHandlerCommands, newLspHandler,
		lspHandlerEvents, lspHandlerPermissions...)
}
