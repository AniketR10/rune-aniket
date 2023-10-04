package main

import (
	"unstable.build/go-tui/cmd/extension_sed/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
