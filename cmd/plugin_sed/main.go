package main

import (
	plugutil "github.com/ernestrc/go-tui/plugin/util"
)

func main() {
	plugutil.ServeEditorEventHandler(sedHandlerCommands, newSedHandler,
		sedHandlerEvents, sedHandlerPermissions...)
}
