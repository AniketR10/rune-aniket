package main

import (
	plugutil "unstable.build/go-tui/plugin/util"
)

func main() {
	plugutil.ServeEditorEventHandler(fileBarHandlerCommands, newFileBarEditorHandler,
		fileBarHandlerEvents, fileBarHandlerPermissions...)
}
