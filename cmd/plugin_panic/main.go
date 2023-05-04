package main

import (
	_ "net/http/pprof"

	"unstable.build/go-tui/cmd/plugin_panic/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	grantee, perms := plugin.Grantee()
	process.Serve(grantee, perms...)
}
