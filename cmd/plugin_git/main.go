package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:3863", nil))
	}()

	plugutil.ServeEditorEventHandler(gitHandlerCommands, newGitHandler,
		gitHandlerEvents, gitHandlerPermissions...)
}
