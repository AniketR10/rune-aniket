package main

import (
	_ "net/http/pprof"

	"unstable.build/go-tui/cmd/extension_panic/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
