package main

import (
	"unstable.build/go-tui/cmd/plugin_sed/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	grantee, perms := plugin.Grantee()
	process.Serve(grantee, perms...)
}
