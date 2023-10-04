package main

import (
	"unstable.build/go-tui/cmd/extension_open_file_cursor/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	/* go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()*/

	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
