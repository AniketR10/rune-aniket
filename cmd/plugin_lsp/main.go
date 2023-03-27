package main

import (
	"log"
	"net/http"

	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()

	plugutil.ServeEditorEventHandler(lspHandlerCommands, newLspHandler,
		lspHandlerEvents, lspHandlerPermissions...)
}
