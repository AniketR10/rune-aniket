package main

import (
	"unstable.build/go-tui/cmd/plugin_open_file_cursor/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	/* go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()*/

	grantee, perms := plugin.Grantee()
	process.Serve(grantee, perms...)
}
