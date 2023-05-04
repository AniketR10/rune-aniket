package main

import (
	"unstable.build/go-tui/cmd/plugin_file_bar/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	grantee, perms := plugin.Grantee()
	process.Serve(grantee, perms...)
}
