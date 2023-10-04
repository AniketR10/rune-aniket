package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/cmd/extension_fuzzy_syntax/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6888", nil))
	}()

	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
