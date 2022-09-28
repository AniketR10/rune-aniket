package main

import (
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	/* go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()*/

	plugutil.ServeEditorEventHandler(gitHandlerCommands, newGitHandler,
		gitHandlerEvents, gitHandlerPermissions...)
}
