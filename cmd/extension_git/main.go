package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/cmd/extension_git/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:3863", nil))
	}()

	grantee, perms := extension.Grantee()
	process.Serve(grantee, perms...)
}
