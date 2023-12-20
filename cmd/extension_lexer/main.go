package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"unstable.build/go-tui/cmd/extension_lexer/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8882", nil))
	}()

	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
